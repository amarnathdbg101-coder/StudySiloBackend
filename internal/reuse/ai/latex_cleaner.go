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

	// Calculus & Higher Math
	limWithCondRx  = regexp.MustCompile(`\\lim_\{([^}]+)\}`)
	intDefiniteRx  = regexp.MustCompile(`\\int_\{([^}]+)\}\^\{([^}]+)\}`)
	intSingleDefRx = regexp.MustCompile(`\\int_([0-9a-zA-Z])\^([0-9a-zA-Z])`)
	sumDefiniteRx  = regexp.MustCompile(`\\sum_\{([^}]+)\}\^\{([^}]+)\}`)

	// Fractions & Roots
	fracSpeechRx    = regexp.MustCompile(`\\frac\{([^{}]*)\}\{([^{}]*)\}`)
	sqrtNthSpeechRx = regexp.MustCompile(`\\sqrt\[([^\]]*)\]\{([^{}]*)\}`)
	sqrtSpeechRx    = regexp.MustCompile(`\\sqrt\{([^{}]*)\}`)

	// Superscripts / Powers
	squaredRx     = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{?2\}?`)
	cubedRx       = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{?3\}?`)
	powerRx       = regexp.MustCompile(`([a-zA-Z0-9)\]])\^\{([^}]+)\}`)
	powerSingleRx = regexp.MustCompile(`([a-zA-Z0-9)\]])\^([a-zA-Z0-9])`)
	subscriptRx   = regexp.MustCompile(`([a-zA-Z])_\{?([0-9a-zA-Z]+)\}?`)

	// Physics Units & Dimensions
	accelUnitRx = regexp.MustCompile(`\b(m/s\^2|m\s*s\^-2|m/s2)\b`)
	speedUnitRx = regexp.MustCompile(`\b(m/s|km/h|km/hr)\b`)

	// Markdown Formatting
	headerRx         = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	bulletRx         = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)
	numberedListRx   = regexp.MustCompile(`(?m)^\s*([0-9]+)\.\s+`)
	boldRx           = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRx         = regexp.MustCompile(`\*([^*]+)\*`)
	strikethroughRx  = regexp.MustCompile(`~~([^~]+)~~`)
	htmlTagRx        = regexp.MustCompile(`<[^>]+>`)
	katexDelimiterRx = regexp.MustCompile(`(\$\$|\$|\\\(|\\\)|\\\[|\\\])`)
	cleanPunctRx     = regexp.MustCompile(`\\([%&_\$#{}\\])`)
)

// GenerateSpokenTranscript converts complex academic markdown, formulas, LaTeX commands,
// and tables into a clean, smooth, natural spoken English & Hinglish transcript.
// Offloading this from mobile clients eliminates heavy regex execution on low-end phones.
func GenerateSpokenTranscript(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	s := text

	// 1. Remove code blocks & JSON diagrams
	s = codeBlockRx.ReplaceAllString(s, " Here is the code implementation. ")

	// 2. Pedagogical pauses & Step boundaries
	s = stepPauseRx.ReplaceAllString(s, ". Step $1. ")
	s = pedagogicalRx.ReplaceAllString(s, "$1, ")

	// 3. Academic Abbreviations
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

	// 4. Markdown tables
	s = tableSeparatorRx.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "|", ", ")

	// 5. Common fractions & percentages
	s = strings.ReplaceAll(s, "1/2", "one half")
	s = strings.ReplaceAll(s, "1/4", "one fourth")
	s = strings.ReplaceAll(s, "3/4", "three fourths")
	s = strings.ReplaceAll(s, "1/3", "one third")
	s = strings.ReplaceAll(s, "2/3", "two thirds")
	s = percentageRx.ReplaceAllString(s, "$1 percent")

	// 6. Calculus limits, integrals, summations
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
	s = strings.ReplaceAll(s, `\sum`, "summation of ")
	s = strings.ReplaceAll(s, `\prod`, "product of ")

	// 7. Fractions & Roots
	for i := 0; i < 4; i++ {
		if !fracSpeechRx.MatchString(s) {
			break
		}
		s = fracSpeechRx.ReplaceAllString(s, "($1 divided by $2)")
	}
	s = sqrtNthSpeechRx.ReplaceAllString(s, "$1 root of ($2)")
	s = sqrtSpeechRx.ReplaceAllString(s, "square root of ($1)")
	s = strings.ReplaceAll(s, `\sqrt`, "square root of ")

	// 8. Powers / Exponents
	s = squaredRx.ReplaceAllString(s, "$1 squared")
	s = cubedRx.ReplaceAllString(s, "$1 cubed")
	s = powerRx.ReplaceAllString(s, "$1 to the power $2")
	s = powerSingleRx.ReplaceAllString(s, "$1 to the power $2")
	s = subscriptRx.ReplaceAllString(s, "$1 sub $2")

	// 9. Physics Units
	s = accelUnitRx.ReplaceAllString(s, " meters per second squared ")
	s = speedUnitRx.ReplaceAllString(s, " meters per second ")

	// 10. Greek letters and math operators (words)
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
		default:
			return word + " "
		}
	})

	// 11. Markdown syntax stripping
	s = headerRx.ReplaceAllString(s, "")
	s = bulletRx.ReplaceAllString(s, ". ")
	s = numberedListRx.ReplaceAllString(s, ". Step $1. ")
	s = boldRx.ReplaceAllString(s, " $1 ")
	s = italicRx.ReplaceAllString(s, " $1 ")
	s = strikethroughRx.ReplaceAllString(s, " $1 ")
	s = inlineCodeRx.ReplaceAllString(s, " $1 ")
	s = htmlTagRx.ReplaceAllString(s, " ")
	s = katexDelimiterRx.ReplaceAllString(s, " ")
	s = cleanPunctRx.ReplaceAllString(s, "$1")

	// 12. Strip backslashes and special chars
	s = strings.ReplaceAll(s, `\`, "")
	s = strings.ReplaceAll(s, `$`, "")
	s = strings.ReplaceAll(s, `*`, "")
	s = strings.ReplaceAll(s, `#`, "")
	s = strings.ReplaceAll(s, `{`, " ")
	s = strings.ReplaceAll(s, `}`, " ")

	// 13. Clean redundant spaces and newlines
	s = multipleSpacesRx.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
