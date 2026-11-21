package services

import (
	"context"
	"strings"
	"testing"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/reuse/ai"
)

func TestCleanConceptAndExtractIntent_Comprehensive(t *testing.T) {
	tests := []struct {
		input             string
		expectedCanonical string
		expectedDirective string
	}{
		{
			input:             "merge sort ko deep me samjhao",
			expectedCanonical: "Merge Sort",
			expectedDirective: "HIGH TECHNICAL DEPTH",
		},
		{
			input:             "photosynthesis step by step explain karo",
			expectedCanonical: "Photosynthesis",
			expectedDirective: "STEP-BY-STEP PROGRESSION",
		},
		{
			input:             "operating system paging with example",
			expectedCanonical: "Operating System Paging",
			expectedDirective: "EXAMPLES / CODE",
		},
		{
			input:             "maxwell equations with formula and derivation",
			expectedCanonical: "Maxwell Equations",
			expectedDirective: "MATHEMATICAL FORMULATION",
		},
		{
			input:             "tcp vs udp exam gotcha and viva questions",
			expectedCanonical: "Tcp Vs Udp",
			expectedDirective: "EXAM SCORING FOCUS",
		},
		{
			input:             "web technology ka syllbus and concept do",
			expectedCanonical: "Web Technology",
			expectedDirective: "ACADEMIC SYLLABUS & CURRICULUM ROADMAP",
		},
		{
			input:             "dbms complete syllabus and roadmap chahiye",
			expectedCanonical: "Dbms",
			expectedDirective: "ACADEMIC SYLLABUS & CURRICULUM ROADMAP",
		},
		{
			input:             "machine learning ka roadmap batao",
			expectedCanonical: "Machine Learning",
			expectedDirective: "ACADEMIC SYLLABUS & CURRICULUM ROADMAP",
		},
	}

	for _, tc := range tests {
		canonical, directives := cleanConceptAndExtractIntent(tc.input)
		if !strings.EqualFold(canonical, tc.expectedCanonical) {
			t.Errorf("cleanConceptAndExtractIntent(%q): canonical=%q, expected=%q", tc.input, canonical, tc.expectedCanonical)
		}
		if !strings.Contains(directives, tc.expectedDirective) {
			t.Errorf("cleanConceptAndExtractIntent(%q): directives=%q, expected to contain %q", tc.input, directives, tc.expectedDirective)
		}
	}
}

func TestBuildDomainSmartMindMap_AllArchetypes(t *testing.T) {
	archetypeConcepts := map[string]string{
		"flow":       "Merge Sort Algorithm",
		"cycle":      "Krebs Cycle Metabolism",
		"comparison": "TCP vs UDP Comparison",
		"stack":      "OSI Layer Stack Hierarchy",
		"hub":        "Newton's Laws of Motion",
		"tree":       "Data Structures Classification",
	}

	for expectedArch, concept := range archetypeConcepts {
		diag := buildDomainSmartMindMap(concept, "mindmap")
		if diag.Archetype != expectedArch {
			t.Errorf("buildDomainSmartMindMap(%q): expected archetype %q, got %q", concept, expectedArch, diag.Archetype)
		}
		if len(diag.Nodes) < 5 {
			t.Errorf("buildDomainSmartMindMap(%q): expected >= 5 nodes, got %d", concept, len(diag.Nodes))
		}
		if len(diag.Edges) < 4 {
			t.Errorf("buildDomainSmartMindMap(%q): expected >= 4 edges, got %d", concept, len(diag.Edges))
		}

		// Ensure layout coordinates can be computed without panic
		ai.ComputeDiagramLayout(&diag, 2800, 2000)

		// Verify root node is positioned
		rootFound := false
		for _, n := range diag.Nodes {
			if n.NodeType == "root" {
				rootFound = true
				if n.X == 0 && n.Y == 0 {
					t.Errorf("Root node not positioned in %s", expectedArch)
				}
				if n.Width <= 0 || n.Height <= 0 {
					t.Errorf("Root node has invalid dimensions in %s", expectedArch)
				}
			}
		}
		if !rootFound {
			t.Errorf("No root node found in %s", expectedArch)
		}
	}
}

