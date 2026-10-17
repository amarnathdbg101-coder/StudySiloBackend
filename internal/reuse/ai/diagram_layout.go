package ai

import (
	"math"
	"strings"

	"goProjectStructure/internal/handler/dto"
)

// ComputeDiagramLayout calculates high-performance, non-overlapping 2D coordinates
// and spatial dimensions for all nodes in an educational mind map.
// Executing this on the Go backend offloads all O(N^2) trigonometric math and collision
// relaxation loops from the client's mobile CPU, allowing instant canvas rendering in 0ms.
func ComputeDiagramLayout(diagram *dto.DiagramResponse, canvasWidth, canvasHeight float64) {
	if diagram == nil || len(diagram.Nodes) == 0 {
		return
	}

	if canvasWidth <= 0 {
		canvasWidth = 2800.0
	}
	if canvasHeight <= 0 {
		canvasHeight = 2000.0
	}

	// 1. Calculate bounding box dimensions (Width, Height) for all nodes
	for i := range diagram.Nodes {
		calculateNodeDimensions(&diagram.Nodes[i])
	}

	// 2. Identify the root node (first node or node with type "root")
	rootIdx := 0
	for i, n := range diagram.Nodes {
		if strings.EqualFold(n.NodeType, "root") || n.ID == "root" {
			rootIdx = i
			break
		}
	}
	diagram.Nodes[rootIdx].NodeType = "root"

	// 3. Map parentID -> slice of child node indices (excluding cross-links)
	childrenMap := make(map[string][]int)
	nodeIndexByID := make(map[string]int, len(diagram.Nodes))
	for i, n := range diagram.Nodes {
		nodeIndexByID[n.ID] = i
	}

	for _, e := range diagram.Edges {
		if e.IsCrossLink {
			continue
		}
		fromID := e.FromID
		if fromID == "" {
			fromID = e.From
		}
		toID := e.ToID
		if toID == "" {
			toID = e.To
		}
		if childIdx, exists := nodeIndexByID[toID]; exists {
			childrenMap[fromID] = append(childrenMap[fromID], childIdx)
		}
	}

	// 4. Execute layout algorithm matching the resolved archetype
	archetype := strings.ToLower(strings.TrimSpace(diagram.Archetype))
	switch archetype {
	case "flow":
		layoutFlow(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "cycle":
		layoutCycle(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "comparison":
		layoutComparison(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "stack":
		layoutStack(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "horizontal", "tree":
		layoutHorizontal(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "vertical":
		layoutVertical(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	case "radial", "hub":
		fallthrough
	default:
		layoutRadial(diagram.Nodes, rootIdx, childrenMap, canvasWidth, canvasHeight)
	}

	// 5. Place any unconnected disconnected nodes safely around perimeter
	placeUnconnectedNodes(diagram.Nodes, rootIdx, canvasWidth, canvasHeight)

	// 6. Fast Iterative Collision-Avoidance Relaxation Pass
	resolveCollisions(diagram.Nodes)
}

func calculateNodeDimensions(node *dto.GraphNode) {
	labelRunes := []rune(node.Label)
	labelLen := float64(len(labelRunes))
	if labelLen == 0 {
		labelLen = 10.0
	}

	nodeType := strings.ToLower(node.NodeType)
	if nodeType == "" {
		if strings.Contains(strings.ToLower(node.Type), "root") || node.Shape == "circle" {
			nodeType = "root"
		} else if strings.Contains(strings.ToLower(node.Type), "branch") || node.Shape == "diamond" {
			nodeType = "branch"
		} else {
			nodeType = "leaf"
		}
		node.NodeType = nodeType
	}

	switch nodeType {
	case "root":
		node.Width = math.Max(195.0, math.Min(260.0, labelLen*9.0+36.0))
		node.Height = 64.0
	case "branch":
		node.Width = math.Max(175.0, math.Min(235.0, labelLen*8.5+40.0))
		node.Height = 58.0
	default:
		node.Width = math.Max(165.0, math.Min(225.0, labelLen*8.2+36.0))
		if len(node.Metrics) > 0 {
			node.Height = 62.0
		} else {
			node.Height = 54.0
		}
	}
}

func layoutRadial(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	centerY := canvasH / 2.0

	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = centerY - (root.Height / 2.0)

	branchIndices := childrenMap[root.ID]
	if len(branchIndices) == 0 {
		for i := range nodes {
			if i != rootIdx && (nodes[i].NodeType == "branch" || nodes[i].BranchID == root.ID) {
				branchIndices = append(branchIndices, i)
			}
		}
	}
	branchCount := len(branchIndices)
	if branchCount == 0 {
		return
	}

	branchRadius := math.Max(280.0, 190.0+float64(branchCount)*18.0)
	sectorStep := (2.0 * math.Pi) / float64(branchCount)

	for i, bIdx := range branchIndices {
		angle := sectorStep * float64(i)
		bx := centerX + branchRadius*math.Cos(angle)
		by := centerY + branchRadius*math.Sin(angle)

		branch := &nodes[bIdx]
		branch.X = bx - (branch.Width / 2.0)
		branch.Y = by - (branch.Height / 2.0)

		leafIndices := childrenMap[branch.ID]
		leafCount := len(leafIndices)

		for j, lIdx := range leafIndices {
			leaf := &nodes[lIdx]
			leafRadius := 190.0
			if j%2 != 0 {
				leafRadius = 255.0
			}

			maxSpread := math.Min(1.1, sectorStep*0.85)
			spreadAngle := angle
			if leafCount > 1 {
				spreadAngle = angle - (maxSpread / 2.0) + (maxSpread * float64(j) / float64(leafCount-1))
			}

			lx := bx + leafRadius*math.Cos(spreadAngle)
			ly := by + leafRadius*math.Sin(spreadAngle)
			leaf.X = lx - (leaf.Width / 2.0)
			leaf.Y = ly - (leaf.Height / 2.0)

			subLeafIndices := childrenMap[leaf.ID]
			subCount := len(subLeafIndices)
			for k, sIdx := range subLeafIndices {
				subLeaf := &nodes[sIdx]
				const subDist = 155.0
				subSpread := math.Min(0.7, sectorStep*0.45)
				subAngle := spreadAngle
				if subCount > 1 {
					subAngle = spreadAngle - (subSpread / 2.0) + (subSpread * float64(k) / float64(subCount-1))
				}
				subLeaf.X = lx + subDist*math.Cos(subAngle) - (subLeaf.Width / 2.0)
				subLeaf.Y = ly + subDist*math.Sin(subAngle) - (subLeaf.Height / 2.0)
			}
		}
	}
}

func layoutFlow(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	root := &nodes[rootIdx]
	stages := childrenMap[root.ID]
	if len(stages) == 0 {
		layoutHorizontal(nodes, rootIdx, childrenMap, canvasW, canvasH)
		return
	}

	totalStages := len(stages)
	const stageWidth = 240.0
	const stageSpacing = 420.0
	totalWidth := stageWidth + (float64(totalStages) * stageSpacing)
	startX := math.Max(120.0, (canvasW-totalWidth)/2.0)
	centerY := canvasH / 2.0

	root.X = startX
	root.Y = centerY - (root.Height / 2.0)

	for i, sIdx := range stages {
		stage := &nodes[sIdx]
		stageX := startX + 280.0 + (float64(i) * stageSpacing)
		stage.X = stageX
		stage.Y = centerY - (stage.Height / 2.0)

		leaves := childrenMap[stage.ID]
		const leafYSpacing = 78.0

		for j, lIdx := range leaves {
			leaf := &nodes[lIdx]
			leaf.X = stageX + 10.0
			if j%2 == 0 {
				leaf.Y = (centerY + (stage.Height / 2.0) + 42.0) + float64(j/2)*leafYSpacing
			} else {
				leaf.Y = (centerY - (stage.Height / 2.0) - 42.0 - leaf.Height) - float64((j-1)/2)*leafYSpacing
			}

			subLeaves := childrenMap[leaf.ID]
			for k, slIdx := range subLeaves {
				subLeaf := &nodes[slIdx]
				if k%2 == 0 {
					subLeaf.X = leaf.X + 190.0
				} else {
					subLeaf.X = leaf.X - 190.0
				}
				subLeaf.Y = leaf.Y + float64(k+1)*60.0
			}
		}
	}
}

func layoutCycle(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	centerY := canvasH / 2.0

	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = centerY - (root.Height / 2.0)

	phases := childrenMap[root.ID]
	count := len(phases)
	if count == 0 {
		layoutRadial(nodes, rootIdx, childrenMap, canvasW, canvasH)
		return
	}

	orbitRadius := math.Max(440.0, 320.0+float64(count)*24.0)
	angleStep := (2.0 * math.Pi) / float64(count)

	for i, pIdx := range phases {
		angle := (-math.Pi / 2.0) + (float64(i) * angleStep)
		phaseX := centerX + orbitRadius*math.Cos(angle)
		phaseY := centerY + orbitRadius*math.Sin(angle)

		phase := &nodes[pIdx]
		phase.X = phaseX - (phase.Width / 2.0)
		phase.Y = phaseY - (phase.Height / 2.0)

		leaves := childrenMap[phase.ID]
		leafCount := len(leaves)
		leafRadius := orbitRadius + 195.0

		for j, lIdx := range leaves {
			leaf := &nodes[lIdx]
			leafAngle := angle
			if leafCount > 1 {
				leafAngle = angle + (float64(j)-(float64(leafCount-1)/2.0))*0.22
			}
			leaf.X = (centerX + leafRadius*math.Cos(leafAngle)) - (leaf.Width / 2.0)
			leaf.Y = (centerY + leafRadius*math.Sin(leafAngle)) - (leaf.Height / 2.0)

			subLeaves := childrenMap[leaf.ID]
			for k, slIdx := range subLeaves {
				subLeaf := &nodes[slIdx]
				subRadius := leafRadius + 160.0
				subLeaf.X = (centerX + subRadius*math.Cos(leafAngle)) - (subLeaf.Width / 2.0)
				subLeaf.Y = (centerY + subRadius*math.Sin(leafAngle)) - (subLeaf.Height / 2.0) + (float64(k) * 60.0)
			}
		}
	}
}

func layoutComparison(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = 140.0

	branches := childrenMap[root.ID]
	if len(branches) == 0 {
		layoutVertical(nodes, rootIdx, childrenMap, canvasW, canvasH)
		return
	}

	const colSpacing = 480.0
	leftColX := centerX - colSpacing
	rightColX := centerX + colSpacing
	const startY = 290.0
	const rowHeight = 90.0

	var leftPillars, rightPillars []int
	for i, bIdx := range branches {
		if i%2 == 0 {
			leftPillars = append(leftPillars, bIdx)
		} else {
			rightPillars = append(rightPillars, bIdx)
		}
	}

	layoutColumn := func(colBranches []int, colX float64) {
		curY := startY
		for _, bIdx := range colBranches {
			branch := &nodes[bIdx]
			branch.X = colX - (branch.Width / 2.0)
			branch.Y = curY
			curY += rowHeight

			leaves := childrenMap[branch.ID]
			for _, lIdx := range leaves {
				leaf := &nodes[lIdx]
				leaf.X = colX - (leaf.Width / 2.0)
				leaf.Y = curY
				curY += rowHeight

				subLeaves := childrenMap[leaf.ID]
				for _, slIdx := range subLeaves {
					subLeaf := &nodes[slIdx]
					subLeaf.X = colX + 180.0
					subLeaf.Y = curY
					curY += rowHeight
				}
			}
			curY += 24.0
		}
	}

	layoutColumn(leftPillars, leftColX)
	layoutColumn(rightPillars, rightColX)
}

func layoutStack(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = 120.0

	layers := childrenMap[root.ID]
	if len(layers) == 0 {
		layoutVertical(nodes, rootIdx, childrenMap, canvasW, canvasH)
		return
	}

	const layerSpacing = 160.0
	const startY = 260.0

	for i, lIdx := range layers {
		layer := &nodes[lIdx]
		curY := startY + (float64(i) * layerSpacing)
		layer.X = centerX - (layer.Width / 2.0)
		layer.Y = curY

		leaves := childrenMap[layer.ID]
		leafStartX := centerX + (layer.Width / 2.0) + 45.0

		for j, leafIdx := range leaves {
			leaf := &nodes[leafIdx]
			leaf.X = leafStartX + (float64(j) * 230.0)
			leaf.Y = curY + ((layer.Height - leaf.Height) / 2.0)

			subLeaves := childrenMap[leaf.ID]
			for k, slIdx := range subLeaves {
				subLeaf := &nodes[slIdx]
				subLeaf.X = leaf.X + 220.0
				subLeaf.Y = leaf.Y + float64(k+1)*50.0
			}
		}
	}
}

func layoutHorizontal(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	centerY := canvasH / 2.0
	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = centerY - (root.Height / 2.0)

	branches := childrenMap[root.ID]
	if len(branches) == 0 {
		return
	}

	var leftBranches, rightBranches []int
	for i, bIdx := range branches {
		if i%2 == 0 {
			rightBranches = append(rightBranches, bIdx)
		} else {
			leftBranches = append(leftBranches, bIdx)
		}
	}

	layoutSide := func(sideBranches []int, isRight bool) {
		count := len(sideBranches)
		if count == 0 {
			return
		}
		var totalHeight float64
		heights := make([]float64, count)
		for i, bIdx := range sideBranches {
			h := calculateSubtreeHeight(nodes, bIdx, childrenMap)
			heights[i] = h
			totalHeight += h
		}

		startY := centerY - (totalHeight / 2.0)
		curY := startY
		const branchGapX = 85.0

		for i, bIdx := range sideBranches {
			branch := &nodes[bIdx]
			bHeight := heights[i]
			var bx float64
			if isRight {
				bx = centerX + (root.Width / 2.0) + branchGapX
			} else {
				bx = centerX - (root.Width / 2.0) - branchGapX - branch.Width
			}
			by := curY + (bHeight / 2.0) - (branch.Height / 2.0)
			branch.X = bx
			branch.Y = by

			layoutHorizontalSubtree(nodes, bIdx, isRight, bx, curY, childrenMap)
			curY += bHeight
		}
	}

	layoutSide(rightBranches, true)
	layoutSide(leftBranches, false)
}

func calculateSubtreeHeight(nodes []dto.GraphNode, nodeIdx int, childrenMap map[string][]int) float64 {
	node := &nodes[nodeIdx]
	children := childrenMap[node.ID]
	if len(children) == 0 {
		return node.Height + 28.0
	}
	var total float64
	for _, cIdx := range children {
		total += calculateSubtreeHeight(nodes, cIdx, childrenMap)
	}
	return math.Max(node.Height+28.0, total)
}

func layoutHorizontalSubtree(nodes []dto.GraphNode, parentIdx int, isRight bool, parentX, startY float64, childrenMap map[string][]int) {
	parent := &nodes[parentIdx]
	children := childrenMap[parent.ID]
	if len(children) == 0 {
		return
	}

	const horizontalGap = 80.0
	curY := startY

	for _, cIdx := range children {
		child := &nodes[cIdx]
		ch := calculateSubtreeHeight(nodes, cIdx, childrenMap)
		var cx float64
		if isRight {
			cx = parentX + parent.Width + horizontalGap
		} else {
			cx = parentX - horizontalGap - child.Width
		}
		cy := curY + (ch / 2.0) - (child.Height / 2.0)
		child.X = cx
		child.Y = cy

		layoutHorizontalSubtree(nodes, cIdx, isRight, cx, curY, childrenMap)
		curY += ch
	}
}

func layoutVertical(nodes []dto.GraphNode, rootIdx int, childrenMap map[string][]int, canvasW, _ float64) {
	centerX := canvasW / 2.0
	root := &nodes[rootIdx]
	root.X = centerX - (root.Width / 2.0)
	root.Y = 80.0

	branches := childrenMap[root.ID]
	branchCount := len(branches)
	if branchCount == 0 {
		return
	}

	var totalWidth float64
	widths := make([]float64, branchCount)
	for i, bIdx := range branches {
		w := calculateSubtreeWidth(nodes, bIdx, childrenMap)
		widths[i] = w
		totalWidth += w
	}

	startX := math.Max(60.0, centerX-(totalWidth/2.0))
	curX := startX
	const verticalGap = 72.0

	for i, bIdx := range branches {
		branch := &nodes[bIdx]
		bw := widths[i]
		bx := curX + (bw / 2.0) - (branch.Width / 2.0)
		by := root.Y + root.Height + verticalGap
		branch.X = bx
		branch.Y = by

		layoutVerticalSubtree(nodes, bIdx, curX, by, childrenMap)
		curX += bw
	}
}

func calculateSubtreeWidth(nodes []dto.GraphNode, nodeIdx int, childrenMap map[string][]int) float64 {
	node := &nodes[nodeIdx]
	children := childrenMap[node.ID]
	if len(children) == 0 {
		return node.Width + 36.0
	}
	var total float64
	for _, cIdx := range children {
		total += calculateSubtreeWidth(nodes, cIdx, childrenMap)
	}
	return math.Max(node.Width+36.0, total)
}

func layoutVerticalSubtree(nodes []dto.GraphNode, parentIdx int, startX, parentY float64, childrenMap map[string][]int) {
	parent := &nodes[parentIdx]
	children := childrenMap[parent.ID]
	if len(children) == 0 {
		return
	}

	curX := startX
	const verticalGap = 72.0

	for _, cIdx := range children {
		child := &nodes[cIdx]
		cw := calculateSubtreeWidth(nodes, cIdx, childrenMap)
		cx := curX + (cw / 2.0) - (child.Width / 2.0)
		cy := parentY + parent.Height + verticalGap
		child.X = cx
		child.Y = cy

		layoutVerticalSubtree(nodes, cIdx, curX, cy, childrenMap)
		curX += cw
	}
}

func placeUnconnectedNodes(nodes []dto.GraphNode, rootIdx int, canvasW, canvasH float64) {
	centerX := canvasW / 2.0
	centerY := canvasH / 2.0
	for i := range nodes {
		if i != rootIdx && nodes[i].X == 0 && nodes[i].Y == 0 {
			hash := float64(len(nodes[i].ID)*31 + len(nodes[i].Label)*17)
			angle := math.Mod(hash, 2.0*math.Pi)
			nodes[i].X = centerX + 440.0*math.Cos(angle)
			nodes[i].Y = centerY + 440.0*math.Sin(angle)
		}
	}
}

func resolveCollisions(nodes []dto.GraphNode) {
	const (
		horizontalGap = 24.0
		verticalGap   = 20.0
		maxPasses     = 15
	)

	for pass := 0; pass < maxPasses; pass++ {
		anyCollision := false

		for i := 0; i < len(nodes); i++ {
			a := &nodes[i]
			rectALeft := a.X - (horizontalGap / 2.0)
			rectATop := a.Y - (verticalGap / 2.0)
			rectARight := a.X + a.Width + (horizontalGap / 2.0)
			rectABottom := a.Y + a.Height + (verticalGap / 2.0)

			for j := i + 1; j < len(nodes); j++ {
				b := &nodes[j]
				rectBLeft := b.X - (horizontalGap / 2.0)
				rectBTop := b.Y - (verticalGap / 2.0)
				rectBRight := b.X + b.Width + (horizontalGap / 2.0)
				rectBBottom := b.Y + b.Height + (verticalGap / 2.0)

				if rectALeft < rectBRight && rectARight > rectBLeft &&
					rectATop < rectBBottom && rectABottom > rectBTop {
					anyCollision = true

					overlapX := math.Min(rectARight, rectBRight) - math.Max(rectALeft, rectBLeft)
					overlapY := math.Min(rectABottom, rectBBottom) - math.Max(rectATop, rectBTop)

					centerAX := a.X + (a.Width / 2.0)
					centerAY := a.Y + (a.Height / 2.0)
					centerBX := b.X + (b.Width / 2.0)
					centerBY := b.Y + (b.Height / 2.0)

					if overlapX < overlapY {
						push := (overlapX / 2.0) + 1.0
						aIsLeft := centerAX < centerBX
						if a.NodeType == "root" {
							if aIsLeft {
								b.X += push * 2.0
							} else {
								b.X -= push * 2.0
							}
						} else if b.NodeType == "root" {
							if aIsLeft {
								a.X -= push * 2.0
							} else {
								a.X += push * 2.0
							}
						} else {
							if aIsLeft {
								a.X -= push
								b.X += push
							} else {
								a.X += push
								b.X -= push
							}
						}
					} else {
						push := (overlapY / 2.0) + 1.0
						aIsTop := centerAY < centerBY
						if a.NodeType == "root" {
							if aIsTop {
								b.Y += push * 2.0
							} else {
								b.Y -= push * 2.0
							}
						} else if b.NodeType == "root" {
							if aIsTop {
								a.Y -= push * 2.0
							} else {
								a.Y += push * 2.0
							}
						} else {
							if aIsTop {
								a.Y -= push
								b.Y += push
							} else {
								a.Y += push
								b.Y -= push
							}
						}
					}
				}
			}
		}

		if !anyCollision {
			break
		}
	}
}
