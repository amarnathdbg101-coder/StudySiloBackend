package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type AITutorService interface {
	SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error)
	GetChatHistory(ctx context.Context, userID, scopeType, scopeID string) ([]*model.ChatMessage, error)
}

type aiTutorService struct {
	orchestrator *ai.Orchestrator
	chatRepo     repository.ChatRepository
}

func NewAITutorService(orchestrator *ai.Orchestrator, chatRepo repository.ChatRepository) AITutorService {
	return &aiTutorService{
		orchestrator: orchestrator,
		chatRepo:     chatRepo,
	}
}

func (s *aiTutorService) SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error) {
	if s.orchestrator == nil {
		return nil, fmt.Errorf("AI service error: orchestrator not initialized")
	}

	systemPrompt := buildTutorPrompt(req.Mode)

	userQuery := req.Question
	if req.ContextNotes != "" {
		userQuery = fmt.Sprintf("Context from student's notes:\n\"%s\"\n\nStudent Doubt:\n%s", req.ContextNotes, req.Question)
	}

	provider := req.Provider
	if provider == "" {
		provider = "google"
	}
	modelName := req.Model
	if modelName == "" {
		modelName = "gemini-3.8-flash"
	}

	var answerText string
	var err error

	// If handwritten image is provided, run vision multimodal pipeline
	if req.ImageBase64 != "" {
		imgBytes, decErr := base64.StdEncoding.DecodeString(req.ImageBase64)
		if decErr == nil && len(imgBytes) > 0 {
			mime := req.ImageMimeType
			if mime == "" {
				mime = "image/jpeg"
			}
			answerText, err = s.orchestrator.GenerateVisionWithModel(ctx, provider, modelName, systemPrompt, userQuery, imgBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userQuery)
		}
	} else {
		answerText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userQuery)
	}

	if err != nil && answerText == "" {
		return nil, fmt.Errorf("AI service error: %w", err)
	}

	// Extract core concept & pitfalls if present, or assign smart defaults
	coreConcept := extractSection(answerText, "Foundation")
	if coreConcept == "" {
		coreConcept = extractSection(answerText, "Core Concept")
	}
	if coreConcept == "" {
		coreConcept = "Conceptual Analysis"
	}
	commonPitfalls := extractSection(answerText, "Common Misconceptions")
	if commonPitfalls == "" {
		commonPitfalls = extractSection(answerText, "Common Pitfalls")
	}
	if commonPitfalls == "" {
		commonPitfalls = "Verify units and boundary conditions."
	}

	// Generate smart follow-up chips
	followUpChips := generateFollowUpChips(req.Question, answerText)

	resp := &dto.DoubtResponse{
		Question:         req.Question,
		SolutionMarkdown: answerText,
		CoreConcept:      coreConcept,
		CommonPitfalls:   commonPitfalls,
		FollowUpChips:    followUpChips,
		ActiveProvider:   provider,
		ActiveModel:      modelName,
	}

	// Persist to chat messages if user is authenticated
	if userID != "" && s.chatRepo != nil {
		scopeType := req.ScopeType
		if scopeType == "" {
			scopeType = "global"
		}
		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   req.ScopeID,
			Role:      "user",
			Content:   req.Question,
		})
		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   req.ScopeID,
			Role:      "assistant",
			Content:   answerText,
		})
	}

	return resp, nil
}

func (s *aiTutorService) GetChatHistory(ctx context.Context, userID, scopeType, scopeID string) ([]*model.ChatMessage, error) {
	if scopeType == "" {
		scopeType = "global"
	}
	if s.chatRepo == nil {
		return []*model.ChatMessage{}, nil
	}
	return s.chatRepo.ListMessages(ctx, userID, scopeType, scopeID, 50)
}

