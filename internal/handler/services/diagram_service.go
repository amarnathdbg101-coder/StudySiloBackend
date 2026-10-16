package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type DiagramService interface {
	GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error)
	ExpandNode(ctx context.Context, userID string, req dto.ExpandNodeRequest) (*dto.ExpandNodeResponse, error)
	GenerateQuizFromMindMap(ctx context.Context, userID string, req dto.MindMapToQuizRequest) (*model.Quiz, error)
	GenerateFlashcardsFromMindMap(ctx context.Context, userID string, req dto.MindMapToFlashcardRequest) (*dto.GenerateFlashcardDeckResponse, error)
	ExportMindMapPDF(ctx context.Context, req dto.ExportMindMapPDFRequest) ([]byte, error)
	SaveMindMap(ctx context.Context, userID string, req dto.SaveMindMapRequest) (*dto.SavedMindMapResponse, error)
	GetMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error)
	ListMindMaps(ctx context.Context, userID string, search string, limit, offset int) ([]dto.SavedMindMapSummary, int, error)
	UpdateMindMap(ctx context.Context, userID, id string, req dto.UpdateMindMapRequest) (*dto.SavedMindMapResponse, error)
	DeleteMindMap(ctx context.Context, userID, id string) error
	TogglePinMindMap(ctx context.Context, userID, id string) (*dto.SavedMindMapResponse, error)
}

type diagramService struct {
	orchestrator  *ai.Orchestrator
	mindMapRepo   repository.MindMapRepository
	quizRepo      repository.QuizRepository
	flashcardRepo repository.FlashcardRepository
	pdfService    PDFExportService
}

func NewDiagramService(
	orchestrator *ai.Orchestrator,
	mindMapRepo repository.MindMapRepository,
	quizRepo repository.QuizRepository,
	flashcardRepo repository.FlashcardRepository,
	pdfService PDFExportService,
) DiagramService {
	return &diagramService{
		orchestrator:  orchestrator,
		mindMapRepo:   mindMapRepo,
		quizRepo:      quizRepo,
		flashcardRepo: flashcardRepo,
		pdfService:    pdfService,
	}
}

