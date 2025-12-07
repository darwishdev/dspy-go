package llms

import (
	"context"
	"fmt"

	"github.com/darwishdev/dspy-go/pkg/core"
	"github.com/darwishdev/dspy-go/pkg/errors"
	"github.com/darwishdev/dspy-go/pkg/utils"
)

// GeminiLLM implements the core.LLM interface for Google's Gemini model.
type GeminiTypedLLM struct {
	*core.BaseLLM
	apiKey           string
	inputSchema      *GeminiSchema
	responseMIMEType string
	outputSchema     *GeminiSchema
}

// GeminiRequest represents the request structure for Gemini API.
type geminiTypedRequest struct {
	Contents         []geminiContent             `json:"contents"`
	GenerationConfig geminiTypedGenerationConfig `json:"generationConfig,omitempty"`
}

// Add this to your existing geminiRequest struct or create a new one for function calling.
type geminiTypedWithFunction struct {
	Contents         []geminiContent             `json:"contents"`
	Tools            []geminiTool                `json:"tools,omitempty"`
	GenerationConfig geminiTypedGenerationConfig `json:"generationConfig,omitempty"`
}
type GeminiSchema struct {
	AnyOf            []*GeminiSchema          `json:"anyOf,omitempty"`
	Default          interface{}              `json:"default,omitempty"`
	Description      string                   `json:"description,omitempty"`
	Enum             []string                 `json:"enum,omitempty"`
	Example          interface{}              `json:"example,omitempty"`
	Format           string                   `json:"format,omitempty"`
	Items            *GeminiSchema            `json:"items,omitempty"`
	MaxItems         *int64                   `json:"maxItems,omitempty"`
	MaxLength        *int64                   `json:"maxLength,omitempty"`
	MaxProperties    *int64                   `json:"maxProperties,omitempty"`
	Maximum          *float64                 `json:"maximum,omitempty"`
	MinItems         *int64                   `json:"minItems,omitempty"`
	MinLength        *int64                   `json:"minLength,omitempty"`
	MinProperties    *int64                   `json:"minProperties,omitempty"`
	Minimum          *float64                 `json:"minimum,omitempty"`
	Nullable         *bool                    `json:"nullable,omitempty"`
	Pattern          string                   `json:"pattern,omitempty"`
	Properties       map[string]*GeminiSchema `json:"properties,omitempty"`
	PropertyOrdering []string                 `json:"propertyOrdering,omitempty"`
	Required         []string                 `json:"required,omitempty"`
	Title            string                   `json:"title,omitempty"`
	Type             string                   `json:"type,omitempty"`
}
type geminiTypedGenerationConfig struct {
	Temperature          float64       `json:"temperature,omitempty"`
	MaxOutputTokens      int           `json:"maxOutputTokens,omitempty"`
	TopP                 float64       `json:"topP,omitempty"`
	ResponseMIMEType     string        `json:"responseMimeType,omitempty"`
	Parameters           *GeminiSchema `json:"parameters,omitempty"`
	ParametersJsonSchema interface{}   `json:"parametersJsonSchema,omitempty"`
	ResponseSchema       *GeminiSchema `json:"responseSchema,omitempty"`
	ResponseJSON         interface{}   `json:"responseJsonSchema,omitempty"`
}

type GeminiProviderConfig struct {
	core.ProviderConfig               // Embeds the original ProviderConfig
	RequestSchema       *GeminiSchema `json:"parameters,omitempty"`
	ResponseSchema      *GeminiSchema `json:"responseSchema,omitempty"`
	ResponseMIMEType    string        `json:"responseMimeType,omitempty"`
}

