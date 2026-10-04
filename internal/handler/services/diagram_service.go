package services

import (
	"context"
	"fmt"
	"strings"

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

	systemPrompt := `You are an elite educational Mind Map and Concept Graph Architect.
Generate a comprehensive, scientifically rigorous visual mind map for the requested student concept.
Your mind map must feature a central root, 4 to 5 major conceptual branches, and 2 to 3 detailed leaves per branch.
Include governing equations in LaTeX format wherever applicable in the "formulas" array.

Output a single valid JSON object strictly matching this schema:
{
  "concept": "Concept Name",
  "title": "Clear Descriptive Title",
  "diagram_type": "mindmap",
  "root": {
    "id": "root-1",
    "label": "Core Concept Name",
    "details": "Clear 1-sentence fundamental definition",
    "color_hex": "#4F46E5",
    "children": [
      {
        "id": "branch-1",
        "label": "Branch / Sub-domain Name",
        "details": "Core mechanism or governing law",
        "color_hex": "#0EA5E9",
        "children": [
          {
            "id": "leaf-1-1",
            "label": "Specific Law / Topic",
            "details": "Detailed explanation and application",
            "color_hex": "#0284C7",
            "children": []
          }
        ]
      }
    ]
  },
  "nodes": [
    {
      "id": "root-1",
      "label": "Core Concept Name",
      "node_type": "root",
      "description": "Fundamental definition and overview",
      "color_hex": "#4F46E5",
      "formulas": ["LaTeX formula if applicable"]
    },
    {
      "id": "branch-1",
      "label": "Branch Name",
      "node_type": "branch",
      "description": "Explanation of this branch",
      "color_hex": "#0EA5E9",
      "formulas": []
    },
    {
      "id": "leaf-1-1",
      "label": "Leaf Topic",
      "node_type": "leaf",
      "description": "Detailed point or edge case",
      "color_hex": "#0284C7",
      "formulas": []
    }
  ],
  "edges": [
    {
      "from": "root-1",
      "to": "branch-1",
      "from_id": "root-1",
      "to_id": "branch-1",
      "label": "Governs"
    },
    {
      "from": "branch-1",
      "to": "leaf-1-1",
      "from_id": "branch-1",
      "to_id": "leaf-1-1",
      "label": "Contains"
    }
  ]
}`

	userPrompt := fmt.Sprintf("Concept: %s\nDiagram Type: %s", req.Concept, diagramType)

	var diagram dto.DiagramResponse
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &diagram)

	// If AI outputted a root hierarchy but nodes were omitted, flatten root into nodes and edges
	if len(diagram.Nodes) == 0 && diagram.Root != nil {
		flattenMindMapTree(diagram.Root, &diagram.Nodes, &diagram.Edges, "")
	}

	// Fallback data if unmarshaled empty (e.g. offline dev mode or parsing error)
	if len(diagram.Nodes) == 0 {
		diagram = buildDomainSmartMindMap(req.Concept, diagramType)
	}

	// Normalize concept and title
	if diagram.Concept == "" {
		diagram.Concept = req.Concept
	}
	if diagram.Title == "" {
		diagram.Title = diagram.Concept
	}
	if diagram.DiagramType == "" {
		diagram.DiagramType = diagramType
	}

	// Ensure edges have both from/to and from_id/to_id populated for 100% client compatibility
	for i := range diagram.Edges {
		if diagram.Edges[i].FromID == "" && diagram.Edges[i].From != "" {
			diagram.Edges[i].FromID = diagram.Edges[i].From
		}
		if diagram.Edges[i].From == "" && diagram.Edges[i].FromID != "" {
			diagram.Edges[i].From = diagram.Edges[i].FromID
		}
		if diagram.Edges[i].ToID == "" && diagram.Edges[i].To != "" {
			diagram.Edges[i].ToID = diagram.Edges[i].To
		}
		if diagram.Edges[i].To == "" && diagram.Edges[i].ToID != "" {
			diagram.Edges[i].To = diagram.Edges[i].ToID
		}
	}

	return &diagram, err
}

func flattenMindMapTree(node *dto.MindMapNode, nodes *[]dto.GraphNode, edges *[]dto.GraphEdge, parentID string) {
	if node == nil {
		return
	}

	nodeType := "leaf"
	if parentID == "" {
		nodeType = "root"
	} else if len(node.Children) > 0 {
		nodeType = "branch"
	}

	color := node.ColorHex
	if color == "" {
		if nodeType == "root" {
			color = "#4F46E5"
		} else if nodeType == "branch" {
			color = "#0EA5E9"
		} else {
			color = "#10B981"
		}
	}

	*nodes = append(*nodes, dto.GraphNode{
		ID:          node.ID,
		Label:       node.Label,
		Description: node.Details,
		NodeType:    nodeType,
		ColorHex:    color,
	})

	if parentID != "" {
		*edges = append(*edges, dto.GraphEdge{
			From:   parentID,
			To:     node.ID,
			FromID: parentID,
			ToID:   node.ID,
			Label:  "leads to",
			Arrow:  true,
		})
	}

	for _, child := range node.Children {
		flattenMindMapTree(child, nodes, edges, node.ID)
	}
}

