package ai

import (
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
)

// ExamCurriculumData holds in-memory canonical syllabus topics and trap warnings for fast resolution.
type ExamCurriculumData struct {
	Code            string
	DisplayName     string
	Keywords        []string
	HighYieldTopics []string
	CommonTraps     []string
}

// Canonical curricula catalog matching all standard academic domains.
var standardCurricula = []ExamCurriculumData{
	{
		Code:        "gate_cs",
		DisplayName: "GATE Computer Science & IT",
		Keywords:    []string{"gate", "computer", "tech", "code", "coding", "software", "cs", "it"},
		HighYieldTopics: []string{
			"Data Structures & Algorithms",
			"Operating Systems & Concurrency",
			"Computer Networks & Protocols",
			"Database Management & Indexing",
			"Computer Architecture & Pipelining",
			"Theory of Computation & Automata",
		},
		CommonTraps: []string{
			"Deadlocks vs Starvation",
			"B+ Tree Order vs Pointers",
			"Pipelining Data Hazards",
			"TCP Congestion Window Transitions",
		},
	},
	{
		Code:        "neet_medical",
		DisplayName: "NEET Medical Entrance (UG)",
		Keywords:    []string{"neet", "medical", "biology", "mbbs", "doctor", "usmle"},
		HighYieldTopics: []string{
			"Molecular Basis of Inheritance",
			"Human Physiology & Circulation",
			"Genetics & Mendelian Principles",
			"Organic Reaction Mechanisms",
			"Chemical Equilibrium & Thermodynamics",
			"Cell Structure & Division",
		},
		CommonTraps: []string{
			"Mendelian Linkage Exceptions",
			"Cardiac Cycle Pressure Timing",
			"Le Chatelier Equilibrium Shifts",
			"Enzyme Inhibition Kinetics",
		},
	},
	{
		Code:        "jee_engineering",
		DisplayName: "JEE Main & Advanced (Engineering)",
		Keywords:    []string{"jee", "iit", "bitsat", "engineering", "physics", "math"},
		HighYieldTopics: []string{
			"Rotational Dynamics & Inertia",
			"Electromagnetic Induction & Optics",
			"Definite Integrals & Calculus",
			"Thermodynamics & Heat Cycles",
			"Matrices & Linear Transformations",
			"Chemical Kinetics & Equilibrium",
		},
		CommonTraps: []string{
			"Moment of Inertia Parallel Axis Sign",
			"Lenz Law Induced EMF Signs",
			"Definite Integral Discontinuity Traps",
			"Carnot Efficiency Limits",
		},
	},
	{
		Code:        "cbse_school",
		DisplayName: "CBSE / ICSE School Boards (Class 10 & 12)",
		Keywords:    []string{"class 10", "class 12", "cbse", "icse", "board", "school"},
		HighYieldTopics: []string{
			"Electricity & Ohm's Law",
			"Light: Reflection & Refraction",
			"Carbon & Its Chemical Compounds",
			"Life Processes & Circulation",
			"Quadratic Equations & Trigonometry",
			"Acids, Bases & Salts",
		},
		CommonTraps: []string{
			"Series vs Parallel Resistance Power Dissipation",
			"Lens Formula Sign Convention",
			"Homologous Series Functional Groups",
			"pH Scale Logarithmic Shifts",
		},
	},
	{
		Code:        "upsc_civil",
		DisplayName: "UPSC Civil Services Examination",
		Keywords:    []string{"upsc", "civil", "ias", "ips", "humanities", "history", "polity"},
		HighYieldTopics: []string{
			"Indian Constitutional Framework & Rights",
			"Monsoon Dynamics & Physical Geography",
			"Macroeconomics, Inflation & Monetary Policy",
			"Modern Indian Freedom Movement",
			"International Relations & Multilateral Treaties",
			"Environmental Ecology & Biodiversity",
		},
		CommonTraps: []string{
			"Fundamental Rights vs DPSP Precedence",
			"El Nino vs Indian Ocean Dipole Patterns",
			"Repo Rate vs Reverse Repo Impact on Money Supply",
			"Ramsar Wetland Classification",
		},
	},
	{
		Code:        "general",
		DisplayName: "General Academic Studies",
		Keywords:    []string{"general", "academic", "studies", "college", "foundation"},
		HighYieldTopics: []string{
			"Active Recall & Spaced Repetition",
			"Critical Thinking & Logic Proofs",
			"Scientific Method & Data Analysis",
			"Core Mathematical Foundations",
			"Thermodynamics Principles",
			"Probability & Applied Statistics",
		},
		CommonTraps: []string{
			"Correlation vs Causation",
			"Bayes Theorem Prior Probability Fallacy",
			"Entropy Increase in Closed Systems",
		},
	},
}

