package core

import (
	"fmt"
	"strings"
)

// FieldType represents the type of data a field can contain.
type FieldType string

const (
	FieldTypeText   FieldType = "text"
	FieldTypeImage  FieldType = "image"
	FieldTypeAudio  FieldType = "audio"
	FieldTypeInt    FieldType = "int"
	FieldTypeBool   FieldType = "bool"
	FieldTypeString FieldType = "string"
	FieldTypeArray  FieldType = "array"
	FieldTypeObject FieldType = "object"
)

// Field represents a single field in a signature.
type Field struct {
	Name        string
	Description string
	Prefix      string
	Type        FieldType         // Data type for the field
	Items       *Field            // For array types, this represents the item type
	Properties  map[string]*Field // For object types, this holds nested fields
}

// NewField creates a new Field with smart defaults and customizable options.
func NewField(name string, opts ...FieldOption) Field {
	// Start with sensible defaults
	f := Field{
		Name:   name,
		Prefix: name + ":",    // Default prefix is the field name with colon
		Type:   FieldTypeText, // Default to text for backward compatibility
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(&f)
	}

	return f
}

// FieldOption allows customization of Field creation.
type FieldOption func(*Field)

// WithDescription sets a custom description for the field.
func WithDescription(desc string) FieldOption {
	return func(f *Field) {
		f.Description = desc
	}
}

// WithCustomPrefix overrides the default prefix.
func WithCustomPrefix(prefix string) FieldOption {
	return func(f *Field) {
		f.Prefix = prefix
	}
}

// WithNoPrefix removes the prefix entirely.
func WithNoPrefix() FieldOption {
	return func(f *Field) {
		f.Prefix = ""
	}
}

// WithFieldType sets the field type.
func WithFieldType(fieldType FieldType) FieldOption {
	return func(f *Field) {
		f.Type = fieldType
	}
}

// WithArrayType sets the field type as array and assigns the item type.
func WithArrayType(itemType *Field) FieldOption {
	return func(f *Field) {
		f.Type = FieldTypeArray
		f.Items = itemType
	}
}

// WithObjectType sets the field type as object and assigns nested fields.
func WithObjectType(properties map[string]*Field) FieldOption {
	return func(f *Field) {
		f.Type = FieldTypeObject
		f.Properties = properties
	}
}

// NewIntField creates a new field of type int.
func NewIntField(name string, opts ...FieldOption) Field {
	opts = append(opts, WithFieldType(FieldTypeInt))
	return NewField(name, opts...)
}

// NewBoolField creates a new field of type bool.
func NewBoolField(name string, opts ...FieldOption) Field {
	opts = append(opts, WithFieldType(FieldTypeBool))
	return NewField(name, opts...)
}

// NewStringField creates a new field of type string.
func NewStringField(name string, opts ...FieldOption) Field {
	opts = append(opts, WithFieldType(FieldTypeString))
	return NewField(name, opts...)
}

// NewArrayField creates a new array field of a specific item type.
func NewArrayField(name string, itemType *Field, opts ...FieldOption) Field {
	opts = append(opts, WithArrayType(itemType))
	return NewField(name, opts...)
}

// NewObjectField creates a new object field with nested fields.
func NewObjectField(name string, properties map[string]*Field, opts ...FieldOption) Field {
	opts = append(opts, WithObjectType(properties))
	return NewField(name, opts...)
}

// InputField represents an input field.
type InputField struct {
	Field
}

// OutputField represents an output field.
type OutputField struct {
	Field
}

// Signature represents the input and output specification of a module.
type Signature struct {
	Inputs      []InputField
	Outputs     []OutputField
	Instruction string
}

// NewSignature creates a new Signature with the given inputs and outputs.
func NewSignature(inputs []InputField, outputs []OutputField) Signature {
	return Signature{
		Inputs:  inputs,
		Outputs: outputs,
	}
}

// WithInstruction adds an instruction to the Signature.
func (s Signature) WithInstruction(instruction string) Signature {
	s.Instruction = instruction
	return s
}

// String returns a string representation of the Signature.
func (s Signature) String() string {
	var sb strings.Builder
	sb.WriteString("Inputs:\n")
	for _, input := range s.Inputs {
		typeStr := ""
		if input.Type != FieldTypeText {
			typeStr = fmt.Sprintf(" [%s]", input.Type)
		}
		sb.WriteString(fmt.Sprintf("  - %s%s (%s)\n", input.Name, typeStr, input.Description))
	}
	sb.WriteString("Outputs:\n")
	for _, output := range s.Outputs {
		typeStr := ""
		if output.Type != FieldTypeText {
			typeStr = fmt.Sprintf(" [%s]", output.Type)
		}
		sb.WriteString(fmt.Sprintf("  - %s%s (%s)\n", output.Name, typeStr, output.Description))
	}
	if s.Instruction != "" {
		sb.WriteString(fmt.Sprintf("Instruction: %s\n", s.Instruction))
	}
	return sb.String()
}

// ParseSignature parses a signature string into a Signature struct.
func ParseSignature(signatureStr string) (Signature, error) {
	parts := strings.Split(signatureStr, "->")
	if len(parts) != 2 {
		return Signature{}, fmt.Errorf("invalid signature format: %s", signatureStr)
	}

	inputs := parseInputFields(strings.TrimSpace(parts[0]))
	outputs := parseOutputFields(strings.TrimSpace(parts[1]))

	return NewSignature(inputs, outputs), nil
}

func parseInputFields(fieldsStr string) []InputField {
	fieldStrs := strings.Split(fieldsStr, ",")
	fields := make([]InputField, len(fieldStrs))
	for i, fieldStr := range fieldStrs {
		fieldStr = strings.TrimSpace(fieldStr)
		fields[i] = InputField{Field: Field{Name: fieldStr}}
	}
	return fields
}

func parseOutputFields(fieldsStr string) []OutputField {
	fieldStrs := strings.Split(fieldsStr, ",")
	fields := make([]OutputField, len(fieldStrs))
	for i, fieldStr := range fieldStrs {
		fieldStr = strings.TrimSpace(fieldStr)
		fields[i] = OutputField{Field: Field{Name: fieldStr}}
	}
	return fields
}

// ShorthandNotation creates a Signature from a shorthand notation string.
func ShorthandNotation(notation string) (Signature, error) {
	return ParseSignature(notation)
}

// AppendInput adds an input field to the signature.
func (s Signature) AppendInput(name string, prefix string, description string) Signature {
	newInput := InputField{
		Field: Field{
			Name:        name,
			Prefix:      prefix,
			Description: description,
		},
	}
	s.Inputs = append(s.Inputs, newInput)
	return s
}

// PrependOutput adds an output field to the beginning of the outputs.
func (s Signature) PrependOutput(name string, prefix string, description string) Signature {
	newOutput := OutputField{
		Field: Field{
			Name:        name,
			Prefix:      prefix,
			Description: description,
		},
	}
	s.Outputs = append([]OutputField{newOutput}, s.Outputs...)
	return s
}
