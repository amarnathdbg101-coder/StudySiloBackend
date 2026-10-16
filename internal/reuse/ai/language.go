package ai

import (
	"fmt"
	"strings"
)

// BuildLanguageInstruction generates strict, robust language and tone directives for AI prompts.
// If lang is "auto", "all", or empty, AI dynamically matches the student's request language and tone.
// If a specific language is selected (e.g. "hinglish", "english", "hindi", etc.), AI MUST respond exclusively
// in that specified language, completely overriding whatever language the student typed their prompt in.
func BuildLanguageInstruction(lang string) string {
	normalized := strings.ToLower(strings.TrimSpace(lang))

	switch normalized {
	case "hinglish":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT HINGLISH):
- You MUST write your ENTIRE explanation, point-by-point breakdown, definitions, formulas context, traps, and feedback STRICTLY in simple, natural HINGLISH (everyday conversational Hindi written in the English / Roman alphabet, Feynman technique style).
- DO NOT use English paragraphs.
- DO NOT use Devanagari script.
- Even if the student asked their question in pure English or pure Hindi, your response MUST be exclusively in simple, conversational Hinglish.`

	case "english":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT ENGLISH):
- You MUST write your ENTIRE response strictly in clear, professional, fluent ENGLISH.
- Even if the student asked their question in Hindi or Hinglish, your response MUST be exclusively in English.`

	case "hindi":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT HINDI):
- You MUST write your ENTIRE response strictly in clear, academic HINDI using Devanagari script (हिन्दी).
- Standard scientific/technical terms (e.g. CPU, Algorithm, Big-O, Entropy) may be kept in their standard technical notation alongside Hindi explanation.
- Even if the student asked in English or Hinglish, your response MUST be in Hindi (Devanagari).`

	case "bengali":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT BENGALI):
- You MUST write your ENTIRE response strictly in clear, academic BENGALI (বাংলা).
- Even if the student asked in another language, your response MUST be in Bengali.`

	case "marathi":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT MARATHI):
- You MUST write your ENTIRE response strictly in clear, academic MARATHI (मराठी).
- Even if the student asked in another language, your response MUST be in Marathi.`

	case "tamil":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT TAMIL):
- You MUST write your ENTIRE response strictly in clear TAMIL (தமிழ்).
- Even if the student asked in another language, your response MUST be in Tamil.`

	case "telugu":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT TELUGU):
- You MUST write your ENTIRE response strictly in clear TELUGU (తెలుగు).
- Even if the student asked in another language, your response MUST be in Telugu.`

	case "gujarati":
		return `MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT GUJARATI):
- You MUST write your ENTIRE response strictly in clear GUJARATI (ગુજરાતી).
- Even if the student asked in another language, your response MUST be in Gujarati.`

	case "auto", "all", "":
		return `LANGUAGE & TONE DIRECTIVE (DYNAMIC USER TONE MATCHING):
- Analyze and detect the language and tone used by the student in their prompt (e.g. Hinglish, English, Hindi, etc.).
- You MUST respond in that EXACT SAME language and conversational tone that the student used in their prompt.`

	default:
		return fmt.Sprintf(`MANDATORY LANGUAGE & TONE DIRECTIVE (STRICT %s):
- You MUST write your ENTIRE response strictly in clear, fluent %s.
- Even if the student asked in another language, your response MUST be in %s.`, strings.ToUpper(normalized), normalized, normalized)
	}
}
