//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ObliqueTorusCavity constructs a native two-shell cavity using sixteen exact
// rational torus charts and the original cylindrical stock. The returned query
// binds the same original inputs; no normalized oblique frame is constructed.
// All torus seams are independently compared continuously before sewing, and
// their nonbinary corners retain exact native rational endpoint identities.
// Strict conservative containment is inherited from the analytic query.
// Cylinder height must be exactly representable. Per-chart proof limits apply
// to sixteen charts, followed by the bounded Sew proof (64 uses).
func ObliqueTorusCavity(ctx context.Context, R, r, c, lower, upper float64, pose [3][4]*big.Rat, tolerance float64, o geom.SurfacePatchOptions) (Solid, geom.ObliqueCylinderTorusCavityMaterial, error) {
	bad := func(e error) (Solid, geom.ObliqueCylinderTorusCavityMaterial, error) {
		return Solid{}, geom.ObliqueCylinderTorusCavityMaterial{}, e
	}
	if !finite(tolerance) || tolerance <= 0 {
		return bad(geom.ErrInvalid)
	}
	source, e := geom.NewObliqueCylinderTorusCavityMaterial(ctx, R, r, c, lower, upper, pose, o)
	if e != nil {
		return bad(e)
	}
	height := new(big.Rat).Sub(new(big.Rat).SetFloat64(upper), new(big.Rat).SetFloat64(lower))
	h, exact := height.Float64()
	if !exact {
		return bad(geom.ErrUnrepresentable)
	}
	frame, e := geom.NewFrame(geom.Vec3{Z: lower}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return bad(e)
	}
	cylinder, e := parametric.Cylinder(frame, c, h)
	if e != nil {
		return bad(e)
	}
	outer, e := FromParametric(cylinder)
	if e != nil {
		return bad(e)
	}
	points := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(1, 1)}}
	parts := make([]SewFace, 0, 16)
	for u := 0; u < 4; u++ {
		for v := 0; v < 4; v++ {
			support, e := source.TorusQuadrant(ctx, u, v, o)
			if e != nil {
				return bad(e)
			}
			part, e := rationalQuadraticSewFace(ctx, Face{Surface: support, Reversed: true}, support.ExactPatch(), points, o)
			if e != nil {
				return bad(e)
			}
			parts = append(parts, part)
		}
	}
	if e := proveRationalSplitVertexIdentity(ctx, parts); e != nil {
		return bad(e)
	}
	inner, _, e := Sew(ctx, parts, SewOptions{Tolerance: tolerance, MaxFaces: 16, MaxUses: 64})
	if e != nil {
		return bad(e)
	}
	vertices := append(outer.Vertices(), inner.Vertices()...)
	edges := outer.Edges()
	vo, eo := len(outer.vertices), len(edges)
	for _, edge := range inner.edges {
		edge.Start += vo
		edge.End += vo
		edges = append(edges, edge)
	}
	faces := outer.Faces()
	for _, face := range inner.Faces() {
		for _, loop := range append([][]Use{face.Outer}, face.Holes...) {
			for i := range loop {
				loop[i].Edge += eo
			}
		}
		faces = append(faces, face)
	}
	solid, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return bad(e)
	}
	return solid, source, nil
}
