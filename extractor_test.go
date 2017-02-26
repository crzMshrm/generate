package generate

import (
	"reflect"
	"testing"

	"github.com/crzmshrm/typextract/jsonschema"
)

func TestExtractor_CreateStructs(t *testing.T) {
	type fields struct {
		schema *jsonschema.Schema
	}

	tests := []struct {
		name    string
		fields  fields
		want    map[string]StructMeta
		want1   []SliceMeta
		wantErr bool
	}{
		{
			name: "simple",
			fields: fields{
				schema: schema1,
			},
			want:    expectedTypes1,
			want1:   []SliceMeta{SliceMeta{"string", 1, 0}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Extractor{
				schema: tt.fields.schema,
			}
			got, got1, err := g.CreateStructs()
			if (err != nil) != tt.wantErr {
				t.Errorf("Extractor.CreateStructs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Extractor.CreateStructs() got = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Extractor.CreateStructs() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

var (
	schema1 = jsonschema.ParseMust(`{
		"$schema": "http://json-schema.org/draft-04/schema#",
		"title": "Product",
		"description": "A product from Acme's catalog",
		"type": "object",
		"properties": {
			"id": {
				"description": "The unique identifier for a product",
				"type": "integer"
			},
			"name": {
				"description": "Name of the product",
				"type": "string",
				"maxLength": 100
			},
			"price": {
				"type": "number",
				"minimum": 0,
				"exclusiveMinimum": true
			},
			"tags": {
				"type": "array",
				"items": {
					"type": "string"
				},
				"minItems": 1,
				"uniqueItems": true
			}
		},
		"required": ["id", "name", "price"]
	}`)
	floatZero      = 0.0
	expectedTypes1 = map[string]StructMeta{
		"Product": StructMeta{
			ID:   "#",
			Name: "Product",
			Fields: map[string]FieldMeta{
				"Id": FieldMeta{
					Name:     "Id",
					JSONName: "id",
					Type:     "int",
					Required: true,
				},
				"Name": FieldMeta{
					Name:     "Name",
					JSONName: "name",
					Type:     "string",
					Required: true,
					String: &StringMeta{
						MaxLength: 100,
					},
				},
				"Price": FieldMeta{
					Name:     "Price",
					JSONName: "price",
					Type:     "float64",
					Required: true,
					Number: &NumberMeta{
						Minimum:          &floatZero,
						ExclusiveMinimum: true,
					},
				},
				"Tags": FieldMeta{
					Name:     "Tags",
					JSONName: "tags",
					Type:     "[]string",
					Slice: &SliceMeta{
						ElemType: "string",
						MinItems: 1,
					},
				},
			},
		},
	}
)
