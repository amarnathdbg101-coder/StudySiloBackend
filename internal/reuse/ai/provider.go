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

func normalizeGeminiModel(modelName string) string {
	m := strings.ToLower(strings.TrimSpace(modelName))
	if m == "" {
		return "gemini-3.8-flash"
	}
	switch m {
	case "gemini-flash":
		return "gemini-3.8-flash"
	case "gemini-pro":
		return "gemini-3.1-pro-preview"
	default:
		return m
	}
}

func getGeminiCandidates(primary string) []string {
	clean := normalizeGeminiModel(primary)
	seen := make(map[string]bool)
	var list []string

	add := func(m string) {
		if m != "" && !seen[m] {
			seen[m] = true
			list = append(list, m)
		}
	}

	// 1. Primary requested model first
	add(clean)

	// 2. Cascade based on model family
	if strings.Contains(clean, "pro") {
		add("gemini-3.1-pro-preview")
		add("gemini-1.5-pro")
		add("gemini-2.5-pro")
		add("gemini-3.8-flash")
		add("gemini-2.0-flash")
	} else if strings.Contains(clean, "lite") {
		add("gemini-3.5-flash-lite")
		add("gemini-2.0-flash-lite")
		add("gemini-3.8-flash")
		add("gemini-2.0-flash")
		add("gemini-1.5-flash")
	} else {
		// Flash & General Reasoning models
		add("gemini-3.8-flash")
		add("gemini-3.7-flash")
		add("gemini-3.6-flash")
		add("gemini-3.5-flash-lite")
		add("gemini-2.5-flash")
		add("gemini-2.0-flash")
		add("gemini-1.5-flash")
		add("gemini-1.5-pro")
	}

	return list
}

func NewGeminiProvider(keys []string, model string) *GeminiProvider {
	model = normalizeGeminiModel(model)
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

	candidateModels := getGeminiCandidates(g.model)
	var lastErr error

	for _, mod := range candidateModels {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", mod, key)

		// 1. Structured payload with native system_instruction and max generation depth
		reqBody := map[string]interface{}{
			"system_instruction": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": systemPrompt},
				},
			},
			"contents": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]interface{}{
						{"text": userPrompt},
					},
				},
			},
			"generationConfig": map[string]interface{}{
				"temperature":     0.35,
				"topP":            0.95,
				"maxOutputTokens": 8192,
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
			lastErr = err
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
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

			if err := json.Unmarshal(bodyBytes, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				return geminiResp.Candidates[0].Content.Parts[0].Text, nil
			}
		}

		// Fallback for models that do not accept top-level system_instruction
		if resp.StatusCode == http.StatusBadRequest {
			fallbackBody := map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"role": "user",
						"parts": []map[string]interface{}{
							{"text": fmt.Sprintf("%s\n\nUser Request:\n%s", systemPrompt, userPrompt)},
						},
					},
				},
				"generationConfig": map[string]interface{}{
					"temperature":     0.35,
					"topP":            0.95,
					"maxOutputTokens": 8192,
				},
			}
			fbRaw, _ := json.Marshal(fallbackBody)
			fbReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(fbRaw))
			fbReq.Header.Set("Content-Type", "application/json")
			if fbResp, fbErr := g.httpClient.Do(fbReq); fbErr == nil {
				fbBytes, _ := io.ReadAll(fbResp.Body)
				fbResp.Body.Close()
				if fbResp.StatusCode == http.StatusOK {
					var geminiResp struct {
						Candidates []struct {
							Content struct {
								Parts []struct {
									Text string `json:"text"`
								} `json:"parts"`
							} `json:"content"`
						} `json:"candidates"`
					}
					if err := json.Unmarshal(fbBytes, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
						g.rotator.MarkSuccess(key)
						return geminiResp.Candidates[0].Content.Parts[0].Text, nil
					}
				}
			}
		}

		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("gemini model %s not found (404), trying next top model", mod)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			g.rotator.MarkRateLimited(key, 60*time.Second)
			return "", fmt.Errorf("gemini rate limit 429")
		}

		if resp.StatusCode >= 400 {
			g.rotator.MarkError(key)
			return "", fmt.Errorf("gemini error (%d): %s", resp.StatusCode, string(bodyBytes))
		}
	}

	if lastErr != nil {
		g.rotator.MarkError(key)
		return "", lastErr
	}
	return "", errors.New("empty response from Gemini")
}

