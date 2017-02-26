package generate

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"errors"

	"github.com/crzmshrm/typextract/jsonschema"
)

// Extractor will produce meta types from the JSON schema.
type Extractor struct {
	schema *jsonschema.Schema
}

// New creates an instance of a Extractor which will produce meta types.
func New(schema *jsonschema.Schema) *Extractor {
	return &Extractor{
		schema: schema,
	}
}

// CreateStructs creates meta types from the JSON schema, keyed by the golang name.
func (g *Extractor) CreateStructs() (map[string]StructMeta, []SliceMeta, error) {
	var slices []SliceMeta

	// Extract nested and complex types from the JSON schema.
	types := g.schema.ExtractTypes()
	structs := make(map[string]StructMeta, len(types))

	errs := []error{}

	for _, typeKey := range getOrderedKeyNamesFromSchemaMap(types) {
		v := types[typeKey]

		fields, _slices, err := getFields(typeKey, v.Properties, types, v.Required)
		slices = append(slices, _slices...)

		if err != nil {
			errs = append(errs, err)
		}

		structName := getStructName(typeKey, v, 1)

		if err != nil {
			errs = append(errs, err)
		}

		s := StructMeta{
			ID:     typeKey,
			Name:   structName,
			Fields: fields,
		}

		structs[s.Name] = s
	}

	if len(errs) > 0 {
		return structs, slices, errors.New(joinErrors(errs))
	}

	return structs, slices, nil
}

