package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Rational polygon vertices must be carried by exact incident trims. Do not
// substitute the rounded UV edge for the source line on the chart.
func rationalQuadraticSewFace(ctx context.Context, face Face, patch *geom.SurfacePatch, points [][2]*big.Rat, o geom.SurfacePatchOptions) (SewFace, error) {
	part := SewFace{Face: Face{Surface: face.Surface, Reversed: face.Reversed}}
	seen := map[geom.Vec3]bool{}
	for _, p := range points {
		j, e := patch.EvaluateRational(ctx, p[0], p[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return SewFace{}, e
		}
		display, e := rationalDisplayPoint(j.Point)
		if e != nil {
			return SewFace{}, e
		}
		if seen[display] {
			return SewFace{}, geom.ErrUnrepresentable
		}
		seen[display] = true
		part.Vertices = append(part.Vertices, display)
	}
	for i, a := range points {
		k := (i + 1) % len(points)
		q, e := quadraticLineWithEndpoints(a, points[k], true)
		if e != nil {
			return SewFace{}, e
		}
		_, xy := q.Endpoints()
		u := Use{Edge: i, UVStart: xy[0], UVEnd: xy[1], Curve: q}
		part.Face.Outer = append(part.Face.Outer, u)
		part.Edges = append(part.Edges, parametric.Edge{Start: i, End: k, Curve: surfaceTrimCurve{face.Surface, xy[0], xy[1], q}, Range: parametric.Interval{Max: 1}})
	}
	return part, nil
}

// Sew matches display coordinates only as a candidate key. In the rational
// path each candidate vertex association must already have exact identity
// before it can become shared topology.
func proveRationalSplitVertexIdentity(ctx context.Context, parts []SewFace) error {
	seen := map[geom.Vec3][3]*big.Rat{}
	for _, f := range parts {
		x, e := collectRationalVertices(ctx, f.Vertices, f.Edges, true)
		if e != nil {
			return e
		}
		exact := map[int][3]*big.Rat{}
		for _, v := range x {
			exact[v.Vertex] = v.Point
		}
		for i, p := range f.Vertices {
			v, ok := exact[i]
			if !ok {
				v = [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
			}
			if old, ok := seen[p]; ok && !sameRationalVertex(v, old) {
				return geom.ErrUnrepresentable
			}
			seen[p] = v
		}
	}
	return ctx.Err()
}
