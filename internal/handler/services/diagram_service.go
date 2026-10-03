package services

import (
	"context"
	"fmt"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/reuse/ai"
)

type DiagramService interface {
	GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error)
}

type diagramService struct {
	orchestrator *ai.Orchestrator
}

func NewDiagramService(orchestrator *ai.Orchestrator) DiagramService {
	return &diagramService{orchestrator: orchestrator}
}

func (s *diagramService) GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error) {
	diagramType := req.DiagramType
	if diagramType == "" {
		diagramType = "mindmap"
	}

	systemPrompt := `You are an expert educational visual diagram and mind map architect.
Generate a comprehensive visual structure for the requested student concept.
Output a single JSON object strictly matching this schema:
{
  "concept": "string",
  "diagram_type": "mindmap | flowchart | concept_tree",
  "root": {
    "id": "node_1",
    "label": "Core Concept Name",
    "details": "Short explanation",
    "color_hex": "#4F46E5",
    "children": [
      {
        "id": "node_2",
        "label": "Subconcept A",
        "details": "Explanation",
        "color_hex": "#06B6D4",
        "children": []
      },
      {
        "id": "node_3",
        "label": "Subconcept B",
        "details": "Explanation",
        "color_hex": "#10B981",
        "children": []
      }
    ]
  },
  "nodes": [
    {"id": "n1", "label": "Start", "x": 150.0, "y": 80.0, "shape": "rounded_rect", "color_hex": "#4F46E5"},
    {"id": "n2", "label": "Action", "x": 150.0, "y": 200.0, "shape": "diamond", "color_hex": "#F59E0B"}
  ],
  "edges": [
    {"from": "n1", "to": "n2", "label": "flow", "arrow": true}
  ]
}`

	userPrompt := fmt.Sprintf("Concept: %s\nDiagram Type: %s", req.Concept, diagramType)

	var diagram dto.DiagramResponse
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &diagram)

	// Fallback data if unmarshaled empty (e.g. offline dev mode)
	if diagram.Root == nil && len(diagram.Nodes) == 0 {
		diagram.Concept = req.Concept
		diagram.DiagramType = diagramType
		diagram.Root = &dto.MindMapNode{
			ID:       "root_1",
			Label:    req.Concept,
			Details:  fmt.Sprintf("Central overview of %s", req.Concept),
			ColorHex: "#4F46E5",
			Children: []*dto.MindMapNode{
				{
					ID:       "sub_1",
					Label:    "Foundations & Definitions",
					Details:  "Core laws and assumptions",
					ColorHex: "#06B6D4",
					Children: []*dto.MindMapNode{
						{ID: "sub_1_1", Label: "Primary Axioms", Details: "Fundamental building blocks", ColorHex: "#3B82F6", Children: []*dto.MindMapNode{}},
					},
				},
				{
					ID:       "sub_2",
					Label:    "Practical Applications",
					Details:  "Engineering and problem solving cases",
					ColorHex: "#10B981",
					Children: []*dto.MindMapNode{
						{ID: "sub_2_1", Label: "Standard Workflows", Details: "Step-by-step algorithms", ColorHex: "#14B8A6", Children: []*dto.MindMapNode{}},
					},
				},
				{
					ID:       "sub_3",
					Label:    "Exam Traps & Edge Cases",
					Details:  "Critical pitfalls to avoid",
					ColorHex: "#F59E0B",
					Children: []*dto.MindMapNode{},
				},
			},
		}

		diagram.Nodes = []dto.GraphNode{
			{ID: "n1", Label: req.Concept, X: 200.0, Y: 80.0, Shape: "rounded_rect", ColorHex: "#4F46E5"},
			{ID: "n2", Label: "Analyze Inputs", X: 100.0, Y: 220.0, Shape: "diamond", ColorHex: "#06B6D4"},
			{ID: "n3", Label: "Apply Formulas", X: 300.0, Y: 220.0, Shape: "rounded_rect", ColorHex: "#10B981"},
			{ID: "n4", Label: "Verify Solution", X: 200.0, Y: 360.0, Shape: "circle", ColorHex: "#F59E0B"},
		}

		diagram.Edges = []dto.GraphEdge{
			{From: "n1", To: "n2", Label: "identify", Arrow: true},
			{From: "n1", To: "n3", Label: "relate", Arrow: true},
			{From: "n2", To: "n4", Label: "compute", Arrow: true},
			{From: "n3", To: "n4", Label: "conclude", Arrow: true},
		}
	}

	return &diagram, err
}
