package ai

import (
	"strings"
	"testing"
)

func TestBuildLanguageInstruction(t *testing.T) {
	tests := []struct {
		lang     string
		contains string
	}{
		{"hinglish", "STRICT HINGLISH"},
		{"HINGLISH", "STRICT HINGLISH"},
		{"english", "STRICT ENGLISH"},
		{"hindi", "STRICT HINDI"},
		{"bengali", "STRICT BENGALI"},
		{"marathi", "STRICT MARATHI"},
		{"tamil", "STRICT TAMIL"},
		{"auto", "DYNAMIC USER TONE MATCHING"},
		{"all", "DYNAMIC USER TONE MATCHING"},
		{"", "DYNAMIC USER TONE MATCHING"},
		{"spanish", "STRICT SPANISH"},
	}

	for _, tc := range tests {
		t.Run(tc.lang, func(t *testing.T) {
			instruction := BuildLanguageInstruction(tc.lang)
			if !strings.Contains(instruction, tc.contains) {
				t.Errorf("for lang '%s', expected instruction to contain '%s', got:\n%s", tc.lang, tc.contains, instruction)
			}
		})
	}
}
