package llm

// Provider identifies an OpenAI-compatible LLM provider. Its zero value is invalid; use one of the
// exported provider constants.
type Provider string

// Supported OpenAI-compatible providers.
const (
	OpenAI   Provider = "openai"
	Groq     Provider = "groq"
	DeepSeek Provider = "deepseek"
	Together Provider = "together"
	Ollama   Provider = "ollama"
)

const (
	// envAPIKey is the provider-agnostic key, preferred over the provider-specific ones below.
	envAPIKey = "LLM_API_KEY" //nolint:gosec // G101: env var name, not a credential
	// envBaseURL overrides the endpoint from configuration when Client.BaseURL is not set.
	envBaseURL = "LLM_BASE_URL"

	envOpenAI   = "OPENAI_API_KEY"
	envGroq     = "GROQ_API_KEY"
	envDeepSeek = "DEEPSEEK_API_KEY"
	envTogether = "TOGETHER_API_KEY"
	envOllama   = "OLLAMA_API_KEY"
)

type providerDefault struct {
	baseURL string
	envVar  string
}

// providerDefaults returns the default base URL and API-key environment variable for a provider.
// The boolean is false for an unknown provider.
func providerDefaults(p Provider) (providerDefault, bool) {
	table := map[Provider]providerDefault{
		OpenAI:   {baseURL: "https://api.openai.com/v1", envVar: envOpenAI},
		Groq:     {baseURL: "https://api.groq.com/openai/v1", envVar: envGroq},
		DeepSeek: {baseURL: "https://api.deepseek.com", envVar: envDeepSeek},
		Together: {baseURL: "https://api.together.xyz/v1", envVar: envTogether},
		Ollama:   {baseURL: "http://localhost:11434/v1", envVar: envOllama},
	}

	d, ok := table[p]

	return d, ok
}