func joinErrors(errs []error) string {
	var buffer bytes.Buffer

	for idx, err := range errs {
		buffer.WriteString(err.Error())

		if idx+1 < len(errs) {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

func getOrderedKeyNamesFromSchemaMap(m map[string]*jsonschema.Schema) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

func getFields(parentTypeKey string, properties map[string]*jsonschema.Schema, types map[string]*jsonschema.Schema, requiredFields []string) (map[string]FieldMeta, []SliceMeta, error) {
	fields := map[string]FieldMeta{}
	var slices []SliceMeta

	missingTypes := []string{}
	errors := []error{}

	for _, fieldName := range getOrderedKeyNamesFromSchemaMap(properties) {
		v := properties[fieldName]

		golangName := getGolangName(fieldName)
		meta, err := getTypeForField(parentTypeKey, fieldName, golangName, v, types, true)

		if err != nil {
			missingTypes = append(missingTypes, golangName)
			errors = append(errors, err)
		}
		meta.JSONName = fieldName
		meta.Name = golangName
		meta.Required = contains(requiredFields, fieldName)

		fields[meta.Name] = meta
		if meta.Slice != nil {
			slices = append(slices, *meta.Slice)
		}
	}

	if len(missingTypes) > 0 {
		return fields, slices, fmt.Errorf("missing types for %s with errors %s", strings.Join(missingTypes, ","), joinErrors(errors))
	}

	return fields, slices, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func getTypeForField(parentTypeKey string, fieldName string, fieldGoName string, schema *jsonschema.Schema, types map[string]*jsonschema.Schema, pointer bool) (meta FieldMeta, err error) {
	majorType := schema.Type
	subType := ""

	// Look up by named reference.
	if schema.Reference != "" {
		if t, ok := types[schema.Reference]; ok {
			sn := getStructName(schema.Reference, t, 1)

			majorType = "object"
			subType = sn
		}
	}

	// Look up any embedded types.
	if subType == "" && majorType == "object" {
		if parentType, ok := types[parentTypeKey+"/properties/"+fieldName]; ok {
			sn := getStructName(parentTypeKey+"/properties/"+fieldName, parentType, 1)

			majorType = "object"
			subType = sn
		}
	}

	// Find named array references.
	if majorType == "array" {
		s, _ := getTypeForField(parentTypeKey, fieldName, fieldGoName, schema.Items, types, false)
		meta.Slice = &SliceMeta{s.Type, schema.MinItems, schema.MaxItems}

		subType = s.Type
	}

	typName, err := getPrimitiveTypeName(majorType, subType, pointer)

	switch typName {
	case "int", "float64":
		if schema.MultipleOf != 0 ||
			schema.Minimum != nil ||
			schema.ExclusiveMinimum ||
			schema.Maximum != nil ||
			schema.ExclusiveMaximum {
			meta.Number = &NumberMeta{schema.MultipleOf, schema.Minimum, schema.ExclusiveMinimum, schema.Maximum, schema.ExclusiveMaximum}
		}
	case "string":
		if schema.MinLength != 0 ||
			schema.MaxLength != 0 ||
			schema.Pattern != "" {
			meta.String = &StringMeta{schema.MinLength, schema.MaxLength, schema.Pattern}
		}
	}

	if err != nil {
		return meta, fmt.Errorf("Failed to get the type for %s with error %s",
			fieldGoName,
			err.Error())
	}

	meta.Type = typName

	return
}

func getPrimitiveTypeName(schemaType string, subType string, pointer bool) (name string, err error) {
	switch schemaType {
	case "array":
		if subType == "" {
			return "error_creating_array", errors.New("can't create an array of an empty subtype")
		}
		return "[]" + subType, nil
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "number":
		return "float64", nil
	case "null":
		return "nil", nil
	case "object":
		if pointer {
			return "*" + subType, nil
		}

		return subType, nil
	case "string":
		return "string", nil
	}

	return "undefined", fmt.Errorf("failed to get a primitive type for schemaType %s and subtype %s", schemaType, subType)
}

// getStructName makes a golang struct name from an input reference in the form of #/definitions/address
// The parts refers to the number of segments from the end to take as the name.
func getStructName(reference string, structType *jsonschema.Schema, n int) string {
	if reference == "#" {
		rootName := structType.Title

		if rootName == "" {
			rootName = structType.Description
		}

		if rootName == "" {
			rootName = "Root"
		}

		return getGolangName(rootName)
	}

	clean := strings.Replace(reference, "#/", "", -1)
	parts := strings.Split(clean, "/")
	partsToUse := parts[len(parts)-n:]

	sb := bytes.Buffer{}

	for _, p := range partsToUse {
		sb.WriteString(getGolangName(p))
	}

	result := sb.String()

	if result == "" {
		return "Root"
	}

	return result
}

// getGolangName strips invalid characters out of golang struct or field names.
func getGolangName(s string) string {
	buf := bytes.NewBuffer([]byte{})

	for _, v := range splitOnAll(s, '_', ' ', '.', '-') {
		buf.WriteString(capitaliseFirstLetter(v))
	}

	return buf.String()
}

func splitOnAll(s string, splitItems ...rune) []string {
	rv := []string{}

	buf := bytes.NewBuffer([]byte{})
	for _, c := range s {
		if matches(c, splitItems) {
			rv = append(rv, buf.String())
			buf.Reset()
		} else {
			buf.WriteRune(c)
		}
	}
	if buf.Len() > 0 {
		rv = append(rv, buf.String())
	}

	return rv
}

func matches(c rune, any []rune) bool {
	for _, a := range any {
		if a == c {
			return true
		}
	}
	return false
}

func capitaliseFirstLetter(s string) string {
	if s == "" {
		return s
	}

	prefix := s[0:1]
	suffix := s[1:]
	return strings.ToUpper(prefix) + suffix
}

// StructMeta defines the data required to generate a struct in Go.
type StructMeta struct {
	// The ID within the JSON schema, e.g. #/definitions/address
	ID string
	// The golang name, e.g. "Address"
	Name   string
	Fields map[string]FieldMeta
}

// FieldMeta defines the data required to generate a field in Go.
type FieldMeta struct {
	// The golang name, e.g. "Address1"
	Name string
	// The JSON name, e.g. "address1"
	JSONName string
	// The golang type of the field, e.g. a built-in type like "string" or the name of a struct generated from the JSON schema.
	Type string
	// Required is set to true when the field is required.
	Required bool
	Slice    *SliceMeta
	String   *StringMeta
	Number   *NumberMeta
}

type SliceMeta struct {
	ElemType string
	MinItems int
	MaxItems int
}

type StringMeta struct {
	MinLength int
	MaxLength int
	Pattern   string
}

type NumberMeta struct {
	MultipleOf       float64
	Minimum          *float64
	ExclusiveMinimum bool
	Maximum          *float64
	ExclusiveMaximum bool
}