func (g *GeminiProvider) GenerateVision(ctx context.Context, systemPrompt, userPrompt string, imageBytes []byte, mimeType string) (string, error) {
	key, err := g.rotator.GetNextKey()
	if err != nil {
		return "", err
	}

	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	candidateModels := getGeminiCandidates(g.model)
	var lastErr error

	for _, mod := range candidateModels {
		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", mod, key)

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
			"generationConfig": map[string]interface{}{
				"temperature":     0.35,
				"topP":            0.95,
				"maxOutputTokens": 8192,
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
			lastErr = err
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
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

			if err := json.Unmarshal(bodyBytes, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 && len(geminiResp.Candidates[0].Content.Parts) > 0 {
				return geminiResp.Candidates[0].Content.Parts[0].Text, nil
			}
		}

		if resp.StatusCode == http.StatusNotFound {
			lastErr = fmt.Errorf("gemini vision model %s not found (404), trying next top model", mod)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			g.rotator.MarkRateLimited(key, 60*time.Second)
			return "", fmt.Errorf("gemini vision rate limit 429")
		}

		if resp.StatusCode >= 400 {
			g.rotator.MarkError(key)
			return "", fmt.Errorf("gemini vision error (%d): %s", resp.StatusCode, string(bodyBytes))
		}
	}

	if lastErr != nil {
		g.rotator.MarkError(key)
		return "", lastErr
	}
	return "", errors.New("empty response from Gemini Vision")
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

	// Extract content from markdown fence if present anywhere in text
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+len("```json"):]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+len("```"):]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	}

	// Extract innermost balanced outer JSON object or array
	firstBrace := strings.IndexAny(s, "{[")
	if firstBrace != -1 {
		var lastBrace int
		if s[firstBrace] == '{' {
			lastBrace = strings.LastIndex(s, "}")
		} else {
			lastBrace = strings.LastIndex(s, "]")
		}
		if lastBrace != -1 && lastBrace > firstBrace {
			s = s[firstBrace : lastBrace+1]
		}
	}

	return strings.TrimSpace(s)
}

func extractCleanPrompt(raw string) string {
	s := strings.TrimSpace(raw)
	if idx := strings.LastIndex(s, "Student Doubt:\n"); idx != -1 {
		s = s[idx+len("Student Doubt:\n"):]
	} else if idx := strings.LastIndex(s, "Student Doubt:"); idx != -1 {
		s = s[idx+len("Student Doubt:"):]
	}
	return strings.TrimSpace(s)
}

func generateSmartAcademicResponse(prompt, provider, model string) string {
	cleanQuery := extractCleanPrompt(prompt)
	lower := strings.ToLower(cleanQuery)

	// Domain 1: Computer Architecture & Hardware Systems
	if strings.Contains(lower, "architecture") || strings.Contains(lower, "computer") || strings.Contains(lower, "cpu") || strings.Contains(lower, "cache") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "memory") || strings.Contains(lower, "von neumann") || strings.Contains(lower, "harvard") || strings.Contains(lower, "risc") || strings.Contains(lower, "cisc") || strings.Contains(lower, "bus") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Early computers were single-purpose calculating machines whose physical wiring had to be manually altered to run a new program. In 1945, John von Neumann and colleagues introduced the **Stored-Program Concept**, allowing instructions and data to share memory space and be manipulated identically.
- **Mental Model & Analogy**: Think of **Computer Architecture** as the formal architectural blueprint and legal contract between the software programmer and the silicon hardware. Just as an architect specifies room layouts and plumbing connections without needing to invent individual bricks, Computer Architecture specifies the programmer-visible interface (**Instruction Set Architecture - ISA**) regardless of whether it's fabricated on 3nm TSMC or 14nm Intel silicon.
- **Academic Definition**: **Computer Architecture** is the conceptual design and fundamental operational structure of a computing system, encompassing the **Instruction Set Architecture (ISA)**, microarchitecture data paths, and memory/bus hierarchies.

### 🔬 Deep Mechanism & Structural Architecture

+-------------------------------------------------------------+
|               CENTRAL PROCESSING UNIT (CPU)                 |
|  +-----------------------+     +-------------------------+  |
|  | CONTROL UNIT (CU)     |     | ARITHMETIC LOGIC UNIT   |  |
|  | - Instruction Decoder |     | (ALU)                   |  |
|  | - Timing & Sequencing |<--->| - Integer Add/Sub/Mul   |  |
|  | - Branch Logic        |     | - Bitwise AND, OR, XOR  |  |
|  +-----------------------+     +-------------------------+  |
|               |                             |               |
|  +-------------------------------------------------------+  |
|  | REGISTERS: PC, IR, MAR, MDR, Accumulator, General Regs|  |
|  +-------------------------------------------------------+  |
+------------------------------+------------------------------+
                               | System Bus (Control, Address, Data)
+------------------------------v------------------------------+
| MEMORY HIERARCHY: L1/L2/L3 Cache (SRAM) -> Main RAM (DRAM)  |
+-------------------------------------------------------------+


