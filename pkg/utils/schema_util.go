package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type Type string

const (
	TypeString  Type = "STRING"
	TypeInteger Type = "INTEGER"
	TypeNumber  Type = "NUMBER"
	TypeBoolean Type = "BOOLEAN"
	TypeObject  Type = "OBJECT"
	TypeArray   Type = "ARRAY"
)

type TypeSchema struct {
	AnyOf            []*TypeSchema          `json:"anyOf,omitempty"`
	Default          interface{}            `json:"default,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Enum             []string               `json:"enum,omitempty"`
	Example          interface{}            `json:"example,omitempty"`
	Format           string                 `json:"format,omitempty"`
	Items            *TypeSchema            `json:"items,omitempty"`
	MaxItems         *int64                 `json:"maxItems,omitempty"`
	MaxLength        *int64                 `json:"maxLength,omitempty"`
	MaxProperties    *int64                 `json:"maxProperties,omitempty"`
	Maximum          *float64               `json:"maximum,omitempty"`
	MinItems         *int64                 `json:"minItems,omitempty"`
	MinLength        *int64                 `json:"minLength,omitempty"`
	MinProperties    *int64                 `json:"minProperties,omitempty"`
	Minimum          *float64               `json:"minimum,omitempty"`
	Nullable         *bool                  `json:"nullable,omitempty"`
	Pattern          string                 `json:"pattern,omitempty"`
	Properties       map[string]*TypeSchema `json:"properties,omitempty"`
	PropertyOrdering []string               `json:"propertyOrdering,omitempty"`
	Required         []string               `json:"required,omitempty"`
	Title            string                 `json:"title,omitempty"`
	Type             string                 `json:"type,omitempty"`
}

func BuildSchemaFromJson(v []byte) (*TypeSchema, error) {
	var genSchema TypeSchema
	err := json.Unmarshal(v, &genSchema)
	if err != nil {
		return nil, fmt.Errorf("❌ getting schema from json failed: %w", err)
	}
	return &genSchema, nil
}

func BuildSchemaFromStruct[T interface{}](t T) *TypeSchema {
	return buildSchemaFromType(reflect.TypeOf(t))
}

func buildSchemaFromType(t reflect.Type) *TypeSchema {
	s := &TypeSchema{}

	switch t.Kind() {
	case reflect.Struct:
		s.Type = string(TypeObject)
		s.Properties = map[string]*TypeSchema{}

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // skip unexportede
				continue
			}

			jsonTag := f.Tag.Get("json")
			parts := strings.Split(jsonTag, ",")
			fieldName := parts[0]
			if fieldName == "" {
				fieldName = f.Name
			}

			fieldSchema := buildSchemaFromType(baseType(f.Type))
			s.Properties[fieldName] = fieldSchema
			isOmitempty := false
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					isOmitempty = true
					break
				}
			}

			// Only append to s.Required if 'omitempty' is NOT found.
			if !isOmitempty {
				s.Required = append(s.Required, fieldName)
			}
		}

	case reflect.Slice, reflect.Array:
		s.Type = string(TypeArray)
		s.Items = buildSchemaFromType(baseType(t.Elem()))

	case reflect.String:
		s.Type = string(TypeString)

	case reflect.Bool:
		s.Type = string(TypeBoolean)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s.Type = string(TypeInteger)

	case reflect.Float32, reflect.Float64:
		s.Type = string(TypeNumber)

	default:
		s.Type = string(TypeString)
	}

	return s
}
func baseType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}
func float32Ptr(v float32) *float32 { return &v }
