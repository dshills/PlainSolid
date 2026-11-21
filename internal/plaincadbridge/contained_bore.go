package plaincadbridge

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
