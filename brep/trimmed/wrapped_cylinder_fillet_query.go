package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedCylinderFilletProof describes the original stock and exact
// retained rim radii, with the corresponding face IDs in the input solid.
type WrappedCylinderFilletProof struct {
	Center         [2]*big.Rat
	LowerZ, UpperZ *big.Rat
	Radius         *big.Rat
	Fillets        [2]*big.Rat
	Lateral        int
	Caps, Blends   [2]int
	Complete       bool
}

// ValidateWrappedCylinderRimFillet recovers the complete canonical blend
// boundary by comparing the original charts and exact topology against a
// newly constructed rolling-circle reference. Reindexing is permitted.
func (s Solid) ValidateWrappedCylinderRimFillet(ctx context.Context, o geom.SurfacePatchOptions) (WrappedCylinderFilletProof, error) {
	bad := WrappedCylinderFilletProof{}
	if ctx == nil {
		return bad, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return bad, e
	}
	if len(s.faces) != 4 && len(s.faces) != 5 {
		return bad, geom.ErrConvergence
	}
	var side *geom.WrappedSurfacePatch
	var heights []float64
	for _, f := range s.faces {
		if len(f.Holes) != 0 {
			return bad, geom.ErrConvergence
		}
		switch w := f.Surface.(type) {
		case *geom.WrappedSurfacePatch:
			u, v := w.FundamentalPatch().ControlShape()
			if u != 9 || f.Reversed {
				return bad, geom.ErrConvergence
			}
			if v == 2 {
				if side != nil {
					return bad, geom.ErrConvergence
				}
				side = w
			} else if v != 3 {
				return bad, geom.ErrConvergence
			}
		case geom.PlaneSurface:
			heights = append(heights, w.Frame().Origin().Z)
		default:
			return bad, geom.ErrConvergence
		}
	}
	if side == nil || len(heights) != 2 {
		return bad, geom.ErrConvergence
	}
	sort.Float64s(heights)
	p, e := geom.CertifyWrappedCylinderEmbedding(ctx, side, o)
	if e != nil || !p.Circular {
		if e == nil {
			e = geom.ErrConvergence
		}
		return bad, e
	}
	z := [2]*big.Rat{ratInset(heights[0]), ratInset(heights[1])}
	r := [2]*big.Rat{new(big.Rat).Sub(p.LowerZ, z[0]), new(big.Rat).Sub(z[1], p.UpperZ)}
	var rf [2]float64
	for i, q := range r {
		if q.Sign() < 0 {
			return bad, geom.ErrConvergence
		}
		rf[i], e = geom.RepresentedExactScalar(q)
		if e != nil {
			return bad, e
		}
	}
	w, e := geom.ReheightWrappedCylinder(ctx, side, z[0], z[1], o)
	if e != nil {
		return bad, e
	}
	stock, _, e := NewWrappedCylinder(ctx, w, s.tolerance, o)
	if e != nil {
		return bad, e
	}
	ref, h, e := FilletWrappedCylinderRims(ctx, stock, rf[0], rf[1], o)
	if e != nil {
		return bad, e
	}
	if len(ref.faces) != len(s.faces) || len(ref.edges) != len(s.edges) || len(ref.vertices) != len(s.vertices) {
		return bad, geom.ErrConvergence
	}
	faceIDs := make([]int, len(ref.faces))
	seen := make([]bool, len(s.faces))
	for i, f := range ref.faces {
		faceIDs[i] = -1
		for j, g := range s.faces {
			if seen[j] || f.Reversed != g.Reversed || len(f.Outer) != len(g.Outer) {
				continue
			}
			same := f.Surface == g.Surface
			if !same {
				a, aok := f.Surface.(*geom.WrappedSurfacePatch)
				b, bok := g.Surface.(*geom.WrappedSurfacePatch)
				if aok && bok && a.Domain() == b.Domain() {
					q, e := geom.CompareExactSurfaces(ctx, a.FundamentalPatch(), b.FundamentalPatch(), geom.ExactSurfaceComparisonOptions{
						MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
					})
					if e != nil {
						return bad, e
					}
					same = q.Complete && q.Equal
				}
			}
			if same {
				if faceIDs[i] >= 0 {
					return bad, geom.ErrConvergence
				}
				faceIDs[i] = j
			}
		}
		if faceIDs[i] < 0 {
			return bad, geom.ErrConvergence
		}
		seen[faceIDs[i]] = true
		ref.faces[i].Surface = s.faces[faceIDs[i]].Surface
	}
	if _, e := matchWrappedStationLoft(ctx, s, ref, faceIDs, o); e != nil {
		return bad, e
	}
	out := WrappedCylinderFilletProof{Center: [2]*big.Rat{p.CenterX, p.CenterY}, Radius: p.Radius,
		LowerZ: z[0], UpperZ: z[1], Fillets: r, Lateral: faceIDs[h.Lateral], Caps: [2]int{faceIDs[h.Caps[0]], faceIDs[h.Caps[1]]},
		Blends: [2]int{-1, -1}, Complete: true}
	for i, id := range h.Blends {
		if id >= 0 {
			out.Blends[i] = faceIDs[id]
		}
	}
	return out, nil
}

