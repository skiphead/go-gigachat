package gigachat

import (
	"encoding/json"
	"fmt"
)

// Role represents a message role in a chat conversation.
type Role string

const (
	// RoleSystem represents a system message.
	RoleSystem Role = "system"
	// RoleUser represents a user message.
	RoleUser Role = "user"
	// RoleAssistant represents an assistant message.
	RoleAssistant Role = "assistant"
	// RoleFunction represents a function message.
	RoleFunction Role = "function"
	// RoleFunctionProgress represents a function progress message.
	RoleFunctionProgress Role = "function_in_progress"
)

// FinishReason represents the reason why a completion finished.
type FinishReason string

const (
	// FinishReasonStop indicates the model finished naturally.
	FinishReasonStop FinishReason = "stop"
	// FinishReasonLength indicates the completion exceeded max tokens.
	FinishReasonLength FinishReason = "length"
	// FinishReasonFunctionCall indicates the model called a function.
	FinishReasonFunctionCall FinishReason = "function_call"
	// FinishReasonBlacklist indicates content was blocked.
	FinishReasonBlacklist FinishReason = "blacklist"
	// FinishReasonError indicates an error occurred.
	FinishReasonError FinishReason = "error"
)

// FunctionCallMode represents how functions should be called.
type FunctionCallMode string

const (
	// FunctionCallNone prevents function calling.
	FunctionCallNone FunctionCallMode = "none"
	// FunctionCallAuto allows the model to decide when to call functions.
	FunctionCallAuto FunctionCallMode = "auto"
)

// FunctionCall represents a function call in a message.
type FunctionCall struct {
	// Name is the function name.
	Name string `json:"name"`
	// Arguments are the function arguments.
	Arguments map[string]interface{} `json:"arguments"`
}

// FunctionCallParam represents the function_call parameter in chat requests.
// It can be either a mode ("auto", "none") or a specific function name.
type FunctionCallParam struct {
	mode FunctionCallMode
	name *string
}

// NewFunctionCallMode creates a FunctionCallParam with a mode.
func NewFunctionCallMode(mode FunctionCallMode) FunctionCallParam {
	return FunctionCallParam{mode: mode, name: nil}
}

// NewFunctionCallName creates a FunctionCallParam with a specific function name.
func NewFunctionCallName(name string) FunctionCallParam {
	return FunctionCallParam{mode: "", name: &name}
}

// MarshalJSON implements json.Marshaler.
func (f FunctionCallParam) MarshalJSON() ([]byte, error) {
	if f.name != nil {
		return json.Marshal(struct {
			Name string `json:"name"`
		}{Name: *f.name})
	}
	if f.mode != "" {
		return json.Marshal(f.mode)
	}
	return json.Marshal(FunctionCallAuto) // default
}

// UnmarshalJSON implements json.Unmarshaler.
func (f *FunctionCallParam) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string (mode)
	var modeStr string
	if err := json.Unmarshal(data, &modeStr); err == nil {
		f.mode = FunctionCallMode(modeStr)
		f.name = nil
		return nil
	}

	// Try to unmarshal as object (name)
	var nameObj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &nameObj); err == nil {
		f.mode = ""
		f.name = &nameObj.Name
		return nil
	}

	return fmt.Errorf("invalid function_call value: %s", string(data))
}

