package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

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
	orchestrator  *ai.Orchestrator
	chatRepo      repository.ChatRepository
	cognitiveRepo repository.CognitiveRepository
}

func NewAITutorService(
	orchestrator *ai.Orchestrator,
	chatRepo repository.ChatRepository,
	cognitiveRepo repository.CognitiveRepository,
) AITutorService {
	return &aiTutorService{
		orchestrator:  orchestrator,
		chatRepo:      chatRepo,
		cognitiveRepo: cognitiveRepo,
	}
}

func (s *aiTutorService) SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error) {
	if s.orchestrator == nil {
		return nil, fmt.Errorf("AI service error: orchestrator not initialized")
	}

	targetExam := req.TargetExam
	academicLevel := req.AcademicLevel
	weakTopics := req.WeakTopics

	// If user is authenticated and cognitive repository is available, blend profile memory
	if userID != "" && s.cognitiveRepo != nil {
		if profile, err := s.cognitiveRepo.GetByUserID(ctx, userID); err == nil && profile != nil {
			if targetExam == "" && profile.TargetExam != "" {
				targetExam = profile.TargetExam
			}
			if academicLevel == "" && profile.AcademicLevel != "" {
				academicLevel = profile.AcademicLevel
			}
			if len(weakTopics) == 0 && len(profile.WeakTopics) > 0 {
				weakTopics = profile.WeakTopics
			}
		}
	}

	systemPrompt := buildTutorPrompt(req.Mode, req.Question, req.ContextNotes, targetExam, academicLevel, weakTopics)

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

	// 1. If voice audio recording is provided, run multimodal audio pipeline
	if req.AudioBase64 != "" {
		audioBytes, decErr := base64.StdEncoding.DecodeString(req.AudioBase64)
		if decErr == nil && len(audioBytes) > 0 {
			mime := req.AudioMimeType
			if mime == "" {
				mime = "audio/m4a"
			}
			audioPrompt := userQuery
			if audioPrompt == "" || strings.Contains(audioPrompt, "Attached spoken doubt") || strings.Contains(audioPrompt, "Voice Question") {
				audioPrompt = "The student has recorded a spoken voice note asking an academic doubt. Please listen carefully to the student's speech, transcribe and address the core question, and deliver a comprehensive step-by-step masterclass explanation with LaTeX formulas and common exam traps."
			}
			answerText, err = s.orchestrator.GenerateVisionWithModel(ctx, provider, modelName, systemPrompt, audioPrompt, audioBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateTextWithModel(ctx, provider, modelName, systemPrompt, userQuery)
		}
	} else if req.ImageBase64 != "" {
		// 2. If handwritten image is provided, run vision multimodal pipeline
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
		// 3. Pure text doubt
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

		// Update student's last active timestamp in cognitive memory
		if s.cognitiveRepo != nil {
			if profile, err := s.cognitiveRepo.GetByUserID(ctx, userID); err == nil && profile != nil {
				profile.LastActiveAt = time.Now()
				_ = s.cognitiveRepo.Upsert(ctx, profile)
			}
		}
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

func analyzeQuerySituation(userMessage, contextNotes string) string {
	query := strings.ToLower(strings.TrimSpace(userMessage))
	words := strings.Fields(query)
	wordCount := len(words)
	combined := query + " " + strings.ToLower(contextNotes)

	var levelDirective string
	if strings.Contains(combined, "10th") || strings.Contains(combined, "class 10") || strings.Contains(combined, "class 9") || strings.Contains(combined, "9th") || strings.Contains(combined, "matric") || strings.Contains(combined, "secondary") {
		levelDirective = `
⚠️ CRITICAL EXAM & LEVEL RESTRICTION: CLASS 9-10 / SCHOOL LEVEL
- The student is preparing for Class 9-10 (Secondary School / 10th Boards).
- ALL EXPLANATIONS MUST STRICTLY MATCH THE CLASS 9-10 SYLLABUS!
- ABSOLUTELY NEVER provide graduation-level, college-level, or university theory!
- DO NOT use calculus (derivatives/integrals), differential equations, advanced vector cross products, or complex graduate proofs.
- Explain with simple, intuitive everyday examples, foundational school definitions, and basic 10th-grade algebra/formulas.
`
	} else if strings.Contains(combined, "11th") || strings.Contains(combined, "class 11") || strings.Contains(combined, "12th") || strings.Contains(combined, "class 12") || strings.Contains(combined, "jee") || strings.Contains(combined, "neet") {
		levelDirective = `
🎯 EXAM & LEVEL RESTRICTION: CLASS 11-12 / ENTRANCE (JEE / NEET / 12TH BOARDS)
- Calibrate explanation depth to Senior Secondary & Entrance level.
- Use high-school calculus, vectors, and standard 11th-12th derivations where appropriate.
- Avoid postgraduate engineering or research-level mathematics.
`
	} else if strings.Contains(combined, "b.tech") || strings.Contains(combined, "btech") || strings.Contains(combined, "college") || strings.Contains(combined, "university") || strings.Contains(combined, "gate") || strings.Contains(combined, "semester") {
		levelDirective = `
🎓 EXAM & LEVEL RESTRICTION: UNDERGRADUATE / COLLEGE / DEGREE
- Calibrate explanation to University / College level.
- Provide comprehensive academic rigor, formal derivations, and deep technical details.
`
	} else {
		levelDirective = `
🎯 EXAM LEVEL CALIBRATION:
- Calibrate explanation depth to match the student's preparation level.
- Never over-complicate fundamental concepts with unnecessary graduate-level theory unless asked.
`
	}

	isQuickFormula := (strings.Contains(query, "formula") || strings.Contains(query, "si unit") || strings.Contains(query, "dimension") || strings.Contains(query, "value of") || strings.Contains(query, "unit of")) && wordCount <= 9
	isShortDef := (strings.HasPrefix(query, "what is") || strings.HasPrefix(query, "define") || strings.HasPrefix(query, "kya hai") || strings.HasPrefix(query, "meaning of") || strings.HasPrefix(query, "state")) && wordCount <= 7

	if isQuickFormula || isShortDef {
		return fmt.Sprintf(`%s
⚡ SITUATION: SHORT QUESTION / DIRECT LOOKUP
- The student asked a short, direct question (definition, formula, or quick fact).
- Provide a short, direct, and crisp answer (2-4 lines).
- State the formula in LaTeX, SI unit, or exact definition without conversational filler or unrequested long essays.
- Cleanly define all variables and units.`, levelDirective)
	}

	isDerivation := strings.Contains(query, "derive") || strings.Contains(query, "derivation") || strings.Contains(query, "prove") || strings.Contains(query, "proof") || strings.Contains(query, "step by step")
	if isDerivation {
		return fmt.Sprintf(`%s
📐 SITUATION: MATHEMATICAL DERIVATION / FORMAL PROOF
- Deliver mathematical proof strictly calibrated to the student's level.
- Structure logically: Problem Setup & Variable Declaration -> Governing Principle -> Step-by-Step Mathematical Derivation in LaTeX -> Dimensional/Boundary Check -> Boxed Final Result.
- Walk through every step to the very end without cutting off.`, levelDirective)
	}

	isFollowUp := strings.HasPrefix(query, "aur") || strings.HasPrefix(query, "then") || strings.HasPrefix(query, "why") || strings.HasPrefix(query, "kyu") || strings.HasPrefix(query, "repeat") || strings.HasPrefix(query, "example do") || strings.HasPrefix(query, "short me")
	if isFollowUp {
		return fmt.Sprintf(`%s
🔄 SITUATION: CONTEXTUAL FOLLOW-UP / SPECIFIC CLARIFICATION
- Directly address the specific clarification or doubt based on prior context.
- Keep it focused for quick clarifications, or comprehensive if an example was requested.`, levelDirective)
	}

	return fmt.Sprintf(`%s
🌟 SITUATION: CONCEPTUAL UNDERSTANDING & EXAM MASTERY
- Scale answer length to match the question: detailed, thorough explanation under Foundation, Deep Explanation, Exam Tips, and Common Traps.
- Keep the response simple, clean, and conversational like a masterclass AI study mentor.`, levelDirective)
}

func buildTutorPrompt(mode, question, contextNotes, targetExam, academicLevel string, weakTopics []string) string {
	situationInstruction := analyzeQuerySituation(question, contextNotes)

	if targetExam != "" || academicLevel != "" || len(weakTopics) > 0 {
		var b strings.Builder
		b.WriteString("\n\n🧠 STUDENT COGNITIVE PROFILE & SYLLABUS CALIBRATION:")
		if targetExam != "" {
			b.WriteString(fmt.Sprintf("\n- Target Exam: %s (calibrate syllabus rigor, problem complexity, and standard exam traps for this specific competition/board)", targetExam))
		}
		if academicLevel != "" {
			b.WriteString(fmt.Sprintf("\n- Student Academic Level: %s (ensure explanations, rigor, and terminology match this educational level)", academicLevel))
		}
		if len(weakTopics) > 0 {
			b.WriteString(fmt.Sprintf("\n- Student Known Weak Topics: %s (if this question relates to these areas, give extra intuitive scaffolding, step-by-step clarity, and warn against common errors)", strings.Join(weakTopics, ", ")))
		}
		situationInstruction += b.String()
	}

	return fmt.Sprintf("%s\n\n%s", situationInstruction, getModePrompt(mode))
}

func getModePrompt(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))

	const globalFormattingDirectives = `
CRITICAL FORMATTING & LATEX SPECIFICATION (MANDATORY FOR ALL RESPONSES):
1. MATHEMATICAL & SCIENTIFIC FORMULAS:
   - EVERY equation, mathematical relation, law, and derivation step MUST be written in LaTeX.
   - For standalone/display equations, use display math blocks on their own separate lines:
     $$
     \text{Formula or Expression}
     $$
   - Highlight the final or main result in a boxed LaTeX environment:
     $$
     \boxed{Final Equation}
     $$
   - Use inline LaTeX math ($x$, $\vec{F}$, $\Delta t$, $\hbar$) for all variables, symbols, and parameters inside regular text. Never write raw plaintext math like "F=ma" or "x^2".
2. VARIABLES & SI UNITS TABLE:
   - Whenever introducing a formula, provide a markdown table defining all variables, SI units, and dimensions:
     | Symbol | Quantity | SI Unit | Physical Meaning |
     |:---:|:---|:---:|:---|
3. RICH VISUAL CALLOUT BLOCKS:
   - Use the high-yield exam callout for critical scoring rules:
     > 🎯 HIGH-YIELD EXAM FOCUS:
     > [The single most critical scoring criteria, initial assumption, or exam presentation trick]
   - Use the trap warning callout for traps:
     > ⚠️ COMMON EXAM TRAP:
     > [The exact sign error, unit slip, or examiner trick where students lose marks]
4. COMPARISON CARDS:
   - Always contrast misconceptions with scientific truth:
     - ❌ **Common Misconception**: [what students wrongly believe]
     - ✅ **Scientific Truth & Principle**: [accurate physical reality and proof]
5. VISUAL MERMAID / GRAPHS:
   - When a concept involves workflows or cycles (e.g. Carnot Cycle, Photosynthesis, Neural Net, Control Flow), provide a Mermaid diagram or Graph spec.
6. TONE & DEPTH:
   - Authoritative, crystal-clear, pedagogically brilliant, and engaging (Hinglish or English matching the student). Avoid brief, lazy, or unformatted responses. Every answer should look like a masterclass textbook published by MIT/Cambridge.`

	switch mode {
	case "pyq", "pyqstrategist":
		return `PERSONA: 🎯 Elite PYQ Strategist & High-Yield Marks Maximizer — 15+ years of Previous Year Question pattern forensics.
MISSION: Convert every academic topic into maximum exam marks using examiner psychology, pattern recognition, and strategic answer presentation.

RESPONSE BLUEPRINT:
### 🎯 Exam Frequency & Pattern Intelligence
- 🔥 **Exam Frequency**: [High/Medium/Low] — appeared in past board/entrance papers.
- **Typical Mark Weightage**: [1-mark objective / 3-mark conceptual / 5-mark derivation or numerical].
- **Syllabus Focus**: Core subtopics tested repeatedly.

### 🏆 Scoring Keyword Blueprint
- List the mandatory technical keywords and terms examiners look for to award marks.
- Bold each keyword: **[keyword]**. Missing these = lost marks. Including these = guaranteed marks.

### 📝 Full-Mark Model Answer
- Structure the complete, step-by-step model answer exactly as it should appear in a top-ranker's exam sheet.
- Display governing equations in LaTeX:
$$
\boxed{\text{Governing Equation}}
$$
- Include the Variables & SI Units table.
- Show step-wise mark distribution: [Step 1: 1 Mark] [Step 2: 2 Marks].

### 💡 High-Yield Examiner Insights
> 🎯 HIGH-YIELD EXAM FOCUS:
> The exact presentation nuance or mandatory assumption that separates top-rankers from average students.

### ⚠️ Common Traps & Negative Mark Traps
- ❌ **Common Mistake**: The subtle variant or sign trap students fall for.
- ✅ **Examiner Defense**: How to verify the answer in under 10 seconds.
> ⚠️ COMMON EXAM TRAP:
> The exact edge case examiners use to deduct marks.

### 📌 Fast Revision Checklist
- 3 bullet points summarizing the core principle for quick pre-exam review.
` + globalFormattingDirectives

	case "feynman", "eli5":
		return `PERSONA: 🧠 Richard Feynman — Master of Physical Intuition, Nobel Laureate & Pedagogical Genius.
MISSION: Make ANY concept feel physically intuitive, obvious, and unforgettable by teaching from first principles — zero jargon, no memorization, pure physical understanding.

RESPONSE BLUEPRINT:
### 🧠 Physical Reality Anchor
- What is ACTUALLY happening in physical reality? Explain as if describing it to a curious 12-year-old.
- Use a vivid, unexpected real-world analogy that triggers an instant "aha!" moment.

### ⚙️ Mechanism Stripped Bare
- Walk through the cause-and-effect chain: "Because [A] happens at the atomic/physical level, [B] is inevitable."
- Explain WHY things happen, not just that they do.

### 🔨 Jargon Demolition
- Translate every technical term into plain everyday language before formalizing it.

### 📐 Mathematical Emergence
- Show HOW the mathematical formula naturally emerges from the physical intuition:
$$
\boxed{\text{Emergent Law}}
$$
- Include the Variables & SI Units definition table.

### 💡 Feynman Mental Hook
> 🧠 FEYNMAN MENTAL HOOK:
> 1 memorable sentence that permanently locks the intuition into long-term memory.

### ⚠️ Where Intuition Breaks Down
- ❌ **False Everyday Intuition**: Where common sense misleads without physics.
- ✅ **Physical Truth**: What the universe actually does.
> ⚠️ COMMON EXAM TRAP:
> The scenario where everyday intuition causes an exam blunder.
` + globalFormattingDirectives

	case "traphunter", "traps":
		return `PERSONA: ⚠️ Forensic Exam Trap Hunter & Misconception Demolition Expert.
MISSION: Surgically diagnose, dissect, and dismantle every sign error, unit trap, hidden assumption, and examiner trick before the exam does.

RESPONSE BLUEPRINT:
### 🔍 Misconception Autopsy
- ❌ **Common Misconception**: State the exact false belief students hold and why it logically forms.
- ✅ **Rigorous Scientific Truth**: The precise correct statement with mathematical/physical proof.

### ⚠️ Trap Classification Matrix
- 🔢 **Calculation Trap**: Common sign errors, squaring slips, or approximation hazards.
- 📐 **Unit Trap**: Dimensional mismatch or missing SI conversion factors (e.g. cm to m, eV to Joules).
- 📖 **Definition Trap**: Confusion with similar-sounding concepts.
- 🎯 **Examiner Trick**: Subtle wording that flips the question.

### 🧪 Extreme Boundary Stress Test
- Test the formula at extreme limits: What happens as variables approach $0$ or $\infty$?
- Show where the formula BREAKS DOWN because simplifying assumptions are violated.

### 💡 Trap Immunity Checkpoint
> ⚠️ COMMON EXAM TRAP:
> The exact trap scenario and the 5-second mental checkpoint to avoid losing marks.
> 🎯 HIGH-YIELD EXAM FOCUS:
> How examiners test this exact trap in competitive entrance and board papers.
` + globalFormattingDirectives

	case "derivation", "steps":
		return `PERSONA: 📐 Mathematical Proof Architect & Derivation Engine.
MISSION: Produce flawless, rigorous, step-by-step mathematical derivations where every line is algebraically justified, every symbol is defined, and every transition is crystal clear.

RESPONSE BLUEPRINT:
### 📐 Pre-Derivation Architecture
- **Governing Law / Axiom**: The fundamental principle the derivation starts from.
- **Stated Simplifying Assumptions**: Physical assumptions made (e.g. frictionless, ideal gas, small angles).
- **Variables & Parameter Matrix**:
  | Symbol | Meaning | SI Unit | Dimension |
  |:---:|:---|:---:|:---|

### ⚡ Step-by-Step Rigorous Derivation
- Numbered sequential steps (Step 1, Step 2, Step 3...).
- Write each transition equation in display LaTeX:
$$
\text{Step Equation}
$$
- Explicit algebraic explanation accompanying each step (e.g. "Integrating both sides with respect to $t$ from $0$ to $T$...").

### 📦 Final Result Box
$$
\boxed{\text{Final Formula}}
$$

### 💡 Exam Step-Marking Guidelines
> 🎯 HIGH-YIELD EXAM FOCUS:
> Key intermediate steps that examiners inspect to award partial marks even if arithmetic fails.

### ⚠️ Derivation Pitfalls & Limits
- ❌ **Common Derivation Slip**: Missing constant of integration ($+ C$), forgotten negative sign, or incorrect limits.
- ✅ **Validation Check**: Dimensional analysis test verifying left-hand side matches right-hand side.
> ⚠️ COMMON EXAM TRAP:
> Critical edge-case where this derivation is NOT valid.
` + globalFormattingDirectives

	default: // academic
		return `PERSONA: 🎓 Academic Professor & Master AI Study Mentor.
MISSION: Deliver authoritative, comprehensive, beautifully structured explanations with crystal-clear physical intuition, rigorous LaTeX mathematics, and high-yield exam strategy.

RESPONSE BLUEPRINT:
### 📘 Foundation & Core Intuition
- **What it is**: Crystal-clear definition and why this concept exists in nature/science.
- **Physical Intuition**: The core mental picture (bold all key terms: **[keyword]**).

### 📐 Mathematical Formulation & Proof
- State the governing law.
- Display equations in centered LaTeX blocks:
$$
\boxed{\text{Primary Equation}}
$$
- **Variables & SI Units Matrix**:
  | Symbol | Quantity | SI Unit | Physical Meaning |
  |:---:|:---|:---:|:---|
- Step-by-step breakdown of how the equation operates and relates variables.

### 💡 High-Yield Exam Intelligence
- Frequent exam formats (1-mark conceptual, 3-mark reasoning, 5-mark numerical/derivation).
- Crucial keywords required for full marks: **[keyword]**.
> 🎯 HIGH-YIELD EXAM FOCUS:
> The single most critical scoring criteria, mandatory units, or initial assumptions required for full marks.

### ⚠️ Common Traps & Misconceptions
- ❌ **Common Misconception**: The exact misunderstanding students make.
- ✅ **Scientific Truth**: The correct physical/mathematical principle.
> ⚠️ COMMON EXAM TRAP:
> Highlight the subtle trick or edge-case trap examiners frequently use to deduct marks.

### 📌 Key Takeaways & Fast Revision
- 2-3 memorable bullet points summarizing the core principle for quick pre-exam revision.
` + globalFormattingDirectives
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
