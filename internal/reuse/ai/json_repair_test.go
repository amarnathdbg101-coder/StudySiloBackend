package ai

import (
	"testing"
)

func TestRepairAndCleanJSON_MarkdownFence(t *testing.T) {
	input := "```json\n{\n  \"concept\": \"Photosynthesis\"\n}\n```"
	expected := "{\n  \"concept\": \"Photosynthesis\"\n}"
	actual := RepairAndCleanJSON(input)
	if actual != expected {
		t.Errorf("Expected %q, got %q", expected, actual)
	}
}

func TestRepairAndCleanJSON_TrailingComma(t *testing.T) {
	input := `{"items": ["a", "b",], "count": 2,}`
	var target struct {
		Items []string `json:"items"`
		Count int      `json:"count"`
	}
	err := UnmarshalJSONSafe([]byte(input), &target)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON with trailing commas: %v", err)
	}
	if len(target.Items) != 2 || target.Count != 2 {
		t.Errorf("Unexpected unmarshal result: %+v", target)
	}
}

func TestRepairAndCleanJSON_LaTeXSingleBackslashes(t *testing.T) {
	// In raw string: \frac, \sum, \alpha, \partial are NOT valid JSON escapes, standard json.Unmarshal FAILS!
	raw := `{"formula": "\frac{d}{dx} \sin(x) = \cos(x)", "sum": "\sum_{i=1}^N i^2", "symbol": "\alpha \beta \gamma"}`

	var target struct {
		Formula string `json:"formula"`
		Sum     string `json:"sum"`
		Symbol  string `json:"symbol"`
	}

	err := UnmarshalJSONSafe([]byte(raw), &target)
	if err != nil {
		t.Fatalf("Expected UnmarshalJSONSafe to succeed for LaTeX escapes, got: %v", err)
	}

	if target.Formula != `\frac{d}{dx} \sin(x) = \cos(x)` {
		t.Errorf("Unexpected formula: %q", target.Formula)
	}
	if target.Sum != `\sum_{i=1}^N i^2` {
		t.Errorf("Unexpected sum: %q", target.Sum)
	}
	if target.Symbol != `\alpha \beta \gamma` {
		t.Errorf("Unexpected symbol: %q", target.Symbol)
	}
}

func TestRepairAndCleanJSON_LiteralNewlinesInStrings(t *testing.T) {
	// Literal unescaped newline inside string literal (common LLM behavior)
	raw := "{\n  \"details\": \"• First point\n• Second point\"\n}"

	var target struct {
		Details string `json:"details"`
	}

	err := UnmarshalJSONSafe([]byte(raw), &target)
	if err != nil {
		t.Fatalf("Expected UnmarshalJSONSafe to handle literal newlines, got: %v", err)
	}

	expected := "• First point\n• Second point"
	if target.Details != expected {
		t.Errorf("Expected %q, got %q", expected, target.Details)
	}
}

func TestRepairAndCleanJSON_TruncatedClosingBraces(t *testing.T) {
	// Truncated by LLM token limit: missing closing }
	raw := `{"title": "Test Map", "nodes": [{"id": "1", "label": "Root"`

	var target struct {
		Title string `json:"title"`
		Nodes []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"nodes"`
	}

	err := UnmarshalJSONSafe([]byte(raw), &target)
	if err != nil {
		t.Fatalf("Expected UnmarshalJSONSafe to repair unclosed braces, got: %v", err)
	}
	if target.Title != "Test Map" {
		t.Errorf("Expected title 'Test Map', got %q", target.Title)
	}
	if len(target.Nodes) != 1 || target.Nodes[0].Label != "Root" {
		t.Errorf("Expected 1 node with label 'Root', got %+v", target.Nodes)
	}
}
