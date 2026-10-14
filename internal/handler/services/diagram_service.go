package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type DiagramService interface {
	GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error)
	ExpandNode(ctx context.Context, userID string, req dto.ExpandNodeRequest) (*dto.ExpandNodeResponse, error)
	SaveMindMap(ctx context.Context, userID string, req dto.SaveMindMapRequest) (*dto.SavedMindMapResponse, error)
	GetMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error)
	ListMindMaps(ctx context.Context, userID string, search string, limit, offset int) ([]dto.SavedMindMapSummary, int, error)
	UpdateMindMap(ctx context.Context, userID, id string, req dto.UpdateMindMapRequest) (*dto.SavedMindMapResponse, error)
	DeleteMindMap(ctx context.Context, userID, id string) error
	TogglePinMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error)
}

type diagramService struct {
	orchestrator *ai.Orchestrator
	mindMapRepo  repository.MindMapRepository
}

func NewDiagramService(orchestrator *ai.Orchestrator, mindMapRepo repository.MindMapRepository) DiagramService {
	return &diagramService{orchestrator: orchestrator, mindMapRepo: mindMapRepo}
}

func (s *diagramService) GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error) {
	diagramType := req.DiagramType
	if diagramType == "" {
		diagramType = "mindmap"
	}

	systemPrompt := `You are an elite academic Mind Map and Visual Knowledge Architect (comparable to Heptabase and Obsidian Canvas).
Generate a deep, structured, scientifically rigorous, and non-generic visual mind map for the requested student concept.

CRITICAL PEDAGOGICAL RULE: NEVER USE VAGUE PLACEHOLDERS (e.g. NEVER just write "Time & Space Complexity" without exact Big-O and conditions).
Every branch and leaf MUST contain concrete technical values, metrics, formulas, or step-by-step mechanisms.

Domain Adaptation Rules:
1. ALGORITHMS & CS:
   - Identify domain_type as "algorithm".
   - Include exact Big-O: Best Case, Average Case, Worst Case (and the exact condition causing it), Auxiliary Space.
   - Include Partition/Traversal Schemes (e.g., Lomuto vs Hoare, Median-of-Three pivot).
   - Recurrence Relation (e.g. T(n) = 2T(n/2) + O(n)).
   - Invariants and Stability (e.g., Stability: Unstable, In-Place: Yes).
   - Short code_snippet (2-4 lines pseudocode/invariant).
   - Populate "metrics" map: {"Best Case": "O(N log N)", "Worst Case": "O(N^2)", "Aux Space": "O(log N)", "Stability": "Unstable", "In-Place": "Yes"}.
2. SCIENCE, PHYSICS & MATH:
   - Identify domain_type as "science" or "math".
   - Include exact LaTeX equations in "formulas", SI units, boundary conditions (e.g. v -> c, T -> 0), physical interpretations.
   - Populate "metrics" map with relevant constants or limits (e.g. {"SI Unit": "J/kg·K", "Conservation": "Energy", "Dimensional Formula": "[M L^2 T^-2]"}).
3. CHEMISTRY & BIOLOGY:
   - Identify domain_type as "chemistry" or "biology".
   - Step-by-step reaction mechanisms, intermediates, enzymes, ATP yield, or clinical/exam gotchas.
   - Populate "metrics" map (e.g. {"Mechanism": "SN2", "Rate Law": "k[Substrate][Nu]", "Solvent": "Polar Aprotic", "Stereochemistry": "Inversion"}).

Structure Requirements:
- Central root node.
- 4 to 6 major conceptual branches (color-coded).
- 2 to 3 detailed leaves per branch.
- Cross-branch relational links: If a node in one branch causally relates to a node in another branch (e.g. "Worst Case O(N^2)" is triggered by "Sorted Array in Lomuto Partition"), populate "cross_links" with that target node ID.

For EVERY node provide:
1. "id": Unique string (e.g. "root-1", "branch-comp", "leaf-worst-case").
2. "label": Crisp title (2 to 4 words).
3. "details": Clear explanation with concrete numbers, benchmarks, or definitions.
4. "domain_type": "algorithm" | "math" | "science" | "chemistry" | "biology" | "general".
5. "key_takeaway": 1 high-yield exam summary sentence.
6. "exam_trap": The #1 common student mistake or calculation trap for this topic.
7. "importance": Integer from 1 to 5 indicating frequency in competitive exams.
8. "badge": Conceptual tag ("Big-O", "Worst Case", "Core Law", "Theorem", "SN2", "High Yield", "Trap Alert").
9. "metrics": Key-value map of concrete numbers/specs (e.g. {"Best": "O(N log N)", "Worst": "O(N^2)"}).
10. "code_snippet": Optional 2-4 lines pseudocode, invariant, or recurrence.
11. "cross_links": Optional array of IDs of related nodes in other branches.
12. "formulas": Array of governing LaTeX equations.

Output a single valid JSON object strictly matching this schema:
{
  "concept": "Concept Name",
  "title": "Clear Descriptive Title",
  "diagram_type": "mindmap",
  "domain_type": "algorithm",
  "root": {
    "id": "root-1",
    "label": "Core Concept Name",
    "details": "Fundamental definition and overarching scope",
    "color_hex": "#4F46E5",
    "domain_type": "algorithm",
    "key_takeaway": "Central foundation connecting all surrounding branches",
    "importance": 5,
    "badge": "Core Foundation",
    "metrics": {},
    "formulas": [],
    "children": [
      {
        "id": "branch-1",
        "label": "Branch Name",
        "details": "Core mechanism or governing law",
        "color_hex": "#6366F1",
        "domain_type": "algorithm",
        "key_takeaway": "Core takeaway for this branch",
        "exam_trap": "Common pitfall to avoid",
        "importance": 5,
        "badge": "Major Pillar",
        "metrics": {"Key": "Value"},
        "formulas": [],
        "children": [
          {
            "id": "leaf-1-1",
            "label": "Specific Law / Topic",
            "details": "Detailed explanation with concrete values",
            "color_hex": "#818CF8",
            "domain_type": "algorithm",
            "key_takeaway": "Key exam takeaway",
            "exam_trap": "Calculation or edge-case trap",
            "importance": 4,
            "badge": "Key Law",
            "metrics": {"Best": "O(N log N)", "Worst": "O(N^2)"},
            "code_snippet": "",
            "cross_links": [],
            "formulas": []
          }
        ]
      }
    ]
  },
  "nodes": [],
  "edges": []
}`

	userPrompt := fmt.Sprintf("Concept: %s\nDiagram Type: %s", req.Concept, diagramType)

	var diagram dto.DiagramResponse
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &diagram)

	// If AI outputted a root hierarchy but nodes were omitted, flatten root into nodes and edges
	if len(diagram.Nodes) == 0 && diagram.Root != nil {
		flattenMindMapTree(diagram.Root, &diagram.Nodes, &diagram.Edges, "", "", 0)
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

func (s *diagramService) ExpandNode(ctx context.Context, userID string, req dto.ExpandNodeRequest) (*dto.ExpandNodeResponse, error) {
	systemPrompt := `You are an elite academic Concept Architect (comparable to Heptabase/Whimsical).
The student is viewing a visual Mind Map for concept: "` + req.Concept + `".
They have clicked on a specific node: "` + req.NodeLabel + `" (Details: ` + req.NodeDetails + `) and requested a deep-dive expansion.

Generate 2 to 3 deep-dive child leaves specifically for this node.
Provide concrete technical facts, Big-O metrics, equations, or edge cases. NEVER USE VAGUE GENERALITIES.

Output JSON strictly matching:
{
  "node_id": "` + req.NodeID + `",
  "new_nodes": [
    {
      "id": "` + req.NodeID + `_child_1",
      "label": "Sub-concept Title",
      "description": "Concrete technical explanation",
      "key_takeaway": "Key takeaway",
      "exam_trap": "Exam gotcha",
      "importance": 4,
      "badge": "Deep Dive",
      "metrics": {"Key": "Value"},
      "code_snippet": "",
      "formulas": []
    }
  ],
  "new_edges": [
    {
      "from": "` + req.NodeID + `",
      "to": "` + req.NodeID + `_child_1",
      "label": "elaborates into"
    }
  ]
}`

	userPrompt := fmt.Sprintf("Topic: %s\nParent Node: %s\nDetails: %s", req.Concept, req.NodeLabel, req.NodeDetails)

	var resp dto.ExpandNodeResponse
	resp.NodeID = req.NodeID
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &resp)
	if err != nil || len(resp.NewNodes) == 0 {
		// Provide intelligent fallback expansion if AI fails
		resp.NewNodes = []dto.GraphNode{
			{
				ID:          req.NodeID + "_deep_1",
				Label:       req.NodeLabel + " Mechanism",
				Description: "Deep dive into algorithmic invariants and implementation nuances for " + req.NodeLabel,
				NodeType:    "leaf",
				BranchID:    req.NodeID,
				ColorHex:    req.BranchColor,
				Importance:  4,
				Badge:       "Mechanics",
				Metrics:     map[string]string{"Aspect": "In-Depth Execution"},
			},
			{
				ID:          req.NodeID + "_deep_2",
				Label:       req.NodeLabel + " Edge Cases",
				Description: "Critical edge cases, boundary failures, and optimization strategies",
				NodeType:    "leaf",
				BranchID:    req.NodeID,
				ColorHex:    req.BranchColor,
				Importance:  5,
				Badge:       "Edge Cases",
				Metrics:     map[string]string{"Risk": "Degradation Pitfall"},
			},
		}
		resp.NewEdges = []dto.GraphEdge{
			{
				From:   req.NodeID,
				To:     req.NodeID + "_deep_1",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_deep_1",
				Label:  "elaborates",
			},
			{
				From:   req.NodeID,
				To:     req.NodeID + "_deep_2",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_deep_2",
				Label:  "edge cases",
			},
		}
	}

	for i := range resp.NewNodes {
		if resp.NewNodes[i].ColorHex == "" && req.BranchColor != "" {
			resp.NewNodes[i].ColorHex = req.BranchColor
		}
		resp.NewNodes[i].NodeType = "leaf"
		resp.NewNodes[i].BranchID = req.NodeID
	}
	for i := range resp.NewEdges {
		if resp.NewEdges[i].FromID == "" {
			resp.NewEdges[i].FromID = resp.NewEdges[i].From
		}
		if resp.NewEdges[i].ToID == "" {
			resp.NewEdges[i].ToID = resp.NewEdges[i].To
		}
	}

	return &resp, nil
}

