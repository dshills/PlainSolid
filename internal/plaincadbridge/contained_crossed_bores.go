package plaincadbridge

import "plainsolid/brep/trimmed"

// cardinalBoreContainsTool proves that every in-stock point of the second tool
// was already removed by an entry-open bore.
func cardinalBoreContainsTool(old *cylinderBody, blind bool, tool *cylinderBody) bool {
	if old.radius < tool.radius || !sameCardinalBoreLine(old, tool) {
		return false
	}
	if !blind {
		return true
	}
	axis := old.frame.ZAxis()
	floor := old.frame.Origin().Add(axis.Scale(old.height))
	start := tool.frame.Origin()
	end := start.Add(tool.frame.ZAxis().Scale(tool.height))
	return exactDotDifference(start, floor, axis) <= 0 && exactDotDifference(end, floor, axis) <= 0
}

// retainedCrossedTools removes only cutters covered by another cutter in
// the batch. Other bore families receive their original tool set.
func retainedCrossedTools(tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (
	[]*cylinderBody, []trimmed.CylinderCut, []float64, []bool,
) {
	if len(tools) <= 2 {
		return tools, cuts, spans, blind
	}
	kept := make([]int, 0, len(tools))
	for i, tool := range tools {
		covered := false
		for j, other := range tools {
			if i == j || !cardinalBoreContainsTool(other, blind[j], tool) {
				continue
			}
			// Mutually covered cutters tie by original position.
			if j < i || !cardinalBoreContainsTool(tool, blind[i], other) {
				covered = true
				break
			}
		}
		if !covered {
			kept = append(kept, i)
		}
	}
	if len(kept) == len(tools) {
		return tools, cuts, spans, blind
	}
	result := make([]*cylinderBody, 0, len(kept))
	resultCuts := make([]trimmed.CylinderCut, 0, len(kept))
	resultSpans := make([]float64, 0, len(kept))
	resultBlind := make([]bool, 0, len(kept))
	for _, i := range kept {
		result = append(result, tools[i])
		resultCuts = append(resultCuts, cuts[i])
		resultSpans = append(resultSpans, spans[i])
		resultBlind = append(resultBlind, blind[i])
	}
	return result, resultCuts, resultSpans, resultBlind
}
