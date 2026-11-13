package intersect

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// TubeBranchOptions bounds mixed-axis endpoint graph construction. Zero
// MaxComparisons allows 32768 pair comparisons, including broad-phase rejects.
type TubeBranchOptions struct {
	// PruneSeparatedEndpoints excludes proved disjoint endpoint boxes before
	// charging MaxComparisons. The bounded endpoint scan still checks cancellation.
	PruneSeparatedEndpoints bool
	Curves                  TubeCurveOptions
	MaxComparisons          int
}

// OrientedTubeCell is a certified local branch in traversal order. Reversed
// means the tube's free parameter decreases along the canonical tangent.
type OrientedTubeCell struct {
	Cell, FreeAxis int
	Reversed       bool
}

// SplineTubeBranch is an open chain or closed loop of proved local junctions.
// Start/EndUnresolved mark possible ambiguous continuations; false on an
// open end does not certify that there is no continuation outside these tubes.
type SplineTubeBranch struct {
	Cells                          []OrientedTubeCell
	Closed                         bool
	StartUnresolved, EndUnresolved bool
}

type TubeBranchReport struct {
	Branches    []SplineTubeBranch
	Comparisons int
	Truncated   bool
}

// JoinSplineTubeBranches joins tubes that meet at exactly the same endpoint,
// including changes of free axis. The consistently oriented implicit tangent
// distinguishes adjacent continuations from two tubes that cover the same
// side of a junction. No connection is made at an ambiguous endpoint. This
// graph does not prove global branch coverage or exclude interior overlaps.
func JoinSplineTubeBranches(ctx context.Context, a, b Patch, seeds SplineSeedReport, o TubeBranchOptions) (TubeBranchReport, error) {
	var out TubeBranchReport
	if o.MaxComparisons == 0 {
		o.MaxComparisons = 32768
	}
	if ctx == nil || !a.valid() || !b.valid() || o.MaxComparisons < 1 || len(seeds.Attempts) > 4096 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	patch := func(p Patch) (*geom.SurfacePatch, error) {
		s, ok := p.Surface.(*geom.BSplineSurface)
		if !ok {
			return nil, geom.ErrInvalid
		}
		q, e := geom.NewSurfacePatch(s, o.Curves.Patches)
		if e != nil {
			return nil, e
		}
		return q.Trim(geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y})
	}
	pa, e := patch(a)
	if e != nil {
		return out, e
	}
	pb, e := patch(b)
	if e != nil {
		return out, e
	}
	type tube struct {
		cell      OrientedTubeCell
		da, db    geom.ParameterRectangle
		proof     geom.VerifiedSurfaceTube
		endpoints [2]geom.SurfaceTubeEvaluation // lower, upper in raw parameter order
	}
	var tubes []tube
	op := o.Curves.Junction
	projection := tubeProjectionLookup(ctx, pa, pb, op)
	for i, x := range seeds.Attempts {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if x.Tube == nil || !x.Tube.ProvenBranch {
			continue
		}
		axis, da, db := x.Tube.FreeAxis, x.Cell.DomainA, x.Cell.DomainB
		if axis < 0 || axis > 3 {
			return out, geom.ErrInvalid
		}
		qa, e := pa.Trim(da)
		if e != nil {
			return out, e
		}
		qb, e := pb.Trim(db)
		if e != nil {
			return out, e
		}
		proof, e := verifySplineTubeCell(ctx, qa, qb, geom.SurfacePairTubeOptions{
			FreeAxis: axis, MaxOperations: op.MaxOperations, MaxCoefficientBits: op.MaxCoefficientBits,
		}, x.BoundaryAnchor)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) {
				return out, fmt.Errorf("%w: invalid local certificate for cell %d", ErrUnresolved, i)
			}
			return out, e
		}
		r := tubePatchRanges(TubeSegment{pa: qa, pb: qb})[axis]
		if r[0] != x.Tube.FreeParameter.Min || r[1] != x.Tube.FreeParameter.Max || proof.FreeTangentSign() == 0 {
			return out, fmt.Errorf("%w: invalid local certificate for cell %d", ErrUnresolved, i)
		}
		t := tube{cell: OrientedTubeCell{i, axis, proof.FreeTangentSign() < 0}, da: da, db: db, proof: proof}
		for j, p := range r {
			v := o.Curves.Evaluation
			v.FreeAxis, v.Parameter = axis, p
			t.endpoints[j], e = proof.Evaluate(v)
			if e != nil {
				return out, e
			}
		}
		tubes = append(tubes, t)
	}
	// Orient every endpoint as entry (even index) or exit (odd index).
	raw := func(endpoint int) int {
		i := endpoint % 2
		if tubes[endpoint/2].cell.Reversed {
			i ^= 1
		}
		return i
	}
	candidates := make([][]int, 2*len(tubes))
	blocked := make([]bool, len(candidates))
