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
	ListSessions(ctx context.Context, userID string) ([]*model.ChatSession, error)
	CreateSession(ctx context.Context, userID, title string) (*model.ChatSession, error)
	GetSessionMessages(ctx context.Context, userID, sessionID string) ([]*model.ChatMessage, error)
	UpdateSession(ctx context.Context, userID, sessionID, title string) error
	DeleteSession(ctx context.Context, userID, sessionID string) error
}

type aiTutorService struct {
	orchestrator *ai.Orchestrator
	chatRepo     repository.ChatRepository
}

func NewAITutorService(
	orchestrator *ai.Orchestrator,
	chatRepo repository.ChatRepository,
) AITutorService {
	return &aiTutorService{
		orchestrator: orchestrator,
		chatRepo:     chatRepo,
	}
}

func (s *aiTutorService) SolveDoubt(ctx context.Context, userID string, req dto.DoubtRequest) (*dto.DoubtResponse, error) {
	if s.orchestrator == nil {
		return nil, fmt.Errorf("AI service error: orchestrator not initialized")
	}

	systemPrompt := buildTutorPrompt(req.Mode, req.Question, req.ContextNotes)

	// Resolve the session for transcript persistence without feeding prior answers to the model.
	sessionID := req.SessionID
	if sessionID == "" && (req.ScopeType == "chat" || req.ScopeType == "session") {
		sessionID = req.ScopeID
	}

	userQuery := req.Question
	if req.ContextNotes != "" {
		userQuery = fmt.Sprintf("%s\n\n%s", req.ContextNotes, userQuery)
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
			answerText, err = s.orchestrator.GenerateVisionWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, audioPrompt, audioBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateTextWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, userQuery)
		}
	} else if req.ImageBase64 != "" {
		// 2. If handwritten image is provided, run vision multimodal pipeline
		imgBytes, decErr := base64.StdEncoding.DecodeString(req.ImageBase64)
		if decErr == nil && len(imgBytes) > 0 {
			mime := req.ImageMimeType
			if mime == "" {
				mime = "image/jpeg"
			}
			answerText, err = s.orchestrator.GenerateVisionWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, userQuery, imgBytes, mime)
		} else {
			answerText, err = s.orchestrator.GenerateTextWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, userQuery)
		}
	} else {
		// 3. Pure text doubt
		answerText, err = s.orchestrator.GenerateTextWithCustomKey(ctx, req.APIKey, provider, modelName, systemPrompt, userQuery)
	}

	if err != nil && answerText == "" {
		return nil, fmt.Errorf("AI service error: %w", err)
	}

	// Extract structured study sections only when the answer actually contains them.
	coreConcept := extractSection(answerText, "Foundation")
	if coreConcept == "" {
		coreConcept = extractSection(answerText, "Core Concept")
	}
	commonPitfalls := extractSection(answerText, "Common Misconceptions")
	if commonPitfalls == "" {
		commonPitfalls = extractSection(answerText, "Common Pitfalls")
	}

	// Generate smart follow-up chips
	followUpChips := []string{}
	if !strings.Contains(analyzeQuerySituation(req.Question, req.ContextNotes), "SITUATION: CONVERSATIONAL") {
		followUpChips = generateFollowUpChips(req.Question, answerText)
	}

	resp := &dto.DoubtResponse{
		Question:         req.Question,
		SolutionMarkdown: answerText,
		CoreConcept:      coreConcept,
		CommonPitfalls:   commonPitfalls,
		FollowUpChips:    followUpChips,
		ActiveProvider:   provider,
		ActiveModel:      modelName,
	}

	// Persist to chat messages and manage session if user is authenticated
	if userID != "" && s.chatRepo != nil {
		scopeType := req.ScopeType
		if scopeType == "" {
			scopeType = "chat"
		}

		var sessionTitle string
		// If sessionID is empty, create a new session record automatically (like ChatGPT)
		if sessionID == "" && (scopeType == "chat" || scopeType == "session" || scopeType == "global") {
			sessionTitle = strings.TrimSpace(req.Question)
			if len(sessionTitle) > 50 {
				sessionTitle = sessionTitle[:50] + "..."
			}
			newSession := &model.ChatSession{
				UserID: userID,
				Title:  sessionTitle,
			}
			if err := s.chatRepo.CreateSession(ctx, newSession); err == nil {
				sessionID = newSession.ID
				resp.SessionID = sessionID
				resp.SessionTitle = sessionTitle
			}
		} else if sessionID != "" {
			_ = s.chatRepo.TouchSession(ctx, userID, sessionID)
			resp.SessionID = sessionID
		}

		effectiveScopeID := sessionID
		if effectiveScopeID == "" {
			effectiveScopeID = req.ScopeID
		}

		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   effectiveScopeID,
			Role:      "user",
			Content:   req.Question,
		})
		_ = s.chatRepo.SaveMessage(ctx, &model.ChatMessage{
			UserID:    userID,
			ScopeType: scopeType,
			ScopeID:   effectiveScopeID,
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

func (s *aiTutorService) ListSessions(ctx context.Context, userID string) ([]*model.ChatSession, error) {
	if s.chatRepo == nil {
		return []*model.ChatSession{}, nil
	}
	return s.chatRepo.ListSessions(ctx, userID, 50)
}

func (s *aiTutorService) CreateSession(ctx context.Context, userID, title string) (*model.ChatSession, error) {
	if s.chatRepo == nil {
		return nil, fmt.Errorf("chat repository not available")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New Conversation"
	}
	session := &model.ChatSession{
		UserID: userID,
		Title:  title,
	}
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *aiTutorService) GetSessionMessages(ctx context.Context, userID, sessionID string) ([]*model.ChatMessage, error) {
	if s.chatRepo == nil {
		return []*model.ChatMessage{}, nil
	}
	return s.chatRepo.ListSessionMessages(ctx, userID, sessionID, 100)
}

func (s *aiTutorService) UpdateSession(ctx context.Context, userID, sessionID, title string) error {
	if s.chatRepo == nil {
		return fmt.Errorf("chat repository not available")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled Chat"
	}
	return s.chatRepo.UpdateSession(ctx, userID, sessionID, title)
}

func (s *aiTutorService) DeleteSession(ctx context.Context, userID, sessionID string) error {
	if s.chatRepo == nil {
		return fmt.Errorf("chat repository not available")
	}
	return s.chatRepo.DeleteSession(ctx, userID, sessionID)
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

	isConversational := query == "hi" || query == "hello" || query == "hey" || query == "hii" || query == "hlo" ||
		strings.HasPrefix(query, "hi ") || strings.HasPrefix(query, "hello ") || strings.HasPrefix(query, "hey ") ||
		strings.Contains(query, "pdf") || strings.Contains(query, "kya tum") || strings.Contains(query, "can you") ||
		strings.Contains(query, "kya kar sakte") || strings.Contains(query, "who are you") || strings.Contains(query, "kaun ho") ||
		strings.Contains(query, "kaise ho") || strings.Contains(query, "how are you") || strings.Contains(query, "aap kaun") ||
		strings.Contains(query, "thanks") || strings.Contains(query, "shukriya") || strings.Contains(query, "thank you") ||
		strings.Contains(query, "good morning") || strings.Contains(query, "good night") || strings.Contains(query, "good evening") ||
		strings.Contains(query, "help me") || (query == "help")
	if isConversational && wordCount <= 15 {
		return fmt.Sprintf(`%s
💬 SITUATION: CONVERSATIONAL & APP CAPABILITY QUESTION
- The student is having a natural conversational chat or asking about app features.
- Directly, warmly, and helpfully answer in their language (Hinglish/Hindi/English).
- For PDF questions: "Haan, StudySilo me aap kisi bhi note, study guide ya formula sheet ko 'Export to PDF' button se PDF format me download kar sakte hain!"
- NEVER output academic headings, LaTeX formulas, or tables for conversational questions.
- Keep it friendly, short, and to the point.`, levelDirective)
	}

	isDeepConcept := strings.Contains(query, "explain in detail") || strings.Contains(query, "in depth") || strings.Contains(query, "deep") || strings.Contains(query, "comprehensive") || strings.Contains(query, "compare") || strings.Contains(query, "difference between") || strings.Contains(query, "working of") || strings.Contains(query, "mechanism of") || strings.Contains(query, "architecture")
	isQuickFormula := !isDeepConcept && (strings.Contains(query, "formula") || strings.Contains(query, "si unit") || strings.Contains(query, "dimension") || strings.Contains(query, "value of") || strings.Contains(query, "unit of") || strings.Contains(query, "constant of")) && wordCount <= 9
	isShortDef := !isDeepConcept && (strings.HasPrefix(query, "what is") || strings.HasPrefix(query, "define") || strings.HasPrefix(query, "kya hai") || strings.HasPrefix(query, "meaning of") || strings.HasPrefix(query, "state") || strings.HasPrefix(query, "who is") || strings.HasPrefix(query, "name the")) && wordCount <= 7
	isVerification := !isDeepConcept && (strings.HasPrefix(query, "is ") || strings.HasPrefix(query, "can ") || strings.HasPrefix(query, "does ") || strings.Contains(query, "true or false") || strings.Contains(query, "sahi hai kya") || strings.Contains(query, "galat hai kya") || strings.Contains(query, "correct or not")) && wordCount <= 10

	if isQuickFormula || isShortDef || isVerification {
		return fmt.Sprintf(`%s
⚡ SITUATION: SHORT QUESTION / DIRECT LOOKUP
- The student asked a short, direct question (definition, formula, quick fact, or verification).
- Provide a short, direct, and crisp answer (2-4 lines maximum).
- State the formula in LaTeX, SI unit, or exact definition.
- NEVER create unnecessary tables or long explanations for quick questions.
- COMPLETE ANSWER MANDATE: Always finish the answer; never cut off mid-sentence.`, levelDirective)
	}

	isDerivation := strings.Contains(query, "derive") || strings.Contains(query, "derivation") || strings.Contains(query, "prove") || strings.Contains(query, "proof") || strings.Contains(query, "step by step") || strings.Contains(query, "solve this") || strings.Contains(query, "numerical")
	if isDerivation {
		return fmt.Sprintf(`%s
📐 SITUATION: MATHEMATICAL DERIVATION / FORMAL PROOF / PROBLEM-SOLVING
- Deliver mathematical proof calibrated to the student's level.
- Structure: Problem Setup & Variables -> Governing Principle -> Step-by-Step Mathematical Derivation in LaTeX -> Dimensional Check -> Boxed Final Result.
- Every step must have brief physical/mathematical justification.
- COMPLETE DERIVATION MANDATE: Walk through every step to the very end without cutting off.`, levelDirective)
	}

	isFollowUp := strings.HasPrefix(query, "aur") || strings.HasPrefix(query, "then") || strings.HasPrefix(query, "why") || strings.HasPrefix(query, "kyu") || strings.HasPrefix(query, "kyun") || strings.HasPrefix(query, "repeat") || strings.HasPrefix(query, "example do") || strings.HasPrefix(query, "short me") || strings.HasPrefix(query, "hindi me") || (wordCount <= 4 && !isShortDef)
	if isFollowUp && !isDeepConcept {
		return fmt.Sprintf(`%s
🔄 SITUATION: CONTEXTUAL FOLLOW-UP / SPECIFIC CLARIFICATION
- Directly address the specific clarification or doubt based on prior context.
- Match answer length proportionately: focused for quick clarifications, comprehensive if deep example was requested.
- COMPLETE EXPLANATION MANDATE: Finish all examples, steps, or workings completely without stopping.`, levelDirective)
	}

	if isDeepConcept {
		return fmt.Sprintf(`%s
🌟 SITUATION: IN-DEPTH CONCEPTUAL REQUEST
- The student explicitly asks for depth or comparison. Explain the reasoning and useful details fully, without padding.
- Choose a clear format that suits this question; use headings, examples, equations, or bullets only when they help.
- Use LaTeX for equations ($$ ... $$ standalone, $ ... $ inline).
- Finish the requested explanation and conclusion.`, levelDirective)
	}

	return fmt.Sprintf(`%s
🎯 SITUATION: NATURAL QUESTION-PROPORTIONATE ANSWER
- Answer the actual question directly and scale length to its intent, not merely its word count.
- For general or conversational questions, respond naturally; do not turn them into a lesson or add academic sections.
- For academic questions, explain only the concepts and context that help answer this question. Use headings or lists only when useful.
- COMPLETE RESPONSE MANDATE: Finish the requested answer without adding unrelated sections or padding.`, levelDirective)
}

func buildTutorPrompt(mode, question, contextNotes string) string {
	situationInstruction := analyzeQuerySituation(question, contextNotes)
	return fmt.Sprintf("%s\n\n%s", situationInstruction, getModePrompt(mode))
}

func getModePrompt(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))

	const masterDiagramDirectives = `
DIAGRAM RULES (Follow strictly for accurate, reliable, and professional visual diagrams):

When a diagram genuinely helps explain the concept, output a clean JSON block in this exact schema:
` + "```json" + `
{
  "title": "Clear Descriptive Title",
  "explanation": "1-2 sentences explaining what this diagram illustrates and how the process flows",
  "mode": "flowchart|cycle|tree|mindmap",
  "nodes": [
    {"id": "1", "label": "Clear Step/Node Name", "subtitle": "Brief detail or role", "shape": "box|circle|diamond|capsule"}
  ],
  "edges": [
    {"from": "1", "to": "2", "label": "Specific action/flow"}
  ]
}
` + "```" + `

CRITICAL RULES FOR ACCURATE & RELIABLE VISUALS:
1. ALWAYS EXPLAIN IN TEXT: Before or after the diagram block, explain the diagram properly in your answer text! Walk through the steps so the student understands the logic behind the drawing. Never drop a random diagram without explanation.
2. CORRECT ARROW DIRECTION: Arrows must strictly follow true scientific and logical direction (e.g., Input -> Processing -> Output; Left Ventricle -> Aorta -> Body -> Vena Cava -> Right Atrium).
3. MEANINGFUL EDGE LABELS: Every edge must clearly describe what is transferred, triggered, or connected (e.g. "Oxidizes to", "Sends impulse", "If condition met").
4. CLEAN & FOCUSED: Keep it between 3 to 6 high-yield nodes. Never draw cluttered, random, or disconnected orphan nodes.
5. ACCURATE SHAPES:
   - "capsule" for Start / End / Terminal states
   - "box" for standard process or component
   - "diamond" for decision / condition
   - "circle" for central hub or core entity
6. TRUE CYCLES ONLY: Only use mode "cycle" for genuine closed-loop cycles (Krebs cycle, Water cycle, Carnot cycle). All other step-by-step processes MUST use mode "flowchart".
`

	const globalFormattingDirectives = `
COMMUNICATION PRIORITIES (OVERRIDE GENERIC PERSONA TEMPLATES WHEN THEY CONFLICT):
1. FOLLOW THE USER'S REQUEST:
	- Respect explicit requests for a short answer, a detailed explanation, a language, tone, or output format.
	- If no depth is specified, infer the useful amount from the question; a long sentence alone does not mean the user wants a long answer.
	- Do not force a fixed set of headings, exam tips, pitfalls, summaries, or follow-up questions. Include them only when requested or genuinely useful.
	- For everyday, general-knowledge, and conversational questions, answer naturally, not like an exam worksheet.
	- For academic questions, teach clearly and accurately while keeping the response focused on what was asked.

2. EXAM & GRADE LEVEL CALIBRATION:
   - Adapt depth, vocabulary, and formulas strictly to the student's preparation level.
   - If Class 9-10 / School level: Explain strictly within the 9th/10th grade school syllabus. Aisa bilkul na ho ki student 10th ka ho aur answer graduation/college level ka de do! DO NOT use university multivariable calculus, differential equations, advanced vectors, or graduate-level proofs.
   - If Class 11-12 / JEE / NEET: Use senior secondary entrance depth.
   - If College / B.Tech / University: Use full graduation-level rigor.

3. MATHEMATICAL & SCIENTIFIC RIGOR:
   - Use LaTeX: $$ ... $$ for standalone formulas, and $ ... $ for inline variables only.
   - Never wrap plain words or descriptions in dollar signs (use **bold** instead).
   - Never invent formulas or equations for non-mathematical topics.

4. MULTILINGUAL & HINGLISH AUTO-ADAPTATION:
   - Match the student's language naturally (fluid Hinglish, Hindi, or Academic English).
` + masterDiagramDirectives

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
   - Bold each keyword: **[keyword]**. Missing these = lost marks. Including these = guaranteed marks.
3. FULL-MARK MODEL ANSWER:
   - Structure the ideal answer point-by-point as it should appear in an exam sheet.
   - Use numbered points, diagrams where needed, and formulae in LaTeX.
   - Show mark allocation: [Definition: 1 mark] [Explanation: 2 marks] [Formula: 1 mark]
4. EXAMINER INTELLIGENCE:
   - "> 🎯 Exam Tip:" — the smart trick that separates top-rankers from average students.
   - "> ⚠️ Trap Question Alert:" — subtle variant the examiner uses to confuse students.
   - "Scoring Shortcut:" — fastest route to full marks without unnecessary length.
5. REVISION PRIORITY:
   - "⭐ Priority Score: [1-5 stars]" based on frequency × marks × difficulty.
   - Suggest which related PYQs to solve next for complete topic coverage.

LANGUAGE: Crisp, action-oriented, motivating. Hinglish or English based on student.
` + globalFormattingDirectives

	case "feynman", "eli5":
		return `PERSONA: 🧠 Richard Feynman — Nobel Laureate, Master of Physical Intuition & Mental Models.
MISSION: Make ANY concept feel obvious and inevitable by teaching from pure physical reality — no jargon, no memorization, just understanding.

FEYNMAN TEACHING PROTOCOL:
1. REALITY ANCHOR (Start here — always):
   - What is ACTUALLY happening in the physical world?
   - Describe it as if explaining to a curious 12-year-old with no textbook.
   - Use a vivid, unexpected analogy that triggers an "aha!" moment.
2. MECHANISM STRIPPED BARE:
   - Walk through the cause-and-effect chain using simple English.
   - Every step must be physically grounded: "The reason [X] happens is because [physical reason]."
   - NO black-box statements. If something "just works," explain WHY it works.
3. JARGON DEMOLITION:
   - Take every technical term in the concept and translate it to plain English.
   - Example: "Entropy isn't disorder — it's the number of ways atoms can arrange themselves."
   - Then reintroduce the term with full understanding attached.
4. BUILD-UP TO FORMALISM:
   - Once intuition is rock-solid, introduce the mathematical form:
$$
oxed{	ext{Emergent Law}}
$$
   - Show HOW the equation naturally emerges from the physical reality.
5. FEYNMAN SEAL:
   - "> 🧠 Feynman Hook:" — 1-sentence that permanently locks the intuition in memory.
   - "If you understand [concept], you can predict [observable effect] without any calculation."

LANGUAGE: Conversational, vivid, warm — like a brilliant friend explaining over chai. Hinglish very welcome.
` + globalFormattingDirectives

	case "socratic":
		return `PERSONA: ⚖️ Socratic Cognitive Coach — Guide, not lecturer.
MISSION: Force the student to THINK. Build metacognition and independent reasoning by asking the right questions, not giving answers.

SOCRATIC DIALOGUE PROTOCOL:
1. ACTIVATE PRIOR KNOWLEDGE:
   - Ask what the student already knows about the topic before revealing anything.
   - "Pehle bata — is topic mein tumhe kya kya pata hai already?"
2. STRATEGIC QUESTION CASCADE:
   - Ask 1-2 targeted questions that force the next logical deduction.
   - Questions must be specific, not vague: "Why does [X] happen when [condition Y] is met?"
   - Do NOT give the answer — guide them to discover it.
3. SCAFFOLD ON STRUGGLE:
   - If the student is stuck: give ONE hint or analogy, then re-ask.
   - Gradually reduce scaffolding as confidence builds.
   - "Here's a clue: think about what happens to energy when..."
4. VALIDATE & DEEPEN:
   - When they answer correctly: confirm and push deeper.
   - "Bilkul sahi! Ab ek aur — agar [condition changes], toh kya hoga?"
   - When wrong: don't correct directly — ask "Are you sure? What happens when...?"
5. SYNTHESIS CHALLENGE:
   - End with a thought-provoking question that requires connecting 2+ concepts.
   - "Can you now explain how [concept A] and [concept B] are related?"

LANGUAGE: Warm, encouraging, thought-provoking. Hinglish or English. Never condescending.
` + globalFormattingDirectives

	case "traphunter", "traps":
		return `PERSONA: ⚠️ Elite Exam Trap Hunter & Error Forensics Expert.
MISSION: Surgically identify and dismantle every misconception, sign error, unit trap, and sneaky examiner trick — before the exam does.

TRAP HUNTING PROTOCOL:
1. MISCONCEPTION AUTOPSY:
   - Identify the EXACT false belief the student (or most students) hold.
   - "❌ FALSE BELIEF: Most students think [X] means [Y]."
   - Explain WHY this misconception forms — it's usually logical but wrong.
2. RIGOROUS CORRECTION:
   - "✅ TRUTH: [Precise correct statement with proof or reasoning]"
   - Show the specific moment where the wrong thinking diverges from reality.
3. TRAP CLASSIFICATION:
   Categorize the trap type:
   - 🔢 Calculation Trap (sign error, arithmetic, approximation)
   - 📐 Unit Trap (wrong conversion, missing factor)
   - 📖 Definition Trap (confused with similar term)
   - 🔄 Direction/Convention Trap (sign of current, force direction)
   - 🎯 Examiner Trick (subtle wording that flips the answer)
4. BOUNDARY CASE STRESS TEST:
   - Test the concept at extreme limits: What happens as [variable] → 0 or → ∞?
   - Identify where the formula BREAKS DOWN (assumptions violated).
5. TRAP IMMUNITY CHECKLIST:
   Before writing any answer in exam, verify:
   ☐ [Check 1: e.g. "Are units consistent on both sides?"]
   ☐ [Check 2: e.g. "Is direction/sign defined correctly?"]
   ☐ [Check 3: e.g. "Am I using the right formula for this regime?"]

LANGUAGE: Sharp, forensic, protective. Hinglish or English. Treat the student as a detective, not a victim.
` + globalFormattingDirectives

	case "derivation", "steps":
		return `PERSONA: 📐 Mathematical Proof Engine & Derivation Architect.
MISSION: Produce flawless, step-by-step mathematical derivations where every line is justified, every symbol defined, and every step is traceable.

DERIVATION MASTER PROTOCOL:
PRE-DERIVATION SETUP (mandatory):
  Variables & Symbols:
  | Symbol | Meaning | SI Unit |
  |--------|---------|--------|
  | [var]  | [def]   | [unit]  |
  Coordinate System: [Cartesian/Polar/Cylindrical — state orientation]
  Governing Law: The derivation begins from [Fundamental Theorem/Law/Axiom].
  Key Assumptions: [List all simplifying assumptions and their physical justification]

STEP-BY-STEP DERIVATION FORMAT:
` + "```derivation" + `
Step N: [Step Title]
Action: [What mathematical operation is performed and why]
Equation: $$ [LaTeX equation] $$
Justification: [Physical law / algebraic rule / identity used]
Note: [Any approximation made, or limiting case applied]
` + "```" + `

POST-DERIVATION VERIFICATION:
1. Dimensional Analysis: Verify LHS dimensions = RHS dimensions
2. Limiting Cases: Check behaviour as [var] → 0 and [var] → ∞
3. Sign/Direction Check: Confirm signs are physically consistent
4. Literature Check: State if result matches standard textbook form

FINAL RESULT BOX:
$$
oxed{	ext{Final Formula}}
$$
Conditions of validity: [When this formula applies]

LANGUAGE: Methodical, precise, zero ambiguity. Hinglish or English. Every step MUST be justified.
` + globalFormattingDirectives

	default: // academic
		return `PERSONA: Academic Tutor.
MISSION: Give clear, accurate, natural answers that help the student with the question they actually asked.

RESPONSE STYLE:
- Follow the user's requested length and format. If unspecified, be concise by default and expand when the question needs explanation.
- Answer general or conversational questions directly without academic headings or exam-focused extras.
- For substantial academic explanations, organize the answer in the clearest format for that topic; headings, examples, formulas, and exam tips are optional, not a checklist.
- Calibrate terminology and rigor to the student's stated level.

IMPORTANT RULES:
- Keep the response natural and focused; use headings and exam tips only when they help answer the request.
- Follow explicit requests for brevity or detail.
- Calibrate the depth strictly to the student's target exam & grade level.

DIAGRAM DIRECTIVE:
- When a visual sketch is required: Do NOT output flowcharts. Only output JSON nodes and edges relevant to the concept.
- No coordinates, no layout, no flowchart keywords. Use clean shapes like "shape": "circle" or "box".

LANGUAGE: Match the student's language naturally (fluid Hinglish, Hindi, or English).
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
			chips = append(chips, "🎯 Is topic ka ek practical use batao")
		}
		chips = append(chips, "⚠️ Isme common galtiyan kya ho sakti hain?")
		chips = append(chips, "📝 Iska 3-point summary do")
	} else {
		if hasMath {
			chips = append(chips, "📐 Step-by-step mathematical derivation & proof")
			chips = append(chips, "🎯 Practice numerical exam problem with solution")
			chips = append(chips, "📏 What are the SI units & boundary conditions?")
		} else if hasComparison {
			chips = append(chips, "📊 Detailed comparison matrix & trade-offs")
			chips = append(chips, "🎯 When is each option more useful?")
		} else if hasCode {
			chips = append(chips, "💻 Show an end-to-end code example with comments")
			chips = append(chips, "⚡ What is the time & space complexity analysis?")
		} else if hasMechanism {
			chips = append(chips, "💡 Explain with a vivid real-world analogy")
			chips = append(chips, "🎯 Give me a real-world example of this")
		} else {
			chips = append(chips, "💡 Can you explain this more simply?")
			chips = append(chips, "🎯 Test my understanding with a quick question")
		}
		chips = append(chips, "⚠️ What should I watch out for?")
		chips = append(chips, "📝 Summarize this in 3 bullets")
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
