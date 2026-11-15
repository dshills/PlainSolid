package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellBooleanOptions bounds embedding, inter-shell separation and
// witness ray proofs. Proof limits apply per call to the component algorithms.
// Ray distance bounds are derived from the exact patch boxes. MaxRays defaults
// to 12, the maximum available deterministic directions.
type SplineShellBooleanOptions struct {
	Embedding  SplineRectangleEmbeddingOptions
	Separation geom.SurfaceSurfaceIsolationOptions
	Ray        SplineRayOptions
	MaxRays    int
}

// SplineShellBooleanHistory maps each input face to an unchanged output
// support and trim or -1 if removed. BReversed identifies the inward cavity
// produced by subtraction. No intersection edges are generated.
type SplineShellBooleanHistory struct {
	FacesA, FacesB     []int
	AInsideB, BInsideA bool
	BReversed          bool
}

// CombineSeparatedSplineShells combines two independently parameterized,
// outward, single-shell spline solids with provably disjoint boundaries.
// Strict nesting, strict separation and empty operands are supported. A
// nested subtraction preserves the inner curved faces as a reversed cavity.
// A contact or an inconclusive proof returns an error without a solid.
func CombineSeparatedSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellBooleanOptions) (out Solid, h SplineShellBooleanHistory, err error) {
	if o.MaxRays == 0 {
		o.MaxRays = 12
	}
	if ctx == nil || op > parametric.Difference || o.MaxRays < 1 || o.MaxRays > 12 {
		return out, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, h, e
	}
	var ap, bp []splineEmbeddingFace
	for _, x := range []struct {
		s Solid
		p *[]splineEmbeddingFace
	}{{a, &ap}, {b, &bp}} {
		if len(x.s.faces) > 0 {
			if _, e := x.s.validateSplineRectangleEmbedding(ctx, o.Embedding, x.p); e != nil {
				return out, h, e
			}
		}
	}
	h.FacesA, h.FacesB = make([]int, len(ap)), make([]int, len(bp))
	for _, m := range [][]int{h.FacesA, h.FacesB} {
		for i := range m {
			m[i] = -1
		}
	}
	keepA, keepB := false, false
	if len(ap) == 0 || len(bp) == 0 {
		keepA = op != parametric.Intersection && len(ap) > 0
		keepB = op == parametric.Union && len(bp) > 0
	} else {
		ba, bb := splineShellPatchBounds(ap), splineShellPatchBounds(bp)
		if _, overlap := ba.Intersect(bb); overlap {
			for i, x := range ap {
				for j, y := range bp {
					if e := ctx.Err(); e != nil {
						return out, h, e
					}
					a, _ := x.patch.Bounds()
					b, _ := y.patch.Bounds()
					if _, overlap := a.Intersect(b); !overlap {
						continue
					}
					isolation := o.Separation
					isolation.SeparatingProjections = true
					q, e := geom.BoundSurfaceSurfaceContext(ctx, x.patch, y.patch, isolation)
					if e != nil && !errors.Is(e, geom.ErrConvergence) {
						return out, h, e
					}
					if !q.Complete || len(q.Candidates) != 0 || len(q.Coincident) != 0 {
						return out, h, fmt.Errorf("trimmed: spline shell faces A:%d and B:%d are not proved disjoint: %w", i, j, geom.ErrConvergence)
					}
				}
			}
			var e error
			h.AInsideB, e = classifySeparatedSplineWitness(ctx, b, bb, a.vertices[0], o)
			if e != nil {
				return out, h, e
			}
			h.BInsideA, e = classifySeparatedSplineWitness(ctx, a, ba, b.vertices[0], o)
			if e != nil {
				return out, h, e
			}
			if h.AInsideB && h.BInsideA {
				return out, h, geom.ErrConvergence
			}
		}
		switch op {
		case parametric.Union:
			keepA, keepB = !h.AInsideB, !h.BInsideA
		case parametric.Intersection:
			keepA, keepB = h.AInsideB, h.BInsideA
		case parametric.Difference:
			keepA, keepB = !h.AInsideB, h.BInsideA
			h.BReversed = keepB
		}
	}
	parts := make([]PlacedSolid, 0, 2)
	if keepA {
		for i := range h.FacesA {
			h.FacesA[i] = i
		}
		parts = append(parts, PlacedSolid{a, geom.Identity()})
	}
	if keepB {
		start := 0
		if keepA {
			start = len(ap)
		}
		for i := range h.FacesB {
			h.FacesB[i] = start + i
		}
		if h.BReversed {
			f := b.Faces()
			for i := range f {
				f[i].Reversed = !f[i].Reversed
			}
			var e error
			b, e = NewSolidContext(ctx, b.vertices, b.edges, f, b.tolerance)
			if e != nil {
				return out, SplineShellBooleanHistory{}, e
			}
		}
		parts = append(parts, PlacedSolid{b, geom.Identity()})
	}
	if len(parts) == 0 {
		return out, h, nil
	}
	if len(parts) == 1 {
		return parts[0].Solid, h, nil
	}
	out, err = AssembleShells(ctx, parts, len(ap)+len(bp))
	if err != nil {
		return Solid{}, SplineShellBooleanHistory{}, err
	}
	return out, h, nil
}

