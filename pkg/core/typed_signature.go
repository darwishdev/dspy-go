package core

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

//
// ────────────────────────────────────────────────
//  FIELD METADATA EXTENDED FOR NESTED STRUCTURES
// ────────────────────────────────────────────────
//

type FieldMetadata struct {
	Name        string
	GoFieldName string
	Required    bool
	Description string
	Prefix      string
	Type        FieldType
	GoType      reflect.Type

	// NEW
	Item       *FieldMetadata            // for arrays
	Properties map[string]*FieldMetadata // for nested objects
}
type SignatureMetadata struct {
	Inputs      []FieldMetadata
	Outputs     []FieldMetadata
	Instruction string
}

//
// ────────────────────────────────────────────────
//  TYPED SIGNATURE IMPLEMENTATION
// ────────────────────────────────────────────────
//

type typedSignatureImpl[TInput, TOutput any] struct {
	inputType  reflect.Type
	outputType reflect.Type
	metadata   SignatureMetadata
}

//
// ────────────────────────────────────────────────
//  MAIN ENTRY — parse types into metadata
// ────────────────────────────────────────────────
//

func createTypedSignatureImpl[TInput, TOutput any](inputType, outputType reflect.Type) *typedSignatureImpl[TInput, TOutput] {
	metadata := SignatureMetadata{
		Inputs:  parseStructFields(inputType, true),
		Outputs: parseStructFields(outputType, false),
	}

	return &typedSignatureImpl[TInput, TOutput]{
		inputType:  inputType,
		outputType: outputType,
		metadata:   metadata,
	}
}

//
// ────────────────────────────────────────────────
//  STRUCT FIELD PARSER (recursive)
// ────────────────────────────────────────────────
//

func parseStructFields(t reflect.Type, isInput bool) []FieldMetadata {
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	var fields []FieldMetadata

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		meta := parseFieldMetadataRecursive(sf, isInput)
		fields = append(fields, meta)
	}

	return fields
}

//
// ────────────────────────────────────────────────
//  FIELD PARSER — RECURSIVE SUPPORT
// ────────────────────────────────────────────────
//

func parseFieldMetadataRecursive(field reflect.StructField, isInput bool) FieldMetadata {
	meta := FieldMetadata{
		Name:        strings.ToLower(field.Name),
		GoFieldName: field.Name,
		GoType:      field.Type,
		Type:        inferFieldType(field.Type),
		Description: field.Name,
		Required:    false,
	}

	// dspy:"name,required"
	if tag := field.Tag.Get("dspy"); tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			meta.Name = parts[0]
		}
		for _, p := range parts[1:] {
			if strings.TrimSpace(p) == "required" {
				meta.Required = true
			}
		}
	}

	// description:"..."
	if desc := field.Tag.Get("description"); desc != "" {
		meta.Description = desc
	}

	// prefix:"..."
	if !isInput {
		meta.Prefix = meta.Name + ":"
	}
	if pfx := field.Tag.Get("prefix"); pfx != "" {
		meta.Prefix = pfx
	}

	//
	// NESTING LOGIC
	//
	switch meta.Type {

	case FieldTypeObject:
		meta.Properties = parseObjectProperties(field.Type)

	case FieldTypeArray:
		meta.Item = parseArrayElement(field.Type)

	}

	return meta
}

//
// ────────────────────────────────────────────────
//  DETECT TYPE: int, bool, []string, struct, nested
// ────────────────────────────────────────────────
//

func inferFieldType(t reflect.Type) FieldType {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {

	case reflect.String:
		return FieldTypeString

	case reflect.Bool:
		return FieldTypeBool

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return FieldTypeInt

	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return FieldTypeImage
		}
		return FieldTypeArray

	case reflect.Map, reflect.Struct:
		return FieldTypeObject

	default:
		return FieldTypeText
	}
}

//
// ────────────────────────────────────────────────
//  RECURSIVE PARSING OF OBJECT FIELDS
// ────────────────────────────────────────────────
//

func parseObjectProperties(t reflect.Type) map[string]*FieldMetadata {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	props := map[string]*FieldMetadata{}
	if t.Kind() != reflect.Struct {
		return props
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		child := parseFieldMetadataRecursive(sf, true)
		props[child.Name] = &child
	}

	return props
}

//
// ────────────────────────────────────────────────
//  RECURSIVE PARSING OF ARRAY ITEMS
// ────────────────────────────────────────────────
//

func parseArrayElement(t reflect.Type) *FieldMetadata {
	elem := t.Elem()

	fakeField := reflect.StructField{
		Name: elem.Name(),
		Type: elem,
		Tag:  "",
	}

	meta := parseFieldMetadataRecursive(fakeField, true)
	return &meta
}

