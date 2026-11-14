package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ExactQuadraticSewFace constructs an independently indexed face on an exact
// spline support. Each model edge retains its original exact quadratic UV
// composition and rational model endpoints; float vertices are display values.
// The loop must have distinct display vertices and exactly connected UV ends.
// This constructor does not prove embedding, simplicity or material orientation.
func ExactQuadraticSewFace(ctx context.Context, surface geom.Surface, outer []ExactQuadraticUV, reversed bool, o geom.SurfacePatchOptions) (SewFace, error) {
	if ctx == nil || len(outer) < 3 || len(outer) > 256 {
		return SewFace{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SewFace{}, e
	}
	patch, e := exactFaceSupportPatch(Face{Surface: surface}, o)
	if e != nil {
		return SewFace{}, e
	}
	part := SewFace{Face: Face{Surface: surface, Reversed: reversed}}
	curves := make([]ExactQuadraticUV, len(outer))
	for i, q := range outer {
		curves[i], e = NewRationalEndpointQuadraticUV(q.HomogeneousControls())
		if e != nil {
			return SewFace{}, e
		}
	}
	seen := map[geom.Vec3]bool{}
	for i, q := range curves {
		if e := ctx.Err(); e != nil {
			return SewFace{}, e
		}
		ends, xy := q.Endpoints()
		next, _ := curves[(i+1)%len(curves)].Endpoints()
		if ends[1][0].Cmp(next[0][0]) != 0 || ends[1][1].Cmp(next[0][1]) != 0 {
			return SewFace{}, geom.ErrInvalid
		}
		j, e := patch.EvaluateRational(ctx, ends[0][0], ends[0][1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
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
		part.Face.Outer = append(part.Face.Outer, Use{Edge: i, UVStart: xy[0], UVEnd: xy[1], Curve: q})
		part.Edges = append(part.Edges, parametric.Edge{Start: i, End: (i + 1) % len(curves), Curve: surfaceTrimCurve{surface, xy[0], xy[1], q}, Range: parametric.Interval{Max: 1}})
	}
	return part, nil
}

// SewRationalQuadratic proves exact equality of candidate shared vertices
// before the existing continuous edge sewing. Distinct rational points with
// the same display coordinates are rejected, never welded. This bounded entry
// is for at most 128 faces and 4096 uses; it makes no global embedding claim.
func SewRationalQuadratic(ctx context.Context, parts []SewFace, o SewOptions) (Solid, SewHistory, error) {
	if ctx == nil {
		return Solid{}, SewHistory{}, geom.ErrInvalid
	}
	if o.MaxFaces == 0 {
		o.MaxFaces = 128
	}
	if o.MaxUses == 0 {
		o.MaxUses = 4096
	}
	if o.MaxFaces < 1 || o.MaxFaces > 128 || o.MaxUses < 1 || o.MaxUses > 4096 || len(parts) > o.MaxFaces {
		return Solid{}, SewHistory{}, parametric.ErrBudget
	}
	edges, vertices := 0, 0
	for _, p := range parts {
		if e := ctx.Err(); e != nil {
			return Solid{}, SewHistory{}, e
		}
		edges += len(p.Edges)
		vertices += len(p.Vertices)
		if edges > o.MaxUses || vertices > 2*o.MaxUses {
			return Solid{}, SewHistory{}, parametric.ErrBudget
		}
	}
	if e := proveRationalSplitVertexIdentity(ctx, parts); e != nil {
		return Solid{}, SewHistory{}, e
	}
	return Sew(ctx, parts, o)
}