// matchesKeyword verifies whether a keyword matches text as a whole word (for short acronyms) or substring.
func matchesKeyword(text, kw string) bool {
	if len(kw) <= 2 {
		words := strings.FieldsFunc(text, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
		})
		for _, w := range words {
			if w == kw {
				return true
			}
		}
		return false
	}
	return strings.Contains(text, kw)
}

// ResolveCurriculum resolves the exam curriculum domain based on student target exam keyword analysis.
func ResolveCurriculum(targetExam string) ExamCurriculumData {
	clean := strings.ToLower(strings.TrimSpace(targetExam))
	if clean == "" {
		return standardCurricula[len(standardCurricula)-1] // general
	}

	for _, curr := range standardCurricula {
		for _, kw := range curr.Keywords {
			if matchesKeyword(clean, kw) {
				return curr
			}
		}
	}

	return standardCurricula[len(standardCurricula)-1] // fallback to general
}

// GenerateSmartContext creates context-aware prompt suggestions, topic hints, and active recall drills.
func GenerateSmartContext(
	targetExam string,
	weakTopics []string,
	strongTopics []string,
	examTraps []string,
	tutorMode string,
) dto.SmartContextResponse {
	curriculum := ResolveCurriculum(targetExam)
	examName := strings.TrimSpace(targetExam)
	if examName == "" {
		examName = curriculum.DisplayName
	}

	// 1. Build Quick Chips (Priority 1: User's weak topics; Priority 2: High-yield syllabus topics)
	quickChips := make([]dto.SmartPromptItemDto, 0, 4)
	usedTopics := make(map[string]bool)

	// User's actual weak topics
	for i, topic := range weakTopics {
		if len(quickChips) >= 4 {
			break
		}
		topic = strings.TrimSpace(topic)
		if topic == "" || usedTopics[topic] {
			continue
		}
		usedTopics[topic] = true

		switch i {
		case 0:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("🎯 %s Traps", topic),
				Prompt: fmt.Sprintf("What are the most common exam traps, sign errors, and misconceptions in %s for %s?", topic, examName),
				Icon:   "track_changes",
			})
		case 1:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("💡 Intuition: %s", topic),
				Prompt: fmt.Sprintf("Explain the core intuition and practical mental model of %s without confusing jargon.", topic),
				Icon:   "lightbulb",
			})
		case 2:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("📐 %s Formulas", topic),
				Prompt: fmt.Sprintf("Provide a structured summary of essential formulas and derivations for %s.", topic),
				Icon:   "functions",
			})
		default:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("⚡ Drill: %s", topic),
				Prompt: fmt.Sprintf("Give me 3 conceptual practice questions with step-by-step solutions for %s.", topic),
				Icon:   "bolt",
			})
		}
	}

	// Fill remaining chips with high-yield syllabus topics
	for _, topic := range curriculum.HighYieldTopics {
		if len(quickChips) >= 4 {
			break
		}
		if usedTopics[topic] {
			continue
		}
		usedTopics[topic] = true

		switch len(quickChips) {
		case 0:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("⚡ %s Overview", topic),
				Prompt: fmt.Sprintf("Explain the fundamental concepts and key theorems of %s for %s.", topic, examName),
				Icon:   "bolt",
			})
		case 1:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("📐 %s Derivation", topic),
				Prompt: fmt.Sprintf("Step-by-step breakdown and mathematical formulation for %s.", topic),
				Icon:   "functions",
			})
		case 2:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("🎯 %s Pitfalls", topic),
				Prompt: fmt.Sprintf("What are the top 3 high-yield problem-solving traps in %s?", topic),
				Icon:   "track_changes",
			})
		default:
			quickChips = append(quickChips, dto.SmartPromptItemDto{
				Label:  fmt.Sprintf("💡 %s Analogy", topic),
				Prompt: fmt.Sprintf("Explain %s using a clear, real-world intuitive analogy.", topic),
				Icon:   "lightbulb",
			})
		}
	}

	// 2. Identify top primary, secondary, and tertiary focus topics for Tutor suggestions
	primaryTopic := "Active Recall & Spaced Repetition"
	if len(weakTopics) > 0 && strings.TrimSpace(weakTopics[0]) != "" {
		primaryTopic = strings.TrimSpace(weakTopics[0])
	} else if len(curriculum.HighYieldTopics) > 0 {
		primaryTopic = curriculum.HighYieldTopics[0]
	}

	secondaryTopic := "Fundamental Laws"
	if len(weakTopics) > 1 && strings.TrimSpace(weakTopics[1]) != "" {
		secondaryTopic = strings.TrimSpace(weakTopics[1])
	} else if len(curriculum.HighYieldTopics) > 1 {
		secondaryTopic = curriculum.HighYieldTopics[1]
	}

	tertiaryTopic := "Problem Solving Traps"
	if len(weakTopics) > 2 && strings.TrimSpace(weakTopics[2]) != "" {
		tertiaryTopic = strings.TrimSpace(weakTopics[2])
	} else if len(curriculum.HighYieldTopics) > 2 {
		tertiaryTopic = curriculum.HighYieldTopics[2]
	}

	// 3. Build Tutor Suggestions tailored to the active Tutor Mode
	mode := strings.ToLower(strings.TrimSpace(tutorMode))
	if mode == "" {
		mode = "socratic"
	}

	var tutorSuggestions []dto.SmartPromptItemDto

	if strings.Contains(mode, "feynman") {
		tutorSuggestions = []dto.SmartPromptItemDto{
			{
				Label:  fmt.Sprintf("💡 Feynman: %s", primaryTopic),
				Prompt: fmt.Sprintf("Explain %s like I am 12 years old using simple everyday analogies and zero technical jargon.", primaryTopic),
				Icon:   "lightbulb",
			},
			{
				Label:  fmt.Sprintf("🔍 Everyday Analogy: %s", secondaryTopic),
				Prompt: fmt.Sprintf("What is a real-world physical analogy to understand the core mechanism of %s?", secondaryTopic),
				Icon:   "compare_arrows",
			},
			{
				Label:  fmt.Sprintf("❓ Test My Intuition: %s", primaryTopic),
				Prompt: fmt.Sprintf("Ask me a simple intuitive question to test if I truly understand %s without formulas.", primaryTopic),
				Icon:   "help",
			},
			{
				Label:  fmt.Sprintf("🔄 Simplify Complex Concept: %s", tertiaryTopic),
				Prompt: fmt.Sprintf("Break down %s into 3 simple, non-intimidating building blocks.", tertiaryTopic),
				Icon:   "layers",
			},
		}
	} else if strings.Contains(mode, "cram") || strings.Contains(mode, "exam") || strings.Contains(mode, "pyq") || strings.Contains(mode, "trap") {
		tutorSuggestions = []dto.SmartPromptItemDto{
			{
				Label:  fmt.Sprintf("🎯 High-Yield Traps: %s", primaryTopic),
				Prompt: fmt.Sprintf("What are the top 5 highest frequency exam traps and tricky questions in %s for %s?", primaryTopic, examName),
				Icon:   "warning",
			},
			{
				Label:  fmt.Sprintf("⏱️ 5-Minute PYQ Summary: %s", secondaryTopic),
				Prompt: fmt.Sprintf("Give me a rapid 5-minute revision sheet for %s with must-know formulas and PYQ shortcuts.", secondaryTopic),
				Icon:   "timer",
			},
			{
				Label:  fmt.Sprintf("⚡ Scoring Formula Cheat Sheet: %s", primaryTopic),
				Prompt: fmt.Sprintf("List the high-probability formula sheet for %s with common sign conventions and SI units.", primaryTopic),
				Icon:   "table",
			},
			{
				Label:  fmt.Sprintf("📊 Elimination Strategy: %s", tertiaryTopic),
				Prompt: fmt.Sprintf("How to quickly eliminate wrong options in multi-choice questions on %s?", tertiaryTopic),
				Icon:   "checklist",
			},
		}
	} else if strings.Contains(mode, "deriv") || strings.Contains(mode, "math") || strings.Contains(mode, "formula") {
		tutorSuggestions = []dto.SmartPromptItemDto{
			{
				Label:  fmt.Sprintf("📐 Rigorous Derivation: %s", primaryTopic),
				Prompt: fmt.Sprintf("Provide complete step-by-step LaTeX derivation of the governing equations for %s.", primaryTopic),
				Icon:   "functions",
			},
			{
				Label:  fmt.Sprintf("🔬 Physical Meaning of Variables: %s", secondaryTopic),
				Prompt: fmt.Sprintf("In %s, explain the physical and dimensional significance of each variable in the formula.", secondaryTopic),
				Icon:   "science",
			},
			{
				Label:  fmt.Sprintf("⚠️ Boundary Conditions: %s", primaryTopic),
				Prompt: fmt.Sprintf("What are the critical assumptions and boundary conditions where the standard formulas for %s break down?", primaryTopic),
				Icon:   "rule",
			},
			{
				Label:  fmt.Sprintf("📈 Graphical Interpretation: %s", tertiaryTopic),
				Prompt: fmt.Sprintf("How do the curves and slope/area properties behave in %s graphs?", tertiaryTopic),
				Icon:   "show_chart",
			},
		}
	} else {
		// Default Socratic Mode (interactive guided discovery)
		tutorSuggestions = []dto.SmartPromptItemDto{
			{
				Label:  fmt.Sprintf("🧠 Socratic Inquiry: %s", primaryTopic),
				Prompt: fmt.Sprintf("Guide me through understanding %s step-by-step using probing questions so I can deduce the laws myself.", primaryTopic),
				Icon:   "psychology",
			},
			{
				Label:  fmt.Sprintf("🎯 Common Traps: %s", secondaryTopic),
				Prompt: fmt.Sprintf("What common logical fallacies or sign errors do students make when solving %s in %s?", secondaryTopic, examName),
				Icon:   "track_changes",
			},
			{
				Label:  fmt.Sprintf("⚖️ Comparative Matrix: %s vs %s", primaryTopic, secondaryTopic),
				Prompt: fmt.Sprintf("Compare and contrast the core principles of %s and %s in a clear matrix table.", primaryTopic, secondaryTopic),
				Icon:   "table",
			},
			{
				Label:  fmt.Sprintf("💡 Core Intuition: %s", tertiaryTopic),
				Prompt: fmt.Sprintf("Explain the fundamental intuition behind %s with a concrete practical example.", tertiaryTopic),
				Icon:   "lightbulb",
			},
		}
	}

	// 4. Build Suggested Topics (Blended weak topics + syllabus up to 8 topics)
	suggestedTopics := make([]string, 0, 8)
	seen := make(map[string]bool)

	for _, w := range weakTopics {
		cleanW := strings.TrimSpace(w)
		if cleanW != "" && !seen[cleanW] {
			seen[cleanW] = true
			suggestedTopics = append(suggestedTopics, cleanW)
		}
	}
	for _, t := range curriculum.HighYieldTopics {
		cleanT := strings.TrimSpace(t)
		if cleanT != "" && !seen[cleanT] && len(suggestedTopics) < 8 {
			seen[cleanT] = true
			suggestedTopics = append(suggestedTopics, cleanT)
		}
	}
	if len(suggestedTopics) == 0 {
		suggestedTopics = curriculum.HighYieldTopics
	}

	// 5. Dynamic Input and Topic Hints
	inputHint := fmt.Sprintf("Ask any %s concept, formula, or trap...", examName)
	if len(weakTopics) > 0 && strings.TrimSpace(weakTopics[0]) != "" {
		inputHint = fmt.Sprintf("Ask about %s or any doubt...", strings.TrimSpace(weakTopics[0]))
	}

	topicHint := "e.g. Fundamental Concepts, Key Formulas..."
	if len(suggestedTopics) >= 3 {
		topicHint = fmt.Sprintf("e.g. %s, %s, %s...", suggestedTopics[0], suggestedTopics[1], suggestedTopics[2])
	} else if len(suggestedTopics) > 0 {
		topicHint = fmt.Sprintf("e.g. %s...", strings.Join(suggestedTopics, ", "))
	}

	return dto.SmartContextResponse{
		TargetExam:       examName,
		QuickChips:       quickChips,
		TutorSuggestions: tutorSuggestions,
		SuggestedTopics:  suggestedTopics,
		FocusTopic:       primaryTopic,
		InputHint:        inputHint,
		TopicHint:        topicHint,
	}
}
