package ai

import (
	"regexp"
	"strings"
)

// Greek and math symbol maps for high-speed lookup
var greekMap = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "Gamma": "Γ",
	"delta": "δ", "Delta": "Δ", "epsilon": "ε", "varepsilon": "ε",
	"zeta": "ζ", "eta": "η", "theta": "θ", "vartheta": "ϑ", "Theta": "Θ",
	"iota": "ι", "kappa": "κ", "lambda": "λ", "Lambda": "Λ",
	"mu": "μ", "nu": "ν", "xi": "ξ", "Xi": "Ξ",
	"pi": "π", "Pi": "Π", "rho": "ρ", "varrho": "ϱ",
	"sigma": "σ", "Sigma": "Σ", "tau": "τ", "upsilon": "υ", "Upsilon": "Υ",
	"phi": "φ", "varphi": "ϕ", "Phi": "Φ", "chi": "χ",
	"psi": "ψ", "Psi": "Ψ", "omega": "ω", "Omega": "Ω",
}

var operatorMap = map[string]string{
	"times": "×", "cdot": "·", "bullet": "•", "div": "÷",
	"pm": "±", "mp": "∓",
	"leq": "≤", "le": "≤", "geq": "≥", "ge": "≥",
	"neq": "≠", "ne": "≠", "approx": "≈", "equiv": "≡",
	"propto": "∝", "sim": "∼", "infty": "∞", "inf": "∞",
	"partial": "∂", "nabla": "∇",
	"in": "∈", "notin": "∉", "subset": "⊂", "subseteq": "⊆",
	"cup": "∪", "cap": "∩", "setminus": "\\",
	"forall": "∀", "exists": "∃", "nexists": "∄",
	"int": "∫", "iint": "∬", "iiint": "∭", "oint": "∮",
	"sum": "∑", "prod": "∏",
	"rightarrow": "→", "to": "→", "leftarrow": "←", "gets": "←",
	"Rightarrow": "⇒", "implies": "⇒", "Leftarrow": "⇐",
	"leftrightarrow": "↔", "Leftrightarrow": "⇔", "iff": "⇔",
	"degree": "°", "circ": "°",
	"therefore": "∴", "because": "∵",
	"angle": "∠", "perp": "⊥", "parallel": "∥",
	"hbar": "ℏ", "ell": "ℓ",
	"hat": "^", "tilde": "~",
}

var (
	escapedPunctRx = regexp.MustCompile(`\\([%&_\$#{}\\])`)
	fracRx         = regexp.MustCompile(`\\frac\{([^{}]*)\}\{([^{}]*)\}`)
	sqrtNthRx      = regexp.MustCompile(`\\sqrt\[([^\]]*)\]\{([^{}]*)\}`)
	sqrtRx         = regexp.MustCompile(`\\sqrt\{([^{}]*)\}`)
	vecRx          = regexp.MustCompile(`\\vec\{([^{}]*)\}`)
	macroWithArgRx = regexp.MustCompile(`\\[a-zA-Z]+(?:\[[^\]]*\])?\{([^{}]*)\}`)
	wordCommandRx  = regexp.MustCompile(`\\([a-zA-Z]+)`)
	multipleSpacesRx = regexp.MustCompile(`\s{2,}`)
)

