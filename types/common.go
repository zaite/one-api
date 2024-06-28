package types

import "encoding/json"

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIError struct {
	Code       any    `json:"code,omitempty"`
	Message    string `json:"message"`
	Param      string `json:"param,omitempty"`
	Type       string `json:"type"`
	InnerError any    `json:"innererror,omitempty"`
}

func (e *OpenAIError) Error() string {
	response := &OpenAIErrorResponse{
		Error: *e,
	}

	// 转换为JSON
	bytes, _ := json.Marshal(response)
	return string(bytes)
}

type OpenAIErrorWithStatusCode struct {
	OpenAIError
	StatusCode int `json:"status_code"`
}

type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error,omitempty"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}
