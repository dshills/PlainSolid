package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedAlgebraicShellEmbedding proves the two source disks have disjoint
// interiors and intersect exactly in their declared whole edge. Their union
// is an embedded open disk. This still does not assert closed material/volume.
// The underlying shell retains both original supports and UV occurrences.
type WrappedAlgebraicShellEmbedding struct {
	shell      *WrappedAlgebraicOpenShell
	shifts     [2]*big.Int
	boundary   []ExactQuadraticUV
	bounds     geom.ParameterRectangle
	axes       [2]int
	operations int
}

func (p *WrappedAlgebraicShellEmbedding) Shell() *WrappedAlgebraicOpenShell {
	if p == nil {
		return nil
	}
	return p.shell
}
func (p *WrappedAlgebraicShellEmbedding) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// BPeriodShift means original B UV = aligned A-chart UV + shift*period.
func (p *WrappedAlgebraicShellEmbedding) BPeriodShift() [2]*big.Int {
	var out [2]*big.Int
	if p == nil {
		return out
	}
	for a := range out {
		out[a] = new(big.Int).Set(p.shifts[a])
	}
	return out
}

// BoundaryUV is the positive exterior in A's original lifted chart, independent
// of a physical face reversal. Exact trim coefficients are immutable.
func (p *WrappedAlgebraicShellEmbedding) BoundaryUV() []ExactQuadraticUV {
	if p == nil {
		return nil
	}
	return append([]ExactQuadraticUV(nil), p.boundary...)
}
func (p *WrappedAlgebraicShellEmbedding) LiftedBounds() geom.ParameterRectangle {
	if p == nil {
		return geom.ParameterRectangle{}
	}
	return p.bounds
}
func (p *WrappedAlgebraicShellEmbedding) ProjectionAxes() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.axes
}

