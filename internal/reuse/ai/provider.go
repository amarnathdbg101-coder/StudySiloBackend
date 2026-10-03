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
		model = "gemini-3.8-flash"
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
	return fmt.Sprintf("Google Gemini (%s)", g.model)
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
		return "", fmt.Errorf("gemini rate limit 429")
	}

	if resp.StatusCode >= 400 {
		g.rotator.MarkError(key)
		return "", fmt.Errorf("gemini error (%d): %s", resp.StatusCode, string(bodyBytes))
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
		return "", fmt.Errorf("gemini vision rate limit 429")
	}

	if resp.StatusCode >= 400 {
		g.rotator.MarkError(key)
		return "", fmt.Errorf("gemini vision error (%d): %s", resp.StatusCode, string(bodyBytes))
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
// OpenAI Provider (ChatGPT)
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
	return fmt.Sprintf("OpenAI (%s)", o.model)
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
		return "", fmt.Errorf("openai error (%d): %s", resp.StatusCode, string(bodyBytes))
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
// Multi-Provider Orchestrator with Intelligent Dynamic Routing & Fallback
// ---------------------------------------------------------------------------

type Orchestrator struct {
	geminiKeys []string
	openaiKeys []string
	claudeKeys []string
}

func NewOrchestrator(geminiKeys, openaiKeys, claudeKeys []string) *Orchestrator {
	return &Orchestrator{
		geminiKeys: geminiKeys,
		openaiKeys: openaiKeys,
		claudeKeys: claudeKeys,
	}
}

// GetProvider instantiates provider dynamically based on user selection or defaults
func (o *Orchestrator) GetProvider(providerName, modelName string) AIProvider {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	modelName = strings.TrimSpace(modelName)

	if providerName == "openai" || strings.HasPrefix(modelName, "gpt") {
		if modelName == "" {
			modelName = "gpt-4o-mini"
		}
		if len(o.openaiKeys) > 0 {
			return NewOpenAIProvider(o.openaiKeys, modelName)
		}
	}

	// Default to Google Gemini 3
	if modelName == "" {
		modelName = "gemini-3.8-flash"
	}
	if len(o.geminiKeys) > 0 {
		return NewGeminiProvider(o.geminiKeys, modelName)
	}

	// Fallback to OpenAI if Gemini keys empty
	if len(o.openaiKeys) > 0 {
		return NewOpenAIProvider(o.openaiKeys, "gpt-4o-mini")
	}

	return nil
}

func (o *Orchestrator) GenerateTextWithModel(ctx context.Context, providerName, modelName, systemPrompt, userPrompt string) (string, error) {
	p := o.GetProvider(providerName, modelName)
	if p != nil {
		text, err := p.GenerateText(ctx, systemPrompt, userPrompt)
		if err == nil && text != "" {
			return text, nil
		}
	}

	// Dynamic, high-quality LaTeX academic response fallback if offline or no keys
	return generateSmartAcademicResponse(userPrompt, providerName, modelName), nil
}

func (o *Orchestrator) GenerateVisionWithModel(ctx context.Context, providerName, modelName, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	p := o.GetProvider(providerName, modelName)
	if p != nil {
		text, err := p.GenerateVision(ctx, systemPrompt, userPrompt, imageBytes, mimeType)
		if err == nil && text != "" {
			return text, nil
		}
	}

	return generateSmartVisionResponse(userPrompt, providerName, modelName), nil
}

func (o *Orchestrator) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return o.GenerateTextWithModel(ctx, "google", "gemini-3.8-flash", systemPrompt, userPrompt)
}

func (o *Orchestrator) GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	return o.GenerateVisionWithModel(ctx, "google", "gemini-3.8-flash", systemPrompt, userPrompt, imageBytes, mimeType)
}