func buildDomainSmartMindMap(concept, diagramType string) dto.DiagramResponse {
	lower := strings.ToLower(concept)

	// Default fallback with domain awareness
	rootID := "root_1"
	resp := dto.DiagramResponse{
		Concept:     concept,
		Title:       concept,
		DiagramType: diagramType,
	}

	var branches []struct {
		id       string
		label    string
		details  string
		color    string
		formulas []string
		leaves   []struct {
			id       string
			label    string
			details  string
			color    string
			formulas []string
		}
	}

	if strings.Contains(lower, "thermo") || strings.Contains(lower, "heat") {
		branches = []struct {
			id       string
			label    string
			details  string
			color    string
			formulas []string
			leaves   []struct {
				id       string
				label    string
				details  string
				color    string
				formulas []string
			}
		}{
			{
				id: "b1", label: "Zeroth & First Laws", details: "Equilibrium and conservation of energy", color: "#0EA5E9",
				formulas: []string{`\Delta U = Q - W`, `W = \int P \, dV`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l1_1", label: "Thermal Equilibrium", details: "Zeroth law establishes temperature scale", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Internal Energy", details: "State function independent of path", color: "#0284C7", formulas: []string{`dU = n C_v dT`}},
				},
			},
			{
				id: "b2", label: "Second Law & Entropy", details: "Direction of spontaneous thermal processes", color: "#8B5CF6",
				formulas: []string{`\Delta S \ge 0`, `dS = rac{dQ_{rev}}{T}`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l2_1", label: "Clausius & Kelvin Statements", details: "No 100% efficient heat engine exists", color: "#A855F7", formulas: nil},
					{id: "l2_2", label: "Carnot Engine", details: "Maximum theoretical efficiency between two reservoirs", color: "#7C3AED", formulas: []string{`\eta = 1 - rac{T_C}{T_H}`}},
				},
			},
			{
				id: "b3", label: "Thermodynamic Processes", details: "Path-dependent gas transformations", color: "#10B981",
				formulas: []string{`P V^\gamma = 	ext{const}`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l3_1", label: "Isothermal vs Adiabatic", details: "Constant temperature vs zero heat transfer", color: "#34D399", formulas: []string{`W_{iso} = nRT \ln(V_f/V_i)`}},
					{id: "l3_2", label: "Isobaric & Isochoric", details: "Constant pressure and constant volume work", color: "#059669", formulas: []string{`W = P \Delta V`}},
				},
			},
			{
				id: "b4", label: "Free Energy & Potentials", details: "Spontaneity criteria in open and closed systems", color: "#F59E0B",
				formulas: []string{`\Delta G = \Delta H - T\Delta S`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l4_1", label: "Gibbs Free Energy", details: "Drives chemical and thermodynamic equilibrium", color: "#FBBF24", formulas: nil},
					{id: "l4_2", label: "Enthalpy & Latent Heat", details: "Total heat content and phase transition energies", color: "#D97706", formulas: []string{`H = U + PV`}},
				},
			},
		}
	} else if strings.Contains(lower, "architecture") || strings.Contains(lower, "cpu") || strings.Contains(lower, "computer") {
		branches = []struct {
			id       string
			label    string
			details  string
			color    string
			formulas []string
			leaves   []struct {
				id       string
				label    string
				details  string
				color    string
				formulas []string
			}
		}{
			{
				id: "b1", label: "CPU & Execution Core", details: "Arithmetic logic and instruction processing", color: "#0EA5E9",
				formulas: []string{`	ext{CPU Time} = I 	imes 	ext{CPI} 	imes T_{clk}`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l1_1", label: "ALU & Registers", details: "High-speed internal registers and integer/FP units", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Control Unit (CU)", details: "Instruction fetch, decode, and microcode dispatch", color: "#0284C7", formulas: nil},
				},
			},
			{
				id: "b2", label: "Memory Hierarchy", details: "Multi-level caching and address translation", color: "#10B981",
				formulas: []string{`T_{eff} = h \cdot T_{cache} + (1-h) \cdot T_{mem}`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l2_1", label: "L1/L2/L3 Cache", details: "SRAM associative caches minimizing latency", color: "#34D399", formulas: nil},
					{id: "l2_2", label: "Virtual Memory & MMU", details: "Paging, TLB lookups, and page fault handling", color: "#059669", formulas: nil},
				},
			},
			{
				id: "b3", label: "Instruction Pipeline", details: "Overlapped instruction execution stages", color: "#8B5CF6",
				formulas: []string{`	ext{Speedup} = rac{k \cdot n}{k + n - 1}`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l3_1", label: "Pipeline Hazards", details: "Structural, data (RAW/WAR), and control stalls", color: "#A855F7", formulas: nil},
					{id: "l3_2", label: "Branch Prediction", details: "Dynamic branch prediction and speculative execution", color: "#7C3AED", formulas: nil},
				},
			},
			{
				id: "b4", label: "System Bus & I/O", details: "Interconnect protocols and peripheral controllers", color: "#F59E0B",
				formulas: nil,
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l4_1", label: "DMA Controller", details: "Direct memory access bypassing CPU for high throughput", color: "#FBBF24", formulas: nil},
					{id: "l4_2", label: "PCIe & System Fabric", details: "Point-to-point serial interconnect lanes", color: "#D97706", formulas: nil},
				},
			},
		}
	} else {
		// General Academic Concept structure
		branches = []struct {
			id       string
			label    string
			details  string
			color    string
			formulas []string
			leaves   []struct {
				id       string
				label    string
				details  string
				color    string
				formulas []string
			}
		}{
			{
				id: "b1", label: "Foundations & Axioms", details: fmt.Sprintf("Fundamental laws and axioms of %s", concept), color: "#0EA5E9",
				formulas: nil,
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l1_1", label: "Definitions & Units", details: "Core terminology, parameters, and SI standards", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Initial Assumptions", details: "Boundary conditions and validity range", color: "#0284C7", formulas: nil},
				},
			},
			{
				id: "b2", label: "Mechanisms & Dynamics", details: "Step-by-step physical and logical working", color: "#10B981",
				formulas: nil,
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l2_1", label: "Core Working Flow", details: "Cause-and-effect chain and transitions", color: "#34D399", formulas: nil},
					{id: "l2_2", label: "State Transformations", details: "Key intermediate phases and lifecycle", color: "#059669", formulas: nil},
				},
			},
			{
				id: "b3", label: "Mathematical Equations", details: "Analytical formulation and quantitative models", color: "#8B5CF6",
				formulas: []string{`f(x) = \sum_{n=1}^\infty a_n \phi_n(x)`},
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l3_1", label: "Governing Formula", details: "Central mathematical expression and constants", color: "#A855F7", formulas: []string{`y = f(x)`}},
					{id: "l3_2", label: "Dimensional Analysis", details: "Units consistency and balance checks", color: "#7C3AED", formulas: nil},
				},
			},
			{
				id: "b4", label: "Applications & Exam Traps", details: "High-yield scoring rules and common blunders", color: "#F59E0B",
				formulas: nil,
				leaves: []struct {
					id string; label string; details string; color string; formulas []string
				}{
					{id: "l4_1", label: "Real-World Engineering", details: "Practical implementation in modern technology", color: "#FBBF24", formulas: nil},
					{id: "l4_2", label: "Common Exam Traps", details: "Sign errors, edge conditions, and trick questions", color: "#D97706", formulas: nil},
				},
			},
		}
	}

	// Assemble root node
	rootNode := dto.GraphNode{
		ID:          rootID,
		Label:       concept,
		Description: fmt.Sprintf("Central overview and conceptual pillars of %s", concept),
		NodeType:    "root",
		ColorHex:    "#4F46E5",
	}
	resp.Nodes = append(resp.Nodes, rootNode)

	rootTreeNode := &dto.MindMapNode{
		ID:       rootID,
		Label:    concept,
		Details:  fmt.Sprintf("Central conceptual hub of %s", concept),
		ColorHex: "#4F46E5",
	}

	for _, b := range branches {
		bNode := dto.GraphNode{
			ID:          b.id,
			Label:       b.label,
			Description: b.details,
			NodeType:    "branch",
			ColorHex:    b.color,
			Formulas:    b.formulas,
		}
		resp.Nodes = append(resp.Nodes, bNode)
		resp.Edges = append(resp.Edges, dto.GraphEdge{
			From:   rootID,
			To:     b.id,
			FromID: rootID,
			ToID:   b.id,
			Label:  "Branches into",
			Arrow:  true,
		})

		bTreeNode := &dto.MindMapNode{
			ID:       b.id,
			Label:    b.label,
			Details:  b.details,
			ColorHex: b.color,
		}

		for _, l := range b.leaves {
			lNode := dto.GraphNode{
				ID:          l.id,
				Label:       l.label,
				Description: l.details,
				NodeType:    "leaf",
				ColorHex:    l.color,
				Formulas:    l.formulas,
			}
			resp.Nodes = append(resp.Nodes, lNode)
			resp.Edges = append(resp.Edges, dto.GraphEdge{
				From:   b.id,
				To:     l.id,
				FromID: b.id,
				ToID:   l.id,
				Label:  "Includes",
				Arrow:  true,
			})

			bTreeNode.Children = append(bTreeNode.Children, &dto.MindMapNode{
				ID:       l.id,
				Label:    l.label,
				Details:  l.details,
				ColorHex: l.color,
			})
		}

		rootTreeNode.Children = append(rootTreeNode.Children, bTreeNode)
	}

	resp.Root = rootTreeNode
	return resp
}
