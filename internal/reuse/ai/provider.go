package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AIProvider defines the standard interface for AI model integrations.
type AIProvider interface {
	Name() string
	GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error)
	GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, target interface{}) error
}

// ---------------------------------------------------------------------------
// Google Gemini Provider
// ---------------------------------------------------------------------------

type GeminiProvider struct {
	rotator    *KeyRotator
	httpClient *http.Client
	model      string
}

func NewGeminiProvider(keys []string, model string) *GeminiProvider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiProvider{
		rotator: NewKeyRotator(keys),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		model: model,
	}
}

func (g *GeminiProvider) Name() string {
	return "Google Gemini"
}

func (g *GeminiProvider) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	key, err := g.rotator.GetNextKey()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, key)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"text": fmt.Sprintf("%s\n\nUser Request:\n%s", systemPrompt, userPrompt)},
				},
			},
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.rotator.MarkError(key)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		g.rotator.MarkRateLimited(key, 60*time.Second)
		return "", fmt.Errorf("gemini rate limit 429 for key")
	}

	if resp.StatusCode >= 400 {
		g.rotator.MarkError(key)
		return "", fmt.Errorf("gemini error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	g.rotator.MarkSuccess(key)

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (g *GeminiProvider) GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	key, err := g.rotator.GetNextKey()
	if err != nil {
		return "", err
	}

	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", g.model, key)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"text": fmt.Sprintf("%s\n\nQuestion / Problem:\n%s", systemPrompt, userPrompt)},
					{
						"inline_data": map[string]interface{}{
							"mime_type": mimeType,
							"data":      base64.StdEncoding.EncodeToString(imageBytes),
						},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.rotator.MarkError(key)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		g.rotator.MarkRateLimited(key, 60*time.Second)
		return "", fmt.Errorf("gemini rate limit 429 for vision key")
	}

	if resp.StatusCode >= 400 {
		g.rotator.MarkError(key)
		return "", fmt.Errorf("gemini vision error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	g.rotator.MarkSuccess(key)

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response from Gemini Vision")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (g *GeminiProvider) GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, target interface{}) error {
	fullSystemPrompt := systemPrompt + "\nIMPORTANT: You MUST respond ONLY with valid JSON. Do not include markdown code block markers (like ```json), commentary, or extra text."
	text, err := g.GenerateText(ctx, fullSystemPrompt, userPrompt)
	if err != nil {
		return err
	}

	cleanJSON := cleanJSONString(text)
	return json.Unmarshal([]byte(cleanJSON), target)
}

// ---------------------------------------------------------------------------
// OpenAI Provider (Failover)
// ---------------------------------------------------------------------------

type OpenAIProvider struct {
	rotator    *KeyRotator
	httpClient *http.Client
	model      string
}

func NewOpenAIProvider(keys []string, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIProvider{
		rotator: NewKeyRotator(keys),
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		model: model,
	}
}

func (o *OpenAIProvider) Name() string {
	return "OpenAI"
}

func (o *OpenAIProvider) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	key, err := o.rotator.GetNextKey()
	if err != nil {
		return "", err
	}

	reqBody := map[string]interface{}{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		o.rotator.MarkError(key)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests {
		o.rotator.MarkRateLimited(key, 60*time.Second)
		return "", fmt.Errorf("openai rate limit 429")
	}

	if resp.StatusCode >= 400 {
		o.rotator.MarkError(key)
		return "", fmt.Errorf("openai error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	o.rotator.MarkSuccess(key)

	var oaResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &oaResp); err != nil {
		return "", err
	}

	if len(oaResp.Choices) == 0 {
		return "", errors.New("empty response from OpenAI")
	}

	return oaResp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	key, err := o.rotator.GetNextKey()
	if err != nil {
		return "", err
	}

	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	b64Image := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(imageBytes))

	reqBody := map[string]interface{}{
		"model": o.model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": userPrompt},
					{"type": "image_url", "image_url": map[string]string{"url": b64Image}},
				},
			},
		},
	}

	raw, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		o.rotator.MarkError(key)
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		o.rotator.MarkError(key)
		return "", fmt.Errorf("openai vision error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	o.rotator.MarkSuccess(key)

	var oaResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &oaResp); err != nil {
		return "", err
	}
	if len(oaResp.Choices) == 0 {
		return "", errors.New("empty response from OpenAI vision")
	}

	return oaResp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, target interface{}) error {
	fullSystemPrompt := systemPrompt + "\nIMPORTANT: You MUST respond ONLY with valid JSON. Do not include markdown code block markers (like ```json), commentary, or extra text."
	text, err := o.GenerateText(ctx, fullSystemPrompt, userPrompt)
	if err != nil {
		return err
	}

	cleanJSON := cleanJSONString(text)
	return json.Unmarshal([]byte(cleanJSON), target)
}

