package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// nativePolygonCapRationalFaces uses existing rational-endpoint native recipes.
// Only UV displays are rounded; model vertices must still be exactly binary
// representable. All original chart points and whole p-curves stay exact.
func nativePolygonCapRationalFaces(ctx context.Context, r *AffineImprintFaceFragments, o geom.SurfacePatchEvaluationOptions) ([]SewFace, error) {
	if ctx == nil || r == nil {
		return nil, geom.ErrInvalid
	}
	maximum := o.MaxOperations
	if maximum == 0 {
		maximum = 8000000
	}
	if maximum < 1 || maximum > 128000000 {
		return nil, geom.ErrInvalid
	}
	work := 0
	patch := r.source.charts[r.owner][r.face].patch
	points := make([]geom.Vec3, len(r.uv))
	for i, uv := range r.uv {
		if work >= maximum {
			return nil, geom.ErrConvergence
		}
		o.MaxOperations = maximum - work
		value, e := patch.EvaluateRational(ctx, uv[0], uv[1], o)
		if e != nil {
			return nil, e
		}
		work += value.ExactOperations
		xyz := [3]float64{}
		for k, q := range value.Point {
			v, exact := q.Float64()
			if !exact {
				return nil, geom.ErrUnrepresentable
			}
			xyz[k] = v
		}
		points[i] = geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	}
	original := r.source.sources[r.owner].faces[r.face]
	out := make([]SewFace, 0, len(r.cycles))
	for _, cycle := range r.cycles {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		part := SewFace{Face: Face{Surface: original.Surface, Reversed: original.Reversed}}
		for i, use := range cycle.Uses {
			curve, e := quadraticLineWithEndpoints(r.uv[use.Start], r.uv[use.End], true)
			if e != nil {
				return nil, e
			}
			_, display := curve.Endpoints()
			part.Vertices = append(part.Vertices, points[use.Start])
			part.Edges = append(part.Edges, parametric.Edge{Start: i, End: (i + 1) % len(cycle.Uses), Curve: surfaceTrimCurve{surface: original.Surface, a: display[0], b: display[1], curve: curve}, Range: parametric.Interval{Max: 1}})
			part.Face.Outer = append(part.Face.Outer, Use{Edge: i, UVStart: display[0], UVEnd: display[1], Curve: curve})
		}
		out = append(out, part)
	}
	return out, nil
}
