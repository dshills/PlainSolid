package geom

import (
	"context"
)

type ComposedConicCollectionContactPair struct {
	Cells        [2]int
	Intersection ComposedConicOverlap
}

// ComposedConicCollectionImageComparison distinguishes exhaustive contact discovery
// from same-image equality. Containment/equality require exact coincident
// interval coverage; isolated point contacts cannot cover an interval gap.
// PartialCoincidence means a positive-length coincident overlap without equality.
type ComposedConicCollectionImageComparison struct {
	Complete, SameImage, AContained, BContained, PartialCoincidence bool
	TraversalRelation                                               int
	Pairs                                                           []ComposedConicCollectionContactPair
	Operations                                                      int
	// Subdivisions counts join and coverage proofs. Pair-internal refinements
	// retain the per-root MaxSubdivisions bound of IntersectCoincidentComposedConics.
	Subdivisions int
}

// CompareComposedConicCollections revalidates continuous ordered collections,
// proves pairwise same-conic Mobius maps with consistent traversal relation,
// and checks exact union coverage in each original cell parameter. Different
// algebraic cuts and nonlinear parameter speeds are supported. A map whose
// denominator crosses zero remains unresolved; no seam is inferred there.
// TraversalRelation is +1 for matching and -1 for reversed traversals. The
// operation never trusts the public Complete flags on input collections.
func CompareComposedConicCollections(ctx context.Context, a, c SurfaceCurveCompositionCells, o ComposedCollectionComparisonOptions) (r ComposedConicCollectionImageComparison, err error) {
	if ctx == nil || len(a.Cells) == 0 || len(c.Cells) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := graphRootOptions(o.ExactCurvePlaneEventOptions)
	if e != nil {
		return r, e
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 4096
	}
	if o.MaxPairs < 1 {
		return r, ErrInvalid
	}
	if len(a.Cells) > o.MaxPairs/len(c.Cells) {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	remaining := func() (ExactCurvePlaneEventOptions, error) {
		x := options
		x.MaxOperations -= budget.operations
		x.MaxSubdivisions -= r.Subdivisions
		if x.MaxOperations < 1 || x.MaxSubdivisions < 1 {
			return x, ErrConvergence
		}
		return x, nil
	}
	collections := [2]SurfaceCurveCompositionCells{a, c}
	for owner, collection := range collections {
		if collection.Cells[0] == nil || collection.Cells[len(collection.Cells)-1] == nil {
			return r, ErrInvalid
		}
		lo, e := collection.Cells[0].EndpointRoot(0)
		if e != nil {
			return r, e
		}
		hi, e := collection.Cells[len(collection.Cells)-1].EndpointRoot(1)
		if e != nil {
			return r, e
		}
		child, e := remaining()
		if e != nil {
			return r, e
		}
		validated, e := collection.RestrictAlgebraic(ctx, lo, hi, child)
		budget.operations += validated.ExactOperations
		r.Subdivisions += validated.Subdivisions
		if e != nil {
			return r, e
		}
		if !validated.Complete {
			return r, ErrConvergence
		}
		collections[owner] = validated

	}
	type interval [2]*ExactGraphLineRoot
	coverage := [2][][]interval{make([][]interval, len(collections[0].Cells)), make([][]interval, len(collections[1].Cells))}
	anyCoincidence := false
	for i, left := range collections[0].Cells {
		for j, right := range collections[1].Cells {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			child, e := remaining()
			if e != nil {
				return r, e
			}
			pair, e := IntersectCoincidentComposedConics(ctx, left, right, child)
			budget.operations += pair.Operations
			r.Pairs = append(r.Pairs, ComposedConicCollectionContactPair{Cells: [2]int{i, j}, Intersection: pair})
			if e != nil {
				return r, e
			}
			if !pair.Complete {
				return r, ErrConvergence
			}

			if r.TraversalRelation == 0 {
				r.TraversalRelation = pair.Orientation
			} else if r.TraversalRelation != pair.Orientation {
				return r, ErrConvergence
			}
			if !pair.Coincident {
				continue
			}
			anyCoincidence = true
			coverage[0][i] = append(coverage[0][i], interval{pair.Boundary[0].A.root, pair.Boundary[1].A.root})
			first, last := pair.Boundary[0].B.root, pair.Boundary[1].B.root
			if pair.Orientation < 0 {
				first, last = last, first
			}
			coverage[1][j] = append(coverage[1][j], interval{first, last})
		}
	}
	compare := func(x, y *ExactGraphLineRoot) (int, error) {
		child, e := remaining()
		if e != nil {
			return 0, e
		}
		q, e := compareRootBudget(ctx, x, y, budget, child.MaxSubdivisions)
		r.Subdivisions += q.Subdivisions
		if e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, ErrConvergence
		}
		return q.Order, nil
	}
	contained := [2]bool{true, true}
	for owner, collection := range collections {
		for index, cell := range collection.Cells {
			intervals := coverage[owner][index]
			// Insertion sort uses exact algebraic ordering, never float brackets.
			for i := 1; i < len(intervals); i++ {
				for j := i; j > 0; j-- {
					order, e := compare(intervals[j-1][0], intervals[j][0])
					if e != nil {
						return r, e
					}
					if order <= 0 {
						break
					}
					intervals[j-1], intervals[j] = intervals[j], intervals[j-1]
				}
			}
			covered := cell.endpoints[0]
			for _, span := range intervals {
				order, e := compare(span[0], covered)
				if e != nil {
					return r, e
				}
				if order > 0 {
					break
				}
				order, e = compare(span[1], covered)
				if e != nil {
					return r, e
				}
				if order > 0 {
					covered = span[1]
				}
			}
			order, e := compare(covered, cell.endpoints[1])
			if e != nil {
				return r, e
			}
			if order != 0 {
				contained[owner] = false
			}
		}
	}
	r.AContained, r.BContained = contained[0], contained[1]
	r.SameImage = r.AContained && r.BContained
	r.PartialCoincidence = anyCoincidence && !r.SameImage
	r.Complete = true
	return r, nil
}
