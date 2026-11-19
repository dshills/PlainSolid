//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestPolynomialSplinePlaneClip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock, _ := polynomialClipStock(t)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{3, 3, 3. / 16}, 0)
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal("clip", op, e)
		}
		want := 23. / 448
		center := geom.Vec3{X: 91. / 230, Y: 1249. / 5520, Z: 2891. / 22080}
		if op == parametric.Difference {
			want = 1 - want
			center = geom.Vec3{X: 2149. / 4250, Y: 52511. / 102000, Z: 279349. / 408000}
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-want) > 1e-9 || m.Centroid.Sub(center).Length() > 1e-9 {
			t.Fatal("mass", m, e, want)
		}
		if len(r.SectionEdges) != 4 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("topology")
		}

		owned := map[int]bool{}
		for _, ids := range r.StockEdges {
			for _, id := range ids {
				if owned[id] {
					t.Fatal("duplicate ancestry")
				}
				owned[id] = true
			}
		}
		for _, id := range r.SectionEdges {
			if owned[id] {
				t.Fatal("section ancestry collision")
			}
			owned[id] = true
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("incomplete ancestry")
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal("mesh", e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
		corrupt := r.Solid
		corrupt.faces = append([]Face(nil), r.Solid.faces...)
		capID := r.ToolFaces[5][0]
		corrupt.faces[capID].Outer = append([]Use(nil), r.Solid.faces[capID].Outer...)
		bottomEdges := make(map[int]bool)
		for _, use := range r.Solid.faces[r.StockFaces[4][0]].Outer {
			bottomEdges[use.Edge] = true
		}
		section := -1
		var q SplineUV
		for i, use := range corrupt.faces[capID].Outer {
			if spline, ok := use.Curve.(SplineUV); bottomEdges[use.Edge] && ok && spline.Curve.Degree() == 4 {
				if section >= 0 {
					t.Fatal("multiple cubic graph sections on the cap")
				}
				section, q = i, spline
			}
		}
		if section < 0 {
			t.Fatal("missing cubic graph section on the cap")
		}
		ps := q.Curve.Poles()
		ps[1].Y += 1. / 64
		q.Curve, e = geom.NewBSpline(q.Curve.Degree(), ps, q.Curve.Knots(), q.Curve.Weights())
		if e != nil {
			t.Fatal(e)
		}
		corrupt.faces[capID].Outer[section].Curve = q
		var bad bytes.Buffer
		if e = WriteJSON(&bad, corrupt); e == nil {
			if _, e = ReadJSONContext(ctx, &bad, JSONLimits{}); e == nil {
				t.Fatal("native accepted unequal interior")
			}
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(ctx, &b, JSONLimits{})
		if e != nil {
			t.Fatal("reload", e)
		}
		lm, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(lm.Volume-want) > 1e-9 {
			t.Fatal("reload mass", e, lm)
		}
	}
}