search:
	for i := range candidates {
		for j := i + 1; j < len(candidates); j++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if o.PruneSeparatedEndpoints {
				if i/2 == j/2 || !overlapTubeEndpoints(tubes[i/2].endpoints[raw(i)], tubes[j/2].endpoints[raw(j)]) {
					continue
				}
			}
			if out.Comparisons == o.MaxComparisons {
				out.Truncated = true
				break search
			}
			out.Comparisons++
			if i/2 == j/2 {
				continue
			}
			x, y := tubes[i/2], tubes[j/2]
			ex, ey := x.endpoints[raw(i)], y.endpoints[raw(j)]
			if !overlapTubeEndpoints(ex, ey) {
				continue
			}
			axisX, axisY := x.cell.FreeAxis, y.cell.FreeAxis
			connected, separated := false, false
			if axisX == axisY && raw(i) != raw(j) {
				p := op
				p.FreeAxis = axisX
				c, e := geom.CertifySurfaceTubeJunction(pa, pb, x.da, x.db, y.da, y.db, p)
				if e != nil && !errors.Is(e, geom.ErrConvergence) {
					return out, e
				}
				connected, separated = c.Connected, c.Excluded
			}
			if !connected && !separated {
				c, e := certifyTubeEndpointJunction(ctx, pa, pb, x.da, x.db, y.da, y.db, geom.SurfaceTubeEndpointOptions{
					LeftFreeAxis: axisX, RightFreeAxis: axisY, LeftEnd: raw(i), RightEnd: raw(j),
					MaxOperations: op.MaxOperations, MaxCoefficientBits: op.MaxCoefficientBits,
				}, ex, ey, x.proof, y.proof, projection)
				if e != nil && !errors.Is(e, geom.ErrConvergence) {
					return out, e
				}
				connected, separated = c.Connected, c.Separated
			}
			switch {
			case connected && i%2 != j%2:
				candidates[i] = append(candidates[i], j)
				candidates[j] = append(candidates[j], i)
			case !separated:
				// Equal entries or equal exits imply overlapping local coverage.
				blocked[i], blocked[j] = true, true
			}
		}
	}
	linked := make([]int, len(candidates))
	for i, c := range candidates {
		linked[i] = -1
		if out.Truncated || blocked[i] || len(c) != 1 {
			continue
		}
		j := c[0]
		if !blocked[j] && len(candidates[j]) == 1 {
			linked[i] = j
		}
	}
	unresolved := func(i int) bool { return out.Truncated || blocked[i] || linked[i] == -1 && len(candidates[i]) > 0 }
	visited := make([]bool, len(tubes))
	trace := func(first int) SplineTubeBranch {
		branch := SplineTubeBranch{StartUnresolved: unresolved(2 * first)}
		i := first
		for !visited[i] {
			visited[i] = true
			branch.Cells = append(branch.Cells, tubes[i].cell)
			j := linked[2*i+1]
			if j < 0 {
				branch.EndUnresolved = unresolved(2*i + 1)
				break
			}
			i = j / 2
		}
		branch.Closed = linked[2*i+1] >= 0 && i == first
		return branch
	}
	for i := range tubes {
		if linked[2*i] < 0 {
			out.Branches = append(out.Branches, trace(i))
		}
	}
	for i := range tubes {
		if !visited[i] {
			out.Branches = append(out.Branches, trace(i))
		}
	}
	return out, nil
}

// Exact chart projection can prove equality of two irrational endpoints
// when the fixed coordinates on opposite charts express the same constraint.
func certifyTubeEndpointJunction(ctx context.Context, a, b *geom.SurfacePatch, da, db, ea, eb geom.ParameterRectangle,
	o geom.SurfaceTubeEndpointOptions, x, y geom.SurfaceTubeEvaluation, left, right geom.VerifiedSurfaceTube,
	projection func() *tubeChartProjection) (geom.SurfaceTubeEndpointJunction, error) {
	o.ExactParameterHint = tubeJunctionHint(x, y)

	r, e := geom.CertifySurfaceTubeEndpointJunction(a, b, da, db, ea, eb, o)
	if r.Connected || r.Separated {
		return r, e
	}
	if e != nil && !errors.Is(e, geom.ErrConvergence) {
		return r, e
	}
	if e == nil && o.LeftFreeAxis != o.RightFreeAxis {
		if mapping := projection(); mapping != nil {
			r = mapping.junction(da, db, ea, eb, o, x, r)
			if r.Connected || r.Separated {
				return r, nil
			}
		}
		q, e := geom.CertifyTubeEndpointsByIsoIntersection(ctx, left, right, o.LeftEnd == 1, o.RightEnd == 1, geom.ExactCurveSegmentOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil && !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) && !errors.Is(e, geom.ErrDomain) {
			return r, e
		}
		r.Connected, r.Separated = q.Connected, q.Separated
		r.ExactOperations += q.ExactOperations
		if r.Connected || r.Separated {
			return r, nil
		}
	}
	limit := o.MaxOperations
	if limit == 0 {
		limit = 8000000
	}
	remaining := limit - r.ExactOperations
	if remaining > 0 {
		connected, operations, e := algebraicGraphTubeJunction(ctx, left, right, o.LeftEnd == 1, o.RightEnd == 1, remaining, o.MaxCoefficientBits)
		r.ExactOperations += operations
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return r, e
		}
		r.Connected = connected
	}
	return r, nil
}

func tubeJunctionHint(a, b geom.SurfaceTubeEvaluation) [4]*big.Rat {
	c, e := canonicalTubePoint(a, b)
	if e != nil {
		return [4]*big.Rat{}
	}
	x, y := midUV(c.ParametersA), midUV(c.ParametersB)
	return [4]*big.Rat{new(big.Rat).SetFloat64(x.X), new(big.Rat).SetFloat64(x.Y),
		new(big.Rat).SetFloat64(y.X), new(big.Rat).SetFloat64(y.Y)}
}

func overlapTubeEndpoints(a, b geom.SurfaceTubeEvaluation) bool {
	overlap := func(x, y geom.UVBox) bool {
		return x.Min.X <= y.Max.X && y.Min.X <= x.Max.X && x.Min.Y <= y.Max.Y && y.Min.Y <= x.Max.Y
	}
	if !overlap(a.ParametersA, b.ParametersA) || !overlap(a.ParametersB, b.ParametersB) {
		return false
	}
	_, ok := a.Bounds.Intersect(b.Bounds)
	return ok
}
