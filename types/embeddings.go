package types

type EmbeddingRequest struct {
	Model          string `json:"model" binding:"required"`
	Input          any    `json:"input" binding:"required"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
	User           string `json:"user,omitempty"`
}

type Embedding struct {
	Object    string `json:"object"`
	Embedding any    `json:"embedding"`
	Index     int    `json:"index"`
}

type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  *Usage      `json:"usage,omitempty"`
}

func (r EmbeddingRequest) ParseInput() []string {
	if r.Input == nil {
		return nil
	}
	var input []string
	switch r.Input.(type) {
	case string:
		input = []string{r.Input.(string)}
	case []any:
		input = make([]string, 0, len(r.Input.([]any)))
		for _, item := range r.Input.([]any) {
			if str, ok := item.(string); ok {
				input = append(input, str)
			}
		}
	}
	return input
}

func (r EmbeddingRequest) ParseInputString() string {
	if r.Input == nil {
		return ""
	}

	var input string
	switch r.Input.(type) {
	case string:
		input = r.Input.(string)
	case []any:
		// 取第一个
		if len(r.Input.([]any)) > 0 {
			input = r.Input.([]any)[0].(string)
		}
	}
	return input
}