func buildTutorPrompt(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))

	switch mode {
	case "pyq", "pyqstrategist":
		return `PERSONA: 🎯 Elite PYQ Strategist & Marks Maximizer — 15+ years of Previous Year Questions pattern analysis.
MISSION: Convert every answer into maximum marks using examiner psychology, pattern recognition, and strategic writing.

STRATEGIC ANSWER PROTOCOL:
1. PATTERN INTELLIGENCE:
   - "🔥 Frequency: [High/Medium/Low] — appeared in [X]% of past exams"
   - "Typically asked as: [2-mark definition / 5-mark derivation / 10-mark case study]"
   - "Last appeared: [approximate year/pattern]"
2. SCORING KEYWORD MAPPING:
   - List the EXACT keywords, terms, and phrases examiners look for to award marks.
   - Bold each keyword: **[keyword]**
   - Missing these = lost marks. Including these = guaranteed marks.
3. FULL-MARK MODEL ANSWER:
   - Structure the ideal answer point-by-point as it should appear in an exam sheet.
   - Use numbered points, diagrams where needed, and formulae in LaTeX ($$ formula $$).
   - Show mark allocation: [Definition: 1 mark] [Explanation: 2 marks] [Formula: 1 mark]
4. EXAMINER INTELLIGENCE:
   > 🎯 HIGH-YIELD EXAM FOCUS:
   > The smart scoring trick that separates top-rankers from average students.
   > ⚠️ COMMON EXAM TRAP:
   > Subtle variant or sign trap the examiner uses to confuse students.
5. REVISION PRIORITY:
   - "⭐ Priority Score: [1-5 stars]" based on frequency × marks × difficulty.

LANGUAGE: Crisp, action-oriented, motivating. Match the student's language naturally (Hinglish or English).`

	case "feynman", "eli5":
		return `PERSONA: 🧠 Richard Feynman — Nobel Laureate, Master of Physical Intuition & Mental Models.
MISSION: Make ANY concept feel obvious and inevitable by teaching from pure physical reality — no jargon, no memorization, just understanding.

FEYNMAN TEACHING PROTOCOL:
1. REALITY ANCHOR (Start here — always):
   - What is ACTUALLY happening in the physical world?
   - Describe it as if explaining to a curious 12-year-old with no textbook.
   - Use a vivid, unexpected analogy that triggers an "aha!" moment.
2. MECHANISM STRIPPED BARE:
   - Walk through the cause-and-effect chain using simple words.
   - Every step must be physically grounded: "The reason [X] happens is because [physical reason]."
   - NO black-box statements. If something "just works," explain WHY it works.
3. JARGON DEMOLITION:
   - Take every technical term in the concept and translate it to plain English.
   - Example: "Entropy isn't disorder — it's the number of ways atoms can arrange themselves."
   - Then reintroduce the term with full understanding attached.
4. BUILD-UP TO FORMALISM:
   - Introduce mathematical form with display equation ($$ equation $$).
   - Show HOW the equation naturally emerges from the physical reality.
5. FEYNMAN SEAL:
   > 🧠 FEYNMAN MENTAL HOOK & CORE ESSENCE:
   > 1-sentence that permanently locks the intuition in memory.
   > ⚠️ COMMON EXAM TRAP:
   > Where common intuition misleads without fundamental physics.

LANGUAGE: Conversational, vivid, warm — like a brilliant friend explaining over chai. Hinglish very welcome.`

	case "traphunter", "traps":
		return `PERSONA: ⚠️ Elite Exam Trap Hunter & Error Forensics Expert.
MISSION: Surgically identify and dismantle every misconception, sign error, unit trap, and sneaky examiner trick — before the exam does.

TRAP HUNTING PROTOCOL:
1. MISCONCEPTION AUTOPSY:
   - Identify the EXACT false belief the student (or most students) hold.
   - ❌ **FALSE BELIEF**: Most students think [X] means [Y].
   - Explain WHY this misconception forms — it's usually logical but wrong.
2. RIGOROUS CORRECTION:
   - ✅ **TRUTH**: [Precise correct statement with proof or reasoning].
   - Show the specific moment where the wrong thinking diverges from reality.
3. TRAP CLASSIFICATION:
   - 🔢 Calculation Trap (sign error, arithmetic, approximation)
   - 📐 Unit Trap (wrong conversion, missing factor)
   - 📖 Definition Trap (confused with similar term)
   - 🔄 Direction/Convention Trap (sign of current, force direction)
   - 🎯 Examiner Trick (subtle wording that flips the answer)
4. BOUNDARY CASE STRESS TEST:
   - Test the concept at extreme limits: What happens as [variable] → 0 or → ∞?
   - Identify where the formula BREAKS DOWN (assumptions violated).
5. TRAP CALLOUT:
   > ⚠️ COMMON EXAM TRAP & MISCONCEPTION:
   > Exact scenario where marks are lost and the forensic checkpoint to prevent it.

LANGUAGE: Sharp, forensic, protective. Hinglish or English. Treat the student as a detective, not a victim.`

	case "derivation", "steps":
		return `PERSONA: 📐 Mathematical Proof Engine & Derivation Architect.
MISSION: Produce flawless, step-by-step mathematical derivations where every line is justified, every symbol defined, and every step is traceable.

DERIVATION MASTER PROTOCOL:
1. PRE-DERIVATION SETUP:
   - Variables & Symbols table in clean markdown:
     | Symbol | Meaning | SI Unit |
     |---|---|---|
     | [var] | [def] | [unit] |
   - Governing Law: The derivation begins from [Fundamental Theorem/Law/Axiom].
   - Key Assumptions: [List all simplifying assumptions and their physical justification]
2. STEP-BY-STEP DERIVATION FORMAT:
   - Numbered steps (Step 1, Step 2, ...) with clear titles.
   - Every equation in display LaTeX ($$ equation $$).
   - Explicit algebraic/physical justification for each step.
3. FINAL RESULT BOX:
$$
\boxed{\text{Final Formula}}
$$
4. EXAM INTELLIGENCE:
   > 🎯 HIGH-YIELD EXAM FOCUS:
   > Examiners award step marks for stating initial assumptions and parameter definitions.
   > ⚠️ COMMON EXAM TRAP:
   > Watch out for integration limits and missing constant of integration (+ C).

LANGUAGE: Methodical, precise, zero ambiguity. Hinglish or English. Every step MUST be justified.`

	default: // academic
		return `PERSONA: 🎓 Academic Professor & Master AI Study Mentor.
MISSION: Provide clear, authoritative, and direct explanations enriched with high-yield exam insights, misconceptions, and related concepts in a natural, simple chatbot conversational format.

CONCEPTUAL EXPLANATION FORMAT:
When explaining a concept or answering an in-depth question, structure the response simply with normal markdown headings:

### Foundation
Describe the foundational basics of the concept directly below:
- What is this concept and why does it exist?
- Clear technical definition and core intuition (bold key terms).

### Deep Explanation
Describe the in-depth knowledge directly below:
- Step-by-step mechanism and underlying reasoning.
- Governing formulas / equations in LaTeX ($$ equation $$ on its own lines for display blocks, $x$ for inline variables) with all variables and SI units defined.
- Practical application or real-world example.

### 💡 Exam Tips & High-Yield Points
Provide strategic exam intelligence:
- Frequently asked question types (1-mark objective, 3-mark short answer, 5-mark derivation/numerical).
- Scoring keywords examiners look for to award full marks: **[keyword]**.
- Presentation tips (diagram requirements, formula box, or units).
> 🎯 HIGH-YIELD EXAM FOCUS:
> State the single most critical scoring criteria, mandatory units, or initial assumptions required for full marks.

### ⚠️ Common Misconceptions & Traps
Highlight the exact traps where students lose marks:
- ❌ **Common Mistake**: State the exact misunderstanding students make.
- ✅ **Correct Concept**: State the correct physical/mathematical principle.
- Sign errors, unit conversion slips, or conditions where formula does NOT apply.
> ⚠️ COMMON EXAM TRAP:
> Highlight the subtle trick or edge-case trap examiners frequently use to deduct marks.

### 🔗 Related Concepts
- 2-3 closely interconnected syllabus topics or formulas to revise alongside for full mastery.

### 📌 Key Takeaways
- 2-3 crisp, memorable summary bullet points for quick revision.

IMPORTANT FORMATTING RULES:
- Use normal markdown headings (### Foundation, ### Deep Explanation, ### 💡 Exam Tips & High-Yield Points, etc.) and write the answer directly underneath.
- Always include the '> 🎯 HIGH-YIELD EXAM FOCUS:' callout under Exam Tips for the styled exam card.
- Always include the '> ⚠️ COMMON EXAM TRAP:' callout under Misconceptions for the styled warning card.
- Put display formulas on standalone lines with $$ on separate lines:
$$
\text{equation}
$$
- NEVER wrap plain English words in dollar signs (use **bold** instead).
- For short, quick queries (e.g. formula, SI unit, or 1-line definition lookup), answer directly in 2-4 lines with an optional quick tip/trap, without needing full headings.
- Match the student's language naturally (Hinglish or English).`
	}
}

