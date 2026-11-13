package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RectangularSplineFace makes a standalone full-domain spline face with
// independent local indices, suitable for Sew. Its four model edges are
// evaluated as exact isocurves of the original support. Reversed changes
// the oriented face normal while retaining the CCW UV wire. The caller
// must establish global embedding before combining unrelated patches.
func RectangularSplineFace(s *geom.BSplineSurface, reversed bool) (SewFace, error) {
	return SplineFaceRectangle(s, geom.ParameterRectangle{}, reversed)
}

// SplineFaceRectangle constructs a standalone bounded rectangular part of
// a spline. A zero domain selects its full active knot range.
func SplineFaceRectangle(s *geom.BSplineSurface, domain geom.ParameterRectangle, reversed bool) (SewFace, error) {
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		return SewFace{}, e
	}
	if domain != (geom.ParameterRectangle{}) {
		p, e = p.Trim(domain)
		if e != nil {
			return SewFace{}, e
		}
	}
	d := p.Domain()
	uv := [4]geom.Vec2{{X: d.UMin, Y: d.VMin}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}, {X: d.UMin, Y: d.VMax}}
	out := SewFace{Face: Face{Surface: s, Reversed: reversed}}
	seen := map[geom.Vec3]bool{}
	for i, a := range uv {
		b := uv[(i+1)%4]
		q, e := p.Eval(a.X, a.Y)
		if e != nil {
			return SewFace{}, e
		}
		if seen[q] {
			return SewFace{}, geom.ErrDegenerate
		}
		seen[q] = true
		out.Vertices = append(out.Vertices, q)
		out.Edges = append(out.Edges, parametric.Edge{Start: i, End: (i + 1) % 4,
			Curve: surfaceTrimCurve{s, a, b, nil}, Range: parametric.Interval{Max: 1}})
		out.Face.Outer = append(out.Face.Outer, Use{Edge: i, UVStart: a, UVEnd: b})
	}
	return out, nil
}

// SewSplineRectangles constructs and sews a collection of independently
// parameterized clamped spline rectangles. Boundary curves may use different
// degrees and knot vectors as long as their normalized models agree exactly.
func SewSplineRectangles(ctx context.Context, surfaces []*geom.BSplineSurface, domains []geom.ParameterRectangle, reversed []bool, o SewOptions) (Solid, SewHistory, error) {
	if ctx == nil || len(domains) != len(surfaces) || len(reversed) != len(surfaces) {
		return Solid{}, SewHistory{}, geom.ErrInvalid
	}
	limit := o.MaxFaces
	if limit == 0 {
		limit = 128
	}
	if len(surfaces) > limit || len(surfaces) > 4096 {
		return Solid{}, SewHistory{}, parametric.ErrBudget
	}
	parts := make([]SewFace, len(surfaces))
	for i, s := range surfaces {
		if e := ctx.Err(); e != nil {
			return Solid{}, SewHistory{}, e
		}
		f, e := SplineFaceRectangle(s, domains[i], reversed[i])
		if e != nil {
			return Solid{}, SewHistory{}, e
		}
		parts[i] = f
	}
	return Sew(ctx, parts, o)
}