// Message represents a chat message.
type Message struct {
	// Role of the message sender.
	Role Role `json:"role"`
	// Content of the message.
	Content string `json:"content,omitempty"`
	// FunctionsStateID is an optional state ID for functions.
	FunctionsStateID *string `json:"functions_state_id,omitempty"`
	// Attachments are file IDs attached to the message.
	Attachments []string `json:"attachments,omitempty"`
	// Name is used for function_in_progress messages.
	Name *string `json:"name,omitempty"`
	// Created is a timestamp for function_in_progress messages.
	Created *int64 `json:"created,omitempty"`
	// FunctionCall represents a function call in the message.
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

// CustomFunction represents a user-defined function.
type CustomFunction struct {
	// Name of the function.
	Name string `json:"name"`
	// Description of what the function does.
	Description *string `json:"description,omitempty"`
	// Parameters schema for the function.
	Parameters map[string]interface{} `json:"parameters"`
	// FewShotExamples are examples for the function.
	FewShotExamples []FewShotExample `json:"few_shot_examples,omitempty"`
	// ReturnParameters schema for the function's return value.
	ReturnParameters map[string]interface{} `json:"return_parameters,omitempty"`
}

// FewShotExample represents an example for a function.
type FewShotExample struct {
	// Request is the user request.
	Request string `json:"request"`
	// Params are the function parameters.
	Params map[string]interface{} `json:"params"`
}

// ChatRequest represents a request to the chat completions API.
type ChatRequest struct {
	// Model ID to use for completion.
	Model string `json:"model"`
	// Messages in the conversation.
	Messages []Message `json:"messages"`
	// FunctionCall controls function calling behavior.
	FunctionCall *FunctionCallParam `json:"function_call,omitempty"`
	// Functions available for the model to call.
	Functions []CustomFunction `json:"functions,omitempty"`
	// Temperature controls randomness (0-2).
	Temperature *float64 `json:"temperature,omitempty"`
	// TopP controls nucleus sampling.
	TopP *float64 `json:"top_p,omitempty"`
	// Stream indicates if the response should be streamed.
	Stream bool `json:"stream"`
	// MaxTokens limits the response length.
	MaxTokens *int32 `json:"max_tokens,omitempty"`
	// RepetitionPenalty discourages repetition.
	RepetitionPenalty *float64 `json:"repetition_penalty,omitempty"`
	// UpdateInterval for streaming updates.
	UpdateInterval *float64 `json:"update_interval,omitempty"`
}

// ChatResponse represents a response from the chat completions API.
type ChatResponse struct {
	// Choices are the completion choices.
	Choices []Choice `json:"choices"`
	// Created is the timestamp of creation.
	Created int64 `json:"created"`
	// Model used for completion.
	Model string `json:"model"`
	// Object is the type of object ("chat.completion").
	Object string `json:"object"`
	// Usage statistics for the request.
	Usage Usage `json:"usage"`
}

// Choice represents a single completion choice.
type Choice struct {
	// Message containing the completion.
	Message Message `json:"message"`
	// Index of this choice.
	Index int `json:"index"`
	// FinishReason indicates why completion stopped.
	FinishReason FinishReason `json:"finish_reason"`
}

// Usage represents token usage statistics.
type Usage struct {
	// PromptTokens used in the request.
	PromptTokens int32 `json:"prompt_tokens"`
	// CompletionTokens generated in the response.
	CompletionTokens int32 `json:"completion_tokens"`
	// PrecachedPromptTokens from cached content.
	PrecachedPromptTokens int32 `json:"precached_prompt_tokens"`
	// TotalTokens used (prompt + completion).
	TotalTokens int32 `json:"total_tokens"`
}

// StreamChunk represents a chunk from a streaming response.
type StreamChunk struct {
	// Choices in this chunk.
	Choices []StreamChoice `json:"choices"`
	// Created timestamp.
	Created int64 `json:"created"`
	// Model being used.
	Model string `json:"model"`
	// Object type ("chat.completion.chunk").
	Object string `json:"object"`
	// Usage statistics (only in final chunk).
	Usage *Usage `json:"usage,omitempty"`
}

// StreamChoice represents a choice in a stream chunk.
type StreamChoice struct {
	// Delta containing the new content.
	Delta StreamDelta `json:"delta"`
	// Index of this choice.
	Index int `json:"index"`
	// FinishReason when the choice is complete.
	FinishReason *FinishReason `json:"finish_reason,omitempty"`
}

// StreamDelta represents the content delta in a stream chunk.
type StreamDelta struct {
	// Role of the message sender.
	Role *string `json:"role,omitempty"`
	// Content delta.
	Content *string `json:"content,omitempty"`
	// Name for function_in_progress messages.
	Name *string `json:"name,omitempty"`
	// FunctionCall delta.
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}