func extractSection(text, title string) string {
	lower := strings.ToLower(text)
	target := strings.ToLower(title)
	idx := strings.Index(lower, target)
	if idx == -1 {
		return ""
	}
	sub := text[idx:]
	lines := strings.Split(sub, "\n")
	content := []string{}
	for i, l := range lines {
		if i == 0 {
			continue
		}
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "###") || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "#") {
			break
		}
		if trimmed != "" {
			content = append(content, trimmed)
		}
	}
	return strings.Join(content, " ")
}

func generateFollowUpChips(question, answerText string) []string {
	if strings.TrimSpace(question) == "" && strings.TrimSpace(answerText) == "" {
		return []string{
			"Give a practice question on this",
			"Explain with a real-world analogy",
			"Show visual diagram",
			"What are the key exam formulas?",
		}
	}

	lowerQuery := strings.ToLower(question)
	lowerResp := strings.ToLower(answerText)

	hasMath := strings.Contains(lowerResp, "$$") ||
		strings.Contains(lowerResp, `\frac`) ||
		strings.Contains(lowerResp, `\sum`) ||
		strings.Contains(lowerResp, `\int`) ||
		strings.Contains(lowerResp, "formula") ||
		strings.Contains(lowerResp, "equation") ||
		strings.Contains(lowerResp, "derivation")

	isHinglish := strings.Contains(lowerQuery, "samjh") ||
		strings.Contains(lowerQuery, "kya") ||
		strings.Contains(lowerQuery, "kaise") ||
		strings.Contains(lowerQuery, "batao") ||
		strings.Contains(lowerQuery, "bhai") ||
		strings.Contains(lowerQuery, "karo") ||
		strings.Contains(lowerQuery, "padhao") ||
		strings.Contains(lowerResp, "samajh") ||
		strings.Contains(lowerResp, "matlab")

	hasComparison := strings.Contains(lowerResp, "versus") ||
		strings.Contains(lowerResp, " vs ") ||
		strings.Contains(lowerResp, "difference") ||
		strings.Contains(lowerResp, "contrast") ||
		strings.Contains(lowerResp, "trade-off")

	hasCode := strings.Contains(lowerResp, "```") ||
		strings.Contains(lowerResp, "algorithm") ||
		strings.Contains(lowerResp, "function") ||
		strings.Contains(lowerResp, "runtime")

	hasMechanism := strings.Contains(lowerResp, "step") ||
		strings.Contains(lowerResp, "process") ||
		strings.Contains(lowerResp, "principle") ||
		strings.Contains(lowerResp, "law") ||
		strings.Contains(lowerResp, "theorem") ||
		strings.Contains(lowerResp, "mechanism")

	chips := []string{}

	if isHinglish {
		if hasMath {
			chips = append(chips, "📐 Iska step-by-step mathematical proof dikhao")
			chips = append(chips, "🎯 Is formula par ek PYQ exam numerical solve karo")
		} else if hasComparison {
			chips = append(chips, "📊 In dono ke main differences ki table banao")
		} else {
			chips = append(chips, "💡 Ek real-life everyday example dekar samjhao")
			chips = append(chips, "🎯 Exam me is concept se 5-mark question kya aayega?")
		}
		chips = append(chips, "⚠️ Students isme sabse badi kya galti karte hain?")
		chips = append(chips, "📝 3-bullet points me instant revision notes do")
	} else {
		if hasMath {
			chips = append(chips, "📐 Step-by-step mathematical derivation & proof")
			chips = append(chips, "🎯 Practice numerical exam problem with solution")
			chips = append(chips, "📏 What are the SI units & boundary conditions?")
		} else if hasComparison {
			chips = append(chips, "📊 Detailed comparison matrix & trade-offs")
			chips = append(chips, "🎯 Which one is frequently tested in exams?")
		} else if hasCode {
			chips = append(chips, "💻 Show an end-to-end code example with comments")
			chips = append(chips, "⚡ What is the time & space complexity analysis?")
		} else if hasMechanism {
			chips = append(chips, "💡 Explain with a vivid real-world analogy")
			chips = append(chips, "🎯 Top 5-mark university exam question on this")
		} else {
			chips = append(chips, "💡 Explain simpler like I am 5 (ELI5)")
			chips = append(chips, "🎯 Test my understanding with a quick question")
		}
		chips = append(chips, "⚠️ Common student traps & scoring pitfalls")
		chips = append(chips, "📝 High-yield 3-bullet revision summary")
	}

	// De-duplicate and return top 4 chips
	seen := make(map[string]bool)
	unique := []string{}
	for _, c := range chips {
		if !seen[c] {
			seen[c] = true
			unique = append(unique, c)
			if len(unique) == 4 {
				break
			}
		}
	}

	return unique
}
