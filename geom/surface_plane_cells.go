package geom

// SurfacePlaneIsolationOptions bounds conservative parameter-cell isolation.
// Zero fields select parameter tolerance 1e-2 in each direction, depth 24,
// 16384 visited cells, 262144 queued poles, 1M exact operations, and 16384
// coefficient bits. The patch's own temporary-pole limit still applies.
type SurfacePlaneIsolationOptions struct {
	ParameterTolerance                                                    float64
	MaxDepth, MaxCells, MaxQueuedPoles, MaxOperations, MaxCoefficientBits int
}

// SurfacePlaneCell encloses possible surface/plane contact. A straddling control
// hull does not prove any intersection exists. No contour connectivity, branch
// count, tangency classification, or point curve is inferred from these cells.
type SurfacePlaneCell struct {
	Domain                ParameterRectangle
	Bounds                Box
	PlaneRange            SurfacePlaneRange
	Depth                 int
	AtRequestedResolution bool
}

// SurfacePlaneIsolation is a conservative covering, not a traced intersection.
// Complete means all possible noncoplanar contact is covered by Candidates at
// requested parameter resolution. Even then candidate cells can be false
// positives. Coplanar rectangles are exact; Pending rectangles need more work.
// On exhaustion, Candidates/Pending still cover unclassified contact, but the
// result must not be used as a complete intersection solution.
type SurfacePlaneIsolation struct {
	Candidates                    []SurfacePlaneCell
	Coplanar                      []ParameterRectangle
	Pending                       []ParameterRectangle
	VisitedCells, ExactOperations int
	Complete                      bool
}

func internalPatchKnot(knots []float64, degree, count int) (float64, bool) {
	start, end := knots[degree], knots[count]
	middle := start + (end-start)/2
	best, distance := 0.0, end-start
	found := false
	for _, k := range knots {
		if k > start && k < end {
			d := k - middle
			if d < 0 {
				d = -d
			}
			if !found || d < distance {
				best, distance, found = k, d, true
			}
		}
	}
	return best, found
}

// BoundSurfacePlane isolates a knot-aligned covering of possible intersection
// cells using exact rational control-hull plane signs. Refinement cuts existing
// knot lines before bisecting Bezier cells. Only strictly one-sided hulls are
// discarded, so even-multiplicity tangencies cannot be lost by sign sampling.
// Exhausted resources or unresolved depth return ErrConvergence and diagnostics.
func BoundSurfacePlane(patch *SurfacePatch, plane Plane, options SurfacePlaneIsolationOptions) (result SurfacePlaneIsolation, err error) {
	if patch == nil || len(patch.net) == 0 || !plane.valid() {
		return result, ErrInvalid
	}
	o := options
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-2
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 16384
	}
	if o.MaxQueuedPoles == 0 {
		o.MaxQueuedPoles = 262144
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxQueuedPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	type node struct {
		patch *SurfacePatch
		depth int
	}
	stack := []node{{patch, 0}}
	queuedPoles := len(patch.net)
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	pending := func(current *SurfacePatch) {
		if current != nil {
			result.Pending = append(result.Pending, current.Domain())
		}
		for _, n := range stack {
			result.Pending = append(result.Pending, n.patch.Domain())
		}
	}
	if queuedPoles > o.MaxQueuedPoles {
		pending(nil)
		return result, ErrConvergence
	}
	depthExhausted := false
	for len(stack) > 0 {
		if result.VisitedCells >= o.MaxCells {
			pending(nil)
			return result, ErrConvergence
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		queuedPoles -= len(current.patch.net)
		result.VisitedCells++
		planeRange, e := current.patch.planeBounds(plane, budget)
		if e != nil {
			pending(current.patch)
			return result, e
		}
		domain := current.patch.Domain()
		switch planeRange.Relation {
		case PlanePatchAbove, PlanePatchBelow:
			continue
		case PlanePatchCoplanar:
			result.Coplanar = append(result.Coplanar, domain)
			continue
		}
		uKnot, hasU := internalPatchKnot(current.patch.knotsU, current.patch.degreeU, current.patch.countU)
		vKnot, hasV := internalPatchKnot(current.patch.knotsV, current.patch.degreeV, current.patch.countV)
		resolved := !hasU && !hasV && domain.UMax-domain.UMin <= o.ParameterTolerance && domain.VMax-domain.VMin <= o.ParameterTolerance
		cell := SurfacePlaneCell{domain, current.patch.bounds, planeRange, current.depth, resolved}
		if resolved {
			result.Candidates = append(result.Candidates, cell)
			continue
		}
		if current.depth >= o.MaxDepth {
			result.Candidates = append(result.Candidates, cell)
			depthExhausted = true
			continue
		}
		splitU, parameter := true, uKnot
		if !hasU {
			if hasV {
				splitU, parameter = false, vKnot
			} else if domain.UMax-domain.UMin >= domain.VMax-domain.VMin {
				parameter = domain.UMin + (domain.UMax-domain.UMin)/2
			} else {
				splitU, parameter = false, domain.VMin+(domain.VMax-domain.VMin)/2
			}
		}
		if splitU && (parameter == domain.UMin || parameter == domain.UMax) || !splitU && (parameter == domain.VMin || parameter == domain.VMax) {
			result.Candidates = append(result.Candidates, cell)
			depthExhausted = true
			continue
		}
		left, right, e := current.patch.split(parameter, splitU, budget)
		if e != nil {
			pending(current.patch)
			return result, e
		}
		newPoles := len(left.net) + len(right.net)
		if newPoles > o.MaxQueuedPoles-queuedPoles {
			pending(current.patch)
			return result, ErrConvergence
		}
		queuedPoles += newPoles
		stack = append(stack, node{right, current.depth + 1}, node{left, current.depth + 1})
	}
	if depthExhausted {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}