//
// ────────────────────────────────────────────────
//  VALIDATION — supports nested object fields
// ────────────────────────────────────────────────
//

func validateStruct(value any, expected []FieldMetadata, fieldType string) error {
	if value == nil {
		return fmt.Errorf("%s cannot be nil", fieldType)
	}

	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("%s must be struct", fieldType)
	}

	for _, expectedField := range expected {
		field := v.FieldByName(expectedField.GoFieldName)

		if !expectedField.Required {
			continue
		}

		if !field.IsValid() || field.IsZero() {
			return fmt.Errorf("required %s field '%s' cannot be empty", fieldType, expectedField.Name)
		}

		// Nested object validation
		if expectedField.Type == FieldTypeObject {
			err := validateStruct(field.Interface(), flatten(expectedField.Properties), fieldType+"."+expectedField.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func flatten(m map[string]*FieldMetadata) []FieldMetadata {
	result := []FieldMetadata{}
	for _, v := range m {
		result = append(result, *v)
	}
	return result
}

//
// REST OF FILE REMAINS UNCHANGED
//

// TypedSignature provides compile-time type safety for module inputs and outputs.
type TypedSignature[TInput, TOutput any] interface {
	// GetInputType returns the reflect.Type for the input struct
	GetInputType() reflect.Type

	// GetOutputType returns the reflect.Type for the output struct
	GetOutputType() reflect.Type

	// ValidateInput performs compile-time and runtime validation of input
	ValidateInput(input TInput) error

	// ValidateOutput performs compile-time and runtime validation of output
	ValidateOutput(output TOutput) error

	// GetFieldMetadata returns parsed struct tag metadata
	GetFieldMetadata() SignatureMetadata

	// ToLegacySignature converts to the existing Signature interface for backward compatibility
	ToLegacySignature() Signature

	// WithInstruction returns a new TypedSignature with the specified instruction
	WithInstruction(instruction string) TypedSignature[TInput, TOutput]
}

// SignatureMetadata contains parsed information from struct tags.

// getReflectTypes extracts and normalizes reflect.Type information for the given generic types.
// It handles pointer types by extracting the underlying element type.
func getReflectTypes[TInput, TOutput any]() (reflect.Type, reflect.Type) {
	var input TInput
	var output TOutput

	inputType := reflect.TypeOf(input)
	outputType := reflect.TypeOf(output)

	// Handle pointer types
	if inputType != nil && inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}
	if outputType != nil && outputType.Kind() == reflect.Ptr {
		outputType = outputType.Elem()
	}

	return inputType, outputType
}

// NewTypedSignature creates a new typed signature for the given input/output types.
func NewTypedSignature[TInput, TOutput any]() TypedSignature[TInput, TOutput] {
	inputType, outputType := getReflectTypes[TInput, TOutput]()
	return createTypedSignatureImpl[TInput, TOutput](inputType, outputType)
}

// Global cache for TypedSignature instances to improve performance.
var typedSignatureCache sync.Map

// signatureCacheKey represents a composite key for caching TypedSignatures.
type signatureCacheKey struct {
	inputType  reflect.Type
	outputType reflect.Type
}

// NewTypedSignatureCached creates a cached typed signature for the given input/output types.
// This function provides better performance for repeated calls with the same types.
func NewTypedSignatureCached[TInput, TOutput any]() TypedSignature[TInput, TOutput] {
	inputType, outputType := getReflectTypes[TInput, TOutput]()

	// Create cache key
	key := signatureCacheKey{
		inputType:  inputType,
		outputType: outputType,
	}

	// Try to get from cache first
	if cached, ok := typedSignatureCache.Load(key); ok {
		return cached.(TypedSignature[TInput, TOutput])
	}

	// Not in cache, create new signature using the helper
	signature := createTypedSignatureImpl[TInput, TOutput](inputType, outputType)

	// Use LoadOrStore to prevent race condition where multiple goroutines
	// could create and store signatures for the same key concurrently
	if actual, loaded := typedSignatureCache.LoadOrStore(key, signature); loaded {
		return actual.(TypedSignature[TInput, TOutput])
	}
	return signature
}

func (ts *typedSignatureImpl[TInput, TOutput]) GetInputType() reflect.Type {
	return ts.inputType
}

func (ts *typedSignatureImpl[TInput, TOutput]) GetOutputType() reflect.Type {
	return ts.outputType
}

func (ts *typedSignatureImpl[TInput, TOutput]) ValidateInput(input TInput) error {
	return validateStruct(input, ts.metadata.Inputs, "input")
}

func (ts *typedSignatureImpl[TInput, TOutput]) ValidateOutput(output TOutput) error {
	return validateStruct(output, ts.metadata.Outputs, "output")
}

func (ts *typedSignatureImpl[TInput, TOutput]) GetFieldMetadata() SignatureMetadata {
	return ts.metadata
}

func (ts *typedSignatureImpl[TInput, TOutput]) ToLegacySignature() Signature {
	// Convert typed signature to legacy format
	inputs := make([]InputField, len(ts.metadata.Inputs))
	for i, field := range ts.metadata.Inputs {
		inputs[i] = InputField{
			Field: Field{
				Name:        field.Name,
				Description: field.Description,
				Type:        field.Type,
			},
		}
	}

	outputs := make([]OutputField, len(ts.metadata.Outputs))
	for i, field := range ts.metadata.Outputs {
		outputs[i] = OutputField{
			Field: Field{
				Name:        field.Name,
				Description: field.Description,
				Type:        field.Type,
			},
		}
	}

	signature := NewSignature(inputs, outputs)
	if ts.metadata.Instruction != "" {
		signature = signature.WithInstruction(ts.metadata.Instruction)
	}

	return signature
}

func (ts *typedSignatureImpl[TInput, TOutput]) WithInstruction(instruction string) TypedSignature[TInput, TOutput] {
	// Create a deep copy with the new instruction to avoid shallow copy issues
	newMetadata := SignatureMetadata{
		Instruction: instruction,
		Inputs:      make([]FieldMetadata, len(ts.metadata.Inputs)),
		Outputs:     make([]FieldMetadata, len(ts.metadata.Outputs)),
	}

	// Deep copy the input and output field metadata
	copy(newMetadata.Inputs, ts.metadata.Inputs)
	copy(newMetadata.Outputs, ts.metadata.Outputs)

	return &typedSignatureImpl[TInput, TOutput]{
		inputType:  ts.inputType,
		outputType: ts.outputType,
		metadata:   newMetadata,
	}
}

// parseFieldMetadata parses struct tag information for a single field.
func parseFieldMetadata(field reflect.StructField, isInput bool) FieldMetadata {
	metadata := FieldMetadata{
		Name:        strings.ToLower(field.Name),
		GoFieldName: field.Name, // Cache the Go field name for efficient lookup
		GoType:      field.Type,
		Type:        FieldTypeText, // Default to text
		Required:    false,         // Default to optional
		Description: field.Name,    // Auto-generate description from field name
	}

	// Parse dspy struct tag: `dspy:"fieldname,required"` (optional overrides)
	if dspyTag := field.Tag.Get("dspy"); dspyTag != "" {
		parts := strings.Split(dspyTag, ",")
		if len(parts) > 0 && parts[0] != "" {
			metadata.Name = parts[0] // Override the lowercase default
		}

		// Check for required flag
		for _, part := range parts[1:] {
			switch strings.TrimSpace(part) {
			case "required":
				metadata.Required = true
			}
		}
	}

	// Parse description tag (overrides auto-generated description)
	if desc := field.Tag.Get("description"); desc != "" {
		metadata.Description = desc
	}

	// Set default prefix (field name with colon for outputs)
	if isInput {
		metadata.Prefix = ""
	} else {
		metadata.Prefix = metadata.Name + ":"
	}

	// Parse prefix tag for outputs (optional override)
	if prefix := field.Tag.Get("prefix"); prefix != "" {
		metadata.Prefix = prefix
	}

	// Determine field type based on Go type
	metadata.Type = inferFieldType(field.Type)

	return metadata
}

// Backward compatibility: convert legacy signature to typed.
func FromLegacySignature(sig Signature) TypedSignature[map[string]any, map[string]any] {
	metadata := SignatureMetadata{
		Instruction: sig.Instruction,
	}

	// Convert input fields
	for _, input := range sig.Inputs {
		metadata.Inputs = append(metadata.Inputs, FieldMetadata{
			Name:        input.Name,
			GoFieldName: input.Name, // For maps, GoFieldName is the same as Name
			Description: input.Description,
			Prefix:      input.Prefix,
			Type:        input.Type,
			Required:    false,              // Default to optional for backward compatibility
			GoType:      reflect.TypeOf(""), // Default to string
		})
	}

	// Convert output fields
	for _, output := range sig.Outputs {
		metadata.Outputs = append(metadata.Outputs, FieldMetadata{
			Name:        output.Name,
			GoFieldName: output.Name, // For maps, GoFieldName is the same as Name
			Description: output.Description,
			Prefix:      output.Prefix,
			Type:        output.Type,
			Required:    false,              // Outputs are generally not "required"
			GoType:      reflect.TypeOf(""), // Default to string
		})
	}

	return &typedSignatureImpl[map[string]any, map[string]any]{
		inputType:  reflect.TypeOf(map[string]any{}),
		outputType: reflect.TypeOf(map[string]any{}),
		metadata:   metadata,
	}
}
