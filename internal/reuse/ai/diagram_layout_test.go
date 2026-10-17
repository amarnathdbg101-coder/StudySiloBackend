package ai

import (
	"testing"
	"time"

	"goProjectStructure/internal/handler/dto"
)

func sampleTestDiagram(archetype string) dto.DiagramResponse {
	return dto.DiagramResponse{
		Concept:   "Newton's Laws of Motion",
		Archetype: archetype,
		Nodes: []dto.GraphNode{
			{ID: "root", Label: "Newton's Laws of Motion", NodeType: "root"},
			{ID: "b1", Label: "First Law (Inertia)", NodeType: "branch", BranchID: "root"},
			{ID: "b2", Label: "Second Law (F=ma)", NodeType: "branch", BranchID: "root"},
			{ID: "b3", Label: "Third Law (Action-Reaction)", NodeType: "branch", BranchID: "root"},
			{ID: "l1_1", Label: "Galileo Thought Experiment", NodeType: "leaf", BranchID: "b1"},
			{ID: "l1_2", Label: "Equilibrium & Reference Frames", NodeType: "leaf", BranchID: "b1"},
			{ID: "l2_1", Label: "Momentum Derivation", NodeType: "leaf", BranchID: "b2"},
			{ID: "l2_2", Label: "Impulse and Force Vector", NodeType: "leaf", BranchID: "b2"},
			{ID: "l3_1", Label: "Normal Reaction Force", NodeType: "leaf", BranchID: "b3"},
			{ID: "l3_2", Label: "Rocket Propulsion Dynamics", NodeType: "leaf", BranchID: "b3"},
		},
		Edges: []dto.GraphEdge{
			{FromID: "root", ToID: "b1"},
			{FromID: "root", ToID: "b2"},
			{FromID: "root", ToID: "b3"},
			{FromID: "b1", ToID: "l1_1"},
			{FromID: "b1", ToID: "l1_2"},
			{FromID: "b2", ToID: "l2_1"},
			{FromID: "b2", ToID: "l2_2"},
			{FromID: "b3", ToID: "l3_1"},
			{FromID: "b3", ToID: "l3_2"},
		},
	}
}

func TestComputeDiagramLayout_Archetypes(t *testing.T) {
	archetypes := []string{"radial", "flow", "cycle", "comparison", "stack", "tree", "vertical"}

	for _, arch := range archetypes {
		t.Run(arch, func(t *testing.T) {
			diag := sampleTestDiagram(arch)
			start := time.Now()
			ComputeDiagramLayout(&diag, 2800, 2000)
			elapsed := time.Since(start)

			t.Logf("Archetype %s computed layout in %v", arch, elapsed)
			if elapsed > 10*time.Millisecond {
				t.Errorf("expected layout to compute in <10ms, took %v", elapsed)
			}

			// Root should be placed
			root := diag.Nodes[0]
			if root.X == 0 && root.Y == 0 {
				t.Errorf("root node was not positioned")
			}
			if root.Width == 0 || root.Height == 0 {
				t.Errorf("root node has zero dimensions")
			}

			// Check all nodes have non-zero dimensions and reasonable positions
			for _, n := range diag.Nodes {
				if n.Width == 0 || n.Height == 0 {
					t.Errorf("node %s has 0 dimensions", n.ID)
				}
				if n.X < 0 || n.X > 2800 || n.Y < 0 || n.Y > 2000 {
					t.Errorf("node %s is outside canvas bounds: (%f, %f)", n.ID, n.X, n.Y)
				}
			}
		})
	}
}

func TestComputeDiagramLayout_CollisionAvoidance(t *testing.T) {
	// 5 nodes initialized at identical overlapping coordinates
	nodes := []dto.GraphNode{
		{ID: "n1", Label: "Concept Alpha", NodeType: "leaf", X: 500, Y: 500, Width: 180, Height: 60},
		{ID: "n2", Label: "Concept Beta", NodeType: "leaf", X: 505, Y: 505, Width: 180, Height: 60},
		{ID: "n3", Label: "Concept Gamma", NodeType: "leaf", X: 510, Y: 510, Width: 180, Height: 60},
	}

	resolveCollisions(nodes)

	// Ensure none of the nodes overlap
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			a := nodes[i]
			b := nodes[j]
			overlapX := a.X < (b.X+b.Width) && (a.X+a.Width) > b.X
			overlapY := a.Y < (b.Y+b.Height) && (a.Y+a.Height) > b.Y
			if overlapX && overlapY {
				t.Fatalf("nodes %s and %s still collide after relaxation: A=(%f, %f) B=(%f, %f)",
					a.ID, b.ID, a.X, a.Y, b.X, b.Y)
			}
		}
	}
}