func splineShellPatchBounds(faces []splineEmbeddingFace) geom.Box {
	b, _ := faces[0].patch.Bounds()
	for _, f := range faces[1:] {
		x, _ := f.patch.Bounds()
		b, _ = b.Union(x)
	}
	return b
}

var splineWitnessDirections = [...]geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1},
	{X: 1, Y: .25, Z: .5}, {X: -1, Y: .5, Z: .25}, {X: .5, Y: 1, Z: -.25},
	{X: .25, Y: -.5, Z: 1}, {X: 1, Y: -.75, Z: -.5}, {X: -.75, Y: -1, Z: -.5}}

func classifySeparatedSplineWitness(ctx context.Context, s Solid, bounds geom.Box, p geom.Vec3, o SplineShellBooleanOptions) (bool, error) {
	if !bounds.Contains(p) {
		return false, nil
	}
	for _, d := range splineWitnessDirections[:o.MaxRays] {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		line, e := geom.NewLine(p, d)
		if e != nil {
			return false, e
		}
		o.Ray.Line.MinParameter, o.Ray.Line.MaxParameter = 0, splineWitnessFarParameter(line, bounds)
		if !finite(o.Ray.Line.MaxParameter) || o.Ray.Line.MaxParameter <= 0 {
			continue
		}
		r, e := s.SplineRay(ctx, p, d, o.Ray)
		if e != nil && !errors.Is(e, geom.ErrConvergence) {
			return false, e
		}
		if inside, known := splineMaterialRayWinding(r, o.Ray.Line.MaxParameter); known {
			return inside, nil
		}
	}
	return false, fmt.Errorf("%w: no transverse spline shell witness ray", geom.ErrConvergence)
}

func splineMaterialRayWinding(r SplineRayResult, end float64) (inside, known bool) {
	if !r.Complete {
		return false, false
	}
	count := 0
	for _, hit := range r.Hits {
		if hit.Direction == 0 || hit.ParameterMin <= 0 || hit.ParameterMax >= end {
			return false, false
		}
		count += hit.Direction
	}
	return count == 1, count == 0 || count == 1
}

// Choose a represented line parameter strictly outside an exact enclosing
// box. The result concerns the line's stored, possibly rounded, direction.
func splineWitnessFarParameter(line geom.Line, b geom.Box) float64 {
	d, p := line.Direction(), line.Origin()
	v, x := [3]float64{d.X, d.Y, d.Z}, [3]float64{p.X, p.Y, p.Z}
	lo, hi := [3]float64{b.Min.X, b.Min.Y, b.Min.Z}, [3]float64{b.Max.X, b.Max.Y, b.Max.Z}
	best := math.Inf(1)
	for i, c := range v {
		if c == 0 {
			continue
		}
		end := hi[i]
		if c < 0 {
			end = lo[i]
		}
		a := new(big.Rat).Sub(new(big.Rat).SetFloat64(end), new(big.Rat).SetFloat64(x[i]))
		a.Quo(a, new(big.Rat).SetFloat64(c))
		// A margin gives the contact isolator room at the endpoint.
		a.Mul(a, big.NewRat(2, 1))
		f, _ := a.Float64()
		if !finite(f) {
			continue
		}
		if ratInset(f).Cmp(a) <= 0 {
			f = math.Nextafter(f, math.Inf(1))
		}
		best = min(best, f)
	}
	return best
}