func TestBuildDomainSmartMindMap_WebTechnologySyllabus(t *testing.T) {
	diag := buildDomainSmartMindMap("Web Technology", "mindmap")

	if diag.Archetype != "stack" {
		t.Errorf("Expected archetype 'stack' for Web Technology syllabus, got %q", diag.Archetype)
	}
	if !strings.Contains(diag.Title, "Web Technology") {
		t.Errorf("Expected title to contain 'Web Technology', got %q", diag.Title)
	}

	// Should contain 5 comprehensive modules
	moduleCount := 0
	for _, n := range diag.Nodes {
		if strings.HasPrefix(n.ID, "b") {
			moduleCount++
		}
	}
	if moduleCount < 5 {
		t.Errorf("Expected at least 5 syllabus modules, got %d", moduleCount)
	}

	// Verify all nodes have non-empty description
	for _, n := range diag.Nodes {
		if n.Description == "" && n.Details == "" {
			t.Errorf("Node %s has empty description", n.ID)
		}
	}
}

func TestDiagramService_CacheHitFlow(t *testing.T) {
	// Initialize diagram service with empty keys (offline mode)
	orch := ai.NewOrchestrator(nil, nil, nil)
	svc := NewDiagramService(orch, nil, nil, nil, nil)

	ctx := context.Background()
	req := dto.DiagramRequest{
		Concept:     "Newton's Laws of Motion",
		DiagramType: "mindmap",
		Archetype:   "hub",
	}

	// 1st call: Computes and stores in cache
	resp1, err := svc.GenerateDiagram(ctx, "test_user", req)
	if err != nil && resp1 == nil {
		t.Fatalf("GenerateDiagram failed: %v", err)
	}
	if resp1 == nil || len(resp1.Nodes) == 0 {
		t.Fatalf("Expected non-empty diagram from 1st call")
	}

	// 2nd call: Should hit cache
	resp2, err := svc.GenerateDiagram(ctx, "test_user", req)
	if err != nil {
		t.Fatalf("GenerateDiagram cache hit failed: %v", err)
	}
	if resp2.Concept != resp1.Concept {
		t.Errorf("Expected identical concept from cache: %q vs %q", resp2.Concept, resp1.Concept)
	}
	if len(resp2.Nodes) != len(resp1.Nodes) {
		t.Errorf("Expected identical node count from cache: %d vs %d", len(resp2.Nodes), len(resp1.Nodes))
	}
}

func TestDiagramService_ExpandNode_TerminalSaturation(t *testing.T) {
	orch := ai.NewOrchestrator(nil, nil, nil)
	svc := NewDiagramService(orch, nil, nil, nil, nil)

	ctx := context.Background()
	req := dto.ExpandNodeRequest{
		Concept:     "Merge Sort",
		NodeID:      "leaf_complexity",
		NodeLabel:   "Time & Space Complexity",
		NodeDetails: "Best, average and worst case asymptotic bounds O(N log N)",
		Force:       false,
	}

	resp, err := svc.ExpandNode(ctx, "test_user", req)
	if err != nil {
		t.Fatalf("ExpandNode failed: %v", err)
	}

	if resp.CanExpand {
		t.Errorf("Expected Time Complexity leaf to be saturated and non-expandable without force")
	}
	if !resp.IsSaturated {
		t.Errorf("Expected IsSaturated = true")
	}
	if resp.SaturationReason == "" {
		t.Errorf("Expected helpful saturation reason explanation")
	}
}

func TestDiagramService_ExpandNode_CoordinatesCalculated(t *testing.T) {
	orch := ai.NewOrchestrator(nil, nil, nil)
	svc := NewDiagramService(orch, nil, nil, nil, nil)

	ctx := context.Background()
	req := dto.ExpandNodeRequest{
		Concept:      "Photosynthesis",
		NodeID:       "branch_light_reaction",
		NodeLabel:    "Light-Dependent Reactions",
		NodeDetails:  "Thylakoid membrane photon absorption and ATP synthesis",
		ParentX:      600.0,
		ParentY:      400.0,
		ParentWidth:  210.0,
		ParentHeight: 60.0,
		Force:        true,
	}

	resp, err := svc.ExpandNode(ctx, "test_user", req)
	if err != nil {
		t.Fatalf("ExpandNode failed: %v", err)
	}

	if !resp.CanExpand {
		t.Errorf("Expected CanExpand = true when force is true")
	}
	if len(resp.NewNodes) < 3 {
		t.Fatalf("Expected >= 3 new nodes, got %d", len(resp.NewNodes))
	}

	// Verify all new nodes have computed coordinates and non-zero dimensions
	for _, n := range resp.NewNodes {
		if n.X == 0 && n.Y == 0 {
			t.Errorf("New node %s has (0, 0) coordinates", n.ID)
		}
		if n.Width <= 0 || n.Height <= 0 {
			t.Errorf("New node %s has invalid dimensions: %fx%f", n.ID, n.Width, n.Height)
		}
	}
}
