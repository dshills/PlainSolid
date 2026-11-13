package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestObliquePolynomialSplinePlaneClip(t *testing.T) {
	ctx := context.Background()
	// Z=3/16+X/32. Use original X,Y knot units on the cap so the
	// exact section's controls remain representable.
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	tool = remapPolynomialClipFixture(t, tool, func(p geom.Vec3) geom.Vec3 {
		p.Z += p.X / 32
		return p
	})
	for _, tc := range []struct {
		name  string
		stock func() Solid
		clip  func(Solid, parametric.BooleanOp) (SplineCornerPlaneClip, error)
		edges int
		vol   [2]float64
		com   [2]geom.Vec3
	}{
		{"cubic", func() Solid { s, _ := polynomialClipStock(t); return s },
			func(s Solid, op parametric.BooleanOp) (SplineCornerPlaneClip, error) {
				return CombinePolynomialSplinePlaneClip(ctx, s, tool, 5, op, SplineRectangleEmbeddingOptions{})
			}, 4, [2]float64{3257. / 53760, 50503. / 53760}, [2]geom.Vec3{
				{X: 5593. / 13028, Y: 24947. / 104224, Z: 29407. / 208448},
				{X: 101927. / 202012, Y: 835213. / 1616096, Z: 2228513. / 3232192},
			}},
		{"creased", func() Solid { return creasedPolynomialClipStock(t) },
			func(s Solid, op parametric.BooleanOp) (SplineCornerPlaneClip, error) {
				p, err := CombinePiecewisePolynomialSplinePlaneClip(ctx, s, tool, 5, op, PolynomialPlaneSpanOptions{})
				if err == nil && len(p.Knots) != 2 {
					t.Fatal("missing original knot junctions")
				}
				return p.SplineCornerPlaneClip, err
			}, 6, [2]float64{23921. / 430080, 406159. / 430080}, [2]geom.Vec3{
				{X: 20517. / 47842, Y: 353813. / 1530944, Z: 876463. / 6123776},
				{X: 409563. / 812318, Y: 13408747. / 25994176, Z: 72237137. / 103976704},
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stock := tc.stock()
			for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
				r, err := tc.clip(stock, op)
				if err != nil {
					t.Fatal(op, err)
				}
				if len(r.SectionEdges) != tc.edges || r.Solid.Topology().EulerCharacteristic != 2 {
					t.Fatal("unexpected topology", len(r.SectionEdges), r.Solid.Topology())
				}
				cap := r.Solid.faces[r.ToolFaces[5][0]]
				if cap.Surface != tool.faces[5].Surface {
					t.Fatal("lost original cutter support")
				}
				for _, id := range r.SectionEdges {
					edge := r.Solid.edges[id]
					p, err := edge.Curve.Eval(.375)
					if err != nil || math.Abs(p.Z-(3./16+p.X/32)) > 1e-13 {
						t.Fatal("section leaves the cutter plane", p, err)
					}
				}
				mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil || math.Abs(mass.Volume-tc.vol[i]) > 1e-8 || mass.Centroid.Sub(tc.com[i]).Length() > 1e-8 {
					t.Fatal("analytic oblique integral", mass, err)
				}
				var native bytes.Buffer
				if err = WriteJSON(&native, r.Solid); err != nil {
					t.Fatal(err)
				}
				if _, err = ReadJSONContext(ctx, &native, JSONLimits{}); err != nil {
					t.Fatal(err)
				}
				display, err := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
				if err != nil {
					t.Fatal(err)
				}
				if err = display.ValidateSolid(); err != nil {
					t.Fatal(err)
				}
				contacts, err := collision.SelfIntersectionsBVHContext(ctx, display, collision.Options{})
				if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
					t.Fatal("contacts", contacts, err)
				}
			}
		})
	}
}

func TestVerticalObliquePolynomialSplinePlaneClip(t *testing.T) {
	ctx := context.Background()
	cycle := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Z, Y: p.X, Z: p.Y} }
	stock, _ := polynomialClipStock(t)
	stock = remapPolynomialClipFixture(t, stock, cycle)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	tool = remapPolynomialClipFixture(t, tool, func(p geom.Vec3) geom.Vec3 {
		p.Z += p.X / 32
		return cycle(p)
	})
	r, err := CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if err != nil || len(r.SectionEdges) != 4 || r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("vertical oblique topology", err)
	}
	mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	want := cycle(geom.Vec3{X: 5593. / 13028, Y: 24947. / 104224, Z: 29407. / 208448})
	if err != nil || math.Abs(mass.Volume-3257./53760) > 1e-8 || mass.Centroid.Sub(want).Length() > 1e-8 {
		t.Fatal("axis-permuted analytic mass", mass, err)
	}
}

func remapPolynomialClipFixture(t *testing.T, s Solid, mapPoint func(geom.Vec3) geom.Vec3) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reverse []bool
	for _, f := range s.faces {
		base := f.Surface.(*geom.BSplineSurface)
		d := base.Data()
		for i, p := range d.Poles {
			d.Poles[i] = mapPoint(p)
		}
		next, err := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
		u0, u1, v0, v1 := base.Domain()
		surfaces = append(surfaces, next)
		domains = append(domains, geom.ParameterRectangle{UMin: u0, UMax: u1, VMin: v0, VMax: v1})
		reverse = append(reverse, f.Reversed)
	}
	out, _, err := SewSplineRectangles(context.Background(), surfaces, domains, reverse, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
