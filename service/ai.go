package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/GPadaka19/attesta-be/config"
	"github.com/GPadaka19/attesta-be/model"
)

const (
	anthropicAPI     = "https://api.anthropic.com/v1/messages"
	anthropicModel   = "claude-sonnet-4-20250514"
	anthropicVersion = "2023-06-01"
	aiMaxTokens      = 1024
	aiTimeout        = 30 * time.Second
)

const systemPrompt = `You are a technical contribution analyst. Analyze this developer's GitHub commits and generate a skill proof.

Rules:
- Be strict: if contributions are mostly translation files (locales/, i18n/, translations/), set i18n_only to true
- If contributions are trivial (whitespace, typo fixes only), set contribution_quality to "low"
- Infer languages from file extensions, not just repo_languages
- skill_tags must be specific and actionable (e.g. "Next.js auth" not just "frontend")
- confidence_score reflects how much data you have: few commits = lower score

Return ONLY a JSON object. No markdown. No explanation. No backticks.

Schema:
{
  "primary_language": "string",
  "secondary_languages": ["string"],
  "contribution_types": ["frontend|backend|testing|docs|i18n|config|devops"],
  "i18n_only": boolean,
  "skill_tags": ["max 5 specific skills"],
  "contribution_quality": "low|medium|high",
  "confidence_score": number (0-100),
  "red_flags": ["string"] or [],
  "summary": "2-3 sentences",
  "period": {
    "first_commit": "YYYY-MM-DD",
    "last_commit": "YYYY-MM-DD"
  }
}`

var aiClient = &http.Client{Timeout: aiTimeout}

// AnalyzeCommits routes to the configured AI provider.
func AnalyzeCommits(payload *model.GitHubPayload) (*model.SkillProof, error) {
	switch config.App.AIProvider {
	case "openai":
		return analyzeOpenAI(payload)
	default:
		return analyzeAnthropic(payload)
	}
}

// --- Anthropic ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func analyzeAnthropic(payload *model.GitHubPayload) (*model.SkillProof, error) {
	log.Printf("[AI] provider=anthropic model=%s commits=%d", anthropicModel, len(payload.Commits))

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal payload: %w", err)
	}

	body, err := json.Marshal(anthropicRequest{
		Model:     anthropicModel,
		MaxTokens: aiMaxTokens,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: string(payloadJSON)}},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", config.App.AnthropicAPIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := aiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ai: status %d: %s", resp.StatusCode, string(raw))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("ai: parse response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("ai: api error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("AI_PARSE_ERROR")
	}

	return parseSkillProof(apiResp.Content[0].Text)
}

// --- OpenAI-compatible (Groq, OpenAI, etc.) ---

type openAIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func analyzeOpenAI(payload *model.GitHubPayload) (*model.SkillProof, error) {
	model := config.App.OpenAIModel
	baseURL := strings.TrimRight(config.App.OpenAIBaseURL, "/")
	log.Printf("[AI] provider=openai model=%s commits=%d", model, len(payload.Commits))

	if baseURL == "" || model == "" {
		return nil, fmt.Errorf("ai: OPENAI_BASE_URL and OPENAI_MODEL are required when AI_PROVIDER=openai")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal payload: %w", err)
	}

	body, err := json.Marshal(openAIRequest{
		Model:     model,
		MaxTokens: aiMaxTokens,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(payloadJSON)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.App.OpenAIAPIKey)

	resp, err := aiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: http call: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ai: status %d: %s", resp.StatusCode, string(raw))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("ai: parse response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("ai: api error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("AI_PARSE_ERROR")
	}

	return parseSkillProof(apiResp.Choices[0].Message.Content)
}

// --- shared ---

func parseSkillProof(text string) (*model.SkillProof, error) {
	text = stripMarkdownFence(strings.TrimSpace(text))

	var proof model.SkillProof
	if err := json.Unmarshal([]byte(text), &proof); err != nil {
		log.Printf("[AI] invalid JSON: %s", text)
		return nil, fmt.Errorf("AI_PARSE_ERROR")
	}

	log.Printf("[AI] analysis done: %s, confidence=%d", proof.PrimaryLanguage, proof.ConfidenceScore)
	return &proof, nil
}

func stripMarkdownFence(s string) string {
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
