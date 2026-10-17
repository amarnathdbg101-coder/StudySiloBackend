package ai_test

import (
	"strings"
	"testing"

	"goProjectStructure/internal/reuse/ai"
)

func TestCognitivePromptEngine_CurriculumResolution(t *testing.T) {
	tests := []struct {
		examInput string
		wantCode  string
	}{
		{"GATE CS 2026", "gate_cs"},
		{"NEET Medical Entrance", "neet_medical"},
		{"JEE Advanced Physics", "jee_engineering"},
		{"CBSE Class 12 Boards", "cbse_school"},
		{"UPSC Civil Services IAS", "upsc_civil"},
		{"Unknown Random Exam", "general"},
		{"", "general"},
	}

	for _, tc := range tests {
		curr := ai.ResolveCurriculum(tc.examInput)
		if curr.Code != tc.wantCode {
			t.Errorf("ResolveCurriculum(%q) = %s, want %s", tc.examInput, curr.Code, tc.wantCode)
		}
	}
}

func TestCognitivePromptEngine_WeakTopicsPrioritization(t *testing.T) {
	weakTopics := []string{"Deadlocks & Concurrency", "B+ Tree Indexing"}
	strongTopics := []string{"Sorting"}
	examTraps := []string{"Starvation vs Deadlock"}

	res := ai.GenerateSmartContext("GATE Computer Science", weakTopics, strongTopics, examTraps, "socratic")

	if len(res.QuickChips) != 4 {
		t.Fatalf("expected 4 quick chips, got %d", len(res.QuickChips))
	}

	// First chip must feature first weak topic
	if !strings.Contains(res.QuickChips[0].Label, "Deadlocks & Concurrency") {
		t.Errorf("expected chip 0 to contain Deadlocks, got: %s", res.QuickChips[0].Label)
	}

	// Second chip must feature second weak topic
	if !strings.Contains(res.QuickChips[1].Label, "B+ Tree Indexing") {
		t.Errorf("expected chip 1 to contain B+ Tree, got: %s", res.QuickChips[1].Label)
	}

	// Input hint should target first weak topic
	if !strings.Contains(res.InputHint, "Deadlocks & Concurrency") {
		t.Errorf("expected input hint to mention Deadlocks, got: %s", res.InputHint)
	}

	// Focus topic should be top weak topic
	if res.FocusTopic != "Deadlocks & Concurrency" {
		t.Errorf("expected focus topic 'Deadlocks & Concurrency', got: %s", res.FocusTopic)
	}
}

func TestCognitivePromptEngine_TutorModes(t *testing.T) {
	weak := []string{"Rotational Dynamics"}

	// 1. Feynman Mode
	feynmanRes := ai.GenerateSmartContext("JEE Advanced", weak, nil, nil, "feynman")
	if len(feynmanRes.TutorSuggestions) != 4 {
		t.Fatalf("expected 4 tutor suggestions, got %d", len(feynmanRes.TutorSuggestions))
	}
	if !strings.Contains(feynmanRes.TutorSuggestions[0].Prompt, "like I am 12") {
		t.Errorf("expected Feynman mode prompt, got: %s", feynmanRes.TutorSuggestions[0].Prompt)
	}

	// 2. Exam Cram / PYQ Mode
	cramRes := ai.GenerateSmartContext("JEE Advanced", weak, nil, nil, "pyq_cram")
	if !strings.Contains(cramRes.TutorSuggestions[0].Prompt, "exam traps") {
		t.Errorf("expected Cram mode prompt, got: %s", cramRes.TutorSuggestions[0].Prompt)
	}

	// 3. Derivations Mode
	derivRes := ai.GenerateSmartContext("JEE Advanced", weak, nil, nil, "derivation")
	if !strings.Contains(derivRes.TutorSuggestions[0].Prompt, "LaTeX derivation") {
		t.Errorf("expected Derivations prompt, got: %s", derivRes.TutorSuggestions[0].Prompt)
	}

	// 4. Default Socratic Mode
	socraticRes := ai.GenerateSmartContext("JEE Advanced", weak, nil, nil, "socratic")
	if !strings.Contains(socraticRes.TutorSuggestions[0].Prompt, "Socratic") &&
		!strings.Contains(socraticRes.TutorSuggestions[0].Label, "Socratic") {
		t.Errorf("expected Socratic prompt, got: %s", socraticRes.TutorSuggestions[0].Label)
	}
}