// CertifyWrappedAlgebraicOpenShellEmbedding is a sufficient exact cofacial
// adjacency proof. Supports must have the same original parameter rectangle
// and equal rational maps, allowing independently refined knots. B may differ
// by constant whole periods; other chart changes remain unsupported.
//
// Both source loops are certified simple positive disks. After exact shared
// UV-edge cancellation, the remaining positive boundary must also be simple.
// Winding-number additivity then gives disjoint disk interiors and no other
// boundary intersections. One projected-injective hull for the entire aligned
// pair transports this planar proof to model space. No sampled separation or
// merely local Jacobian claim substitutes for these three global conditions.
func CertifyWrappedAlgebraicOpenShellEmbedding(ctx context.Context, s *WrappedAlgebraicOpenShell, o geom.SurfaceInjectivityOptions) (*WrappedAlgebraicShellEmbedding, error) {
	if ctx == nil || s == nil || !s.pair.Proven() || s.sources[0] == nil || s.sources[1] == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	if e := wrappedShellSourceBinding(ctx, s); e != nil {
		return nil, e
	}
	if !s.pair.ReversedParameter() || s.faces[0].Reversed != s.faces[1].Reversed {
		return nil, geom.ErrConvergence
	}
	a, ok := s.faces[0].Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return nil, geom.ErrConvergence
	}
	b, ok := s.faces[1].Surface.(*geom.WrappedSurfacePatch)
	if !ok || a.Domain() != b.Domain() {
		return nil, geom.ErrConvergence
	}
	relation, comparison, e := geom.CompareWrappedSurfaceCharts(ctx, a, b, geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: o.MaxCells})
	if e != nil {
		return nil, e
	}
	if relation == nil || !comparison.Complete || !comparison.Equal {
		return nil, geom.ErrConvergence
	}
	used := comparison.ExactOperations
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	remaining := func() int {
		n := o.MaxOperations - used
		if n <= 0 {
			return -1
		}
		return n
	}
	ownerA, _ := s.pair.Owner(0)
	ownerB, _ := s.pair.Owner(1)
	periods := a.Periods()
	var shifts [2]*big.Int
	var delta [2]*big.Rat
	for axis := 0; axis < 2; axis++ {
		delta[axis] = new(big.Rat).Sub(ownerB.UV[1][axis], ownerA.UV[0][axis])
		if e := check(delta[axis]); e != nil {
			return nil, e
		}
		shifts[axis] = new(big.Int)
		if periods[axis] == nil {
			if delta[axis].Sign() != 0 {
				return nil, geom.ErrConvergence
			}
		} else {
			q := new(big.Rat).Quo(delta[axis], periods[axis])
			if e := check(q); e != nil {
				return nil, e
			}
			if !q.IsInt() {
				return nil, geom.ErrConvergence
			}
			shifts[axis].Set(q.Num())
		}
	}
	aligned := [2][]ExactQuadraticUV{append([]ExactQuadraticUV(nil), s.sources[0].face.loops[0]...), nil}
	for _, q := range s.sources[1].face.loops[0] {
		h := q.HomogeneousControls()
		for i := range h {
			for axis := 0; axis < 2; axis++ {
				h[i][axis].Sub(h[i][axis], new(big.Rat).Mul(delta[axis], h[i][2]))
				if e := check(h[i][axis]); e != nil {
					return nil, e
				}
			}
		}
		x, e := NewExactQuadraticUV(h)
		if e != nil {
			return nil, e
		}
		aligned[1] = append(aligned[1], x)
	}
	// Establish the entire shared UV map, not just its endpoint shift.
	qa := aligned[0][ownerA.Edge]
	qb := aligned[1][ownerB.Edge]
	var curves [2]*geom.ExactSplineCurve
	for i, q := range []ExactQuadraticUV{qa, qb} {
		curves[i], e = q.modelParameterCurve(ctx, geom.SurfacePatchOptions{MaxOperations: remaining(), MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
		used += curves[i].ExactOperations()
	}
	same, e := geom.CompareExactCurvesContext(ctx, curves[0], curves[1], geom.ExactCurveComparisonOptions{ReverseB: true, MaxSpans: 1, MaxOperations: remaining(), MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	used += same.ExactOperations
	if !same.Complete || !same.Equal {
		return nil, geom.ErrConvergence
	}
	out := &WrappedAlgebraicShellEmbedding{shell: s, shifts: shifts, bounds: geom.ParameterRectangle{UMin: math.Inf(1), UMax: math.Inf(-1), VMin: math.Inf(1), VMax: math.Inf(-1)}}
	// Bound all source control hulls, not merely the remaining exterior. This
	// includes the entire cancelled curve and each original face interior.
	for _, loop := range aligned {
		for _, q := range loop {
			for _, h := range q.control {
				var xy [2]float64
				for axis := 0; axis < 2; axis++ {
					v := new(big.Rat).Quo(h[axis], h[2])
					if e := check(v); e != nil {
						return nil, e
					}
					xy[axis], e = geom.RepresentedExactScalar(v)
					if e != nil {
						return nil, e
					}
				}
				out.bounds.UMin = math.Min(out.bounds.UMin, xy[0])
				out.bounds.UMax = math.Max(out.bounds.UMax, xy[0])
				out.bounds.VMin = math.Min(out.bounds.VMin, xy[1])
				out.bounds.VMax = math.Max(out.bounds.VMax, xy[1])
			}
		}
	}
	var exterior trimvalidate.Loop
	for i := range s.boundary {
		at := i
		if s.faces[0].Reversed {
			at = len(s.boundary) - 1 - i
		}
		use := s.boundary[at]
		q := aligned[use.Face][use.LocalEdge]
		segment, e := trimvalidate.ExactQuadratic(q.HomogeneousControls())
		if e != nil {
			return nil, e
		}
		out.boundary = append(out.boundary, q)
		exterior = append(exterior, segment)
	}
	certificate, report, e := trimvalidate.CertifyLoops(ctx, exterior, nil, trimvalidate.Options{MaxEdges: 64, MaxWork: remaining(), MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return nil, e
	}
	used += report.Work
	if report.Status == trimvalidate.Invalid {
		return nil, geom.ErrDomain
	}
	if certificate == nil || report.Status != trimvalidate.Certified {
		return nil, geom.ErrConvergence
	}
	proofOptions := o
	proofOptions.MaxOperations = remaining()
	embedding, e := a.CertifyLiftedRectangleInjectivity(ctx, out.bounds, proofOptions)
	if e != nil {
		return nil, e
	}
	used += embedding.ExactOperations
	if !embedding.ProvenInjective {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.axes = embedding.ProjectionAxes
	out.operations = used
	return out, nil
}