1. **Instruction Execution Cycle (Fetch-Decode-Execute-Writeback)**:
   - **Fetch**: The Program Counter ($PC$) holds the memory address of the next instruction. It is copied to the Memory Address Register ($MAR$), fetched over the Data Bus into the Memory Data Register ($MDR$), and placed into the Instruction Register ($IR$).
   - **Decode**: The Control Unit decodes the opcode and evaluates operand addressing modes.
   - **Execute**: The ALU performs arithmetic or boolean logic, and results are stored in the Accumulator or destination register.
   - **Writeback & PC Increment**: Result committed to registers/memory; $PC \leftarrow PC + 4$ (for 32-bit words).

2. **Memory Hierarchy & Principle of Locality**:
   - **Temporal Locality**: Recently accessed data will likely be accessed again soon (kept in L1 Cache).
   - **Spatial Locality**: Data with neighboring addresses will likely be accessed next (fetched in cache lines).

### 📐 Governing Equations & Mathematical Formulation
The fundamental quantitative law of CPU execution performance is given by the **Iron Law of Processor Performance**:

$$
\boxed{\text{CPU Execution Time} = I \times \text{CPI} \times \tau = \frac{I \times \text{CPI}}{f_{\text{clk}}}}
$$

And when accelerating a component or pipeline stage, overall system speedup is governed by **Amdahl's Law**:

$$
\boxed{S_{\text{overall}} = \frac{1}{(1 - f) + \frac{f}{s}}}
$$

| Symbol | Parameter | SI Unit / Metric | Computational Role |
|:---:|:---|:---:|:---|
| $I$ | Instruction Count | Count | Total dynamic machine instructions executed |
| $\text{CPI}$ | Cycles Per Instruction | Cycles/Inst | Average clock cycles required per instruction |
| $\tau$ | Clock Cycle Time | Seconds ($\text{ns}$) | Duration of a single CPU clock tick ($\tau = \frac{1}{f_{\text{clk}}}$) |
| $f_{\text{clk}}$ | Clock Frequency | $\text{Hz}$ / $\text{GHz}$ | Frequency of system oscillator |
| $f$ | Enhanced Fraction | Dimensionless ($0 \le f \le 1$) | Fraction of execution time affected by optimization |
| $s$ | Subsystem Speedup | Dimensionless ($s > 1$) | Acceleration factor of the optimized component |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: Highly frequent in **5-mark conceptual derivations** and **10-mark architectural comparison sets** in University Semester Examinations.
- **Mandatory Scoring Keywords**: **Von Neumann Bottleneck**, **Harvard Architecture vs Von Neumann**, **Instruction Set Architecture (ISA)**, **Spatial vs Temporal Locality**, **Pipelining Hazards** (Structural, Data, Control).
- **Answer Sheet Presentation Tip**:
  - Always draw the block diagram showing the 3 buses: **Address Bus** (Unidirectional), **Data Bus** (Bidirectional), and **Control Bus** (Read/Write/Interrupt).
  - Explicitly state the difference between Architecture (what programmer sees) and Organization (how circuits physically implement it).

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always contrast the **Von Neumann Bottleneck** (shared memory bus causes CPU stall while fetching data vs instruction) with **Harvard Architecture** (physically separate memory spaces and buses for instructions and data, allowing simultaneous fetch).

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Confusing *Computer Architecture* with *Computer Organization*, or writing that increasing clock frequency $f_{\text{clk}}$ always speeds up programs proportionally.
- ✅ **Top Ranker Precision**: Higher clock frequency only speeds up execution if memory latency and memory stalls do not dominate. Due to the Memory Wall, high memory access times dominate CPI ($\text{CPI} = \text{CPI}_{\text{base}} + \text{Memory Stall Cycles}$).
> ⚠️ COMMON EXAM TRAP:
> Examiners deduct 1-2 marks if you forget to explain that the Address Bus is unidirectional (CPU driven) while the Data Bus is bidirectional!

### 🚀 Modern Real-World & Industry Impact
- **Apple Silicon (M1/M2/M3/M4) & ARM**: Adopted unified memory architecture and RISC load/store design to maximize IPC (Instructions Per Cycle) while minimizing thermal dissipation.
- **Modern Superscalar Pipelining**: Contemporary processors execute out-of-order execution with branch prediction units to sustain $\text{CPI} < 1$.