// NewGeminiLLMFromConfig creates a new GeminiLLM instance from configuration.
func NewGeminiTypedLLMFromConfig(ctx context.Context, config GeminiProviderConfig, modelID core.ModelID) (*GeminiTypedLLM, error) {
	baseConfig := core.ProviderConfig{
		Name:     config.Name,
		APIKey:   config.APIKey,
		BaseURL:  config.BaseURL,
		Models:   config.Models,
		Params:   config.Params,
		Endpoint: config.Endpoint,
	}
	geminConfig, err := NewGeminiLLMFromConfig(ctx, baseConfig, modelID)
	return &GeminiTypedLLM{
		apiKey:       config.APIKey,
		BaseLLM:      geminConfig.BaseLLM,
		inputSchema:  config.RequestSchema,
		outputSchema: config.ResponseSchema,
	}, err
}

// Generate implements the core.LLM interface.
func (g *GeminiTypedLLM) Generate(ctx context.Context, prompt string, options ...core.GenerateOption) (*core.LLMResponse, error) {
	opts := core.NewGenerateOptions()
	for _, opt := range options {
		opt(opts)
	}

	reqBody := geminiTypedRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiTypedGenerationConfig{
			Temperature:      opts.Temperature,
			MaxOutputTokens:  opts.MaxTokens,
			TopP:             opts.TopP,
			Parameters:       g.inputSchema,
			ResponseSchema:   g.outputSchema,
			ResponseMIMEType: g.responseMIMEType,
		},
	}

	resp, err := handleGeminiGenerateRequest(ctx, g.GetHTTPClient(), reqBody, g.GetEndpointConfig(), prompt, g.ModelID(), g.apiKey)
	if err != nil {
		return nil, errors.WithFields(
			errors.New(errors.LLMGenerationFailed, fmt.Sprintf("LLMGenerationFailed: failed to send gemin generate request: %v", err)),
			errors.Fields{
				"model": g.ModelID(),
			})
	}
	return handleGeminiResponseParsing(resp, g.ModelID())
}

type geminiTypedRequestWithFunction struct {
	Contents         []geminiContent             `json:"contents"`
	Tools            []geminiTool                `json:"tools,omitempty"`
	GenerationConfig geminiTypedGenerationConfig `json:"generationConfig,omitempty"`
}

// GenerateWithJSON implements the core.LLM interface.
func (g *GeminiTypedLLM) GenerateWithJSON(ctx context.Context, prompt string, options ...core.GenerateOption) (map[string]interface{}, error) {
	response, err := g.Generate(ctx, prompt, options...)
	if err != nil {
		return nil, err
	}

	return utils.ParseJSONResponse(response.Content)
}

func (g *GeminiTypedLLM) streamRequest(ctx context.Context, reqBody interface{}) (*core.StreamResponse, error) {
	// Create a channel for streaming chunks
	chunkChan := make(chan core.StreamChunk)
	streamCtx, cancelStream := context.WithCancel(ctx)

	// Create the streaming response
	response := &core.StreamResponse{
		ChunkChannel: chunkChan,
		Cancel: func() {
			cancelStream()
		},
	}

	// Call the base function to handle the stream request
	resp, err := handleGeminiStreamRequest(ctx, g.GetHTTPClient(), reqBody, g.GetEndpointConfig(), g.apiKey)
	if err != nil {
		return nil, err
	}

	// Handle the response using the base function
	go handleGeminiStreamResponse(resp, chunkChan, streamCtx)

	return response, nil
}

// StreamGenerate for Gemini.
func (g *GeminiTypedLLM) StreamGenerate(ctx context.Context, prompt string, options ...core.GenerateOption) (*core.StreamResponse, error) {
	opts := core.NewGenerateOptions()
	for _, opt := range options {
		opt(opts)
	}

	reqBody := geminiTypedRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiTypedGenerationConfig{
			Temperature:      opts.Temperature,
			MaxOutputTokens:  opts.MaxTokens,
			TopP:             opts.TopP,
			Parameters:       g.inputSchema,
			ResponseSchema:   g.outputSchema,
			ResponseMIMEType: g.responseMIMEType,
		},
	}

	return g.streamRequest(ctx, reqBody)
}