// SanitizeLatex universally cleans raw text containing LaTeX commands,
// mathematical notation, or accidental backslashes.
// Unpredicted words (e.g., \cent, \example, \center) have their backslashes cleanly stripped.
func SanitizeLatex(raw string) string {
	if raw == "" || !strings.Contains(raw, `\`) {
		return raw
	}

	s := raw

	// 1. Un-escape common punctuation: \%, \&, \$, \_, \#, \{, \}
	s = escapedPunctRx.ReplaceAllString(s, `$1`)

	// 2. Remove LaTeX spacing commands and keywords
	s = strings.ReplaceAll(s, `\qquad`, "  ")
	s = strings.ReplaceAll(s, `\quad`, " ")
	s = strings.ReplaceAll(s, `\,`, " ")
	s = strings.ReplaceAll(s, `\;`, " ")
	s = strings.ReplaceAll(s, `\:`, " ")
	s = strings.ReplaceAll(s, `\!`, "")
	s = strings.ReplaceAll(s, `\ `, " ")
	s = strings.ReplaceAll(s, `\left`, "")
	s = strings.ReplaceAll(s, `\right`, "")
	s = strings.ReplaceAll(s, `\displaystyle`, "")

	// 3. Known compound structures: Fractions & Roots
	s = fracRx.ReplaceAllString(s, `($1 / $2)`)
	s = sqrtNthRx.ReplaceAllString(s, `$1√($2)`)
	s = sqrtRx.ReplaceAllString(s, `√($1)`)
	s = strings.ReplaceAll(s, `\sqrt`, "√")

	// 4. Vectors
	s = vecRx.ReplaceAllString(s, `$1⃗`)

	// 5. Unpack any macro with arguments: \text{...}, \example{...}, \bold{...}
	for i := 0; i < 5; i++ {
		if !macroWithArgRx.MatchString(s) {
			break
		}
		s = macroWithArgRx.ReplaceAllString(s, `$1`)
	}

	// 6. Universal Word Command Resolver
	s = wordCommandRx.ReplaceAllStringFunc(s, func(match string) string {
		word := match[1:] // strip leading \
		if sym, ok := greekMap[word]; ok {
			return sym
		}
		if op, ok := operatorMap[word]; ok {
			return op
		}
		// Universal Fallback: Strip the backslash and preserve the natural word
		return word
	})

	// 7. Strip isolated KaTeX delimiters
	s = strings.ReplaceAll(s, `\(`, "")
	s = strings.ReplaceAll(s, `\)`, "")
	s = strings.ReplaceAll(s, `\[`, "")
	s = strings.ReplaceAll(s, `\]`, "")
	s = strings.ReplaceAll(s, `$$`, "")

	// 8. Clean up spaces
	s = multipleSpacesRx.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

var (
	codeBlockRx      = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9_-]+)?\n.*?```")
	inlineCodeRx     = regexp.MustCompile("`([^`]+)`")
	stepPauseRx      = regexp.MustCompile(`(?i)\bStep\s*([0-9]+)\s*:\s*`)
	pedagogicalRx    = regexp.MustCompile(`(?i)\b(Therefore|Hence|Furthermore|In other words|Notice that|Keep in mind|Remember|Specifically|First|Secondly|Finally)\s*:\s*`)
	tableSeparatorRx = regexp.MustCompile(`\|[-:\s|]+\|`)
	percentageRx     = regexp.MustCompile(`([0-9.]+)\s*%`)

	// Markdown Formatting & Blockquotes
	blockquoteRx     = regexp.MustCompile(`(?m)^\s*>+\s*(?:\[[!a-zA-Z_-]+\])?\s*`)
	headerRx         = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	bulletRx         = regexp.MustCompile(`(?m)^\s*[-*+•]\s+`)
	numberedListRx   = regexp.MustCompile(`(?m)^\s*([0-9]+)\.\s+`)
	boldRx           = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRx         = regexp.MustCompile(`\*([^*]+)\*`)
	strikethroughRx  = regexp.MustCompile(`~~([^~]+)~~`)
	htmlTagRx        = regexp.MustCompile(`<[^>]+>`)
	katexDelimiterRx = regexp.MustCompile(`(\$\$|\$|\\\(|\\\)|\\\[|\\\])`)
	latexEnvRx       = regexp.MustCompile(`\\(?:begin|end)\{[a-zA-Z0-9*]+\}`)

	// Calculus & Limits
	limWithCondRx  = regexp.MustCompile(`\\lim_\{([^}]+)\}`)
	intDefiniteRx  = regexp.MustCompile(`\\int_\{([^}]+)\}\^\{([^}]+)\}`)
	intSingleDefRx = regexp.MustCompile(`\\int_([0-9a-zA-Z])\^([0-9a-zA-Z])`)
	sumDefiniteRx  = regexp.MustCompile(`\\sum_\{([^}]+)\}\^\{([^}]+)\}`)
	sumSingleDefRx = regexp.MustCompile(`\\sum_([0-9a-zA-Z])\^([0-9a-zA-Z])`)

	// Powers & Subscripts
	squaredRx     = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{?2\}?`)
	cubedRx       = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{?3\}?`)
	powerRx       = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{([^}]+)\}`)
	powerSingleRx = regexp.MustCompile(`([a-zA-Z0-9)\]])\^([a-zA-Z0-9+\-])`)
	subscriptRx   = regexp.MustCompile(`([a-zA-Z])_\{?([0-9a-zA-Z]+)\}?`)

	// Physics Units
	accelUnitRx = regexp.MustCompile(`\b(?:m/s\^?2|m/s²|m\s*s\^?-2)\b`)
	speedUnitRx = regexp.MustCompile(`\b(?:m/s|km/h|km/hr)\b`)

	// Algorithmic Big-O
	bigOComplexRx = regexp.MustCompile(`(?i)O\(\s*(?:\\log|log)\s*N\s*\)`)
	bigONlogNRx   = regexp.MustCompile(`(?i)O\(\s*N\s*(?:\\log|log)\s*N\s*\)`)
	bigO1Rx       = regexp.MustCompile(`(?i)O\(\s*1\s*\)`)
	bigONRx       = regexp.MustCompile(`(?i)O\(\s*N\s*\)`)
	bigON2Rx      = regexp.MustCompile(`(?i)O\(\s*N\s*\^?\s*2\s*\)`)
	bigO2nRx      = regexp.MustCompile(`(?i)O\(\s*2\s*\^\s*[nN]\s*\)`)
	bigONFactRx   = regexp.MustCompile(`(?i)O\(\s*[nN]!\s*\)`)
)

// extractBracedContent scans from startIdx (which must point to '{') and returns the slice inside the balanced braces,
// plus the index right after the closing '}'. If braces are not balanced, returns ("", -1).
func extractBracedContent(s string, startIdx int) (string, int) {
	if startIdx < 0 || startIdx >= len(s) || s[startIdx] != '{' {
		return "", -1
	}
	depth := 0
	for i := startIdx; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[startIdx+1 : i], i + 1
			}
		}
	}
	return "", -1
}

// replaceBalancedFrac recursively converts any \frac{num}{den} into natural spoken "(num divided by den)".
// Handles arbitrary nested fractions and formulas with balanced curly braces without regex limits.
func replaceBalancedFrac(s string) string {
	limit := 10 // avoid pathological infinite loops
	for limit > 0 {
		idx := strings.Index(s, `\frac{`)
		if idx == -1 {
			idx = strings.Index(s, `\frac {`)
			if idx == -1 {
				break
			}
		}
		open1 := strings.Index(s[idx:], "{") + idx
		num, after1 := extractBracedContent(s, open1)
		if after1 == -1 {
			break
		}
		open2 := after1
		for open2 < len(s) && (s[open2] == ' ' || s[open2] == '\t') {
			open2++
		}
		if open2 >= len(s) || s[open2] != '{' {
			break
		}
		den, after2 := extractBracedContent(s, open2)
		if after2 == -1 {
			break
		}

		cleanNum := replaceBalancedFrac(num)
		cleanDen := replaceBalancedFrac(den)
		replacement := "(" + strings.TrimSpace(cleanNum) + " divided by " + strings.TrimSpace(cleanDen) + ")"
		s = s[:idx] + replacement + s[after2:]
		limit--
	}
	return s
}

// replaceBalancedSqrt converts \sqrt[n]{content} and \sqrt{content} into natural spoken root phrases.
func replaceBalancedSqrt(s string) string {
	limit := 10
	for limit > 0 {
		idx := strings.Index(s, `\sqrt`)
		if idx == -1 {
			break
		}
		afterCmd := idx + 5
		rootDegree := ""
		if afterCmd < len(s) && s[afterCmd] == '[' {
			closeBracket := strings.Index(s[afterCmd:], "]")
			if closeBracket != -1 {
				rootDegree = s[afterCmd+1 : afterCmd+closeBracket]
				afterCmd = afterCmd + closeBracket + 1
			}
		}
		for afterCmd < len(s) && (s[afterCmd] == ' ' || s[afterCmd] == '\t') {
			afterCmd++
		}
		if afterCmd < len(s) && s[afterCmd] == '{' {
			content, afterBrace := extractBracedContent(s, afterCmd)
			if afterBrace != -1 {
				cleanContent := replaceBalancedSqrt(content)
				var replacement string
				if rootDegree == "3" || rootDegree == "cube" {
					replacement = "cube root of (" + strings.TrimSpace(cleanContent) + ")"
				} else if rootDegree != "" {
					replacement = rootDegree + " root of (" + strings.TrimSpace(cleanContent) + ")"
				} else {
					replacement = "square root of (" + strings.TrimSpace(cleanContent) + ")"
				}
				s = s[:idx] + replacement + s[afterBrace:]
				limit--
				continue
			}
		}
		s = s[:idx] + "square root of " + s[afterCmd:]
		limit--
	}
	return s
}

// unwrapMacros strips LaTeX packaging macros like \text{...}, \mathrm{...}, \mathbf{...}, \boxed{...}, etc.,
// keeping only their clean inner words so Edge-TTS never pronounces "text", "mathrm", "boxed", etc.
func unwrapMacros(s string) string {
	macros := []string{
		`\text{`, `\textbf{`, `\textit{`, `\mathrm{`, `\mathbf{`, `\mathit{`,
		`\operatorname{`, `\cancel{`, `\boxed{`, `\underline{`, `\overline{`,
		`\displaystyle{`, `\large{`, `\huge{`, `\small{`, `\bold{`,
	}
	for _, m := range macros {
		for {
			idx := strings.Index(s, m)
			if idx == -1 {
				break
			}
			openBrace := idx + len(m) - 1
			content, after := extractBracedContent(s, openBrace)
			if after == -1 {
				break
			}
			s = s[:idx] + " " + content + " " + s[after:]
		}
	}
	return s
}

// GenerateSpokenTranscript converts complex academic markdown, formulas, LaTeX commands,
// and tables into a clean, smooth, natural spoken English & Hinglish transcript.
// Offloading this from mobile clients eliminates heavy regex execution on low-end phones.
func GenerateSpokenTranscript(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	s := text

	// 1. Remove code blocks & JSON diagrams (never read syntax soup)
	s = codeBlockRx.ReplaceAllString(s, " Here is the code implementation. ")

	// 2. Pedagogical pauses & Step boundaries (natural teacher cadence)
	s = stepPauseRx.ReplaceAllString(s, ". Step $1. ")
	s = pedagogicalRx.ReplaceAllString(s, "$1, ")

	// 3. Strip Markdown Blockquotes (never vocalize '>' as 'greater than')
	s = blockquoteRx.ReplaceAllString(s, "")

	// 4. Translate Visual Study Emojis to warm, polite teacher cues
	s = strings.ReplaceAll(s, "💡", ". Pro tip: ")
	s = strings.ReplaceAll(s, "⚠️", ". Watch out, exam trap warning: ")
	s = strings.ReplaceAll(s, "🎯", ". Key exam takeaway: ")
	s = strings.ReplaceAll(s, "🧠", ". Intuition: ")
	s = strings.ReplaceAll(s, "📌", ". Key concept: ")
	s = strings.ReplaceAll(s, "✅", ". Correct: ")
	s = strings.ReplaceAll(s, "❌", ". Misconception: ")
	s = strings.ReplaceAll(s, "🔥", ". High yield topic: ")
	s = strings.ReplaceAll(s, "🚀", ". Fast shortcut: ")
	s = strings.ReplaceAll(s, "⭐", ". Important: ")
	s = strings.ReplaceAll(s, "✨", " ")

	// 5. Academic Abbreviations
	s = strings.ReplaceAll(s, "w.r.t.", "with respect to ")
	s = strings.ReplaceAll(s, "w.r.t", "with respect to ")
	s = strings.ReplaceAll(s, "e.g.,", "for example, ")
	s = strings.ReplaceAll(s, "e.g.", "for example, ")
	s = strings.ReplaceAll(s, "i.e.,", "that is, ")
	s = strings.ReplaceAll(s, "i.e.", "that is, ")
	s = strings.ReplaceAll(s, "etc.", "etcetera ")
	s = strings.ReplaceAll(s, "approx.", "approximately ")
	s = strings.ReplaceAll(s, "eqn.", "equation ")
	s = strings.ReplaceAll(s, "eq.", "equation ")
	s = strings.ReplaceAll(s, "soln.", "solution ")
	s = strings.ReplaceAll(s, "vs.", "versus ")
	s = strings.ReplaceAll(s, "max.", "maximum ")
	s = strings.ReplaceAll(s, "min.", "minimum ")
	s = strings.ReplaceAll(s, "const.", "constant ")
	s = strings.ReplaceAll(s, "temp.", "temperature ")
	s = strings.ReplaceAll(s, "diff.", "differentiation ")

	// 6. Markdown tables
	s = tableSeparatorRx.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "|", ", ")

	// 7. Strip LaTeX Environments and clean alignment/newlines
	s = latexEnvRx.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, `\\`, ". ")
	s = strings.ReplaceAll(s, `&=`, " = ")
	s = strings.ReplaceAll(s, `&`, " ")

	// 8. Unwrap LaTeX text & formatting macros (\text{...}, \mathrm{...}, \boxed{...})
	s = unwrapMacros(s)

	// 9. Common simple fractions & percentages
	s = strings.ReplaceAll(s, "1/2", "one half")
	s = strings.ReplaceAll(s, "1/4", "one fourth")
	s = strings.ReplaceAll(s, "3/4", "three fourths")
	s = strings.ReplaceAll(s, "1/3", "one third")
	s = strings.ReplaceAll(s, "2/3", "two thirds")
	s = percentageRx.ReplaceAllString(s, "$1 percent")

	// 10. Calculus limits, integrals, summations
	s = limWithCondRx.ReplaceAllStringFunc(s, func(m string) string {
		sub := limWithCondRx.FindStringSubmatch(m)
		if len(sub) > 1 {
			cond := strings.ReplaceAll(sub[1], `\to`, " approaches ")
			cond = strings.ReplaceAll(cond, `\rightarrow`, " approaches ")
			cond = strings.ReplaceAll(cond, "→", " approaches ")
			return "limit as " + cond + " of "
		}
		return "limit of "
	})
	s = strings.ReplaceAll(s, `\lim`, "limit of ")

	s = intDefiniteRx.ReplaceAllString(s, "definite integral from $1 to $2 of ")
	s = intSingleDefRx.ReplaceAllString(s, "definite integral from $1 to $2 of ")
	s = strings.ReplaceAll(s, `\iint`, "double integral of ")
	s = strings.ReplaceAll(s, `\iiint`, "triple integral of ")
	s = strings.ReplaceAll(s, `\oint`, "contour integral of ")
	s = strings.ReplaceAll(s, `\int`, "integral of ")

	s = sumDefiniteRx.ReplaceAllString(s, "summation from $1 to $2 of ")
	s = sumSingleDefRx.ReplaceAllString(s, "summation from $1 to $2 of ")
	s = strings.ReplaceAll(s, `\sum`, "summation of ")
	s = strings.ReplaceAll(s, `\prod`, "product of ")

	s = strings.ReplaceAll(s, `\partial`, "partial derivative of ")
	s = strings.ReplaceAll(s, `\nabla`, "del ")

	// 11. Robust Balanced Brace Fractions & Roots
	s = replaceBalancedFrac(s)
	s = replaceBalancedSqrt(s)

	// 12. Powers / Exponents
	s = squaredRx.ReplaceAllString(s, "$1 squared")
	s = cubedRx.ReplaceAllString(s, "$1 cubed")
	s = powerRx.ReplaceAllString(s, "$1 to the power $2")
	s = powerSingleRx.ReplaceAllString(s, "$1 to the power $2")

	// 13. Chemical Reactions, Compounds & States (handled before generic subscripts)
	heatRx := regexp.MustCompile(`(?i)\\xrightarrow\{(?:\\Delta|heat)\}`)
	s = heatRx.ReplaceAllString(s, " reacts under heat to form ")
	s = strings.ReplaceAll(s, `\rightleftharpoons`, " is in dynamic reversible equilibrium with ")
	s = strings.ReplaceAll(s, `⇌`, " is in dynamic reversible equilibrium with ")
	s = strings.ReplaceAll(s, "(aq)", " aqueous ")
	s = strings.ReplaceAll(s, "(s)", " solid ")
	s = strings.ReplaceAll(s, "(l)", " liquid ")
	s = strings.ReplaceAll(s, "(g)", " gas ")

	// Common Chemistry Compounds
	s = strings.ReplaceAll(s, "CaCO3", "calcium carbonate, C a C O 3")
	s = strings.ReplaceAll(s, "CaCO_3", "calcium carbonate, C a C O 3")
	s = strings.ReplaceAll(s, "CaO", "calcium oxide, C a O")
	s = strings.ReplaceAll(s, "NaCl", "sodium chloride, N A C L")
	s = strings.ReplaceAll(s, "HCl", "hydrochloric acid, H C L")
	s = strings.ReplaceAll(s, "H2SO4", "sulfuric acid, H 2 S O 4")
	s = strings.ReplaceAll(s, "H_2SO_4", "sulfuric acid, H 2 S O 4")
	s = strings.ReplaceAll(s, "HNO3", "nitric acid, H N O 3")
	s = strings.ReplaceAll(s, "NaOH", "sodium hydroxide, N A O H")
	s = strings.ReplaceAll(s, "KOH", "potassium hydroxide, K O H")
	s = strings.ReplaceAll(s, "CH4", "methane, C H 4")
	s = strings.ReplaceAll(s, "NH3", "ammonia, N H 3")
	s = strings.ReplaceAll(s, "CO2", "carbon dioxide, C O 2")
	s = strings.ReplaceAll(s, "CO_2", "carbon dioxide, C O 2")
	s = strings.ReplaceAll(s, "H2O", "water, H 2 O")
	s = strings.ReplaceAll(s, "H_2O", "water, H 2 O")

	// 14. Physics Subscript Variables
	s = strings.ReplaceAll(s, "v_0", "v naught")
	s = strings.ReplaceAll(s, "v_o", "v naught")
	s = strings.ReplaceAll(s, "v_i", "v initial")
	s = strings.ReplaceAll(s, "v_f", "v final")
	s = strings.ReplaceAll(s, "x_0", "x naught")
	s = strings.ReplaceAll(s, "t_0", "t naught")
	s = strings.ReplaceAll(s, "a_net", "a net")
	s = strings.ReplaceAll(s, "F_net", "F net")
	s = strings.ReplaceAll(s, "m_total", "total mass")
	s = subscriptRx.ReplaceAllString(s, "$1 sub $2")

	// 15. Vectors & Unit Vectors
	vecRegex := regexp.MustCompile(`\\vec\{?([a-zA-Z0-9]+)\}?`)
	s = vecRegex.ReplaceAllString(s, "vector $1 ")
	s = strings.ReplaceAll(s, `\hat{i}`, "unit vector i ")
	s = strings.ReplaceAll(s, `\hat{j}`, "unit vector j ")
	s = strings.ReplaceAll(s, `\hat{k}`, "unit vector k ")

	// 16. Algorithmic Big-O
	s = bigONlogNRx.ReplaceAllString(s, "O of N log N linearithmic time")
	s = bigOComplexRx.ReplaceAllString(s, "O of log N logarithmic time")
	s = bigO1Rx.ReplaceAllString(s, "O of 1 constant time")
	s = bigONRx.ReplaceAllString(s, "O of N linear time")
	s = bigON2Rx.ReplaceAllString(s, "O of N squared quadratic time")
	s = bigO2nRx.ReplaceAllString(s, "O of 2 to the power n exponential time")
	s = bigONFactRx.ReplaceAllString(s, "O of n factorial time")

	// 17. Physics SI Units & Dimensions
	s = accelUnitRx.ReplaceAllString(s, " meters per second squared ")
	s = speedUnitRx.ReplaceAllString(s, " meters per second ")
	s = strings.ReplaceAll(s, `\Omega`, " ohms ")
	s = strings.ReplaceAll(s, `k\Omega`, " kiloohms ")
	s = strings.ReplaceAll(s, `M\Omega`, " megaohms ")
	s = strings.ReplaceAll(s, `\mu F`, " microfarads ")
	s = strings.ReplaceAll(s, `\mu C`, " microcoulombs ")
	s = strings.ReplaceAll(s, "kJ/mol", " kilojoules per mole ")
	s = strings.ReplaceAll(s, "J/mol", " joules per mole ")
	s = strings.ReplaceAll(s, "mol/L", " moles per liter ")
	s = strings.ReplaceAll(s, "°C", " degrees Celsius ")
	s = strings.ReplaceAll(s, "°F", " degrees Fahrenheit ")

	// 18. Greek letters and math operators (words)
	s = wordCommandRx.ReplaceAllStringFunc(s, func(match string) string {
		word := match[1:]
		switch word {
		case "alpha", "beta", "gamma", "delta", "epsilon", "theta", "lambda", "mu", "pi", "sigma", "tau", "omega", "phi", "psi":
			return word + " "
		case "leq", "le":
			return " is less than or equal to "
		case "geq", "ge":
			return " is greater than or equal to "
		case "neq", "ne":
			return " is not equal to "
		case "pm":
			return " plus or minus "
		case "mp":
			return " minus or plus "
		case "times":
			return " times "
		case "cdot":
			return " dot "
		case "div":
			return " divided by "
		case "approx":
			return " is approximately equal to "
		case "equiv":
			return " is equivalent to "
		case "propto":
			return " is directly proportional to "
		case "infty", "inf":
			return " infinity "
		case "to", "rightarrow":
			return " approaches "
		case "leftarrow":
			return " from "
		case "Rightarrow", "implies":
			return " implies that "
		case "therefore":
			return " therefore, "
		case "because":
			return " because "
		case "degree", "circ":
			return " degrees "
		case "hat":
			return " unit vector "
		case "vec":
			return " vector "
		case "nabla":
			return " del "
		case "partial":
			return " partial derivative of "
		case "text", "mathrm", "mathbf", "displaystyle", "left", "right":
			return ""
		default:
			return word + " "
		}
	})

	// 19. Markdown syntax stripping
	s = headerRx.ReplaceAllString(s, ". ")
	s = bulletRx.ReplaceAllString(s, ". ")
	s = numberedListRx.ReplaceAllString(s, ". Step $1. ")
	s = boldRx.ReplaceAllString(s, " $1 ")
	s = italicRx.ReplaceAllString(s, " $1 ")
	s = strikethroughRx.ReplaceAllString(s, " $1 ")
	s = inlineCodeRx.ReplaceAllString(s, " $1 ")
	s = htmlTagRx.ReplaceAllString(s, " ")
	s = katexDelimiterRx.ReplaceAllString(s, " ")

	// 20. Strict Zero-Leakage: Strip raw backslashes and special chars
	s = strings.ReplaceAll(s, `\`, "")
	s = strings.ReplaceAll(s, `$`, "")
	s = strings.ReplaceAll(s, `*`, "")
	s = strings.ReplaceAll(s, `#`, "")
	s = strings.ReplaceAll(s, `{`, " ")
	s = strings.ReplaceAll(s, `}`, " ")
	s = strings.ReplaceAll(s, `~`, " ")
	s = strings.ReplaceAll(s, `^`, " to the power of ")
	s = strings.ReplaceAll(s, `_`, " ")

	// 21. Clean redundant spaces and newlines
	s = multipleSpacesRx.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " . ", ". ")
	return strings.TrimSpace(s)
}