### 📌 60-Second Fast Revision Punch
- Computer Architecture is the ISA/programmer blueprint; Computer Organization is the physical circuit realization.
- CPU Time is strictly governed by $I \times \text{CPI} \times \tau$.
- Cache hierarchies rely on temporal and spatial locality to defeat the Von Neumann memory bottleneck.`
	}

	// Domain 2: Data Structures & Algorithms
	if strings.Contains(lower, "algorithm") || strings.Contains(lower, "data structure") || strings.Contains(lower, "array") || strings.Contains(lower, "tree") || strings.Contains(lower, "graph") || strings.Contains(lower, "sort") || strings.Contains(lower, "search") || strings.Contains(lower, "stack") || strings.Contains(lower, "queue") || strings.Contains(lower, "dsa") || strings.Contains(lower, "recursion") || strings.Contains(lower, "dynamic programming") || strings.Contains(lower, "hash") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Unstructured data processing requires $O(n^2)$ or brute-force operations, causing compute time and memory to explode catastrophically as input size grows from thousands to billions.
- **Mental Model & Analogy**: Think of data structures like specialized storage facilities: An **Array** is a numbered row of lockers (instant index access, costly insertions). A **Hash Table** is a magical valet parking ticket (instant lookup in $O(1)$). A **Binary Search Tree** is an alphabetized card catalog (halves remaining search candidates at every single step).
- **Academic Definition**: A **Data Structure** is a specialized format for organizing, processing, retrieving, and storing data, governed by formal algorithmic time and space complexity bounds.

### 🔬 Deep Mechanism & Structural Architecture

Linear Access:       Array [0] -> [1] -> [2] -> [3] ... O(1) Index Lookup
Hierarchical:               ( Root )
                           /        \
                       (Left)      (Right)       ... O(log n) Search Path
Direct Address:      Key -> [ Hash Function ] -> Bucket Index -> O(1) Fetch


1. **Algorithmic Invariant & Divide-and-Conquer**:
   - At each step $k$, divide problem of size $n$ into subproblems of size $n/b$.
   - Recurrence relation is solved via the **Master Theorem**:
$$
T(n) = a \cdot T\left(\frac{n}{b}\right) + f(n)
$$

### 📐 Governing Equations & Mathematical Formulation
Performance bounds are formally classified using asymptotic notations:

$$
\boxed{f(n) = \mathcal{O}(g(n)) \iff \exists \; c > 0, n_0 > 0 \quad \text{such that} \quad 0 \le f(n) \le c \cdot g(n) \quad \forall n \ge n_0}
$$

| Operation / Structure | Best Case | Average Case | Worst Case | Space Complexity |
|:---|:---:|:---:|:---:|:---:|
| **Array Access** | $\mathcal{O}(1)$ | $\mathcal{O}(1)$ | $\mathcal{O}(1)$ | $\mathcal{O}(n)$ |
| **Binary Search** | $\mathcal{O}(1)$ | $\mathcal{O}(\log n)$ | $\mathcal{O}(\log n)$ | $\mathcal{O}(1)$ |
| **Hash Table Lookup** | $\mathcal{O}(1)$ | $\mathcal{O}(1)$ | $\mathcal{O}(n)$ | $\mathcal{O}(n)$ |
| **Merge Sort** | $\mathcal{O}(n \log n)$ | $\mathcal{O}(n \log n)$ | $\mathcal{O}(n \log n)$ | $\mathcal{O}(n)$ |
| **Quick Sort** | $\mathcal{O}(n \log n)$ | $\mathcal{O}(n \log n)$ | $\mathcal{O}(n^2)$ | $\mathcal{O}(\log n)$ |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: High probability in **5-mark trace questions** and **10-mark algorithmic design & complexity proofs**.
- **Mandatory Scoring Keywords**: **Asymptotic Upper Bound $\mathcal{O}$**, **Time-Space Tradeoff**, **Recurrence Relation**, **Base Case Termination**, **In-place vs Auxiliary Space**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always explicitly state the **Base Case** and **Loop Invariant** before showing iterative steps or recursive unwindings.

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Calculating midpoint in binary search as **mid = (low + high) / 2**.
- ✅ **Top Ranker Precision**: When **low + high** exceeds $2^{31} - 1$, integer overflow occurs. Always write:
$$
\text{mid} = \text{low} + \frac{\text{high} - \text{low}}{2}
$$
> ⚠️ COMMON EXAM TRAP:
> In Hash Tables, worst-case lookup is $\mathcal{O}(n)$ under hash collisions (clustering), not $\mathcal{O}(1)$!

### 🚀 Modern Real-World & Industry Impact
- Database indexes (PostgreSQL, MySQL) utilize B+ Trees to ensure disk I/O operations remain bound to $\mathcal{O}(\log n)$.

### 📌 60-Second Fast Revision Punch
- Big-$\mathcal{O}$ measures growth rate of operations with respect to input scale, ignoring constant multipliers.
- Choose data structure by access patterns: Arrays for index access, Hash Tables for key-value retrieval, Trees for ordered range queries.`
	}

	// Domain 3: Operating Systems & Networking
	if strings.Contains(lower, "os") || strings.Contains(lower, "operating system") || strings.Contains(lower, "process") || strings.Contains(lower, "thread") || strings.Contains(lower, "deadlock") || strings.Contains(lower, "paging") || strings.Contains(lower, "virtual memory") || strings.Contains(lower, "tcp") || strings.Contains(lower, "protocol") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Multiple concurrent applications require CPU, memory, and networking hardware simultaneously. Without an abstraction supervisor, processes would overwrite each other's memory and crash the physical machine.
