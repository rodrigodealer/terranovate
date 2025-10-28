package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// Analyzer provides AI-powered breaking change detection using OpenAI API
type Analyzer struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// BreakingChangeAnalysis represents the AI analysis result
type BreakingChangeAnalysis struct {
	HasBreakingChanges bool     `json:"has_breaking_changes"`
	Summary            string   `json:"summary"`
	Details            []string `json:"details"`
	Confidence         string   `json:"confidence"` // high, medium, low
}

// AIAnalysis is an alias for BreakingChangeAnalysis to match version package expectations
type AIAnalysis = BreakingChangeAnalysis

// New creates a new AI analyzer instance
func New(apiKey, model, baseURL string) *Analyzer {
	return &Analyzer{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AnalyzeBreakingChanges analyzes changelog/release notes for breaking changes
func (a *Analyzer) AnalyzeBreakingChanges(ctx context.Context, moduleName, currentVersion, latestVersion, changelogURL string) (*AIAnalysis, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	prompt := a.buildPrompt(moduleName, currentVersion, latestVersion, changelogURL)

	log.Debug().
		Str("module", moduleName).
		Str("current", currentVersion).
		Str("latest", latestVersion).
		Msg("analyzing breaking changes with AI")

	response, err := a.callOpenAI(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}

	analysis, err := a.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	log.Debug().
		Str("module", moduleName).
		Bool("breaking_changes", analysis.HasBreakingChanges).
		Str("confidence", analysis.Confidence).
		Msg("AI analysis completed")

	return analysis, nil
}

// buildPrompt creates the prompt for OpenAI
func (a *Analyzer) buildPrompt(moduleName, currentVersion, latestVersion, changelogURL string) string {
	return fmt.Sprintf(`You are an expert in analyzing Terraform module and provider updates for breaking changes.

Analyze the upgrade from version %s to %s for the following module/provider:
Module/Provider: %s
Changelog URL: %s

Please analyze if this update contains breaking changes. Consider:
1. API changes (removed variables, changed types, new required variables)
2. Resource replacements or deletions
3. Major behavioral changes
4. Deprecations that affect functionality

Respond ONLY with valid JSON in this exact format (no markdown, no code blocks):
{
  "has_breaking_changes": true or false,
  "summary": "Brief summary of the changes (1-2 sentences)",
  "details": ["detail 1", "detail 2", "detail 3"],
  "confidence": "high, medium, or low"
}

Important: Base your analysis on version number patterns and Terraform best practices. If you cannot determine with certainty, set confidence to "low" or "medium".`, currentVersion, latestVersion, moduleName, changelogURL)
}

// callOpenAI makes a request to the OpenAI API
func (a *Analyzer) callOpenAI(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model": a.model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.3, // Lower temperature for more consistent/deterministic results
		"max_tokens":  1000,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := a.baseURL + "/chat/completions"
	log.Debug().
		Str("endpoint", endpoint).
		Str("model", a.model).
		Str("api_key_prefix", maskAPIKey(a.apiKey)).
		Msg("calling AI API")

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s (%s)", result.Error.Message, result.Error.Type)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// parseResponse parses the AI response into structured data
func (a *Analyzer) parseResponse(response string) (*BreakingChangeAnalysis, error) {
	// Try to find JSON in the response (in case the model wrapped it in markdown)
	jsonStart := bytes.Index([]byte(response), []byte("{"))
	jsonEnd := bytes.LastIndex([]byte(response), []byte("}"))

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonContent := response[jsonStart : jsonEnd+1]

	var analysis BreakingChangeAnalysis
	if err := json.Unmarshal([]byte(jsonContent), &analysis); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Validate confidence level
	if analysis.Confidence != "high" && analysis.Confidence != "medium" && analysis.Confidence != "low" {
		analysis.Confidence = "low"
	}

	return &analysis, nil
}

// maskAPIKey masks an API key for logging, showing only first 4 and last 4 characters
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
