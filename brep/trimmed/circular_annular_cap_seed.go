package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const maxCircularAnnularSeedLevel = 5 // eight original arcs, at most 256 vertices

// Re-prove the full original rational rings before using any sampled seed.
func certifyCircularAnnularSeedCap(ctx context.Context, f Face) (bool, error) {
	if _, ok := f.Surface.(geom.PlaneSurface); !ok || len(f.Outer) != 4 ||
		len(f.Holes) != 1 || len(f.Holes[0]) != 4 {
		return false, nil
	}
	a, outer, ok := exactSeedCircle(f.Outer)
	if !ok {
		return false, nil
	}
	b, inner, ok := exactSeedCircle(f.Holes[0])
	if !ok {
		return false, nil
	}
	dx, dy := new(big.Rat).Sub(a[0], b[0]), new(big.Rat).Sub(a[1], b[1])
	separation2 := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
	gap := new(big.Rat).Sub(outer, inner)
	if gap.Sign() <= 0 || gap.Mul(gap, gap).Cmp(separation2) <= 0 {
		return false, nil
	}
	proof, err := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if err != nil {
		return false, err
	}
	return proof.Status == trimvalidate.Certified, nil
}

// Search only the already authenticated annular cap's represented polygons.
// Original vertices and trim parameterization remain the native source.
func circularAnnularSeedLevel(ctx context.Context, f Face) (int, error) {
	for level := 1; level <= maxCircularAnnularSeedLevel; level++ {
		var rings [2][]geom.Vec3
		n := 1 << level
		for i, loop := range [2][]Use{f.Outer, f.Holes[0]} {
			for _, u := range loop {
				for j := range n {
					if err := ctx.Err(); err != nil {
						return 0, err
					}
					p, err := u.uv(float64(j) / float64(n))
					if err != nil {
						return 0, err
					}
					rings[i] = append(rings[i], geom.Vec3{X: p.X, Y: p.Y})
				}
			}
		}
		if _, err := triangulateExactUVFallback(ctx, rings[:], brep.MaxHoledFaceVertices); err == nil {
			return level, nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
	return 0, parametric.ErrBudget
}

// exactSeedCircle recovers a rational circle from opposite cardinal ends and
// independently proves the full original quadratic law of each quadrant.
func exactSeedCircle(loop []Use) (center [2]*big.Rat, radius *big.Rat, ok bool) {
	for _, u := range loop {
		if !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) {
			return center, nil, false
		}
	}
	p, q := loop[0].UVStart, loop[2].UVStart
	center[0], center[1] = seedCircleMid(p.X, q.X), seedCircleMid(p.Y, q.Y)
	seen := map[[2]int]bool{}
	for _, u := range loop {
		x, y := new(big.Rat).Sub(ratInset(u.UVStart.X), center[0]), new(big.Rat).Sub(ratInset(u.UVStart.Y), center[1])
		key := [2]int{x.Sign(), y.Sign()}
		if (key[0] == 0) == (key[1] == 0) || seen[key] {
			return center, nil, false
		}
		seen[key] = true
		length := new(big.Rat).Add(x.Abs(x), y.Abs(y))
		if radius == nil {
			radius = length
		}
		if length.Cmp(radius) != 0 {
			return center, nil, false
		}
	}
	r2 := new(big.Rat).Mul(radius, radius)
	for _, u := range loop {
		q, ok := TrimValidationSegment(u)
		if !ok || q.Kind != trimvalidate.RationalQuadratic || !seedArcOnCircle(q, center, r2) {
			return center, nil, false
		}
	}
	return center, radius, true
}

func seedCircleMid(a, b float64) *big.Rat {
	return new(big.Rat).Quo(new(big.Rat).Add(ratInset(a), ratInset(b)), big.NewRat(2, 1))
}

func seedArcOnCircle(arc trimvalidate.Segment, center [2]*big.Rat, radius2 *big.Rat) bool {
	// Check every coefficient of X²+Y²-r²W² in degree-four Bernstein
	// form. The common output binomial factor cannot affect a zero.
	coefficient := [3]int64{1, 2, 1}
	var x, y, w [3]*big.Rat
	for i, p := range [3]geom.Vec2{arc.Start, arc.Control, arc.End} {
		if !uvFinite(p) || !finite(arc.Weights[i]) || arc.Weights[i] <= 0 {
			return false
		}
		w[i] = new(big.Rat).Mul(ratInset(arc.Weights[i]), big.NewRat(coefficient[i], 1))
		x[i] = new(big.Rat).Mul(new(big.Rat).Sub(ratInset(p.X), center[0]), w[i])
		y[i] = new(big.Rat).Mul(new(big.Rat).Sub(ratInset(p.Y), center[1]), w[i])
	}
	var amounts [5]big.Rat
	for i := range 3 {
		for j := range 3 {
			a := new(big.Rat).Add(new(big.Rat).Mul(x[i], x[j]), new(big.Rat).Mul(y[i], y[j]))
			a.Sub(a, new(big.Rat).Mul(radius2, new(big.Rat).Mul(w[i], w[j])))
			amounts[i+j].Add(&amounts[i+j], a)
		}
	}
	for i := range amounts {
		if amounts[i].Sign() != 0 {
			return false
		}
	}
	return true
}

// The sampled fallback is restricted to two circular annuli and a bounded
// collection of authenticated rectangular circular-frustum wall charts.
func supportsCircularAnnularSeedShell(ctx context.Context, faces []Face) (bool, error) {
	if len(faces) < 4 || len(faces) > 18 {
		return false, nil
	}
	caps := 0
	for _, f := range faces {
		switch w := f.Surface.(type) {
		case geom.PlaneSurface:
			ok, err := certifyCircularAnnularSeedCap(ctx, f)
			if err != nil || !ok {
				return false, err
			}
			caps++
		case *geom.WrappedSurfacePatch:
			if len(f.Outer) != 10 {
				return false, nil
			}
			if _, ok := straightFullWrappedRectangle(w, f); !ok {
				return false, nil
			}
			p, err := geom.CertifyWrappedFrustumEmbedding(ctx, w, geom.SurfacePatchOptions{})
			if err != nil || !p.Circular {
				return false, err
			}
		default:
			return false, nil
		}
	}
	return caps == 2, nil
}

func (s Solid) annularRimEdges() []bool {
	rim := make([]bool, len(s.edges))
	for _, f := range s.faces {
		if _, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			for _, u := range f.Outer {
				if u.UVStart.Y == u.UVEnd.Y {
					rim[u.Edge] = true
				}
			}
		}
	}
	return rim
}

func (s Solid) tessellateCircularAnnularSeeds(ctx context.Context, o parametric.TessellationOptions) (mesh.Mesh, error) {
	if o.AnalyticBound {
		return mesh.Mesh{}, parametric.ErrDeflectionUnsupported
	}
	rim := s.annularRimEdges()
	for level := s.minimumLevel; level <= maxCircularAnnularSeedLevel && 1<<level <= o.MaxSubdivisions; level++ {
		count := 0
		for fi, f := range s.faces {
			vertices := 0
			for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
				for _, u := range loop {
					n := 1
					if rim[u.Edge] {
						n = 1 << level
					}
					vertices += n
				}
			}
			triangles := vertices + 2*len(f.Holes) - 2
			if s.splitSeeds[fi] {
				triangles *= 12
			}
			if triangles > o.MaxTriangles-count {
				return mesh.Mesh{}, parametric.ErrBudget
			}
			count += triangles
		}
		m, err := s.refine(ctx, level, o.Deflection, o.Angle)
		if err != errRefine && err != errTrimFold {
			return m, err
		}
	}
	return mesh.Mesh{}, parametric.ErrBudget
}
