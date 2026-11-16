package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/mesh"
)

const maxCircularAnnularSeedLevel = 5 // eight original arcs, at most 256 vertices

type circularSeedCap struct {
	height  *big.Rat
	centers [2][2]*big.Rat
	radii   [2]*big.Rat
}

// Re-prove the full original rational rings in their canonical model XY
// planes before using any sampled seed.
func certifyCircularAnnularSeedCap(ctx context.Context, f Face) (circularSeedCap, bool, error) {
	plane, ok := f.Surface.(geom.PlaneSurface)
	if !ok || len(f.Outer) != 4 || len(f.Holes) != 1 || len(f.Holes[0]) != 4 {
		return circularSeedCap{}, false, nil
	}
	frame := plane.Frame()
	location := frame.Origin()
	if !location.IsFinite() || location.X != 0 || location.Y != 0 ||
		frame.XAxis() != (geom.Vec3{X: 1}) || frame.YAxis() != (geom.Vec3{Y: 1}) {
		return circularSeedCap{}, false, nil
	}
	cap := circularSeedCap{height: ratInset(location.Z)}
	for i, loop := range [2][]Use{f.Outer, f.Holes[0]} {
		cap.centers[i], cap.radii[i], ok = exactSeedCircle(loop)
		if !ok {
			return circularSeedCap{}, false, nil
		}
	}
	dx := new(big.Rat).Sub(cap.centers[0][0], cap.centers[1][0])
	dy := new(big.Rat).Sub(cap.centers[0][1], cap.centers[1][1])
	separation2 := new(big.Rat).Add(dx.Mul(dx, dx), dy.Mul(dy, dy))
	gap := new(big.Rat).Sub(cap.radii[0], cap.radii[1])
	if gap.Sign() <= 0 || gap.Mul(gap, gap).Cmp(separation2) <= 0 {
		return circularSeedCap{}, false, nil
	}
	proof, err := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if err != nil {
		return circularSeedCap{}, false, err
	}
	return cap, proof.Status == trimvalidate.Certified, nil
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

// The sampled fallback resolves two cap-complete, strictly nested circular
// wall chains. The test is based on each original band, so it does not
// recurse through a solid construction or infer nesting from the end caps.
func supportsCircularAnnularSeedShell(ctx context.Context, faces []Face) (bool, error) {
	if len(faces) < 4 || len(faces) > 18 {
		return false, nil
	}
	var caps [2]circularSeedCap
	var bands [2][]geom.WrappedFrustumEmbedding
	for _, f := range faces {
		switch w := f.Surface.(type) {
		case geom.PlaneSurface:
			end := 1
			if f.Reversed {
				end = 0
			}
			if caps[end].height != nil {
				return false, nil
			}
			cap, ok, err := certifyCircularAnnularSeedCap(ctx, f)
			if err != nil || !ok {
				return false, err
			}
			caps[end] = cap
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
			side := 0
			if f.Reversed {
				side = 1
			}
			bands[side] = append(bands[side], p)
		default:
			return false, nil
		}
	}
	if caps[0].height == nil || caps[1].height == nil {
		return false, nil
	}
	var chains [2]WrappedStationLoftProof
	for side, input := range bands {
		chain, ok := circularSeedWallChain(input)
		if !ok {
			return false, geom.ErrConvergence
		}
		for end, station := range [2]int{0, len(chain.Heights) - 1} {
			cap := caps[end]
			if chain.Center[0].Cmp(cap.centers[side][0]) != 0 || chain.Center[1].Cmp(cap.centers[side][1]) != 0 ||
				chain.Heights[station].Cmp(cap.height) != 0 || chain.Radii[station].Cmp(cap.radii[side]) != 0 {
				return false, geom.ErrConvergence
			}
		}
		chains[side] = chain
	}
	if err := proveEccentricCircularStationNesting(ctx, chains[0], chains[1]); err != nil {
		return false, err
	}
	return true, nil
}

func circularSeedWallChain(bands []geom.WrappedFrustumEmbedding) (WrappedStationLoftProof, bool) {
	if len(bands) == 0 {
		return WrappedStationLoftProof{}, false
	}
	sort.Slice(bands, func(i, j int) bool { return bands[i].LowerZ.Cmp(bands[j].LowerZ) < 0 })
	first := bands[0]
	out := WrappedStationLoftProof{
		Center:  [2]*big.Rat{first.CenterX, first.CenterY},
		Heights: []*big.Rat{first.LowerZ},
		Radii:   []*big.Rat{first.LowerRadius},
	}
	for i, b := range bands {
		if b.CenterX.Cmp(out.Center[0]) != 0 || b.CenterY.Cmp(out.Center[1]) != 0 ||
			b.LowerZ.Cmp(out.Heights[i]) != 0 || b.LowerRadius.Cmp(out.Radii[i]) != 0 {
			return WrappedStationLoftProof{}, false
		}
		out.Heights = append(out.Heights, b.UpperZ)
		out.Radii = append(out.Radii, b.UpperRadius)
	}
	return out, true
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