var branchPalette = []string{
	"#6366F1", // Indigo
	"#10B981", // Emerald
	"#F59E0B", // Amber
	"#0EA5E9", // Sky Blue
	"#EC4899", // Rose
	"#8B5CF6", // Violet
	"#14B8A6", // Teal
}

func flattenMindMapTree(node *dto.MindMapNode, nodes *[]dto.GraphNode, edges *[]dto.GraphEdge, parentID, branchID string, branchIndex int) {
	if node == nil {
		return
	}

	nodeType := "leaf"
	if parentID == "" {
		nodeType = "root"
	} else if len(node.Children) > 0 {
		nodeType = "branch"
		branchID = node.ID
	}

	color := node.ColorHex
	if color == "" {
		if nodeType == "root" {
			color = "#4F46E5"
		} else if nodeType == "branch" {
			color = branchPalette[branchIndex%len(branchPalette)]
		} else {
			color = branchPalette[branchIndex%len(branchPalette)]
		}
	}

	importance := node.Importance
	if importance <= 0 {
		if nodeType == "root" {
			importance = 5
		} else if nodeType == "branch" {
			importance = 4
		} else {
			importance = 3
		}
	}

	badge := node.Badge
	if badge == "" {
		if nodeType == "root" {
			badge = "Core Foundation"
		} else if nodeType == "branch" {
			badge = "Major Pillar"
		} else if len(node.Formulas) > 0 {
			badge = "Key Formula"
		} else {
			badge = "High Yield"
		}
	}

	*nodes = append(*nodes, dto.GraphNode{
		ID:          node.ID,
		Label:       node.Label,
		Description: node.Details,
		NodeType:    nodeType,
		BranchID:    branchID,
		DomainType:  node.DomainType,
		KeyTakeaway: node.KeyTakeaway,
		ExamTrap:    node.ExamTrap,
		Importance:  importance,
		Badge:       badge,
		Metrics:     node.Metrics,
		CodeSnippet: node.CodeSnippet,
		CrossLinks:  node.CrossLinks,
		ColorHex:    color,
		Formulas:    node.Formulas,
	})

	if parentID != "" {
		*edges = append(*edges, dto.GraphEdge{
			From:   parentID,
			To:     node.ID,
			FromID: parentID,
			ToID:   node.ID,
			Label:  "branches into",
			Arrow:  true,
		})
	}

	for _, targetID := range node.CrossLinks {
		if targetID != "" {
			*edges = append(*edges, dto.GraphEdge{
				From:        node.ID,
				To:          targetID,
				FromID:      node.ID,
				ToID:        targetID,
				Label:       "relates to",
				Arrow:       true,
				IsCrossLink: true,
				Style:       "dashed",
			})
		}
	}

	for i, child := range node.Children {
		idx := branchIndex
		if nodeType == "root" {
			idx = i
		}
		flattenMindMapTree(child, nodes, edges, node.ID, branchID, idx)
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

	type fallbackLeaf struct {
		id       string
		label    string
		details  string
		color    string
		formulas []string
		metrics  map[string]string
		snippet  string
		trap     string
	}

	type fallbackBranch struct {
		id       string
		label    string
		details  string
		color    string
		formulas []string
		metrics  map[string]string
		leaves   []fallbackLeaf
	}

	var branches []fallbackBranch

	if strings.Contains(lower, "sort") || strings.Contains(lower, "quick") || strings.Contains(lower, "algorithm") {
		resp.DomainType = "algorithm"
		branches = []fallbackBranch{
			{
				id: "b1", label: "Asymptotic Complexity", details: "Divide-and-conquer recurrence and rigorous runtime analysis", color: "#6366F1",
				formulas: []string{`T(n) = 2T(n/2) + \Theta(n)`, `T(n) = T(n-1) + \Theta(n)`},
				metrics:  map[string]string{"Best Case": "O(N log N)", "Worst Case": "O(N^2)", "Aux Space": "O(log N)"},
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "Average & Best: O(N log N)",
						details: "Even balanced split yields tree height log2(n) with n work per level",
						color:   "#818CF8",
						formulas: []string{`T(n) = 2T(n/2) + O(n) \implies O(N \log N)`},
						metrics: map[string]string{"Condition": "Balanced Pivot Split", "Complexity": "O(N log N)"},
						snippet: "pivot = arr[high]\ni = low - 1\nfor j in range(low, high):\n  if arr[j] <= pivot: i += 1; swap(arr[i], arr[j])\nswap(arr[i+1], arr[high])",
						trap:    "Assuming best case occurs only when pivot is exact median",
					},
					{
						id: "l1_2", label: "Worst Case: O(N^2)",
						details: "Degenerates to skewed linear chain when array is already sorted and pivot is first/last",
						color:   "#4F46E5",
						formulas: []string{`T(n) = T(n-1) + O(n) \implies O(N^2)`},
						metrics: map[string]string{"Condition": "Sorted / Reverse Sorted", "Worst Case": "O(N^2)"},
						trap:    "Failing to randomize pivot causes StackOverflow on large sorted arrays",
					},
					{
						id: "l1_3", label: "Space: O(log N) Call Stack",
						details: "In-place array swaps require only auxiliary recursion call stack memory",
						color:   "#4338CA",
						metrics: map[string]string{"Auxiliary Space": "O(log N)", "In-Place": "Yes", "Stability": "Unstable"},
						trap:    "Claiming QuickSort is O(1) space: call stack uses O(log N) memory",
					},
				},
			},
			{
				id: "b2", label: "Partitioning Schemes", details: "Lomuto vs Hoare two-pointer invariant mechanics", color: "#0EA5E9",
				metrics: map[string]string{"Standard": "Hoare (Fewer Swaps)", "Simple": "Lomuto"},
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "Hoare's Partition",
						details: "Two pointers converge from both ends; performs 3x fewer swaps than Lomuto",
						color:   "#38BDF8",
						metrics: map[string]string{"Swaps": "O(N/3) avg", "Pointers": "Two-way inward"},
						snippet: "i = low - 1; j = high + 1\nwhile true:\n  do i++ while arr[i] < pivot\n  do j-- while arr[j] > pivot\n  if i >= j return j; swap(arr[i], arr[j])",
						trap:    "Pivot is not necessarily placed at its final sorted position in Hoare's",
					},
					{
						id: "l2_2", label: "Lomuto's Partition",
						details: "Single forward scanner; easier to implement and reason about",
						color:   "#0284C7",
						metrics: map[string]string{"Swaps": "O(N) swaps", "Direction": "Unidirectional"},
						trap:    "Extremely slow with identical elements (all duplicates cause O(N^2))",
					},
				},
			},
			{
				id: "b3", label: "Pivot Selection Strategies", details: "Techniques to prevent degradation to O(N^2)", color: "#10B981",
				metrics: map[string]string{"Best Practical": "Median-of-3", "Dual-Pivot": "Java Arrays.sort"},
				leaves: []fallbackLeaf{
					{
						id: "l3_1", label: "Median-of-Three",
						details: "Takes median of {first, middle, last} elements; immunizes against sorted inputs",
						color:   "#34D399",
						metrics: map[string]string{"Samples": "First, Mid, Last", "Overhead": "3 comparisons"},
						trap:    "Killer adversaries can still trigger O(N^2) without randomized shuffling",
					},
					{
						id: "l3_2", label: "Dual-Pivot & IntroSort",
						details: "Uses 2 pivots partitioning into 3 regions (used in Java & Python timsort hybrids)",
						color:   "#059669",
						metrics: map[string]string{"Partitions": "3-way", "Cache Misses": "Reduced by 20%"},
						trap:    "Falling back to HeapSort when recursion depth exceeds 2*log(N)",
					},
				},
			},
			{
				id: "b4", label: "Trade-offs vs MergeSort", details: "Cache locality and memory comparison in production systems", color: "#F59E0B",
				metrics: map[string]string{"QuickSort": "In-Place & Cache-Friendly", "MergeSort": "Stable & O(N) Space"},
				leaves: []fallbackLeaf{
					{
						id: "l4_1", label: "Cache Locality & Constants",
						details: "Linear memory scans have high spatial locality in CPU L1/L2 caches",
						color:   "#FBBF24",
						metrics: map[string]string{"Hardware": "L1/L2 Cache Friendly", "Hidden Constant": "Very Small"},
						trap:    "Assuming MergeSort is faster because both are O(N log N); QuickSort has much lower constant factor",
					},
					{
						id: "l4_2", label: "Stability & Failure Modes",
						details: "QuickSort is fundamentally unstable; swaps can invert relative order of equal keys",
						color:   "#D97706",
						metrics: map[string]string{"Stability": "Unstable", "Solution": "Pair with Index"},
						trap:    "Using standard QuickSort when stable sorting of multi-field records is required",
					},
				},
			},
		}
	} else if strings.Contains(lower, "thermo") || strings.Contains(lower, "heat") {
		branches = []fallbackBranch{
			{
				id: "b1", label: "Zeroth & First Laws", details: "Equilibrium and conservation of energy", color: "#0EA5E9",
				formulas: []string{`\Delta U = Q - W`, `W = \int P \, dV`},
				leaves: []fallbackLeaf{
					{id: "l1_1", label: "Thermal Equilibrium", details: "Zeroth law establishes temperature scale", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Internal Energy", details: "State function independent of path", color: "#0284C7", formulas: []string{`dU = n C_v dT`}},
				},
			},
			{
				id: "b2", label: "Second Law & Entropy", details: "Direction of spontaneous thermal processes", color: "#8B5CF6",
				formulas: []string{`\Delta S \ge 0`, `dS = \frac{dQ_{rev}}{T}`},
				leaves: []fallbackLeaf{
					{id: "l2_1", label: "Clausius & Kelvin Statements", details: "No 100% efficient heat engine exists", color: "#A855F7", formulas: nil},
					{id: "l2_2", label: "Carnot Engine", details: "Maximum theoretical efficiency between two reservoirs", color: "#7C3AED", formulas: []string{`\eta = 1 - \frac{T_C}{T_H}`}},
				},
			},
			{
				id: "b3", label: "Thermodynamic Processes", details: "Path-dependent gas transformations", color: "#10B981",
				formulas: []string{`P V^\gamma = \text{const}`},
				leaves: []fallbackLeaf{
					{id: "l3_1", label: "Isothermal vs Adiabatic", details: "Constant temperature vs zero heat transfer", color: "#34D399", formulas: []string{`W_{iso} = nRT \ln(V_f/V_i)`}},
					{id: "l3_2", label: "Isobaric & Isochoric", details: "Constant pressure and constant volume work", color: "#059669", formulas: []string{`W = P \Delta V`}},
				},
			},
			{
				id: "b4", label: "Free Energy & Potentials", details: "Spontaneity criteria in open and closed systems", color: "#F59E0B",
				formulas: []string{`\Delta G = \Delta H - T\Delta S`},
				leaves: []fallbackLeaf{
					{id: "l4_1", label: "Gibbs Free Energy", details: "Drives chemical and thermodynamic equilibrium", color: "#FBBF24", formulas: nil},
					{id: "l4_2", label: "Enthalpy & Latent Heat", details: "Total heat content and phase transition energies", color: "#D97706", formulas: []string{`H = U + PV`}},
				},
			},
		}
	} else if strings.Contains(lower, "architecture") || strings.Contains(lower, "cpu") || strings.Contains(lower, "computer") {
		branches = []fallbackBranch{
			{
				id: "b1", label: "CPU & Execution Core", details: "Arithmetic logic and instruction processing", color: "#0EA5E9",
				formulas: []string{`\text{CPU Time} = I \times \text{CPI} \times T_{clk}`},
				leaves: []fallbackLeaf{
					{id: "l1_1", label: "ALU & Registers", details: "High-speed internal registers and integer/FP units", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Control Unit (CU)", details: "Instruction fetch, decode, and microcode dispatch", color: "#0284C7", formulas: nil},
				},
			},
			{
				id: "b2", label: "Memory Hierarchy", details: "Multi-level caching and address translation", color: "#10B981",
				formulas: []string{`T_{eff} = h \cdot T_{cache} + (1-h) \cdot T_{mem}`},
				leaves: []fallbackLeaf{
					{id: "l2_1", label: "L1/L2/L3 Cache", details: "SRAM associative caches minimizing latency", color: "#34D399", formulas: nil},
					{id: "l2_2", label: "Virtual Memory & MMU", details: "Paging, TLB lookups, and page fault handling", color: "#059669", formulas: nil},
				},
			},
			{
				id: "b3", label: "Instruction Pipeline", details: "Overlapped instruction execution stages", color: "#8B5CF6",
				formulas: []string{`\text{Speedup} = \frac{k \cdot n}{k + n - 1}`},
				leaves: []fallbackLeaf{
					{id: "l3_1", label: "Pipeline Hazards", details: "Structural, data (RAW/WAR), and control stalls", color: "#A855F7", formulas: nil},
					{id: "l3_2", label: "Branch Prediction", details: "Dynamic branch prediction and speculative execution", color: "#7C3AED", formulas: nil},
				},
			},
			{
				id: "b4", label: "System Bus & I/O", details: "Interconnect protocols and peripheral controllers", color: "#F59E0B",
				formulas: nil,
				leaves: []fallbackLeaf{
					{id: "l4_1", label: "DMA Controller", details: "Direct memory access bypassing CPU for high throughput", color: "#FBBF24", formulas: nil},
					{id: "l4_2", label: "PCIe & System Fabric", details: "Point-to-point serial interconnect lanes", color: "#D97706", formulas: nil},
				},
			},
		}
	} else {
		// General Academic Concept structure
		branches = []fallbackBranch{
			{
				id: "b1", label: "Foundations & Axioms", details: fmt.Sprintf("Fundamental laws and axioms of %s", concept), color: "#0EA5E9",
				formulas: nil,
				leaves: []fallbackLeaf{
					{id: "l1_1", label: "Definitions & Units", details: "Core terminology, parameters, and SI standards", color: "#38BDF8", formulas: nil},
					{id: "l1_2", label: "Initial Assumptions", details: "Boundary conditions and validity range", color: "#0284C7", formulas: nil},
				},
			},
			{
				id: "b2", label: "Mechanisms & Dynamics", details: "Step-by-step physical and logical working", color: "#10B981",
				formulas: nil,
				leaves: []fallbackLeaf{
					{id: "l2_1", label: "Core Working Flow", details: "Cause-and-effect chain and transitions", color: "#34D399", formulas: nil},
					{id: "l2_2", label: "State Transformations", details: "Key intermediate phases and lifecycle", color: "#059669", formulas: nil},
				},
			},
			{
				id: "b3", label: "Mathematical Equations", details: "Analytical formulation and quantitative models", color: "#8B5CF6",
				formulas: []string{`f(x) = \sum_{n=1}^\infty a_n \phi_n(x)`},
				leaves: []fallbackLeaf{
					{id: "l3_1", label: "Governing Formula", details: "Central mathematical expression and constants", color: "#A855F7", formulas: []string{`y = f(x)`}},
					{id: "l3_2", label: "Dimensional Analysis", details: "Units consistency and balance checks", color: "#7C3AED", formulas: nil},
				},
			},
			{
				id: "b4", label: "Applications & Exam Traps", details: "High-yield scoring rules and common blunders", color: "#F59E0B",
				formulas: nil,
				leaves: []fallbackLeaf{
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
			Metrics:     b.metrics,
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
			Metrics:  b.metrics,
		}

		for _, l := range b.leaves {
			lNode := dto.GraphNode{
				ID:          l.id,
				Label:       l.label,
				Description: l.details,
				NodeType:    "leaf",
				ColorHex:    l.color,
				Formulas:    l.formulas,
				Metrics:     l.metrics,
				CodeSnippet: l.snippet,
				ExamTrap:    l.trap,
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
				ID:          l.id,
				Label:       l.label,
				Details:     l.details,
				ColorHex:    l.color,
				Metrics:     l.metrics,
				CodeSnippet: l.snippet,
				ExamTrap:    l.trap,
			})
		}

		rootTreeNode.Children = append(rootTreeNode.Children, bTreeNode)
	}

	resp.Root = rootTreeNode
	return resp
}

func (s *diagramService) SaveMindMap(ctx context.Context, userID string, req dto.SaveMindMapRequest) (*dto.SavedMindMapResponse, error) {
	if s.mindMapRepo == nil {
		return nil, errors.New("mind map repository not initialized")
	}
	m := &model.MindMap{
		UserID:      userID,
		Title:       req.Title,
		Concept:     req.Concept,
		DiagramType: req.DiagramType,
		DomainType:  req.DomainType,
		NoteID:      req.NoteID,
		Data:        req.Data,
		IsPinned:    req.IsPinned,
	}
	if err := s.mindMapRepo.Create(ctx, m); err != nil {
		return nil, err
	}
	return toSavedMindMapResponse(m), nil
}

func (s *diagramService) GetMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error) {
	if s.mindMapRepo == nil {
		return nil, errors.New("mind map repository not initialized")
	}
	m, err := s.mindMapRepo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toSavedMindMapResponse(m), nil
}

func (s *diagramService) ListMindMaps(ctx context.Context, userID string, search string, limit, offset int) ([]dto.SavedMindMapSummary, int, error) {
	if s.mindMapRepo == nil {
		return nil, 0, errors.New("mind map repository not initialized")
	}
	list, total, err := s.mindMapRepo.List(ctx, userID, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]dto.SavedMindMapSummary, 0, len(list))
	for _, item := range list {
		summaries = append(summaries, dto.SavedMindMapSummary{
			ID:          item.ID,
			Title:       item.Title,
			Concept:     item.Concept,
			DiagramType: item.DiagramType,
			DomainType:  item.DomainType,
			NoteID:      item.NoteID,
			IsPinned:    item.IsPinned,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return summaries, total, nil
}

func (s *diagramService) UpdateMindMap(ctx context.Context, userID, id string, req dto.UpdateMindMapRequest) (*dto.SavedMindMapResponse, error) {
	if s.mindMapRepo == nil {
		return nil, errors.New("mind map repository not initialized")
	}
	var rawData json.RawMessage
	if req.Data != nil {
		rawData = *req.Data
	}
	m, err := s.mindMapRepo.Update(ctx, userID, id, req.Title, rawData, req.IsPinned)
	if err != nil {
		return nil, err
	}
	return toSavedMindMapResponse(m), nil
}

func (s *diagramService) DeleteMindMap(ctx context.Context, userID, id string) error {
	if s.mindMapRepo == nil {
		return errors.New("mind map repository not initialized")
	}
	return s.mindMapRepo.Delete(ctx, userID, id)
}

func (s *diagramService) TogglePinMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error) {
	if s.mindMapRepo == nil {
		return nil, errors.New("mind map repository not initialized")
	}
	m, err := s.mindMapRepo.TogglePin(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return toSavedMindMapResponse(m), nil
}

func toSavedMindMapResponse(m *model.MindMap) *dto.SavedMindMapResponse {
	return &dto.SavedMindMapResponse{
		ID:          m.ID,
		Title:       m.Title,
		Concept:     m.Concept,
		DiagramType: m.DiagramType,
		DomainType:  m.DomainType,
		NoteID:      m.NoteID,
		Data:        m.Data,
		IsPinned:    m.IsPinned,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

