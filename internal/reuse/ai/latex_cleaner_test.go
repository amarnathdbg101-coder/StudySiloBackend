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
