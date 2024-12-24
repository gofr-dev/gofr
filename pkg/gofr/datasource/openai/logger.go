package openai

type Logger interface {
	Debug(args ...interface{})
	Debugf(pattern string, args ...interface{})
	Logf(pattern string, args ...interface{})
	Errorf(pattern string, args ...interface{})
}

type OpenAiAPILog struct {
	ID                string `json:"id,omitempty"`
	Object            string `json:"object,omitempty"`
	Created           int    `json:"created,omitempty"`
	Model             string `json:"model,omitempty"`
	ServiceTier       string `json:"service_tier,omitempty"`
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
	Duration          int64  `json:"duration,omitempty"`

	Usage struct {
		PromptTokens           int         `json:"prompt_tokens,omitempty"`
		CompletionTokens       int         `json:"completion_tokens,omitempty"`
		TotalTokens            int         `json:"total_tokens,omitempty"`
		CompletionTokelDetails interface{} `json:"completion_tokens_details,omitempty"`
		PromptTokenDetails     interface{} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`

	Error *Error `json:"error,omitempty"`
}
