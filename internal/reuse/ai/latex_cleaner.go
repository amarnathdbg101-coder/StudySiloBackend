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
