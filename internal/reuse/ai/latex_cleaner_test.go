package ai

import (
	"testing"
)

func TestSanitizeLatexUniversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Arbitrary unpredicted words like cent and example",
			input:    `For \example: 50 \cent`,
			expected: "For example: 50 cent",
		},
		{
			name:     "Greek symbols and operators",
			input:    `\alpha + \beta = \gamma and \tau = r \times F`,
			expected: "α + β = γ and τ = r × F",
		},
		{
			name:     "Nested macros with braces",
			input:    `\text{Hello World} \customTag{Important Concept}`,
			expected: "Hello World Important Concept",
		},
		{
			name:     "Fractions and square roots",
			input:    `Speed = \frac{\Delta x}{\Delta t} and \sqrt{16}`,
			expected: "Speed = (Δ x / Δ t) and √(16)",
		},
		{
			name:     "Punctuation un-escaping",
			input:    `100\% score in \$50 test with \_var`,
			expected: "100% score in $50 test with _var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SanitizeLatex(tt.input)
			if actual != tt.expected {
				t.Errorf("SanitizeLatex(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestGenerateSpokenTranscript(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		contains   []string
		notContain []string
	}{
		{
			name:     "Calculus integral and fraction conversion",
			input:    `The formula is $$\int_0^1 \frac{x^2}{2} dx$$`,
			contains: []string{"definite integral from 0 to 1 of", "divided by 2", "x squared"},
			notContain: []string{`\int`, `\frac`, `$`, `\`, `^`},
		},
		{
			name:     "Bold highlighted text never says one or keeps asterisks",
			input:    `The **Newton's Second Law** states that $F = ma$`,
			contains: []string{"Newton's Second Law", "F = ma"},
			notContain: []string{"**", " one ", "$"},
		},
		{
			name:     "Pedagogical steps and academic abbreviations",
			input:    `Step 1: Calculate acceleration w.r.t. time. e.g., when $a = 5 m/s^2$`,
			contains: []string{"Step 1.", "with respect to", "for example", "meters per second squared"},
			notContain: []string{"w.r.t.", "e.g.", "m/s^2", "$"},
		},
		{
			name:     "Quadratic formula with nested sqrt in fraction and text macro",
			input:    `The roots are $$x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}$$ with speed $v = 10\text{ m/s}$`,
			contains: []string{"divided by 2a", "square root of", "b squared", "plus or minus", "10 meters per second"},
			notContain: []string{"frac", "text", `\frac`, `\sqrt`, `\text`, "$", `\`},
		},
		{
			name:     "Markdown blockquote and emoji translation",
			input:    "> 💡 **Pro Tip:** Remember that $v_f = v_i + at$\n> ⚠️ Watch out for units!",
			contains: []string{"Pro tip:", "v final = v initial + at", "Watch out, exam trap warning:"},
			notContain: []string{">", "💡", "⚠️", "v_f", "v_i"},
		},
		{
			name:     "LaTeX environments and chemistry reactions",
			input:    "\\begin{aligned} CaCO_3(s) \\xrightarrow{\\Delta} CaO(s) + CO_2(g) \\end{aligned}",
			contains: []string{"solid", "reacts under heat to form", "carbon dioxide, C O 2", "gas"},
			notContain: []string{"begin", "aligned", "end", `\xrightarrow`, `\Delta`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transcript := GenerateSpokenTranscript(tt.input)
			for _, exp := range tt.contains {
				if !containsString(transcript, exp) {
					t.Errorf("Expected transcript to contain %q, but got: %q", exp, transcript)
				}
			}
			for _, notExp := range tt.notContain {
				if containsString(transcript, notExp) {
					t.Errorf("Expected transcript NOT to contain %q, but got: %q", notExp, transcript)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstr(s, substr)))
}

func searchSubstr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