// ---------------------------------------------------------------------------
// Multi-Provider Orchestrator with Intelligent Failover
// ---------------------------------------------------------------------------

type Orchestrator struct {
	providers []AIProvider
}

func NewOrchestrator(geminiKeys, openaiKeys, claudeKeys []string) *Orchestrator {
	providers := []AIProvider{}

	if len(geminiKeys) > 0 {
		providers = append(providers, NewGeminiProvider(geminiKeys, "gemini-2.0-flash"))
	}
	if len(openaiKeys) > 0 {
		providers = append(providers, NewOpenAIProvider(openaiKeys, "gpt-4o-mini"))
	}

	return &Orchestrator{
		providers: providers,
	}
}

// GenerateText executes with primary provider and falls back automatically on failure.
func (o *Orchestrator) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if len(o.providers) == 0 {
		return getMockTextResponse(userPrompt), nil
	}

	var lastErr error
	for _, p := range o.providers {
		res, err := p.GenerateText(ctx, systemPrompt, userPrompt)
		if err == nil && res != "" {
			return res, nil
		}
		lastErr = err
	}

	// If all configured providers fail or are exhausted, return offline mock response in dev
	return getMockTextResponse(userPrompt), lastErr
}

// GenerateVision processes images with primary vision provider and falls back.
func (o *Orchestrator) GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	if len(o.providers) == 0 {
		return getMockVisionResponse(userPrompt), nil
	}

	var lastErr error
	for _, p := range o.providers {
		res, err := p.GenerateVision(ctx, systemPrompt, userPrompt, imageBytes, mimeType)
		if err == nil && res != "" {
			return res, nil
		}
		lastErr = err
	}

	return getMockVisionResponse(userPrompt), lastErr
}

// GenerateStructured generates and unmarshals JSON responses with fallback.
func (o *Orchestrator) GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, target interface{}) error {
	if len(o.providers) == 0 {
		return populateMockStructured(userPrompt, target)
	}

	var lastErr error
	for _, p := range o.providers {
		err := p.GenerateStructured(ctx, systemPrompt, userPrompt, target)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if lastErr != nil {
		_ = populateMockStructured(userPrompt, target)
		return lastErr
	}

	return populateMockStructured(userPrompt, target)
}

func cleanJSONString(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}

// Fallback high-quality mock responses for offline development / zero keys
func getMockTextResponse(prompt string) string {
	return fmt.Sprintf("### 💡 Conceptual Explanation\n\nHere is the step-by-step resolution for your doubt:\n\n1. **Core Concept**: Understanding the fundamental relation in $%s$.\n2. **Step-by-Step Breakdown**: Apply standard principles to decouple dependent variables.\n3. **Exam Trap Alert**: Watch out for sign conventions and units!", prompt)
}

func getMockVisionResponse(prompt string) string {
	return "### 📷 Handwritten Problem Analysis\n\n**Identified Equation**: $F = m \\cdot a$ or system of forces.\n\n1. Draw the Free Body Diagram (FBD).\n2. Equate sum of forces $\\Sigma F_x = 0$ and $\\Sigma F_y = m a_y$.\n3. Solve for acceleration $a$.\n\n*Key Tip: Always verify normal force reactions before applying friction coefficient!*"
}

func populateMockStructured(prompt string, target interface{}) error {
	// Attempt generic mock json structure if possible
	emptyJSON := "{}"
	return json.Unmarshal([]byte(emptyJSON), target)
}