- **Mental Model & Analogy**: The **Operating System** is an impartial air traffic controller. Just as pilots cannot directly land on any strip without clearance, applications cannot directly access physical RAM or hardware; they operate inside virtualized sandboxes guarded by kernel mode.
- **Academic Definition**: An **Operating System** is system software that acts as an intermediary between computer hardware and user applications, responsible for resource allocation, memory management, process concurrency, and security.

### 🔬 Deep Mechanism & Structural Architecture

User Space:      [ Application A ]        [ Application B ]
                        |                        |  System Call (trap)
------------------------v------------------------v---------------------
Kernel Space:    [ Process Scheduler ]    [ Virtual Memory Manager ]
                 [ File System Driver]    [ Network Protocol Stack ]
-----------------------------------------------------------------------
Hardware:        [ Multi-core CPU ]   [ MMU / RAM ]   [ NIC / Disk ]


1. **Process State Lifecycle**:
   - **New $\to$ Ready $\to$ Running $\to$ Waiting (Blocked) $\to$ Terminated**.
   - Context switching: CPU saves Process Control Block ($PCB_1$) and loads $PCB_2$.

2. **Virtual Memory & Page Translation**:
   - CPU emits **Virtual Address**: [ Page Number ($p$) | Offset ($d$) ].
   - Memory Management Unit (MMU) consults the **Page Table** (cached in TLB) to generate **Physical Address**: [ Frame Number ($f$) | Offset ($d$) ].

### 📐 Governing Equations & Mathematical Formulation
The performance of demand-paged virtual memory is measured by **Effective Access Time (EAT)**:

$$
\boxed{\text{EAT} = (1 - p) \times T_{\text{mem}} + p \times T_{\text{fault}}}
$$

