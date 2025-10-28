package ai

// NewAdapter creates a new analyzer that can be used with version checker
// The analyzer already implements the required interface
func NewAdapter(apiKey, model, baseURL string) *Analyzer {
	return New(apiKey, model, baseURL)
}
