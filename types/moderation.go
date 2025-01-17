package types

type ModerationRequest struct {
	Input any    `json:"input,omitempty" binding:"required"`
	Model string `json:"model,omitempty"`
}

type ModerationResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Results any    `json:"results"`
}