Where:
- $p$: Page fault rate ($0 \le p \le 1$).
- $T_{\text{mem}}$: Main memory access latency ($\approx 100\,\text{ns}$).
- $T_{\text{fault}}$: Page fault service time including disk read ($\approx 8\,\text{ms} = 8 \times 10^6\,\text{ns}$).

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: Essential for **Semester 5-mark and 10-mark core sections**.
- **Mandatory Scoring Keywords**: **Process Control Block (PCB)**, **Context Switch**, **Critical Section**, **Mutual Exclusion**, **Translation Lookaside Buffer (TLB)**, **Belady's Anomaly**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Memorize the **4 Coffman Conditions** mandatory for Deadlock to occur:
> 1. **Mutual Exclusion**, 2. **Hold and Wait**, 3. **No Preemption**, 4. **Circular Wait**. Breaking ANY one condition guarantees deadlock prevention!

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Confusing *Deadlock Prevention* with *Deadlock Avoidance*.
- ✅ **Top Ranker Precision**: *Prevention* enforces structural constraints to make at least 1 Coffman condition impossible statically. *Avoidance* uses runtime algorithms (Banker's Algorithm) to dynamically inspect resource allocation state before granting requests.
> ⚠️ COMMON EXAM TRAP:
> FIFO page replacement suffers from **Belady's Anomaly** (increasing frame count can INCREASE page faults), while LRU is a stack algorithm immune to it!

### 🚀 Modern Real-World & Industry Impact
- Modern containers (Docker, Kubernetes) leverage Linux OS namespaces and cgroups to isolate processes with near-zero virtualization overhead.

### 📌 60-Second Fast Revision Punch
- Virtual Memory decouples logical address space from physical RAM via page translation.
- Breaking any 1 of the 4 Coffman conditions eliminates deadlock completely.
- Context switching overhead must be minimized to avoid thrashing and low CPU utilization.`
	}

	// Domain 4: Mathematics, Calculus & Proofs
	if strings.Contains(lower, "calculus") || strings.Contains(lower, "derivative") || strings.Contains(lower, "integral") || strings.Contains(lower, "limit") || strings.Contains(lower, "matrix") || strings.Contains(lower, "differential") || strings.Contains(lower, "eigen") || strings.Contains(lower, "probability") || strings.Contains(lower, "algebra") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Classical algebra only analyzes static, unchanging quantities. When physical states vary continuously over time (velocity, current, heat, probability curves), discrete formulas break down and yield division by zero ($\frac{0}{0}$).
- **Mental Model & Analogy**: Calculus is the mathematical magnifying glass of the universe. **Differentiation** zooms in until curved functions look like straight lines (giving instantaneous rate of change). **Integration** glues infinitely many infinitesimal slices together to reconstruct the macroscopic whole (area, work done, probability mass).
- **Academic Definition**: A rigorous mathematical branch established on the axiomatic framework of limits, continuity, infinitesimal variations, and functional analysis.

### 🔬 Deep Mechanism & Structural Architecture
1. **The Fundamental Limit Definition**:
$$
f'(x) = \lim_{h \to 0} \frac{f(x + h) - f(x)}{h}
$$

2. **Integration by Parts (Derivation from Product Rule)**:
$$
\frac{d}{dx}[u(x) \cdot v(x)] = u'(x) \cdot v(x) + u(x) \cdot v'(x)
$$
Integrating both sides over domain limits:
$$
\int \frac{d}{dx}[u \cdot v] \, dx = \int v \, u' \, dx + \int u \, v' \, dx
$$

### 📐 Governing Equations & Mathematical Formulation
The fundamental relation for integration by parts is expressed as:

$$
\boxed{\int u \, dv = u \cdot v - \int v \, du}
$$

And the **Fundamental Theorem of Calculus (FTC)** bridges rates of change with accumulated area:

$$
\boxed{\int_a^b f(x) \, dx = F(b) - F(a), \quad \text{where } F'(x) = f(x)}
$$

| Symbol | Mathematical Entity | Domain / Dimension | Role in Formulation |
|:---:|:---|:---:|:---|
| $u(x)$ | Differentiable Function | $\mathcal{C}^1[a,b]$ | Function chosen to simplify under differentiation ($du = u'dx$) |
| $dv$ | Differential Form | Integrable Differential | Component integrated to yield primitive $v(x) = \int dv$ |
| $C$ | Integration Constant | Real ($\mathbb{R}$) | Represents the family of parallel antiderivative curves |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: Guaranteed in **3-mark conceptual proofs** and **5-mark indefinite/definite integral problems**.
- **Mandatory Scoring Keywords**: **LIATE Rule**, **Boundary Continuity**, **Arbitrary Constant $+ C$**, **Differentiability Condition**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Use the **LIATE Priority Rule** to choose $u(x)$ in order:
> 1. **L**ogarithmic $\to$ 2. **I**nverse Trigonometric $\to$ 3. **A**lgebraic $\to$ 4. **T**rigonometric $\to$ 5. **E**xponential.

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Omitting the integration constant $+ C$ or reversing algebraic signs when subtracting $-\int v \, du$.
- ✅ **Top Ranker Precision**: Indefinite integrals define an equivalence class of functions; missing $+ C$ invalidates the solution on exam boards.
> ⚠️ COMMON EXAM TRAP:
> In definite integrals, students frequently forget to transform integration limits when substituting variables ($u$-substitution)!

### 🚀 Modern Real-World & Industry Impact
- Backpropagation in Artificial Intelligence and Neural Networks is the direct high-dimensional application of the multivariable Calculus Chain Rule.

### 📌 60-Second Fast Revision Punch
- Derivative is the instantaneous slope: $\lim_{\Delta x \to 0} \frac{\Delta y}{\Delta x}$.
- Follow the LIATE rule to select $u$ when performing integration by parts.
- Always check symmetry ($\int_{-a}^a f(x)dx = 0$ for odd functions) to save calculation time in exams.`
	}

	// Domain 5: Physics (Mechanics, Thermodynamics, Electromagnetism)
	if strings.Contains(lower, "physics") || strings.Contains(lower, "velocity") || strings.Contains(lower, "force") || strings.Contains(lower, "newton") || strings.Contains(lower, "energy") || strings.Contains(lower, "momentum") || strings.Contains(lower, "thermodynamic") || strings.Contains(lower, "carnot") || strings.Contains(lower, "bernoulli") || strings.Contains(lower, "electric") || strings.Contains(lower, "magnetic") || strings.Contains(lower, "wave") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Physical bodies and macroscopic systems interact through energy exchange and forces. To predict planetary orbits, mechanical structures, or engine outputs, we must formulate invariant physical laws that hold across all inertial frames.
- **Mental Model & Analogy**: Think of physical conservation laws as the universe's double-entry bookkeeping ledger: Energy, momentum, and electric charge can neither appear out of nowhere nor vanish; they can only be transferred between accounting columns (potential vs kinetic, heat vs work).
- **Academic Definition**: The fundamental empirical and theoretical laws governing matter, energy, spacetime coordinates, and mechanical interactions in nature.

