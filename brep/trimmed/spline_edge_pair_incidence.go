package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// SplineEdgePairFace identifies one incident chart and the exact coordinates
// of an interior point of its original coedge.
type SplineEdgePairFace struct {
	Face       int
	Side       geom.SurfaceEdge
	Parameters [2]*big.Rat
}

// SplineEdgePairGerm describes the local intersection of a pair of original
// face supports at a simultaneous edge event. Inward says one regular
// half-branch enters both face interiors. Direction is a proportional
// four-coordinate tangent in original chart units, oriented into face A.
type SplineEdgePairGerm struct {
	Faces     [2]SplineEdgePairFace
	Direction [4]*big.Rat
	Inward    bool
}

// SplineEdgePairIncidence retains all four possible pairings of the two
// original coedges on each operand. Complete is a local certificate only.
type SplineEdgePairIncidence struct {
	Point    [3]*big.Rat
	Germs    []SplineEdgePairGerm
	Complete bool
}

// CertifySplineEdgePairIncidence proves local face continuation at a proposed
// simultaneous contact of two original spline edges. The anchors must be
// strictly interior, with exact continuous agreement to both of their
// rectangular spline coedges. The model point is independently evaluated
// in all four charts. Regularity and the exact 3-by-4 intersection tangent
// decide which face-pair germs enter both trim interiors. Tangencies,
// nonregular charts and contact at an interior knot remain unresolved.
// This does not enumerate the global sections or prove shell embedding.
func CertifySplineEdgePairIncidence(ctx context.Context, a, b Solid, anchors [2]EdgeAnchor,
	o geom.TubeEndpointEdgeOptions) (SplineEdgePairIncidence, error) {
	var result SplineEdgePairIncidence
	if ctx == nil || o.MaxOperations < 0 || o.MaxCoefficientBits != 0 && o.MaxCoefficientBits < 64 {
		return result, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	x, e := splineEdgePairCoedges(ctx, a, anchors[0], o)
	if e != nil {
		return result, e
	}
	y, e := splineEdgePairCoedges(ctx, b, anchors[1], o)
	if e != nil {
		return result, e
	}
	point := x[0].jet.Point
	for _, p := range []splineEdgePairCoedge{x[1], y[0], y[1]} {
		if !exactPointCoordinatesEqual(point, p.jet.Point) {
			return result, geom.ErrConvergence
		}
	}
	for _, u := range x {
		for _, v := range y {
			if e := ctx.Err(); e != nil {
				return SplineEdgePairIncidence{}, e
			}
			columns := [4][3]*big.Rat{u.jet.Du, u.jet.Dv, v.jet.Du, v.jet.Dv}
			for j := 2; j < 4; j++ {
				for k := range columns[j] {
					columns[j][k] = new(big.Rat).Neg(columns[j][k])
				}
			}
			g := SplineEdgePairGerm{Faces: [2]SplineEdgePairFace{u.face, v.face}}
			for i := range g.Direction {
				c := make([][3]*big.Rat, 0, 3)
				for j, col := range columns {
					if j != i {
						c = append(c, col)
					}
				}
				r, e := splineIncidenceDeterminant(c[0], c[1], c[2], o.MaxCoefficientBits)
				if e != nil {
					return SplineEdgePairIncidence{}, e
				}
				if i%2 == 1 {
					r.Neg(r)
				}
				g.Direction[i] = r
			}
			fixed := func(side geom.SurfaceEdge) int {
				if side >= geom.SurfaceEdgeVMin {
					return 1
				}
				return 0
			}
			sense := func(r *big.Rat, side geom.SurfaceEdge) int {
				sign := r.Sign()
				if side == geom.SurfaceEdgeUMax || side == geom.SurfaceEdgeVMax {
					sign = -sign
				}
				return sign
			}
			da := sense(g.Direction[fixed(u.face.Side)], u.face.Side)
			db := sense(g.Direction[2+fixed(v.face.Side)], v.face.Side)
			if da == 0 || db == 0 {
				return SplineEdgePairIncidence{}, geom.ErrConvergence
			}
			// A nonzero minor omitting a coordinate on A (and one on
			// B) proves rank three and rank two on each support chart.
			if da < 0 {
				for _, r := range g.Direction {
					r.Neg(r)
				}
			}
			g.Inward = da == db
			result.Germs = append(result.Germs, g)
		}
	}
	result.Point, result.Complete = point, true
	return result, nil
}

type splineEdgePairCoedge struct {
	face SplineEdgePairFace
	jet  geom.RationalSurfacePatchJet
}

func splineEdgePairCoedges(ctx context.Context, s Solid, anchor EdgeAnchor, o geom.TubeEndpointEdgeOptions) ([]splineEdgePairCoedge, error) {
	if anchor.Edge < 0 || anchor.Edge >= len(s.edges) || !finite(anchor.Parameter) {
		return nil, geom.ErrInvalid
	}
	edge := s.edges[anchor.Edge]
	if anchor.Parameter <= edge.Range.Min || anchor.Parameter >= edge.Range.Max {
		return nil, geom.ErrDomain
	}
	l := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	t := new(big.Rat).Quo(new(big.Rat).Sub(ratInset(anchor.Parameter), ratInset(edge.Range.Min)),
		new(big.Rat).Sub(ratInset(edge.Range.Max), ratInset(edge.Range.Min)))
	var result []splineEdgePairCoedge
	for id, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for _, use := range f.Outer {
			if use.Edge != anchor.Edge {
				continue
			}
			base, ok := f.Surface.(*geom.BSplineSurface)
			if !ok || len(f.Outer) != 4 || len(f.Holes) != 0 || use.Curve != nil || len(result) != 0 && result[0].face.Face == id {
				return nil, geom.ErrConvergence
			}
			d, e := faceUVBounds(f)
			if e != nil {
				return nil, e
			}
			corners := insetCorners(d)
			sides := [4]geom.SurfaceEdge{geom.SurfaceEdgeVMin, geom.SurfaceEdgeUMax, geom.SurfaceEdgeVMax, geom.SurfaceEdgeUMin}
			var side geom.SurfaceEdge
			for i, boundary := range f.Outer {
				if boundary.Curve != nil || boundary.UVEnd != f.Outer[(i+1)%4].UVStart {
					return nil, geom.ErrConvergence
				}
				j := -1
				for k, q := range corners {
					if boundary.UVStart == q && boundary.UVEnd == corners[(k+1)%4] {
						j = k
						break
					}
				}
				if j < 0 {
					return nil, geom.ErrConvergence
				}
				if boundary.Edge == anchor.Edge {
					side = sides[j]
				}
			}
			if !sameSplineTrim(edge, f.Surface, use) {
				ok, e := compareSplineEdgeUse(ctx, edge, f.Surface, use, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
				if e != nil || !ok {
					if e == nil {
						e = geom.ErrConvergence
					}
					return nil, e
				}
			}
			alpha := new(big.Rat).Set(t)
			if use.Reversed {
				alpha.Sub(big.NewRat(1, 1), alpha)
			}
			at := func(x, y float64) *big.Rat {
				a, b := ratInset(x), ratInset(y)
				return a.Add(a, b.Mul(b.Sub(b, a), alpha))
			}
			uv := [2]*big.Rat{at(use.UVStart.X, use.UVEnd.X), at(use.UVStart.Y, use.UVEnd.Y)}
			p, e := geom.NewSurfacePatch(base, l)
			if e != nil {
				return nil, e
			}
			p, e = p.Trim(d)
			if e != nil {
				return nil, e
			}
			// An interior knot at the event may have two derivatives.
			data := base.Data()
			for j, ks := range [2][]float64{data.KnotsU, data.KnotsV} {
				for _, k := range ks {
					min, max := d.UMin, d.UMax
					if j == 1 {
						min, max = d.VMin, d.VMax
					}
					if k > min && k < max && uv[j].Cmp(ratInset(k)) == 0 {
						return nil, geom.ErrConvergence
					}
				}
			}
			jet, e := p.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			if e != nil {
				return nil, e
			}
			result = append(result, splineEdgePairCoedge{SplineEdgePairFace{id, side, uv}, jet})
		}
	}
	if len(result) != 2 {
		return nil, geom.ErrConvergence
	}
	return result, nil
}

func splineIncidenceDeterminant(a, b, c [3]*big.Rat, limit int) (*big.Rat, error) {
	s := new(big.Rat)
	check := func(r *big.Rat) bool { return r.Num().BitLen() <= limit && r.Denom().BitLen() <= limit }
	for i := range a {
		j, k := (i+1)%3, (i+2)%3
		p, q := new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j])
		if !check(p) || !check(q) || !check(p.Sub(p, q)) || !check(p.Mul(p, a[i])) || !check(s.Add(s, p)) {
			return nil, geom.ErrConvergence
		}
	}
	return s, nil
}
