package ellipticprism

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Annulus constructs a strictly nested concentric aligned elliptical extrusion.
// Returned rim IDs are [outer/inner][lower/upper][quarter].
func Annulus(ctx context.Context, frame geom.Frame, a, b, ia, ib, height, tol float64) (trimmed.Solid, [2][2][4]int, error) {
	bad := func(e error) (trimmed.Solid, [2][2][4]int, error) { return trimmed.Solid{}, [2][2][4]int{}, e }
	if !finite(ia) || !finite(ib) || ia <= ib || ib <= 0 || ia >= a || ib >= b {
		return bad(geom.ErrInvalid)
	}
	outer, e := Parts(ctx, frame, a, b, height, tol)
	if e != nil {
		return bad(e)
	}
	inner, e := Parts(ctx, frame, ia, ib, height, tol)
	if e != nil {
		return bad(e)
	}
	for k := 0; k < 2; k++ {
		nv, ne := len(outer[k].Vertices), len(outer[k].Edges)
		hole := []trimmed.Use{}
		for q := 3; q >= 0; q-- {
			u := inner[k].Face.Outer[q]
			u.Edge += ne
			u.Reversed = !u.Reversed
			u.UVStart, u.UVEnd = u.UVEnd, u.UVStart
			c, ok := u.Curve.(trimmed.SplineUV)
			if !ok {
				return bad(geom.ErrInvalid)
			}
			c.Reversed = !c.Reversed
			u.Curve = c
			hole = append(hole, u)
		}
		for i := range inner[k].Edges {
			inner[k].Edges[i].Start += nv
			inner[k].Edges[i].End += nv
		}
		outer[k].Vertices = append(outer[k].Vertices, inner[k].Vertices...)
		outer[k].Edges = append(outer[k].Edges, inner[k].Edges...)
		outer[k].Face.Holes = [][]trimmed.Use{hole}
	}
	for _, p := range inner[2:] {
		p.Face.Reversed = !p.Face.Reversed
		outer = append(outer, p)
	}
	s, h, e := trimmed.Sew(ctx, outer, trimmed.SewOptions{Tolerance: tol, MaxFaces: 10, MaxUses: 48, Proof: edgecheck.Options{}})
	if e != nil {
		return bad(e)
	}
	var rims [2][2][4]int
	for w := 0; w < 2; w++ {
		for k := 0; k < 2; k++ {
			for q := 0; q < 4; q++ {
				rims[w][k][q] = h.Edges[k][4*w+q]
			}
		}
	}
	return s, rims, nil
}