func (s *diagramService) GenerateDiagram(ctx context.Context, userID string, req dto.DiagramRequest) (*dto.DiagramResponse, error) {
	diagramType := req.DiagramType
	if diagramType == "" {
		diagramType = "mindmap"
	}

	systemPrompt := `You are an elite academic Master Professor and Visual Knowledge Architect (combining Feynman's first-principles intuition, 3Blue1Brown's visual clarity, and top competitive exam rigor like JEE, NEET, GATE, and USMLE).
Your mission is to generate a deep, engaging, scientifically rigorous, and high-yield visual mind map for the requested student concept.

CRITICAL PEDAGOGICAL RULES (NEVER VIOLATE):
1. ZERO VAGUE PLACEHOLDERS: Never write generic statements like "Time & Space Complexity" without exact Big-O and conditions. Never just say "Crucial enzyme" without stating its specific active site mechanism and kinetic parameters.
2. 3-PART DEEP LEARNING STRUCTURE FOR EVERY NODE'S DETAILS:
   - 💡 Intuition: 1-2 sentences with a memorable real-world analogy or mental model (Feynman technique).
   - ⚙️ Mechanism & Exact Logic: Precise scientific/algorithmic explanation of how it operates step-by-step.
   - 📌 Key Takeaway: Concrete, high-yield factual or numerical takeaway.
3. CONCRETE QUANTITATIVE METRICS: Every node MUST populate "metrics" with 2 to 4 tangible key-value pairs (e.g., Big-O time/space, SI units, threshold limits, typical temperatures, ATP yields, or throughput specs). NEVER leave empty!
4. EXAM TRAP ("exam_trap"): Provide the #1 student misconception tested in competitive exams in format:
   "❌ False Belief: [What 90% students answer wrong] | ✅ Truth: [Exact scientific principle] | ⚠️ Examiner Twist: [How tricky questions are designed]".
5. VIVA & INTERVIEW DRILL ("viva_insight"): Practical interviewer/oral examiner question with sharp 2-line model answer:
   "🎯 Viva Question: [Tough question] | 💡 Model Answer: [High-impact 2-line answer]".
6. FORMULAS & EQUATIONS ("formula_latex"): When applicable, provide standard KaTeX LaTeX equations with variables cleanly formatted (e.g. "\Delta G^\circ = -RT \ln K_{eq}" or "T(n) = 2T(n/2) + \Theta(n)").
7. ALGORITHM / CODE SNIPPET ("code_snippet"): For computer science, algorithms, or math topics, include a 2-4 line pseudocode, recurrence, or clean syntax invariant.
8. CROSS-DISCIPLINARY LINKS ("cross_subject"): Interlink with other scientific or mathematical fields (e.g. "Physics ↔ Chemistry: Second Law of Thermodynamics").

6 Specialized Domain Archetypes:
Detect the conceptual nature of the topic (or use requested archetype if provided) and populate "archetype":
1. "tree" (Taxonomy / Classification): E.g., Operating System Schedulers, Plant Kingdom, Network Protocols.
2. "flow" (Algorithm & Process): E.g., Merge Sort dry-run, Nerve Impulse Transmission, Glycolysis enzymatic steps.
3. "cycle" (Closed Loop Cycles): E.g., Krebs Cycle, Carnot Heat Engine, Nitrogen/Calvin Cycle.
4. "comparison" (Versus Matrix): E.g., TCP vs UDP, OSI vs TCP/IP, RISC vs CISC.
5. "hub" (360° Concept Core): E.g., Newton's Laws of Motion, Quantum Mechanics.
6. "stack" (Layered Architecture): E.g., Memory Hierarchy, OSI 7-Layer Stack.

Structure Requirements:
- 1 Central Root Node (comprehensive overview).
- 4 to 6 Major Conceptual Branches (color-coded foundational pillars).
- 2 to 3 Detailed Leaf Nodes per branch (deep-dive stations with formulas, metrics, and traps).
- Populate "cross_links" with target node IDs if nodes causally influence each other.

Output a single valid JSON object strictly matching this schema:
{
  "concept": "Concept Name",
  "title": "Authoritative Master Title",
  "archetype": "cycle|flow|tree|comparison|hub|stack",
  "summary": "1-sentence executive summary of the entire map",
  "diagram_type": "mindmap",
  "domain_type": "algorithm|math|science|chemistry|biology|general",
  "root": {
    "id": "root_1",
    "label": "Core Concept Name",
    "subtitle": "Overview & Governing Scope",
    "details": "💡 Intuition: Core mental model. ⚙️ Mechanism: Overarching scientific/logical foundation. 📌 Key Takeaway: Central thesis.",
    "description": "💡 Intuition: Core mental model. ⚙️ Mechanism: Overarching scientific/logical foundation. 📌 Key Takeaway: Central thesis.",
    "shape": "circle",
    "type": "concept",
    "color_hex": "#4F46E5",
    "domain_type": "algorithm",
    "formula_latex": "",
    "key_takeaway": "Central foundation connecting all surrounding branches",
    "exam_trap": "",
    "viva_insight": "",
    "importance": 5,
    "badge": "Core Foundation",
    "metrics": {"Domain": "Core Foundation", "Scope": "Comprehensive"},
    "formulas": [],
    "children": [
      {
        "id": "branch_1",
        "label": "Branch Name",
        "subtitle": "Pillar focus area",
        "details": "💡 Intuition: Mental model. ⚙️ Mechanism: Core governing law or structural architecture. 📌 Key Takeaway: Critical branch law.",
        "description": "💡 Intuition: Mental model. ⚙️ Mechanism: Core governing law or structural architecture. 📌 Key Takeaway: Critical branch law.",
        "shape": "box",
        "type": "process",
        "color_hex": "#6366F1",
        "domain_type": "algorithm",
        "formula_latex": "",
        "key_takeaway": "Core takeaway for this branch",
        "exam_trap": "❌ False: ... | ✅ Truth: ... | ⚠️ Examiner Twist: ...",
        "viva_insight": "🎯 Viva Question: ... | 💡 Model Answer: ...",
        "importance": 5,
        "badge": "Major Pillar",
        "metrics": {"Key": "Value"},
        "formulas": [],
        "children": [
          {
            "id": "leaf_1_1",
            "label": "Specific Law / Topic",
            "subtitle": "Sub-mechanism & Phase",
            "details": "💡 Intuition: Vivid analogy. ⚙️ Mechanism: Step-by-step operational mechanics. 📌 Key Takeaway: Crucial fact.",
            "description": "💡 Intuition: Vivid analogy. ⚙️ Mechanism: Step-by-step operational mechanics. 📌 Key Takeaway: Crucial fact.",
            "shape": "capsule",
            "type": "equation",
            "color_hex": "#818CF8",
            "domain_type": "algorithm",
            "formula_latex": "\\Delta G = \\Delta H - T\\Delta S",
            "key_takeaway": "Key exam takeaway",
            "exam_trap": "❌ False: ... | ✅ Truth: ... | ⚠️ Examiner Twist: ...",
            "viva_insight": "🎯 Viva Question: ... | 💡 Model Answer: ...",
            "cross_subject": "Physics ↔ Chemistry",
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

	userPrompt := fmt.Sprintf(`Topic / Concept: "%s"
Diagram Type: %s
Architectural Directives:
- Build a master-grade, highly engaging visual mind map for "%s".
- Select the best Archetype (tree, flow, cycle, comparison, hub, or stack) to represent this topic naturally.
- Break down into 4 to 6 strong conceptual branches (e.g. Foundations & Axioms, Step-by-step Mechanics, Mathematical Derivations, Edge Cases & Failure Modes, Real-world Benchmarks).
- For every node, deliver rich 3-part details (💡 Intuition analogy + ⚙️ Technical mechanism + 📌 Key Takeaway).
- Include concrete numbers/complexities in "metrics", precise KaTeX equations in "formula_latex", razor-sharp "exam_trap" gotchas, and tough "viva_insight" interview drills.`, req.Concept, diagramType, req.Concept)

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

	// Resolve and enforce the 6 Domain Archetypes (flow, cycle, comparison, stack, tree, hub)
	lowerConcept := strings.ToLower(req.Concept)
	if req.Archetype != "" && req.Archetype != "auto" {
		diagram.Archetype = req.Archetype
	} else if diagram.Archetype == "" || diagram.Archetype == "auto" {
		if strings.Contains(lowerConcept, "flow") || strings.Contains(lowerConcept, "sort") || strings.Contains(lowerConcept, "search") || strings.Contains(lowerConcept, "algorithm") || strings.Contains(lowerConcept, "pipeline") || strings.Contains(lowerConcept, "glycolysis") || strings.Contains(lowerConcept, "respiration") {
			diagram.Archetype = "flow"
		} else if strings.Contains(lowerConcept, "cycle") || strings.Contains(lowerConcept, "krebs") || strings.Contains(lowerConcept, "carnot") || strings.Contains(lowerConcept, "nitrogen") || strings.Contains(lowerConcept, "calvin") {
			diagram.Archetype = "cycle"
		} else if strings.Contains(lowerConcept, "comparison") || strings.Contains(lowerConcept, " vs ") || strings.Contains(lowerConcept, "versus") || strings.Contains(lowerConcept, "compare") || strings.Contains(lowerConcept, "difference") {
			diagram.Archetype = "comparison"
		} else if strings.Contains(lowerConcept, "stack") || strings.Contains(lowerConcept, "osi") || strings.Contains(lowerConcept, "layer") || strings.Contains(lowerConcept, "hierarchy") || strings.Contains(lowerConcept, "memory hierarchy") {
			diagram.Archetype = "stack"
		} else if strings.Contains(lowerConcept, "hub") || strings.Contains(lowerConcept, "newton") || strings.Contains(lowerConcept, "quantum") || strings.Contains(lowerConcept, "motion") {
			diagram.Archetype = "hub"
		} else {
			diagram.Archetype = "tree"
		}
	}

	// Normalize and cross-fill description, details, and formulas for all nodes
	for i := range diagram.Nodes {
		if diagram.Nodes[i].Description == "" && diagram.Nodes[i].Details != "" {
			diagram.Nodes[i].Description = diagram.Nodes[i].Details
		}
		if diagram.Nodes[i].Details == "" && diagram.Nodes[i].Description != "" {
			diagram.Nodes[i].Details = diagram.Nodes[i].Description
		}
		if diagram.Nodes[i].FormulaLatex != "" && len(diagram.Nodes[i].Formulas) == 0 {
			diagram.Nodes[i].Formulas = []string{diagram.Nodes[i].FormulaLatex}
		}
		if len(diagram.Nodes[i].Formulas) > 0 && diagram.Nodes[i].FormulaLatex == "" {
			diagram.Nodes[i].FormulaLatex = diagram.Nodes[i].Formulas[0]
		}
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
	systemPrompt := `You are an elite academic Master Professor and Visual Concept Architect.
The student is exploring an interactive 4D Mind Map for: "` + req.Concept + `".
They tapped on node: "` + req.NodeLabel + `" (Details: ` + req.NodeDetails + `) and requested a deep-dive expansion.

Generate exactly 3 to 4 deep-dive child sub-branches that explore the internal sub-mechanisms, mathematical derivations, edge cases, failure modes, or advanced applications of this specific node.

Pedagogical Rules:
- NEVER REPEAT parent info. Provide fresh, microscopic-level deep dives.
- For each sub-branch node's description:
  1. 💡 Intuition: A crisp, 1-line mental model or analogy.
  2. ⚙️ Mechanism: Step-by-step technical explanation.
  3. 📌 Takeaway: 1 high-yield revision bullet.
- "metrics": Provide 2 to 4 tangible specs (e.g. Big-O, units, limits, benchmarks).
- "formula_latex": Primary KaTeX formula for this sub-concept if applicable.
- "exam_trap": Specific student gotcha in format: "❌ False: ... | ✅ Truth: ... | ⚠️ Examiner Twist: ...".
- "viva_insight": Tough oral/interview question with 2-line model answer: "🎯 Viva: ... | 💡 Answer: ...".

For EVERY generated sub-branch node provide:
1. "id": Unique string (e.g. "` + req.NodeID + `_sub_1").
2. "label": Crisp title (2 to 5 words).
3. "subtitle": Specific sub-context or focus area.
4. "description": Deep technical explanation with 💡 Intuition, ⚙️ Mechanism, and 📌 Takeaway.
5. "details": Same content as description.
6. "shape": "capsule" | "box" | "diamond".
7. "type": "process" | "equation" | "concept" | "trap".
8. "formula_latex": Primary KaTeX formula if applicable.
9. "exam_trap": Examiner mistake or pitfall in "❌ False: ... | ✅ Truth: ... | ⚠️ Examiner Twist: ..." format.
10. "viva_insight": Practical interviewer insight: "🎯 Viva: ... | 💡 Answer: ...".
11. "cross_subject": Cross-subject interlink if applicable.
12. "importance": Integer 1 to 5.
13. "badge": Conceptual tag ("Sub-Branch", "Derivation", "Gotcha", "Deep Dive").
14. "metrics": Key-value map of concrete numbers or complexity.

Output JSON strictly matching:
{
  "node_id": "` + req.NodeID + `",
  "new_nodes": [
    {
      "id": "` + req.NodeID + `_sub_1",
      "label": "Deep Sub-concept",
      "subtitle": "Internal Mechanism",
      "description": "💡 Intuition: ... ⚙️ Mechanism: ... 📌 Takeaway: ...",
      "details": "💡 Intuition: ... ⚙️ Mechanism: ... 📌 Takeaway: ...",
      "shape": "capsule",
      "type": "process",
      "formula_latex": "f(x) = y",
      "exam_trap": "❌ False: ... | ✅ Truth: ... | ⚠️ Examiner Twist: ...",
      "viva_insight": "🎯 Viva: ... | 💡 Answer: ...",
      "cross_subject": "Physics ↔ Chemistry",
      "importance": 4,
      "badge": "Sub-Branch",
      "metrics": {"Key": "Value"}
    }
  ],
  "new_edges": [
    {
      "from": "` + req.NodeID + `",
      "to": "` + req.NodeID + `_sub_1",
      "from_id": "` + req.NodeID + `",
      "to_id": "` + req.NodeID + `_sub_1",
      "label": "branches into",
      "style": "solid",
      "arrow": true
    }
  ]
}`

	userPrompt := fmt.Sprintf(`Topic: %s
Parent Node: "%s"
Parent Details: %s
Task: Generate 3 to 4 microscopic deep-dive sub-branches uncovering internal mechanics, formulas, exam traps, and viva drills for "%s".`, req.Concept, req.NodeLabel, req.NodeDetails, req.NodeLabel)

	var resp dto.ExpandNodeResponse
	resp.NodeID = req.NodeID
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &resp)
	if err != nil || len(resp.NewNodes) < 3 {
		// Provide intelligent fallback with 3-4 rich domain-aware sub-branches if AI fails or returns too few
		color := req.BranchColor
		if color == "" {
			color = "#0EA5E9"
		}

		resp.NewNodes = []dto.GraphNode{
			{
				ID:           req.NodeID + "_sub_1",
				Label:        req.NodeLabel + " Mechanism",
				Subtitle:     "Analytical Mechanics & Derivation",
				Description:  "Detailed operational principles, execution phases, and governing physics for " + req.NodeLabel,
				NodeType:     "leaf",
				Type:         "equation",
				Shape:        "capsule",
				BranchID:     req.NodeID,
				ColorHex:     color,
				FormulaLatex: `\mathcal{L} = T - V`,
				Importance:   4,
				Badge:        "Mechanism",
				Metrics:      map[string]string{"Execution Phase": "Active State", "Invariants": "Conserved"},
				VivaInsight:  "Explain the primary state transformation step and governing boundary invariants.",
			},
			{
				ID:           req.NodeID + "_sub_2",
				Label:        req.NodeLabel + " Examiner Traps",
				Subtitle:     "High-Yield Scoring Pitfalls",
				Description:  "Common boundary misconceptions, sign blunders, and trick questions tested in competitive exams",
				NodeType:     "leaf",
				Type:         "trap",
				Shape:        "diamond",
				BranchID:     req.NodeID,
				ColorHex:     "#F59E0B",
				ExamTrap:     "❌ False: Assumed universal validity across all regimes. ✅ Truth: Valid only under specified boundary invariants!",
				Importance:   5,
				Badge:        "Gotchas",
				Metrics:      map[string]string{"Risk": "Score Deduction Pitfall"},
			},
			{
				ID:           req.NodeID + "_sub_3",
				Label:        req.NodeLabel + " Numerical Benchmarks",
				Subtitle:     "Quantitative Metrics & Limits",
				Description:  "Asymptotic performance boundaries, hardware/circuit benchmarks, and operational constants",
				NodeType:     "leaf",
				Type:         "process",
				Shape:        "box",
				BranchID:     req.NodeID,
				ColorHex:     color,
				Importance:   4,
				Badge:        "Quantitative",
				Metrics:      map[string]string{"Worst Bound": "Bounded", "Efficiency": "Optimal"},
				CrossSubject: "Computer Science ↔ Applied Mathematics",
			},
			{
				ID:           req.NodeID + "_sub_4",
				Label:        req.NodeLabel + " Viva & Industry Nuance",
				Subtitle:     "Oral Examination & Modern Tech",
				Description:  "Practical industry engineering trade-offs, standard interview questions, and real-world system applications",
				NodeType:     "leaf",
				Type:         "concept",
				Shape:        "capsule",
				BranchID:     req.NodeID,
				ColorHex:     "#10B981",
				Importance:   3,
				Badge:        "Viva Insight",
				VivaInsight:  "How does this concept scale when inputs increase by 1000x in production systems?",
			},
		}

		resp.NewEdges = []dto.GraphEdge{
			{
				From:   req.NodeID,
				To:     req.NodeID + "_sub_1",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_sub_1",
				Label:  "mechanism",
				Arrow:  true,
				Style:  "solid",
			},
			{
				From:   req.NodeID,
				To:     req.NodeID + "_sub_2",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_sub_2",
				Label:  "gotchas",
				Arrow:  true,
				Style:  "solid",
			},
			{
				From:   req.NodeID,
				To:     req.NodeID + "_sub_3",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_sub_3",
				Label:  "metrics",
				Arrow:  true,
				Style:  "solid",
			},
			{
				From:   req.NodeID,
				To:     req.NodeID + "_sub_4",
				FromID: req.NodeID,
				ToID:   req.NodeID + "_sub_4",
				Label:  "viva insight",
				Arrow:  true,
				Style:  "solid",
			},
		}
	}

	for i := range resp.NewNodes {
		if resp.NewNodes[i].ColorHex == "" && req.BranchColor != "" {
			resp.NewNodes[i].ColorHex = req.BranchColor
		}
		resp.NewNodes[i].NodeType = "leaf"
		resp.NewNodes[i].BranchID = req.NodeID
		if resp.NewNodes[i].Shape == "" {
			resp.NewNodes[i].Shape = "capsule"
		}
		if resp.NewNodes[i].Type == "" {
			if resp.NewNodes[i].FormulaLatex != "" {
				resp.NewNodes[i].Type = "equation"
			} else if resp.NewNodes[i].ExamTrap != "" {
				resp.NewNodes[i].Type = "trap"
			} else {
				resp.NewNodes[i].Type = "process"
			}
		}
	}
	for i := range resp.NewEdges {
		if resp.NewEdges[i].FromID == "" {
			resp.NewEdges[i].FromID = resp.NewEdges[i].From
		}
		if resp.NewEdges[i].ToID == "" {
			resp.NewEdges[i].ToID = resp.NewEdges[i].To
		}
		if resp.NewEdges[i].Style == "" {
			resp.NewEdges[i].Style = "solid"
		}
		resp.NewEdges[i].Arrow = true
	}

	// Efficient Database Auto-Persistence: If mind_map_id is provided, automatically append nodes to PostgreSQL
	if req.MindMapID != "" && userID != "" && s.mindMapRepo != nil {
		savedMap, getErr := s.mindMapRepo.GetByID(ctx, userID, req.MindMapID)
		if getErr == nil && len(savedMap.Data) > 0 {
			var diagData dto.DiagramResponse
			if jsonErr := json.Unmarshal(savedMap.Data, &diagData); jsonErr == nil {
				for _, n := range resp.NewNodes {
					exists := false
					for _, existing := range diagData.Nodes {
						if existing.ID == n.ID {
							exists = true
							break
						}
					}
					if !exists {
						diagData.Nodes = append(diagData.Nodes, n)
					}
				}
				for _, e := range resp.NewEdges {
					exists := false
					for _, existing := range diagData.Edges {
						if existing.FromID == e.FromID && existing.ToID == e.ToID {
							exists = true
							break
						}
					}
					if !exists {
						diagData.Edges = append(diagData.Edges, e)
					}
				}
				updatedBytes, _ := json.Marshal(diagData)
				_, _ = s.mindMapRepo.Update(ctx, userID, req.MindMapID, nil, updatedBytes, nil)
			}
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

	formulaLatex := node.FormulaLatex
	if formulaLatex == "" && len(node.Formulas) > 0 {
		formulaLatex = node.Formulas[0]
	}

	shape := node.Shape
	if shape == "" {
		if nodeType == "root" {
			shape = "circle"
		} else if nodeType == "branch" {
			shape = "box"
		} else {
			shape = "capsule"
		}
	}

	nType := node.Type
	if nType == "" {
		if nodeType == "root" {
			nType = "concept"
		} else if nodeType == "branch" {
			nType = "process"
		} else if formulaLatex != "" {
			nType = "equation"
		} else {
			nType = "concept"
		}
	}

	desc := node.Details
	if desc == "" {
		desc = node.Description
	}

	formulas := node.Formulas
	if len(formulas) == 0 && formulaLatex != "" {
		formulas = []string{formulaLatex}
	}

	*nodes = append(*nodes, dto.GraphNode{
		ID:           node.ID,
		Label:        node.Label,
		Subtitle:     node.Subtitle,
		Description:  desc,
		Details:      desc,
		NodeType:     nodeType,
		Type:         nType,
		Shape:        shape,
		BranchID:     branchID,
		DomainType:   node.DomainType,
		KeyTakeaway:  node.KeyTakeaway,
		ExamTrap:     node.ExamTrap,
		VivaInsight:  node.VivaInsight,
		Importance:   importance,
		Badge:        badge,
		Metrics:      node.Metrics,
		CodeSnippet:  node.CodeSnippet,
		CrossLinks:   node.CrossLinks,
		CrossSubject: node.CrossSubject,
		ColorHex:     color,
		Formulas:     formulas,
		FormulaLatex: formulaLatex,
		ChildrenIDs:  node.ChildrenIDs,
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
	rootID := "root_1"

	resp := dto.DiagramResponse{
		Concept:     concept,
		Title:       concept,
		DiagramType: diagramType,
	}

	type fallbackLeaf struct {
		id           string
		label        string
		subtitle     string
		details      string
		shape        string
		nodeType     string
		color        string
		formulas     []string
		formulaLatex string
		metrics      map[string]string
		snippet      string
		trap         string
		viva         string
		crossSubject string
		crossLinks   []string
	}

	type fallbackBranch struct {
		id           string
		label        string
		subtitle     string
		details      string
		shape        string
		nodeType     string
		color        string
		formulas     []string
		formulaLatex string
		metrics      map[string]string
		trap         string
		viva         string
		crossSubject string
		leaves       []fallbackLeaf
	}

	var branches []fallbackBranch
	var archetype = "tree"
	var summary = fmt.Sprintf("Structured academic mind map for %s with governing formulas and traps.", concept)

	// 1. FLOW ARCHETYPE: Merge Sort, Algorithms, Glycolysis, Pipelines
	if strings.Contains(lower, "merge") || strings.Contains(lower, "sort") || strings.Contains(lower, "algorithm") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "glycolysis") || strings.Contains(lower, "respiration") || strings.Contains(lower, "pathway") || strings.Contains(lower, "flow") {
		archetype = "flow"
		if strings.Contains(lower, "sort") || strings.Contains(lower, "merge") || strings.Contains(lower, "algorithm") {
			resp.DomainType = "algorithm"
			resp.Title = "Merge Sort: Divide & Conquer Execution Flow"
			summary = "Deterministic 4-stage divide-and-conquer sequential pipeline with guaranteed O(N log N) asymptotic time complexity."
			branches = []fallbackBranch{
				{
					id: "b1", label: "Phase 1: Midpoint Divide", subtitle: "Halving Subproblem • O(1)",
					details: "💡 Intuition: Divide a thick telephone directory exactly in half until single name pages remain. ⚙️ Mechanism: Calculate mid = l + (r-l)/2 to guard against integer overflow. 📌 Takeaway: Recursion tree depth is strictly bounded by log2(N).",
					shape: "box", nodeType: "process", color: "#6366F1",
					formulaLatex: `m = l + \left\lfloor \frac{r - l}{2} \right\rfloor`,
					metrics: map[string]string{"Step Work": "O(1)", "Recursion Depth": "log2(N) levels"},
					trap: "❌ False Belief: `(l + r) / 2` is safe. ✅ Truth: For large arrays, `l + r` causes 32-bit signed integer overflow! Always write `l + (r - l) / 2`.",
					viva: "🎯 Viva Question: What is the recurrence relation of Merge Sort? 💡 Model Answer: T(n) = 2T(n/2) + Theta(n), resolving to Theta(n log n) by Master Theorem Case 2.",
					leaves: []fallbackLeaf{
						{
							id: "l1_1", label: "Base Case Guard", subtitle: "Size <= 1 Trivial Order",
							details: "💡 Intuition: A list of 1 element is already sorted. ⚙️ Mechanism: If left >= right, return immediately without further stack frame allocation. 📌 Takeaway: Halts recursion.",
							shape: "capsule", nodeType: "concept", color: "#818CF8",
							trap: "Missing base case guard triggers immediate StackOverflowException.",
						},
					},
				},
				{
					id: "b2", label: "Phase 2: Conquer (Recursion)", subtitle: "T(N) = 2T(N/2)",
					details: "💡 Intuition: Delegate sorting of the left half and right half independently. ⚙️ Mechanism: Invoke sort(left, mid) followed by sort(mid+1, right). 📌 Takeaway: Balanced subproblems eliminate worst-case skew.",
					shape: "box", nodeType: "process", color: "#0EA5E9",
					formulaLatex: `T(n) = 2T(n/2) + \Theta(n)`,
					metrics: map[string]string{"Best Case": "O(N log N)", "Worst Case": "O(N log N)"},
					trap: "❌ False Belief: MergeSort degrades to O(N^2) on reverse-sorted inputs. ✅ Truth: MergeSort maintains O(N log N) under ALL input distributions unconditionally!",
					viva: "🎯 Viva Question: Does MergeSort adapt to already-sorted arrays? 💡 Model Answer: Standard MergeSort still takes O(N log N), but checking if arr[mid] <= arr[mid+1] can optimize to O(N).",
					leaves: []fallbackLeaf{
						{
							id: "l2_1", label: "Recursion Stack Bounds", subtitle: "Stack Frame Overhead",
							details: "💡 Intuition: Maximum height of active stack frames. ⚙️ Mechanism: Call stack depth is log2(N); memory is reused upon return. 📌 Takeaway: O(log N) stack frames.",
							shape: "capsule", nodeType: "equation", color: "#38BDF8",
							metrics: map[string]string{"Call Stack": "O(log N)", "Work/Level": "O(N)"},
						},
					},
				},
				{
					id: "b3", label: "Phase 3: Two-Pointer Merge", subtitle: "Linear Auxiliary Assembly",
					details: "💡 Intuition: Zip two sorted decks of cards together by repeatedly taking the smaller top card. ⚙️ Mechanism: Compare arr[i] <= arr[j] and copy smaller into auxiliary buffer B. 📌 Takeaway: Requires O(N) auxiliary space.",
					shape: "box", nodeType: "process", color: "#10B981",
					formulaLatex: `\text{MergeTime}(n_1, n_2) \le n_1 + n_2 - 1`,
					metrics: map[string]string{"Merge Work": "O(N)", "Auxiliary Buffer": "O(N)", "Stability": "Stable (with <=)"},
					trap: "❌ False Belief: Using `<` preserves stability. ✅ Truth: You MUST use `<=` so equal elements from the left subarray precede right elements, preserving stable order!",
					viva: "🎯 Viva Question: Why is MergeSort preferred over QuickSort for Linked Lists? 💡 Model Answer: Linked lists allow pointer rewiring in O(1) space, eliminating the O(N) array allocation overhead!",
					leaves: []fallbackLeaf{
						{
							id: "l3_1", label: "Linear Merge Invariant", subtitle: "Two-Pointer Logic",
							details: "💡 Intuition: Maintain current minimum. ⚙️ Mechanism: Increment index pointer of whichever subarray contributed the element. 📌 Takeaway: Linear time merger.",
							shape: "capsule", nodeType: "process", color: "#34D399",
							snippet: "while i <= mid && j <= r:\n  if arr[i] <= arr[j]: B[k++] = arr[i++]\n  else: B[k++] = arr[j++]\ncopy back B to arr",
						},
					},
				},
				{
					id: "b4", label: "Phase 4: Output Invariant", subtitle: "Monotonic Ascending Order",
					details: "💡 Intuition: Merged buffer is copied back into the primary array slice. ⚙️ Mechanism: Transcribes auxiliary buffer B[0..k-1] to arr[l..r]. 📌 Takeaway: Section is now completely sorted in-place relative to parent.",
					shape: "box", nodeType: "concept", color: "#F59E0B",
					metrics: map[string]string{"Guaranteed Time": "Theta(N log N)", "Used in": "Java Object Sorting (Timsort), GNU sort"},
					trap: "❌ False Belief: In-place MergeSort with O(1) space is practical. ✅ Truth: Block merge algorithms with O(1) space have massive constant factor overhead and are rarely used in practice.",
					viva: "🎯 Viva Question: What is Timsort? 💡 Model Answer: A hybrid algorithm combining Merge Sort and Insertion Sort that achieves O(N) on partially sorted data, used in Python and Java.",
					leaves: []fallbackLeaf{
						{
							id: "l4_1", label: "External Sorting on Disk", subtitle: "Sequential Access Advantage",
							details: "💡 Intuition: Sorting petabytes of data that cannot fit into RAM. ⚙️ Mechanism: Merges sorted chunks sequentially via streaming disk reads. 📌 Takeaway: Standard for big data.",
							shape: "capsule", nodeType: "concept", color: "#FBBF24",
							metrics: map[string]string{"I/O Efficiency": "Optimal Sequential Streaming", "K-Way Merge": "Heap of size K"},
						},
					},
				},
			}
		} else {
			resp.DomainType = "biology"
			resp.Title = "Glycolysis & Cellular Respiration Master Flow"
			summary = "10-step enzymatic breakdown of glucose into pyruvate in the cytoplasm yielding net 2 ATP and 2 NADH."
			branches = []fallbackBranch{
				{
					id: "b1", label: "Energy Investment Phase", subtitle: "Steps 1–5 • Cytoplasm",
					details: "Consumes 2 ATP molecules to phosphorylate glucose into Fructose-1,6-bisphosphate",
					shape: "box", nodeType: "process", color: "#6366F1",
					formulaLatex: `\text{Glucose} + 2\text{ATP} \xrightarrow{\text{Hexokinase, PFK-1}} \text{Fructose-1,6-BP} + 2\text{ADP}`,
					metrics: map[string]string{"ATP Cost": "2 ATP", "Committed Step": "PFK-1 Enzyme"},
					trap: "❌ False: Hexokinase is the committed step. ✅ Truth: PFK-1 (Phosphofructokinase-1) is the true committed rate-limiting step!",
					viva: "PFK-1 is allosterically inhibited by high ATP and citrate; activated by AMP and Fructose-2,6-bisphosphate.",
					leaves: []fallbackLeaf{
						{
							id: "l1_1", label: "Hexokinase / Glucokinase", subtitle: "Step 1 • Irreversible",
							details: "Traps neutral glucose inside cell by phosphorylation to Glucose-6-phosphate",
							shape: "capsule", nodeType: "process", color: "#818CF8",
							formulaLatex: `\text{Glucose} + \text{ATP} \rightarrow \text{G6P} + \text{ADP}`,
							trap: "❌ False: Glucokinase has lower Km than Hexokinase. ✅ Truth: Glucokinase has much higher Km (lower affinity), operating only at high blood glucose in liver!",
							viva: "Glucokinase is not inhibited by its product G6P, whereas Hexokinase is product-inhibited.",
						},
						{
							id: "l1_2", label: "PFK-1 Pacemaker Step", subtitle: "Step 3 • Rate-Limiting",
							details: "Converts F6P to F1,6BP; the master energetic valve of mammalian glycolysis",
							shape: "diamond", nodeType: "trap", color: "#4F46E5",
							formulaLatex: `\text{F6P} + \text{ATP} \rightarrow \text{F1,6BP} + \text{ADP}`,
							trap: "❌ False: AMP inhibits PFK-1. ✅ Truth: AMP signals low cellular energy and powerfully stimulates PFK-1!",
							viva: "Fructose-2,6-bisphosphate is the most potent physiological allosteric activator of PFK-1.",
							crossSubject: "Biology ↔ Chemistry: Le Chatelier principle and allosteric feedback inhibition",
						},
					},
				},
				{
					id: "b2", label: "Cleavage Phase", subtitle: "Steps 4–5 • Aldolase & TPI",
					details: "Cleaves 6-carbon F1,6BP into two interchangeable 3-carbon triose phosphates (DHAP & G3P)",
					shape: "box", nodeType: "process", color: "#0EA5E9",
					formulaLatex: `\text{Fructose-1,6-BP} \leftrightarrow \text{DHAP} + \text{G3P}`,
					metrics: map[string]string{"Enzyme": "Triose Phosphate Isomerase", "Efficiency": "Diffusion-Limited 'Perfect' Enzyme"},
					trap: "❌ False: Both DHAP and G3P directly enter payoff phase. ✅ Truth: Only G3P continues; DHAP must first isomerize into G3P via TPI!",
					viva: "Triose phosphate isomerase (TPI) operates at the catalytic diffusion limit (~10^8 M^-1 s^-1).",
					leaves: []fallbackLeaf{
						{
							id: "l2_1", label: "Triose Phosphate Isomerase (TPI)", subtitle: "Equilibrium Step",
							details: "Rapidly converts DHAP to G3P, funneling all carbon flux down payoff pathway",
							shape: "capsule", nodeType: "process", color: "#38BDF8",
							trap: "TPI deficiency leads to hemolytic anemia and severe progressive neuromuscular dysfunction.",
							viva: "TPI suppresses the formation of toxic methylglyoxal by-product.",
						},
					},
				},
				{
					id: "b3", label: "Energy Payoff Phase", subtitle: "Steps 6–10 • Cytoplasm",
					details: "Oxidative phosphorylation of G3P produces 4 ATP and 2 NADH (Net = 2 ATP + 2 NADH)",
					shape: "box", nodeType: "process", color: "#10B981",
					formulaLatex: `2\text{G3P} + 4\text{ADP} + 2\text{P}_i + 2\text{NAD}^+ \rightarrow 2\text{Pyruvate} + 4\text{ATP} + 2\text{NADH}`,
					metrics: map[string]string{"Gross ATP": "4 ATP", "Net ATP": "2 ATP", "NADH": "2 NADH"},
					trap: "❌ False: Glycolysis requires oxygen. ✅ Truth: Glycolysis is strictly anaerobic; NAD+ recycling in absence of O2 occurs via lactate fermentation!",
					viva: "Substrate-level phosphorylation occurs at Phosphoglycerate Kinase (step 7) and Pyruvate Kinase (step 10).",
					leaves: []fallbackLeaf{
						{
							id: "l3_1", label: "GAPDH Dehydrogenase", subtitle: "Step 6 • High-Energy Acyl Phosphate",
							details: "Oxidizes G3P to 1,3-bisphosphoglycerate, reducing NAD+ to NADH",
							shape: "capsule", nodeType: "equation", color: "#34D399",
							formulaLatex: `\text{G3P} + \text{NAD}^+ + \text{P}_i \rightarrow \text{1,3-BPG} + \text{NADH} + \text{H}^+`,
							trap: "❌ False: Arsenate blocks ATP hydrolysis. ✅ Truth: Arsenate competes with inorganic phosphate at GAPDH, causing uncoupling with zero net ATP yield!",
							viva: "Iodoacetate inhibits GAPDH by alkylating the active site catalytic cysteine thiol (-SH).",
						},
						{
							id: "l3_2", label: "Pyruvate Kinase (PK)", subtitle: "Step 10 • Final Substrate Phosphorylation",
							details: "Transfers high-energy phosphate from phosphoenolpyruvate (PEP) to ADP, generating Pyruvate",
							shape: "capsule", nodeType: "process", color: "#059669",
							formulaLatex: `\text{PEP} + \text{ADP} \rightarrow \text{Pyruvate} + \text{ATP}`,
							trap: "❌ False: Pyruvate kinase reaction is easily reversible. ✅ Truth: The PK step is strongly exergonic and irreversible; gluconeogenesis requires two enzymes (PC + PEPCK) to bypass it!",
							viva: "Feed-forward activation: F1,6BP strongly stimulates downstream Pyruvate Kinase.",
							crossSubject: "Biology ↔ Chemistry: High group transfer potential of enol phosphate bond",
						},
					},
				},
			}
		}
	} else if strings.Contains(lower, "krebs") || strings.Contains(lower, "citric") || strings.Contains(lower, "carnot") || strings.Contains(lower, "cycle") {
		// 2. CYCLE ARCHETYPE: Krebs Cycle / Carnot Engine
		archetype = "cycle"
		resp.DomainType = "chemistry"
		resp.Title = "Krebs Citric Acid Cycle & Thermodynamic Engines"
		summary = "Closed-loop cyclic biochemical pathway oxidizing Acetyl-CoA into CO2, regenerating Oxaloacetate."
		branches = []fallbackBranch{
			{
				id: "b1", label: "Entry & Condensation", subtitle: "Step 1 • Mitochondrial Matrix",
				details: "Citrate Synthase condenses 2-carbon Acetyl-CoA with 4-carbon Oxaloacetate to form 6-carbon Citrate",
				shape: "box", nodeType: "process", color: "#6366F1",
				formulaLatex: `\text{Acetyl-CoA} + \text{OAA} + \text{H}_2\text{O} \rightarrow \text{Citrate} + \text{CoA-SH}`,
				metrics: map[string]string{"Carbon Input": "2 C (Acetyl)", "Acceptor": "4 C (OAA)", "Product": "6 C (Citrate)"},
				trap: "❌ False: Citrate Synthase requires ATP. ✅ Truth: Highly exergonic thioester bond cleavage of Acetyl-CoA drives the reaction without ATP!",
				viva: "Fluoroacetate acts as a suicide substrate; converted to fluorocitrate which inhibits aconitase.",
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "Citrate Synthase", subtitle: "Committed Entry Step",
						details: "Sequential induced fit mechanism: OAA binds first, creating binding site for Acetyl-CoA",
						shape: "capsule", nodeType: "process", color: "#818CF8",
						trap: "Prevents premature futile hydrolysis of Acetyl-CoA thioester bond before OAA is bound.",
						viva: "Allosterically inhibited by ATP, NADH, and succinyl-CoA.",
					},
				},
			},
			{
				id: "b2", label: "Oxidative Decarboxylations", subtitle: "Steps 3 & 4 • CO2 & NADH Release",
				details: "Two successive decarboxylations release 2 CO2 and produce 2 NADH, yielding 4-carbon Succinyl-CoA",
				shape: "box", nodeType: "process", color: "#0EA5E9",
				formulaLatex: `\text{Isocitrate} \rightarrow \alpha\text{-Ketoglutarate} + \text{CO}_2 + \text{NADH}`,
				metrics: map[string]string{"NADH Yield": "2 NADH", "CO2 Released": "2 CO2"},
				trap: "❌ False: The 2 CO2 released come from the incoming Acetyl-CoA. ✅ Truth: Both CO2 molecules released come from the initial Oxaloacetate backbone, not the newly arrived Acetyl group!",
				viva: "alpha-Ketoglutarate Dehydrogenase complex is structurally homologous to Pyruvate Dehydrogenase (PDH), requiring identical 5 cofactors: TPP, Lipoamide, FAD, NAD+, CoA.",
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "Isocitrate Dehydrogenase", subtitle: "Master Cycle Rate-Limiter",
						details: "Catalyzes oxidative decarboxylation of isocitrate to alpha-ketoglutarate",
						shape: "capsule", nodeType: "equation", color: "#38BDF8",
						trap: "❌ False: High NADH accelerates Krebs cycle. ✅ Truth: NADH is a potent allosteric inhibitor of IDH and alpha-KGDH!",
						viva: "IDH is stimulated by ADP and NAD+; inhibited by ATP and NADH.",
					},
					{
						id: "l2_2", label: "alpha-KGDH Complex", subtitle: "Second Decarboxylation",
						details: "Multienzyme complex generating Succinyl-CoA with high-energy thioester bond",
						shape: "diamond", nodeType: "trap", color: "#0284C7",
						trap: "Arsenic poisoning inhibits lipoamide in alpha-KGDH and PDH, halting aerobic respiration completely.",
						viva: "Succinyl-CoA is also the precursor for heme synthesis via ALA synthase.",
					},
				},
			},
			{
				id: "b3", label: "Substrate Phosphorylation & Regeneration", subtitle: "Steps 5–8 • Matrix & Inner Membrane",
				details: "Generates 1 GTP/ATP, 1 FADH2, 1 NADH, and regenerates Oxaloacetate to close the cycle loop",
				shape: "box", nodeType: "cycle", color: "#10B981",
				formulaLatex: `\text{Succinate} + \text{FAD} \xrightarrow{\text{Succinate Dehydrogenase}} \text{Fumarate} + \text{FADH}_2`,
				metrics: map[string]string{"Per Turn": "1 GTP, 1 FADH2, 3 NADH, 2 CO2", "Total ATP Equiv": "10 ATP per Acetyl-CoA"},
				trap: "❌ False: All Krebs cycle enzymes are soluble in matrix. ✅ Truth: Succinate Dehydrogenase is physically embedded in inner mitochondrial membrane (Complex II of ETC)!",
				viva: "Malonate is a classic competitive inhibitor of Succinate Dehydrogenase due to its structural similarity to succinate.",
				crossSubject: "Chemistry ↔ Physics: Thermodynamic reversibility and Gibbs free energy delta G < 0",
				leaves: []fallbackLeaf{
					{
						id: "l3_1", label: "Succinyl-CoA Synthetase", subtitle: "Step 5 • Direct High-Energy Nucleotide",
						details: "Couples cleavage of thioester bond to phosphorylation of GDP to GTP (or ADP to ATP)",
						shape: "capsule", nodeType: "process", color: "#34D399",
						viva: "Nucleoside diphosphate kinase readily equilibrates GTP + ADP <-> GDP + ATP without energy loss.",
					},
					{
						id: "l3_2", label: "Malate Dehydrogenase", subtitle: "Step 8 • Cycle Closure",
						details: "Oxidizes L-Malate to Oxaloacetate (OAA), reducing final NAD+ to NADH",
						shape: "capsule", nodeType: "equation", color: "#059669",
						formulaLatex: `\text{L-Malate} + \text{NAD}^+ \rightleftharpoons \text{Oxaloacetate} + \text{NADH} + \text{H}^+`,
						trap: "❌ False: Step 8 has negative delta G. ✅ Truth: delta G is highly positive (+29.7 kJ/mol); it is pulled forward exclusively by the rapid continuous removal of OAA by Citrate Synthase!",
						viva: "Classic textbook example of reaction coupling driving an endergonic step in metabolic pathways.",
					},
				},
			},
		}
	} else if strings.Contains(lower, "tcp") || strings.Contains(lower, "udp") || strings.Contains(lower, "comparison") || strings.Contains(lower, " vs ") {
		// 3. COMPARISON ARCHETYPE: TCP vs UDP / Versus Matrix
		archetype = "comparison"
		resp.DomainType = "algorithm"
		resp.Title = "TCP vs UDP: Transport Layer Protocol Matrix"
		summary = "Direct architectural comparison between Connection-Oriented Reliable Byte Stream (TCP) and Connectionless Datagram Service (UDP)."
		branches = []fallbackBranch{
			{
				id: "b1", label: "Connection Paradigm", subtitle: "Stateful Handshake vs Stateless",
				details: "TCP establishes a dedicated full-duplex virtual connection via 3-way handshake; UDP is fire-and-forget",
				shape: "box", nodeType: "concept", color: "#6366F1",
				metrics: map[string]string{"TCP": "Connection-Oriented (3-way Handshake)", "UDP": "Connectionless (0 RTT)"},
				trap: "❌ False: UDP does not have ports. ✅ Truth: UDP uses 16-bit source and destination port numbers just like TCP to route to application sockets!",
				viva: "TCP SYN floods exhaust embryonic connection backlog queues; mitigated via SYN Cookies.",
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "TCP 3-Way Handshake", subtitle: "SYN ➔ SYN-ACK ➔ ACK",
						details: "Synchronizes initial sequence numbers (ISN) and establishes window sizes before any payload bytes send",
						shape: "capsule", nodeType: "process", color: "#818CF8",
						metrics: map[string]string{"Latency Penalty": "1 RTT upfront", "Security": "Randomized ISN"},
						trap: "❌ False: Payload can never be sent in handshake. ✅ Truth: TCP Fast Open (TFO) allows data in SYN packets if cryptographic cookie is validated!",
						viva: "Why 3-way and not 2-way? 2-way cannot handle delayed duplicate packets arriving after session terminates.",
					},
					{
						id: "l1_2", label: "UDP Zero-State Dispatch", subtitle: "Immediate Transmission",
						details: "Pushes raw datagram directly to IP layer without negotiation; zero connection setup latency",
						shape: "capsule", nodeType: "concept", color: "#4F46E5",
						metrics: map[string]string{"Setup Delay": "0 RTT", "Server Memory": "Zero per-client state"},
						trap: "❌ False: UDP cannot be used for secure comms. ✅ Truth: DTLS (Datagram TLS) and QUIC run on top of UDP with full encryption!",
						viva: "DNS, DHCP, NTP, and real-time multiplayer gaming use UDP to avoid head-of-line blocking.",
					},
				},
			},
			{
				id: "b2", label: "Reliability & Flow Control", subtitle: "Guaranteed Delivery vs Best-Effort",
				details: "TCP implements ARQ, sliding window, and congestion control (Tahoe/Reno/CUBIC/BBR); UDP has zero flow control",
				shape: "box", nodeType: "comparison", color: "#0EA5E9",
				metrics: map[string]string{"TCP Reliability": "ACKs, Retransmission, In-Order", "UDP Reliability": "Best Effort (Drop/Duplicate)"},
				trap: "❌ False: UDP has no error checking at all. ✅ Truth: UDP includes an optional 16-bit one's complement checksum over header + pseudo-header + data!",
				viva: "Head-of-Line (HoL) blocking: In TCP, if packet #2 drops, packets #3 and #4 must wait in receiver buffer even if already arrived.",
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "TCP Sliding Window & Congestion", subtitle: "Flow & Network Congestion Guard",
						details: "Receiver Window (rwnd) prevents buffer overflow; Congestion Window (cwnd) prevents network collapse",
						shape: "capsule", nodeType: "equation", color: "#38BDF8",
						formulaLatex: `W = \min(\text{rwnd}, \text{cwnd})`,
						trap: "❌ False: Fast Retransmit triggers on 1 duplicate ACK. ✅ Truth: Requires exactly 3 duplicate ACKs (4 total identical ACKs) to prevent spurious retransmits from reordering!",
						viva: "Explain BBR vs CUBIC: CUBIC is loss-based; BBR is model-based pacing measuring bottleneck bandwidth and RTT.",
					},
					{
						id: "l2_2", label: "UDP Header Overhead", subtitle: "Ultra-Lightweight 8 Bytes",
						details: "Only 4 fields: Source Port, Destination Port, Length, Checksum (8 bytes total vs TCP's 20–60 bytes)",
						shape: "capsule", nodeType: "concept", color: "#0284C7",
						metrics: map[string]string{"Header Size": "8 Bytes", "Bandwidth Overhead": "Minimal (60% smaller)"},
						viva: "In IPv6, the UDP checksum is mandatory, whereas in IPv4 it was optional (all zeros meant disabled).",
					},
				},
			},
		}
	} else if strings.Contains(lower, "hierarchy") || strings.Contains(lower, "stack") || strings.Contains(lower, "osi") {
		// 4. STACK ARCHETYPE: Memory Hierarchy / OSI Layers
		archetype = "stack"
		resp.DomainType = "algorithm"
		resp.Title = "Memory Hierarchy & Hardware Cache Stack"
		summary = "Vertical architectural pyramid trading off speed, cost per bit, and storage capacity across execution registers down to disk."
		branches = []fallbackBranch{
			{
				id: "b1", label: "L1 / L2 / L3 CPU Caches", subtitle: "Level 1–3 • On-Die SRAM",
				details: "Static RAM associative caches directly integrated into processor die minimizing CPU stall cycles",
				shape: "box", nodeType: "concept", color: "#6366F1",
				formulaLatex: `T_{\text{avg}} = T_{L1} + (1 - h_1)(T_{L2} + (1 - h_2)T_{\text{mem}})`,
				metrics: map[string]string{"L1 Latency": "0.5–1 ns (4 cycles)", "L2 Latency": "3–5 ns", "L3 Latency": "10–20 ns"},
				trap: "❌ False: Doubling cache size halves miss rate. ✅ Truth: Power-law of cache misses (Square root rule): miss rate drops only by ~sqrt(2) (roughly 30%), not 50%!",
				viva: "Explain False Sharing: Two threads updating independent variables residing on the same 64-byte cache line force continuous MESI cache invalidations.",
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "Spatial & Temporal Locality", subtitle: "Governing Law of Caching",
						details: "Temporal: recently accessed address reused soon; Spatial: adjacent addresses loaded together in 64-byte lines",
						shape: "capsule", nodeType: "process", color: "#818CF8",
						trap: "Traversing a 2D array column-major in C/C++ causes a cache miss on almost every element compared to row-major iteration!",
						viva: "Modern CPUs have hardware prefetchers that detect stride patterns and speculatively pre-fill L1/L2.",
					},
				},
			},
			{
				id: "b2", label: "DRAM Main Memory & TLB", subtitle: "Off-Die Dynamic RAM & MMU",
				details: "Capacitor-based dynamic RAM requiring periodic refresh cycles (~64 ms); managed via virtual memory paging",
				shape: "box", nodeType: "concept", color: "#0EA5E9",
				metrics: map[string]string{"DRAM Latency": "50–100 ns (~200 cycles)", "Page Size": "4 KB (Standard) / 2 MB (Huge)"},
				trap: "❌ False: Virtual memory access requires 1 memory lookup. ✅ Truth: A 4-level page table walk in x86-64 requires 4 successive DRAM reads on a TLB miss!",
				viva: "Translation Lookaside Buffer (TLB) caches virtual-to-physical address mappings with ~1 ns lookup.",
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "Page Fault & Swapping", subtitle: "OS Kernel Trap Handler",
						details: "Accessing an unmapped virtual address triggers page fault exception #14, loading frame from NVMe SSD",
						shape: "diamond", nodeType: "trap", color: "#38BDF8",
						metrics: map[string]string{"Fault Penalty": "5,000–10,000 ns (Millions of cycles)", "Thrashing": "System locked swapping"},
						trap: "Thrashing occurs when active working set size exceeds physical RAM capacity.",
						viva: "Inverted page tables vs multi-level page tables: inverted scales with physical RAM size rather than virtual address space.",
					},
				},
			},
		}
	} else if strings.Contains(lower, "newton") || strings.Contains(lower, "motion") || strings.Contains(lower, "force") || strings.Contains(lower, "quantum") {
		// 5. HUB ARCHETYPE: Newton's Laws / Core Foundations
		archetype = "hub"
		resp.DomainType = "science"
		resp.Title = "Newton's Laws of Motion: Mechanics Core Hub"
		summary = "360-degree foundational laws governing classical mechanics, conservation of momentum, and dynamical systems."
		branches = []fallbackBranch{
			{
				id: "b1", label: "First Law: Law of Inertia", subtitle: "Galilean Relativity & Inertial Frames",
				details: "An object continues in uniform rectilinear motion unless acted upon by a non-zero net external resultant force",
				shape: "box", nodeType: "concept", color: "#6366F1",
				formulaLatex: `\sum \mathbf{F} = 0 \iff \frac{d\mathbf{v}}{dt} = 0`,
				trap: "❌ False: First law is merely a special case of F=ma when a=0. ✅ Truth: First law fundamentally defines the existence of inertial reference frames where F=ma is valid!",
				viva: "In non-inertial (accelerating/rotating) reference frames, fictitious pseudo-forces (Coriolis, Centrifugal) must be added.",
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "Inertial vs Non-Inertial Frames", subtitle: "Frame of Reference",
						details: "Accelerating observer observes apparent accelerations requiring d'Alembert pseudo-forces",
						shape: "capsule", nodeType: "concept", color: "#818CF8",
						formulaLatex: `\mathbf{F}_{\text{pseudo}} = -m\mathbf{a}_{\text{frame}}`,
						trap: "Centrifugal force is NOT a real Newton Third Law reaction force; it is an artifact of rotating coordinates.",
						viva: "Foucault pendulum empirically demonstrates the rotation of Earth's non-inertial frame.",
					},
				},
			},
			{
				id: "b2", label: "Second Law: Fundamental Dynamics", subtitle: "Rate of Change of Momentum",
				details: "Net external force equals the time rate of change of linear momentum in an inertial frame",
				shape: "box", nodeType: "equation", color: "#0EA5E9",
				formulaLatex: `\mathbf{F}_{\text{net}} = \frac{d\mathbf{p}}{dt} = m\mathbf{a} + \mathbf{v}\frac{dm}{dt}`,
				metrics: map[string]string{"Constant Mass": "F = ma", "Variable Mass": "Rocket Thrust Eq (Tsiolkovsky)"},
				trap: "❌ False: F = ma is universally valid for rocket propulsion. ✅ Truth: For variable mass systems (rockets), one must use Tsiolkovsky's equation; simple F=ma yields incorrect acceleration!",
				viva: "State the impulse-momentum theorem: J = integral F dt = delta p.",
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "Tsiolkovsky Rocket Equation", subtitle: "Variable Mass Systems",
						details: "Governs propulsion via momentum conservation of expelled exhaust propellant mass",
						shape: "capsule", nodeType: "equation", color: "#38BDF8",
						formulaLatex: `\Delta v = v_e \ln\left(\frac{m_0}{m_f}\right)`,
						trap: "Logarithmic scaling means exponential fuel is required for linear increments in delta-v.",
						viva: "Specific impulse (I_sp) measures engine efficiency: I_sp = v_e / g_0.",
						crossSubject: "Physics ↔ Aerospace: Orbital mechanics and escape velocity",
					},
				},
			},
			{
				id: "b3", label: "Third Law: Action-Reaction", subtitle: "Conservation of Linear Momentum",
				details: "Forces always occur in matched pairs: equal in magnitude, opposite in direction, acting on different bodies",
				shape: "box", nodeType: "concept", color: "#10B981",
				formulaLatex: `\mathbf{F}_{A \to B} = -\mathbf{F}_{B \to A}`,
				trap: "❌ False: Action-reaction forces cancel out to zero acceleration. ✅ Truth: Action-reaction pairs act on two DIFFERENT objects, so they CANNOT cancel each other out!",
				viva: "Normal force on a book resting on a table is NOT the reaction to gravity! The reaction to Earth's gravity on the book is the book's gravity pulling Earth up.",
				leaves: []fallbackLeaf{
					{
						id: "l3_1", label: "Isolated System Momentum Conservation", subtitle: "Universal Conservation Law",
						details: "If external force sum is zero, total linear momentum of the multi-body system is invariant in time",
						shape: "capsule", nodeType: "process", color: "#34D399",
						formulaLatex: `\sum \mathbf{p}_{\text{initial}} = \sum \mathbf{p}_{\text{final}}`,
						trap: "Kinetic energy is conserved only in perfectly elastic collisions; momentum is conserved in ALL collisions without external forces.",
						viva: "Coefficient of restitution e = 1 (elastic), 0 < e < 1 (inelastic), e = 0 (perfectly plastic).",
					},
				},
			},
		}
	} else {
		// 6. TREE ARCHETYPE: Operating System Schedulers / Sorting Taxonomy / General
		archetype = "tree"
		resp.DomainType = "algorithm"
		resp.Title = fmt.Sprintf("%s Master Classification Tree", concept)
		summary = fmt.Sprintf("Taxonomy, complexity invariants, and edge-case classification for %s.", concept)
		branches = []fallbackBranch{
			{
				id: "b1", label: "Foundations & Axioms", subtitle: "Core Definition",
				details: fmt.Sprintf("Fundamental mathematical and logical invariants governing %s", concept),
				shape: "box", nodeType: "concept", color: "#6366F1",
				formulaLatex: `T(n) = 2T(n/2) + \Theta(n)`,
				metrics: map[string]string{"Best Case": "O(N log N)", "Worst Case": "O(N^2)"},
				trap: "❌ False: Assumed universal efficiency under all inputs. ✅ Truth: Edge-case inputs (sorted, reverse-sorted, duplicates) trigger asymptotic degradation!",
				viva: fmt.Sprintf("What are the strict boundary conditions where %s fails in production?", concept),
				leaves: []fallbackLeaf{
					{
						id: "l1_1", label: "Governing Law / Recurrence", subtitle: "Mathematical Relation",
						details: "Rigorous recurrence relation and asymptotic bound derivation",
						shape: "capsule", nodeType: "equation", color: "#818CF8",
						formulaLatex: `T(n) = aT(n/b) + f(n)`,
						trap: "Master Theorem fails when f(n) is not polynomially larger/smaller than n^(log_b a).",
						viva: "Use Akra-Bazzi method when subproblems have non-uniform partition sizes.",
					},
				},
			},
			{
				id: "b2", label: "Mechanics & Invariants", subtitle: "Step-by-Step Dynamics",
				details: "Operational flow, state transitions, and two-pointer or loop invariant preservation",
				shape: "box", nodeType: "process", color: "#0EA5E9",
				metrics: map[string]string{"Space Overhead": "O(1) Auxiliary", "Stability": "Unstable"},
				trap: "❌ False: In-place implies stable order. ✅ Truth: Swapping non-adjacent elements fundamentally destroys relative order of equal keys!",
				viva: "How to make an unstable algorithm stable? Augment keys with their original indices.",
				leaves: []fallbackLeaf{
					{
						id: "l2_1", label: "Partition / Pivot Invariant", subtitle: "Core Execution Loop",
						details: "Maintains invariant that all elements left of boundary satisfy the predicate",
						shape: "capsule", nodeType: "process", color: "#38BDF8",
						snippet: "while i < j:\n  while arr[i] < pivot: i++\n  while arr[j] > pivot: j--\n  swap(arr[i], arr[j])",
						trap: "Off-by-one errors in while conditions cause infinite loops on duplicate keys.",
						viva: "3-way Dutch National Flag partition completely eliminates degradation from duplicate elements.",
					},
				},
			},
			{
				id: "b3", label: "Competitive Exam Traps", subtitle: "High-Yield Scoring Pitfalls",
				details: "Edge cases, corner constraints, and common numerical gotchas tested in competitive exams",
				shape: "box", nodeType: "trap", color: "#F59E0B",
				trap: "❌ False: Recursion depth is always bounded by log(N). ✅ Truth: Unbalanced recursion degrades to O(N) stack frames, causing StackOverflow!",
				viva: "Tail-call recursion optimization (TCO) eliminates auxiliary stack frame accumulation on the larger subproblem.",
				leaves: []fallbackLeaf{
					{
						id: "l3_1", label: "Worst-Case Immune Hybrids", subtitle: "IntroSort / Dual-Pivot",
						details: "Monitors recursion depth; switches to HeapSort if depth exceeds 2*log(N) to guarantee O(N log N) worst-case",
						shape: "diamond", nodeType: "trap", color: "#FBBF24",
						metrics: map[string]string{"Worst Bound": "O(N log N) Guaranteed", "Used In": "C++ std::sort, Rust"},
						trap: "Standard QuickSort without depth check is vulnerable to algorithmic complexity denial-of-service (DoS) attacks.",
						viva: "Why not use HeapSort directly? HeapSort suffers from poor spatial cache locality compared to QuickSort.",
					},
				},
			},
		}
	}

	resp.Archetype = archetype
	resp.Summary = summary

	// Assemble root node
	rootNode := dto.GraphNode{
		ID:           rootID,
		Label:        resp.Title,
		Subtitle:     fmt.Sprintf("4D Interactive Knowledge Core • %s", strings.ToUpper(archetype)),
		Description:  summary,
		NodeType:     "root",
		Type:         "concept",
		Shape:        "circle",
		ColorHex:     "#4F46E5",
		Importance:   5,
		Badge:        "Master Map",
		ChildrenIDs:  make([]string, 0, len(branches)),
	}

	rootTreeNode := &dto.MindMapNode{
		ID:          rootID,
		Label:       resp.Title,
		Subtitle:    fmt.Sprintf("Archetype: %s", archetype),
		Details:     summary,
		Shape:       "circle",
		Type:        "concept",
		ColorHex:    "#4F46E5",
		Importance:  5,
		Badge:       "Master Map",
	}

	for _, b := range branches {
		rootNode.ChildrenIDs = append(rootNode.ChildrenIDs, b.id)

		bShape := b.shape
		if bShape == "" {
			bShape = "box"
		}
		bType := b.nodeType
		if bType == "" {
			bType = "process"
		}

		bChildrenIDs := make([]string, 0, len(b.leaves))
		for _, l := range b.leaves {
			bChildrenIDs = append(bChildrenIDs, l.id)
		}

		bNode := dto.GraphNode{
			ID:           b.id,
			Label:        b.label,
			Subtitle:     b.subtitle,
			Description:  b.details,
			NodeType:     "branch",
			Type:         bType,
			Shape:        bShape,
			ColorHex:     b.color,
			Formulas:     b.formulas,
			FormulaLatex: b.formulaLatex,
			Metrics:      b.metrics,
			ExamTrap:     b.trap,
			VivaInsight:  b.viva,
			CrossSubject: b.crossSubject,
			Importance:   4,
			Badge:        "Major Pillar",
			ChildrenIDs:  bChildrenIDs,
		}
		resp.Nodes = append(resp.Nodes, bNode)
		resp.Edges = append(resp.Edges, dto.GraphEdge{
			From:   rootID,
			To:     b.id,
			FromID: rootID,
			ToID:   b.id,
			Label:  "branches into",
			Arrow:  true,
			Style:  "solid",
		})

		bTreeNode := &dto.MindMapNode{
			ID:           b.id,
			Label:        b.label,
			Subtitle:     b.subtitle,
			Details:      b.details,
			Shape:        bShape,
			Type:         bType,
			ColorHex:     b.color,
			Metrics:      b.metrics,
			ExamTrap:     b.trap,
			VivaInsight:  b.viva,
			CrossSubject: b.crossSubject,
			FormulaLatex: b.formulaLatex,
			ChildrenIDs:  bChildrenIDs,
		}

		for _, l := range b.leaves {
			lShape := l.shape
			if lShape == "" {
				lShape = "capsule"
			}
			lType := l.nodeType
			if lType == "" {
				if l.formulaLatex != "" {
					lType = "equation"
				} else if l.trap != "" {
					lType = "trap"
				} else {
					lType = "concept"
				}
			}

			lNode := dto.GraphNode{
				ID:           l.id,
				Label:        l.label,
				Subtitle:     l.subtitle,
				Description:  l.details,
				NodeType:     "leaf",
				Type:         lType,
				Shape:        lShape,
				ColorHex:     l.color,
				Formulas:     l.formulas,
				FormulaLatex: l.formulaLatex,
				Metrics:      l.metrics,
				CodeSnippet:  l.snippet,
				ExamTrap:     l.trap,
				VivaInsight:  l.viva,
				CrossSubject: l.crossSubject,
				CrossLinks:   l.crossLinks,
				Importance:   3,
				Badge:        "Interactive Station",
			}
			resp.Nodes = append(resp.Nodes, lNode)
			resp.Edges = append(resp.Edges, dto.GraphEdge{
				From:   b.id,
				To:     l.id,
				FromID: b.id,
				ToID:   l.id,
				Label:  "elaborates",
				Arrow:  true,
				Style:  "solid",
			})

			for _, tgt := range l.crossLinks {
				resp.Edges = append(resp.Edges, dto.GraphEdge{
					From:        l.id,
					To:          tgt,
					FromID:      l.id,
					ToID:        tgt,
					Label:       "cross-link",
					Arrow:       true,
					IsCrossLink: true,
					Style:       "dashed",
				})
			}

			bTreeNode.Children = append(bTreeNode.Children, &dto.MindMapNode{
				ID:           l.id,
				Label:        l.label,
				Subtitle:     l.subtitle,
				Details:      l.details,
				Shape:        lShape,
				Type:         lType,
				ColorHex:     l.color,
				Metrics:      l.metrics,
				CodeSnippet:  l.snippet,
				ExamTrap:     l.trap,
				VivaInsight:  l.viva,
				CrossSubject: l.crossSubject,
				CrossLinks:   l.crossLinks,
				FormulaLatex: l.formulaLatex,
			})
		}

		rootTreeNode.Children = append(rootTreeNode.Children, bTreeNode)
	}

	// Insert root as first node
	resp.Nodes = append([]dto.GraphNode{rootNode}, resp.Nodes...)
	resp.Root = rootTreeNode
	return resp
}

func (s *diagramService) ExportMindMapPDF(ctx context.Context, req dto.ExportMindMapPDFRequest) ([]byte, error) {
	if s.pdfService == nil {
		return nil, errors.New("pdf export service is not initialized")
	}
	return s.pdfService.ExportMindMapPoster(ctx, req)
}

func (s *diagramService) GenerateQuizFromMindMap(ctx context.Context, userID string, req dto.MindMapToQuizRequest) (*model.Quiz, error) {
	numQ := req.NumQuestions
	if numQ <= 0 {
		numQ = 5
	}
	if numQ > 10 {
		numQ = 10
	}
	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = "medium"
	}
	title := req.Title
	if title == "" {
		title = fmt.Sprintf("⚡ 4D Active Recall: %s", req.Concept)
	}

	type aiQuizQuestion struct {
		QuestionText       string   `json:"question_text"`
		Options            []string `json:"options"`
		CorrectOptionIndex int      `json:"correct_option_index"`
		Explanation        string   `json:"explanation"`
		TrapWarning        string   `json:"trap_warning"`
		Difficulty         string   `json:"difficulty"`
		Topic              string   `json:"topic"`
	}

	type aiQuizResponse struct {
		Questions []aiQuizQuestion `json:"questions"`
	}

	systemPrompt := fmt.Sprintf(`You are an elite competitive examination test setter.
Given the rich 4D mind map nodes, KaTeX formulas, and examiner traps for concept: "%s",
generate an active recall quiz of exactly %d high-yield 4-choice questions.

CRITICAL RULES:
1. Test real conceptual understanding, formula calculations, and examiner traps.
2. For nodes with an "exam_trap", use the trap misconception as one of the tempting distractor options!
3. Provide crystal-clear step-by-step explanations and trap warnings.
4. Output JSON strictly matching:
{
  "questions": [
    {
      "question_text": "Detailed question text...",
      "options": ["Option A", "Option B", "Option C", "Option D"],
      "correct_option_index": 0,
      "explanation": "Why correct option holds...",
      "trap_warning": "Common mistake...",
      "difficulty": "%s",
      "topic": "%s"
    }
  ]
}`, req.Concept, numQ, difficulty, req.Concept)

	nodesSummary, _ := json.Marshal(req.Nodes)
	userPrompt := fmt.Sprintf("Concept: %s\nArchetype: %s\nNodes:\n%s", req.Concept, req.Archetype, string(nodesSummary))

	var aiResp aiQuizResponse
	var questions []*model.QuizQuestion

	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &aiResp)
	if err == nil && len(aiResp.Questions) > 0 {
		for i, q := range aiResp.Questions {
			if len(q.Options) >= 2 {
				idx := q.CorrectOptionIndex
				if idx < 0 || idx >= len(q.Options) {
					idx = 0
				}
				diff := q.Difficulty
				if diff == "" {
					diff = difficulty
				}
				top := q.Topic
				if top == "" {
					top = req.Concept
				}
				questions = append(questions, &model.QuizQuestion{
					ID:                 fmt.Sprintf("q_%d", i+1),
					QuestionText:       q.QuestionText,
					Options:            q.Options,
					CorrectOptionIndex: idx,
					Explanation:        q.Explanation,
					TrapWarning:        q.TrapWarning,
					Difficulty:         diff,
					Topic:              top,
					CreatedAt:          time.Now(),
				})
			}
		}
	}

	// Intelligent fallback if AI output was empty or offline
	if len(questions) == 0 {
		questions = buildFallbackQuestionsFromNodes(req.Concept, difficulty, req.Nodes, numQ)
	}

	quiz := &model.Quiz{
		UserID:         userID,
		Title:          title,
		Topic:          req.Concept,
		Difficulty:     difficulty,
		TotalQuestions: len(questions),
		CreatedAt:      time.Now(),
		Questions:      questions,
	}

	if userID != "" && s.quizRepo != nil {
		if err := s.quizRepo.CreateQuizWithQuestions(ctx, quiz, questions); err != nil {
			quiz.ID = fmt.Sprintf("quiz_%d", time.Now().UnixNano())
		}
	} else {
		quiz.ID = fmt.Sprintf("quiz_%d", time.Now().UnixNano())
	}

	return quiz, nil
}

func buildFallbackQuestionsFromNodes(concept, difficulty string, nodes []dto.GraphNode, count int) []*model.QuizQuestion {
	var list []*model.QuizQuestion
	qIdx := 1

	for _, node := range nodes {
		if qIdx > count {
			break
		}
		if node.ExamTrap != "" {
			parts := strings.Split(node.ExamTrap, "✅")
			trapText := strings.TrimSpace(strings.TrimPrefix(parts[0], "❌"))
			truthText := ""
			if len(parts) > 1 {
				truthText = strings.TrimSpace(parts[1])
			} else {
				truthText = node.Description
			}

			q := &model.QuizQuestion{
				ID:           fmt.Sprintf("q_%d", qIdx),
				QuestionText: fmt.Sprintf("Which of the following statements is scientifically TRUE regarding %s?", node.Label),
				Options: []string{
					truthText,
					trapText,
					fmt.Sprintf("%s operates independently of boundary conditions", node.Label),
					fmt.Sprintf("%s violates thermodynamic conservation laws", node.Label),
				},
				CorrectOptionIndex: 0,
				Explanation:        fmt.Sprintf("Correct! %s. Remember to avoid the common pitfall: %s", truthText, trapText),
				TrapWarning:        fmt.Sprintf("⚠️ Examiner Trap: %s", trapText),
				Difficulty:         difficulty,
				Topic:              concept,
				CreatedAt:          time.Now(),
			}
			list = append(list, q)
			qIdx++
		} else if node.FormulaLatex != "" {
			q := &model.QuizQuestion{
				ID:           fmt.Sprintf("q_%d", qIdx),
				QuestionText: fmt.Sprintf("What is the governing KaTeX formula/equation for %s?", node.Label),
				Options: []string{
					node.FormulaLatex,
					strings.ReplaceAll(node.FormulaLatex, "+", "-"),
					"E = m c^2 + \\hbar \\omega",
					"\\Delta S_{\\text{universe}} < 0",
				},
				CorrectOptionIndex: 0,
				Explanation:        fmt.Sprintf("The fundamental governing relation for %s is $%s$.", node.Label, node.FormulaLatex),
				TrapWarning:        "Watch out for sign errors and boundary constants.",
				Difficulty:         difficulty,
				Topic:              concept,
				CreatedAt:          time.Now(),
			}
			list = append(list, q)
			qIdx++
		} else if node.Description != "" && node.NodeType != "root" {
			q := &model.QuizQuestion{
				ID:           fmt.Sprintf("q_%d", qIdx),
				QuestionText: fmt.Sprintf("What primary mechanism defines %s?", node.Label),
				Options: []string{
					node.Description,
					"An arbitrary heuristic with no theoretical backing",
					"A deprecated legacy rule abandoned in modern systems",
					"A hypothetical construct lacking empirical evidence",
				},
				CorrectOptionIndex: 0,
				Explanation:        node.Description,
				TrapWarning:        node.ExamTrap,
				Difficulty:         difficulty,
				Topic:              concept,
				CreatedAt:          time.Now(),
			}
			list = append(list, q)
			qIdx++
		}
	}

	if len(list) == 0 {
		list = append(list, &model.QuizQuestion{
			ID:                 "q_1",
			QuestionText:       fmt.Sprintf("What is the core conceptual foundation of %s?", concept),
			Options:            []string{"Fundamental governing law", "Unverified assumption", "Experimental anomaly", "None of the above"},
			CorrectOptionIndex: 0,
			Explanation:        fmt.Sprintf("%s forms the primary theoretical basis for this topic.", concept),
			TrapWarning:        "Ensure solid grasp of first principles.",
			Difficulty:         difficulty,
			Topic:              concept,
			CreatedAt:          time.Now(),
		})
	}

	return list
}

func (s *diagramService) GenerateFlashcardsFromMindMap(ctx context.Context, userID string, req dto.MindMapToFlashcardRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	cardCount := req.CardCount
	if cardCount <= 0 {
		cardCount = len(req.Nodes)
		if cardCount > 10 {
			cardCount = 10
		} else if cardCount < 3 {
			cardCount = 3
		}
	}
	difficulty := req.Difficulty
	if difficulty == "" {
		difficulty = "medium"
	}
	title := req.Title
	if title == "" {
		title = fmt.Sprintf("⚡ 4D Active Recall Deck: %s", req.Concept)
	}

	type aiCard struct {
		Front      string `json:"front"`
		Back       string `json:"back"`
		Difficulty string `json:"difficulty"`
		Topic      string `json:"topic"`
	}

	type aiDeckResp struct {
		Cards []aiCard `json:"cards"`
	}

	systemPrompt := fmt.Sprintf(`You are an expert Spaced Repetition Flashcard Designer (Anki SuperMemo SM-2 methodology).
Given the 4D Mind Map nodes for: "%s", generate %d high-yield active recall flashcards.
CRITICAL FORMATTING:
- Front: Crisp question or conceptual prompt.
- Back: Comprehensive answer with Markdown, LaTeX formulas, Examiner Trap alerts, and Viva insights.
- Output JSON strictly matching:
{
  "cards": [
    {
      "front": "Question...",
      "back": "Answer with formula and trap warning...",
      "difficulty": "%s",
      "topic": "%s"
    }
  ]
}`, req.Concept, cardCount, difficulty, req.Concept)

	nodesSummary, _ := json.Marshal(req.Nodes)
	userPrompt := fmt.Sprintf("Concept: %s\nArchetype: %s\nNodes:\n%s", req.Concept, req.Archetype, string(nodesSummary))

	var aiCards aiDeckResp
	var deckCards []dto.FlashcardDeckCard

	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &aiCards)
	if err == nil && len(aiCards.Cards) > 0 {
		for i, c := range aiCards.Cards {
			deckCards = append(deckCards, dto.FlashcardDeckCard{
				ID:          fmt.Sprintf("card_%d", i+1),
				Front:       c.Front,
				Back:        c.Back,
				Difficulty:  c.Difficulty,
				Topic:       c.Topic,
				IsMastered:  false,
				IsReviewed:  false,
				Repetitions: 0,
				Interval:    1,
				EaseFactor:  2.5,
			})
		}
	}

	// Fallback to building cards directly from nodes if AI failed or offline
	if len(deckCards) == 0 {
		cIdx := 1
		for _, node := range req.Nodes {
			if cIdx > cardCount {
				break
			}
			front := fmt.Sprintf("What is the core mechanism and governing principle of **%s**?", node.Label)
			var back strings.Builder
			back.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", node.Label, node.Description))
			if node.FormulaLatex != "" {
				back.WriteString(fmt.Sprintf("**Governing Formula:**\n$$%s$$\n\n", node.FormulaLatex))
			}
			if node.ExamTrap != "" {
				back.WriteString(fmt.Sprintf("**⚠️ Examiner Trap Alert:**\n%s\n\n", node.ExamTrap))
			}
			if node.VivaInsight != "" {
				back.WriteString(fmt.Sprintf("**💡 Viva / Interview Insight:**\n%s\n\n", node.VivaInsight))
			}
			if node.CrossSubject != "" {
				back.WriteString(fmt.Sprintf("**🌐 Cross-Subject Link:**\n%s\n\n", node.CrossSubject))
			}

			deckCards = append(deckCards, dto.FlashcardDeckCard{
				ID:          fmt.Sprintf("card_%d", cIdx),
				Front:       front,
				Back:        strings.TrimSpace(back.String()),
				Difficulty:  difficulty,
				Topic:       req.Concept,
				IsMastered:  false,
				IsReviewed:  false,
				Repetitions: 0,
				Interval:    1,
				EaseFactor:  2.5,
			})
			cIdx++
		}
	}

	resp := &dto.GenerateFlashcardDeckResponse{
		Title:         title,
		Topic:         req.Concept,
		Difficulty:    difficulty,
		TotalCards:    len(deckCards),
		MasteredCount: 0,
		Cards:         deckCards,
	}

	if req.SaveDeck && userID != "" && s.flashcardRepo != nil {
		cardsJson, _ := json.Marshal(deckCards)
		deckModel := &model.FlashcardDeck{
			UserID:        userID,
			Title:         title,
			Topic:         req.Concept,
			Difficulty:    difficulty,
			CardCount:     len(deckCards),
			Cards:         cardsJson,
			MasteredCount: 0,
			IsPinned:      false,
		}
		if err := s.flashcardRepo.Create(ctx, deckModel); err == nil {
			resp.DeckID = deckModel.ID
		}
	}

	return resp, nil
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

