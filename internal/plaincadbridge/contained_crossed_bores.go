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

// retainedCrossedToolPair removes only cutters covered by another cutter in
// the batch. Other bore families receive their original inputs when the two
// retained cutters do not form a qualified crossed pair.
func retainedCrossedToolPair(tools []*cylinderBody, cuts []trimmed.CylinderCut, spans []float64, blind []bool) (
	[]*cylinderBody, []trimmed.CylinderCut, []float64, []bool,
) {
	if len(tools) <= 2 {
		return tools, cuts, spans, blind
	}
	var kept [2]int
	count := 0
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
			if count == len(kept) {
				return tools, cuts, spans, blind
			}
			kept[count] = i
			count++
		}
	}
	if count != len(kept) {
		return tools, cuts, spans, blind
	}
	i, j := kept[0], kept[1]
	return []*cylinderBody{tools[i], tools[j]}, []trimmed.CylinderCut{cuts[i], cuts[j]},
		[]float64{spans[i], spans[j]}, []bool{blind[i], blind[j]}
}
