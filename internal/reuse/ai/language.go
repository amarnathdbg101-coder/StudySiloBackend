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
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT HINGLISH RESPONSE MANDATORY 🚨
================================================================================
- Target Language: HINGLISH ONLY (Conversational Hindi written in English / Roman script, Feynman style).
- ABSOLUTELY FORBIDDEN: Writing the explanation in pure English paragraphs!
  ❌ NEVER DO THIS: "Merge sort is an efficient sorting algorithm based on divide and conquer..."
  ✅ ALWAYS DO THIS: "Merge sort ek bahut hi efficient divide-and-conquer algorithm hai. Iska basic funda ye hai ki array ko tab tak recursively aadha-aadha todte raho jab tak single elements na bachein, aur fir unhe sorted order me merge kardo."
- Step-by-Step Simplicity: Explain the concept step-by-step using simple, friendly Hinglish words.
- Technical Keywords: Standard technical words, variable names, and LaTeX formulas are kept in English notation (e.g. O(N log N), arr[mid], recursion), but all sentences explaining them MUST be in natural Hinglish.
- Absolute Rule: Even if the student's query is in 100% English (e.g., "Explain merge sort"), you MUST ignore the English query language and respond STRICTLY in HINGLISH as chosen in student settings.
================================================================================`

	case "english":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT ENGLISH MANDATORY 🚨
================================================================================
- Target Language: ENGLISH ONLY.
- Deliver your ENTIRE explanation, steps, and formulas in clear, professional, academic English.
- Even if the student asked in Hindi or Hinglish, your response MUST be exclusively in English.
================================================================================`

	case "hindi":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT HINDI (DEVANAGARI) MANDATORY 🚨
================================================================================
- Target Language: HINDI ONLY in Devanagari script (हिन्दी).
- Deliver your ENTIRE explanation in clean, student-friendly Hindi.
- Standard scientific terms (e.g. CPU, Algorithm, Big-O, Entropy) can remain in standard notation alongside Hindi explanations.
- Even if the student asked in English or Hinglish, your response MUST be in Hindi (Devanagari).
================================================================================`

	case "bengali":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT BENGALI (বাংলা) MANDATORY 🚨
================================================================================
- Target Language: BENGALI ONLY (বাংলা).
- Deliver your ENTIRE explanation in clear, student-friendly Bengali.
- Even if the student asked in another language, your response MUST be in Bengali.
================================================================================`

	case "marathi":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT MARATHI (मराठी) MANDATORY 🚨
================================================================================
- Target Language: MARATHI ONLY (मराठी).
- Deliver your ENTIRE explanation in clear, student-friendly Marathi.
- Even if the student asked in another language, your response MUST be in Marathi.
================================================================================`

	case "tamil":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT TAMIL (தமிழ்) MANDATORY 🚨
================================================================================
- Target Language: TAMIL ONLY (தமிழ்).
- Deliver your ENTIRE explanation in clear, student-friendly Tamil.
- Even if the student asked in another language, your response MUST be in Tamil.
================================================================================`

	case "telugu":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT TELUGU (తెలుగు) MANDATORY 🚨
================================================================================
- Target Language: TELUGU ONLY (తెలుగు).
- Deliver your ENTIRE explanation in clear, student-friendly Telugu.
- Even if the student asked in another language, your response MUST be in Telugu.
================================================================================`

	case "gujarati":
		return `================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT GUJARATI (ગુજરાતી) MANDATORY 🚨
================================================================================
- Target Language: GUJARATI ONLY (ગુજરાતી).
- Deliver your ENTIRE explanation in clear, student-friendly Gujarati.
- Even if the student asked in another language, your response MUST be in Gujarati.
================================================================================`

	case "auto", "all", "":
		return `LANGUAGE & TONE DIRECTIVE (DYNAMIC USER TONE MATCHING):
- Analyze and detect the language and tone used by the student in their prompt (e.g. Hinglish, English, Hindi, etc.).
- You MUST respond in that EXACT SAME language and conversational tone that the student used in their prompt.`

	default:
		return fmt.Sprintf(`================================================================================
🚨 CRITICAL OVERRIDING DIRECTIVE: STRICT %s MANDATORY 🚨
================================================================================
- Target Language: %s.
- Deliver your ENTIRE explanation in clear, student-friendly %s.
- Even if the student asked in another language, your response MUST be in %s.
================================================================================`, strings.ToUpper(normalized), normalized, normalized, normalized)
	}
}