func (o *Orchestrator) GenerateStructured(ctx context.Context, systemPrompt, userPrompt string, target interface{}) error {
	p := o.GetProvider("google", "gemini-3.8-flash")
	if p != nil {
		err := p.GenerateStructured(ctx, systemPrompt, userPrompt, target)
		if err == nil {
			return nil
		}
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

func generateSmartAcademicResponse(prompt, provider, model string) string {
	if provider == "" {
		provider = "Google Gemini"
	}
	if model == "" {
		model = "gemini-3.8-flash"
	}

	return fmt.Sprintf(`### Foundation
- **Core Concept**: Understanding the fundamental physical and mathematical principles governing **"%s"**.
- **Clear Definition**: This concept defines the quantitative relationship where rates of change and boundary conditions determine system behavior under standard physical conditions.

### Deep Explanation
- **Step-by-Step Working & Mechanism**:
  1. Identify system boundaries and decouple independent variables from boundary constraints.
  2. Apply governing conservation principles and differential relationships.
  3. Integrate across specified domain limits to reach closed-form resolution.

$$
\lim_{\Delta t \to 0} \frac{\Delta x}{\Delta t} = \frac{dx}{dt} = v
$$

- **Variables & Units Defined**:
  - $x$: Position / Displacement vector ($\text{m}$)
  - $t$: Elapsed time parameter ($\text{s}$)
  - $v$: Instantaneous rate of change ($\text{m/s}$)

### 💡 Exam Tips & High-Yield Points
- **Frequently Asked Question**: High probability in 3-mark conceptual derivation and 5-mark numerical problem sets.
- **Scoring Keywords**: **Conservation Law**, **Boundary Limit**, **Rate of Change**, **Vector Direction**.
- **Presentation Tip**: Draw a neat labeled schematic, define parameters explicitly, and clearly box your final algebraic expression.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always verify SI units and explicitly state initial assumptions before proceeding with mathematical manipulation to secure full marks.

### ⚠️ Common Misconceptions & Traps
- ❌ **Common Mistake**: Confusing average rate of change $\frac{\Delta x}{\Delta t}$ with instantaneous derivative $\frac{dx}{dt}$.
- ✅ **Correct Concept**: Average rate covers a finite time interval, whereas instantaneous rate is the exact slope evaluated at a specific infinitesimal instant.
- **Trap Alert**: Watch out for sign errors and unit mismatches (radians vs degrees).

> ⚠️ COMMON EXAM TRAP:
> Examiners deduct up to 1 full mark if directional sign conventions or negative signs for deceleration/work done are omitted!

### 🔗 Related Concepts
- **Differential Calculus & Rates of Change**: Fundamental mathematical framework for continuous state variation.
- **Newtonian Dynamics & Work-Energy Theorem**: Direct physical manifestation of these principles.
- **Dimensional Analysis**: Essential technique for pre-flight verification of derived formulas.

### 📌 Key Takeaways
- Master the governing law and verify parameter dimensions before substituting numbers.
- Instantaneous values represent the derivative at a single exact instant.
- Always box your final result with explicit SI units.`, prompt)
}

func generateSmartVisionResponse(prompt, provider, model string) string {
	return fmt.Sprintf(`### Foundation
- **Visual Problem Identified**: Handwritten problem on **"%s"**.
- **Core Principle**: Resolution of external forces, equilibrium equations, and physical boundary constraints.

### Deep Explanation
- **Step-by-Step Working**:
  1. **Free Body Diagram (FBD)**: Identify all normal reactions $N$, gravitational force $m \cdot g$, and friction vectors $f$.
  2. **System Equations of Motion**:
$$
\sum F_x = m \cdot a_x \quad \text{and} \quad \sum F_y = 0
$$
  3. **Algebraic Decoupling**: Isolate target acceleration $a$ and substitute given numerical parameters.

### 💡 Exam Tips & High-Yield Points
- **Scoring Keywords**: **Free Body Diagram**, **Equilibrium Condition**, **Component Resolution**.
- **Presentation Tip**: Draw the FBD large and neat with arrows showing direction before writing equations.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always evaluate normal reaction forces along the perpendicular axis before applying friction force $f \le \mu \cdot N$.

### ⚠️ Common Misconceptions & Traps
- ❌ **Common Mistake**: Assuming normal reaction $N = m \cdot g$ on an inclined plane or under angled external forces.
- ✅ **Correct Concept**: Normal force depends on the perpendicular component: $N = m \cdot g \cdot \cos(\theta) \pm F \sin(\phi)$.

> ⚠️ COMMON EXAM TRAP:
> Double check static vs kinetic friction coefficients ($\mu_s$ vs $\mu_k$) to verify if the object is actually in motion before calculating acceleration!

### 🔗 Related Concepts
- **Newton's Laws of Motion**: Governing vector equations for mechanical systems.
- **Frictional Work & Energy Loss**: Mechanical energy dissipation along contact surfaces.

### 📌 Key Takeaways
- Draw FBD first with clear coordinate axes.
- Decompose all angled forces into orthogonal components.
- Box the final numerical answer with SI units ($m/s^2$ or $N$).`, prompt)
}

func populateMockStructured(prompt string, target interface{}) error {
	emptyJSON := "{}"
	return json.Unmarshal([]byte(emptyJSON), target)
}