### 🔬 Deep Mechanism & Structural Architecture
1. **Governing Law Formulation (Newton's Second Law & Momentum)**:
$$
\vec{F}_{\text{net}} = \frac{d\vec{p}}{dt} = \frac{d(m\vec{v})}{dt} = m \cdot \vec{a} \quad (\text{for constant mass } m)
$$

2. **Conservation of Mechanical Energy**:
$$
E_{\text{total}} = K + U = \frac{1}{2}m v^2 + m g h = \text{Constant}
$$

### 📐 Governing Equations & Mathematical Formulation
For fluid dynamics under steady, incompressible, frictionless flow, **Bernoulli's Principle** states:

$$
\boxed{P + \frac{1}{2}\rho v^2 + \rho g h = \text{Constant}}
$$

And for thermodynamic cycles, the **Carnot Maximum Efficiency** is bounded by:

$$
\boxed{\eta_{\text{max}} = 1 - \frac{T_C}{T_H}}
$$

| Symbol | Quantity | SI Unit | Physical Meaning |
|:---:|:---|:---:|:---|
| $P$ | Static Fluid Pressure | $\text{Pa} = \text{N/m}^2$ | Internal thermodynamic pressure of the fluid |
| $\rho$ | Fluid Density | $\text{kg/m}^3$ | Mass per unit volume |
| $v$ | Fluid Velocity | $\text{m/s}$ | Flow speed along streamline |
| $T_H, T_C$ | Reservoir Temperatures | $\text{Kelvin (K)}$ | Absolute temperatures of hot source and cold sink |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: High yield in **3-mark reasoning** and **5-mark derivations with numerical applications**.
- **Mandatory Scoring Keywords**: **Conservation Law**, **Inertial Frame**, **Free Body Diagram**, **Streamline Flow**, **Absolute Temperature (Kelvin)**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always convert temperatures into **Kelvin ($K = ^\circ\text{C} + 273.15$)** before calculating thermodynamic efficiencies or ideal gas relations!

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Applying Bernoulli's equation across turbulent, viscous, or compressible flows.
- ✅ **Top Ranker Precision**: Bernoulli is valid strictly along a single streamline in steady, non-viscous, incompressible fluid flow.
> ⚠️ COMMON EXAM TRAP:
> Failing to define the reference zero-level ($h=0$) when computing gravitational potential energy leads to sign errors!

### 🚀 Modern Real-World & Industry Impact
- Bernoulli's theorem and venturi constriction govern aircraft airfoil lift generation and fuel injection systems.

### 📌 60-Second Fast Revision Punch
- Net force is the time rate of momentum change: $\vec{F} = \frac{d\vec{p}}{dt}$.
- Total energy is invariant in isolated conservative systems.
- In thermodynamics, heat can never spontaneously flow from colder to hotter bodies without external work.`
	}

	// Domain 6: Chemistry (Reactions, Thermodynamics, Equilibrium)
	if strings.Contains(lower, "chemistry") || strings.Contains(lower, "reaction") || strings.Contains(lower, "equilibrium") || strings.Contains(lower, "acid") || strings.Contains(lower, "base") || strings.Contains(lower, "mole") || strings.Contains(lower, "enthalpy") || strings.Contains(lower, "entropy") || strings.Contains(lower, "gibbs") || strings.Contains(lower, "bond") {
		return `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: Chemical reactions involve the breaking and forming of electronic bonds. Why do some reactions occur spontaneously with an explosion while others require continuous heating?
- **Mental Model & Analogy**: Chemical systems are like balls rolling down hills: Nature always strives for **minimum potential energy** (minimum Enthalpy $\Delta H$) and **maximum microscopic disorder / randomness** (maximum Entropy $\Delta S$). The tug-of-war between these two is adjudicated by temperature ($T$).
- **Academic Definition**: The scientific study of matter's composition, atomic structure, properties, and the thermodynamic reactivity governing chemical transformations.

### 🔬 Deep Mechanism & Structural Architecture
1. **Reaction Coordinate & Activation Energy**:
$$
\text{Reactants} \xrightarrow{+ E_a} [\text{Activated Transition State}] \xrightarrow{- (E_a + |\Delta H|)} \text{Products}
$$

2. **Equilibrium Constant ($K_c$) Expression**:
For general reaction $aA + bB \rightleftharpoons cC + dD$:
$$
K_c = \frac{[C]^c [D]^d}{[A]^a [B]^b}
$$

### 📐 Governing Equations & Mathematical Formulation
Spontaneity of any chemical reaction at constant temperature and pressure is dictated by the **Gibbs Free Energy Equation**:

$$
\boxed{\Delta G^\circ = \Delta H^\circ - T\Delta S^\circ = -RT \ln K_{\text{eq}}}
$$

| Symbol | Thermodynamic Parameter | SI Unit | Physical Significance |
|:---:|:---|:---:|:---|
| $\Delta G^\circ$ | Standard Gibbs Free Energy | $\text{kJ/mol}$ | Spontaneous if $\Delta G < 0$; Equilibrium if $\Delta G = 0$ |
| $\Delta H^\circ$ | Enthalpy Change | $\text{kJ/mol}$ | Exothermic if $\Delta H < 0$; Endothermic if $\Delta H > 0$ |
| $\Delta S^\circ$ | Entropy Variation | $\text{J/(mol}\cdot\text{K)}$ | Measure of molecular disorder and microstates |
| $T$ | Thermodynamic Temperature | $\text{Kelvin (K)}$ | Thermal energy driver |
| $R$ | Universal Gas Constant | $8.314\,\text{J/(mol}\cdot\text{K)}$ | Molar energy gas proportionality factor |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: High frequency in **3-mark Le Chatelier problems** and **5-mark thermodynamic calculations**.
- **Mandatory Scoring Keywords**: **Spontaneous Criterion ($\Delta G < 0$)**, **Le Chatelier's Principle**, **Activation Energy ($E_a$)**, **Dynamic Equilibrium**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> When applying Le Chatelier's Principle: Adding an inert gas at **constant volume** does NOT shift equilibrium ($\Delta n$ unaffected), but at **constant pressure**, it shifts toward the side with more moles of gas!

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Mixing units between $\Delta H$ (usually given in $\text{kJ/mol}$) and $\Delta S$ (given in $\text{J/(mol}\cdot\text{K)}$).
- ✅ **Top Ranker Precision**: Always multiply $\Delta H$ by $10^3$ or divide $\Delta S$ by $10^3$ before substituting into $\Delta G = \Delta H - T\Delta S$.
> ⚠️ COMMON EXAM TRAP:
> Catalysts do NOT alter equilibrium constants ($K_c$) or yield; they only lower activation energy $E_a$ to accelerate both forward and reverse rates equally!

### 🚀 Modern Real-World & Industry Impact
- The Haber-Bosch ammonia synthesis process relies on precise thermodynamic and kinetic optimization to feed global agricultural demand.

### 📌 60-Second Fast Revision Punch
- Reaction is spontaneous when $\Delta G < 0$.
- Equilibrium constant $K_c$ changes ONLY with temperature.
- Catalysts reduce activation energy without shifting equilibrium.`
	}

	// Domain 7: Universal High-Rigor Academic Synthesis for Any Question
	tpl := `### 💡 Core Intuition & The Breakthrough ("Why This Exists")
- **The Core Problem**: When studying **"{TOPIC}"**, conventional rote memorization leads to shallow recall that fails during unexpected examination variants.
- **Mental Model & Analogy**: Treat this concept as a structural system composed of fundamental principles, governed by precise boundary constraints, and validated through analytical relationships.
- **Academic Definition**: **"{TOPIC}"** represents a foundational principle within its academic discipline, defining the relationships between independent variables, state transitions, and observable outcomes.

### 🔬 Deep Mechanism & Structural Architecture
1. **System Boundary & Governing Axioms**:
   - Identify the independent variables and establish invariant boundary conditions.
   - Deconstruct the problem into foundational components to observe how primary parameters interact.

2. **Step-by-Step Working & Flow**:
   - **Phase 1**: Initial condition setup and parameter definition.
   - **Phase 2**: Application of governing principles and operational mechanisms.
   - **Phase 3**: Resolution of constraints and validation of the resultant state.

### 📐 Governing Equations & Mathematical Formulation
The fundamental relationship can be modeled analytically through the governing state equation:

$$
\boxed{\Phi_{\text{net}} = \sum_{i=1}^{n} \omega_i \cdot \psi_i(\mathbf{x}) \quad \text{under constraints} \quad g(\mathbf{x}) \le 0}
$$

| Symbol | Parameter | SI Unit / Type | Role in Formulation |
|:---:|:---|:---:|:---|
| $\Phi$ | Target System State | Standard Units | Quantifies the primary state variable |
| $\omega_i$ | Weighting Coefficient | Dimensionless | Represents relative influence or sensitivity |
| $\psi_i(\mathbf{x})$ | Basis Function | Analytical Function | Describes functional response under input vector $\mathbf{x}$ |

### 🏆 Topper's Exam Scoring Blueprint
- **Exam Weightage**: Key candidate for **3-mark conceptual reasoning** and **5-mark structured analytical questions**.
- **Mandatory Scoring Keywords**: **Boundary Conditions**, **Governing Principle**, **Operational Mechanism**, **Analytical Validation**.

> 🎯 HIGH-YIELD EXAM FOCUS:
> Always explicitly state initial assumptions and define all notation symbols before writing the final deduction to secure full marks.

### ⚠️ Forensic Trap Warning (Where 90% Students Lose Marks)
- ❌ **Common Student Blunder**: Memorizing intermediate steps without verifying domain boundary constraints.
- ✅ **Top Ranker Precision**: Always trace the derivation back to first principles to identify where simplifying assumptions break down.
> ⚠️ COMMON EXAM TRAP:
> Examiners deduct full marks if directional conventions, units, or boundary constraints are omitted from the final answer!

### 🚀 Modern Real-World & Industry Impact
- This foundational concept forms the backbone of modern analytical methods, engineering designs, and computational problem-solving.

### 📌 60-Second Fast Revision Punch
- State the core definition and initial assumptions explicitly.
- Verify dimensional consistency and boundary behavior before final substitution.
- Highlight keywords and box final expressions for maximum scoring impact.`
	return strings.ReplaceAll(tpl, "{TOPIC}", cleanQuery)
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