// LocateWrappedCylinderRimFillet exactly classifies a represented point
// relative to the capped, selectively rounded circular cylinder. In the
// blend slabs the radial square root is eliminated after sign tests.
func (s Solid) LocateWrappedCylinderRimFillet(ctx context.Context, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	if !point.IsFinite() {
		return parametric.Outside, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return parametric.Outside, e
	}
	z := ratInset(point.Z)
	if z.Cmp(p.LowerZ) < 0 || z.Cmp(p.UpperZ) > 0 {
		return parametric.Outside, nil
	}
	x, y := new(big.Rat).Sub(ratInset(point.X), p.Center[0]), new(big.Rat).Sub(ratInset(point.Y), p.Center[1])
	q := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
	square := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	var radial int
	for i, rim := range []*big.Rat{p.LowerZ, p.UpperZ} {
		r := p.Fillets[i]
		z0 := new(big.Rat)
		if i == 0 {
			z0.Add(rim, r)
		} else {
			z0.Sub(rim, r)
		}
		if r.Sign() == 0 || i == 0 && z.Cmp(z0) >= 0 || i == 1 && z.Cmp(z0) <= 0 {
			continue
		}
		major := new(big.Rat).Sub(p.Radius, r)
		m2 := square(major)
		k := new(big.Rat).Sub(square(r), square(new(big.Rat).Sub(z, z0)))
		// rho <= major+sqrt(k). If q <= major²+k the inequality
		// is strict except at k=0 and q=major². Otherwise compare
		// (q-major²-k)² with 4*major²*k.
		d := new(big.Rat).Sub(new(big.Rat).Sub(q, m2), k)
		if d.Sign() <= 0 {
			radial = -1
			if k.Sign() == 0 && d.Sign() == 0 {
				radial = 0
			}
		} else {
			radial = square(d).Cmp(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(m2, k)))
		}
		return classifyWrappedFilletPoint(radial, z.Cmp(rim) == 0), nil
	}
	radial = q.Cmp(square(p.Radius))
	return classifyWrappedFilletPoint(radial, z.Cmp(p.LowerZ) == 0 || z.Cmp(p.UpperZ) == 0), nil
}

func classifyWrappedFilletPoint(radial int, cap bool) parametric.Location {
	if radial > 0 {
		return parametric.Outside
	}
	if radial == 0 || cap {
		return parametric.Boundary
	}
	return parametric.Inside
}

// IntersectWrappedCylinderRimFilletLine proves all contacts with the validated
// filleted cylinder. Exact shared rims return all incident original face IDs;
// an interval on a cap or a lateral generator is reported explicitly.
func (s Solid) IntersectWrappedCylinderRimFilletLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64,
	o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	var result WrappedLoftLineIntersections
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o.Surface)
	if e != nil {
		return result, e
	}
	q, e := geom.IntersectCircularRoundedCylinderLine(ctx, geom.CircularRoundedCylinderProfile{
		Center: p.Center, Height: [2]*big.Rat{p.LowerZ, p.UpperZ}, Radius: p.Radius, Blend: p.Fillets,
	}, origin, direction, minT, maxT, o)
	if e != nil {
		return result, e
	}
	parts := [...]struct {
		part geom.CylinderBoundaryPart
		face int
	}{
		{geom.CylinderLateral, p.Lateral},
		{geom.CylinderLowerCap, p.Caps[0]}, {geom.CylinderUpperCap, p.Caps[1]},
		{geom.CylinderLowerBlend, p.Blends[0]}, {geom.CylinderUpperBlend, p.Blends[1]},
	}
	for _, h := range q.Hits {
		x := WrappedLoftLineHit{Parameter: h.Parameter, Parts: h.Parts}
		for _, f := range parts {
			if f.face >= 0 && h.Parts&f.part != 0 {
				x.Faces = append(x.Faces, f.face)
			}
		}
		sort.Ints(x.Faces)
		result.Hits = append(result.Hits, x)
	}
	for _, h := range q.Overlaps {
		for _, f := range parts {
			if f.face >= 0 && h.Part == f.part {
				result.Overlaps = append(result.Overlaps, WrappedLoftLineOverlap{h.Start, h.End, f.face, h.Part})
			}
		}
	}
	return result, nil
}

// BoundWrappedCylinderRimFilletDistance encloses global squared distance
// to the complete validated boundary and retains the witness face index.
func (s Solid) BoundWrappedCylinderRimFilletDistance(ctx context.Context, point geom.Vec3,
	o geom.FrustumDistanceOptions) (geom.RoundedCylinderDistanceBounds, int, error) {
	p, e := s.ValidateWrappedCylinderRimFillet(ctx, o.Surface)
	if e != nil {
		return geom.RoundedCylinderDistanceBounds{}, -1, e
	}
	r, e := geom.BoundPointCircularRoundedCylinderDistance(ctx, geom.CircularRoundedCylinderProfile{
		Center: p.Center, Height: [2]*big.Rat{p.LowerZ, p.UpperZ}, Radius: p.Radius, Blend: p.Fillets,
	}, point, o)
	if e != nil {
		return r, -1, e
	}
	f := map[geom.CylinderBoundaryPart]int{
		geom.CylinderLateral:  p.Lateral,
		geom.CylinderLowerCap: p.Caps[0], geom.CylinderUpperCap: p.Caps[1],
		geom.CylinderLowerBlend: p.Blends[0], geom.CylinderUpperBlend: p.Blends[1],
	}
	return r, f[r.Witness.Part], nil
}
