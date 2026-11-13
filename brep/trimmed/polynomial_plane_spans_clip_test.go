package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPiecewisePolynomialPlaneNativeClip(t *testing.T) {
	ctx := context.Background()
	stock := creasedPolynomialClipStock(t)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, PolynomialPlaneSpanOptions{})
		if e != nil {
			t.Fatal(op, e)
		}
		// Integrate q(x)^2/8 separately on [0,1/2] and [1/2,1], where
		// q=3/4-x³/2-|x-1/2|/8. The original stock has unit volume.
		volume, center := 4031./86016, geom.Vec3{X: 7966. / 20155, Y: 56239. / 257984, Z: 137249. / 1031936}
		if op == parametric.Difference {
			volume = 81985. / 86016
			center = geom.Vec3{X: 207074. / 409925, Y: 2696273. / 5247040, Z: 14485471. / 20988160}
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-volume) > 1e-8 || m.Centroid.Sub(center).Length() > 1e-8 {
			t.Fatal("independent crease integral", m, e)
		}
		if len(r.Knots) != 2 || len(r.SectionEdges) != 6 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("knot topology", len(r.Knots), len(r.SectionEdges))
		}
		for _, k := range r.Knots {
			if k.Pieces != [2]int{0, 1} || k.Edges[0] == k.Edges[1] {
				t.Fatal("piece ownership")
			}
			for _, id := range k.Edges {
				edge := r.Solid.edges[id]
				if edge.Start != k.Vertex && edge.End != k.Vertex {
					t.Fatal("shared vertex")
				}
			}
			v := r.Solid.vertices[k.Vertex]
			if k.Point[0].Cmp(ratInset(v.X)) != 0 || k.Point[1].Cmp(ratInset(v.Y)) != 0 || k.Point[2].Cmp(ratInset(v.Z)) != 0 {
				t.Fatal("exact knot point")
			}
			if r.Solid.faces[r.StockFaces[k.Face][0]].Surface != stock.faces[k.Face].Surface {
				t.Fatal("original support replaced")
			}
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
				t.Fatal("duplicate section")
			}
			owned[id] = true
		}
		if len(owned) != len(r.Solid.edges) {
			t.Fatal("missing ancestry")
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		lm, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(lm.Volume-volume) > 1e-8 {
			t.Fatal("native reload", e)
		}
		mesh, e := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("contacts", e)
		}
	}
	if r, e := CombineQuadraticSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); e == nil || len(r.Solid.faces) != 0 {
		t.Fatal("legacy span gate widened")
	}
	for _, o := range []PolynomialPlaneSpanOptions{{MaxSpans: 1}, {MaxSpans: -1}, {Faces: SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}}} {
		if r, e := CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, o); e == nil || len(r.Solid.faces) != 0 || len(r.Knots) != 0 {
			t.Fatal("atomic budget")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CombinePiecewisePolynomialSplinePlaneClip(canceled, stock, tool, 5, parametric.Intersection, PolynomialPlaneSpanOptions{}); e != context.Canceled {
		t.Fatal("cancel", e)
	}
}

func TestPiecewisePolynomialPlaneRetainsLocalProjection(t *testing.T) {
	ctx := context.Background()
	// Reparameterize X piecewise, keeping its range [0,1]. The physical
	// derivative jumps from 3/4 to 5/4 at U=1/2. Both source cells must
	// therefore supply their own affine cap transport.
	stock := remapPolynomialClipFixture(t, creasedPolynomialClipStock(t), func(p geom.Vec3) geom.Vec3 {
		if p.X <= .5 {
			p.X *= .75
		} else {
			p.X = (5*p.X - 1) / 4
		}
		return p
	})
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, err := CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, op, PolynomialPlaneSpanOptions{})
		if err != nil {
			t.Fatal(op, err)
		}
		if len(r.Knots) != 2 || len(r.SectionEdges) != 6 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("expected two original junctions on one closed shell")
		}
		for _, k := range r.Knots {
			if k.Knot.Cmp(big.NewRat(1, 2)) != 0 || k.Point[0].Cmp(big.NewRat(3, 8)) != 0 {
				t.Fatal("lost original parameter or model knot", k.Knot, k.Point)
			}
		}
		v := [2]float64{37097. / 860160, 823063. / 860160}[i]
		c := [2]geom.Vec3{
			{X: 223017. / 593552, Y: 503587. / 2374208, Z: 1277069. / 9496832},
			{X: 6658263. / 13169008, Y: 27021533. / 52676032, Z: 146455411. / 210704128},
		}[i]
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(mass.Volume-v) > 1e-8 || mass.Centroid.Sub(c).Length() > 1e-8 {
			t.Fatal("piecewise Jacobian integral", mass, err)
		}
		var native bytes.Buffer
		if err = WriteJSON(&native, r.Solid); err != nil {
			t.Fatal(err)
		}
		if _, err = ReadJSONContext(ctx, &native, JSONLimits{}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestPolynomialPlaneWholeKnotSeamRequired(t *testing.T) {
	ctx := context.Background()
	s := creasedPolynomialClipStock(t)
	proof, e := CertifyPiecewisePolynomialPlaneFaceSection(ctx, s, 4, 3./16, 0, PolynomialPlaneSpanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := proof.patch.Trim(proof.domains[0])
	if e != nil {
		t.Fatal(e)
	}
	b, e := proof.patch.Trim(proof.domains[1])
	if e != nil {
		t.Fatal(e)
	}
	// Scaling the right residual about the cutting plane preserves the exact
	// branch and its junction, but makes the REST of the source seam discontinuous.
	b, e = b.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 2, -3. / 16}})
	if e != nil {
		t.Fatal(e)
	}
	q, _, e := polynomialPlaneTrimAxis(b, ratInset(3./16), 0)
	if e != nil {
		t.Fatal(e)
	}
	right, _, e := polynomialPlaneEndpoints(q)
	if e != nil {
		t.Fatal(e)
	}
	left, _, e := polynomialPlaneEndpoints(proof.pieces[0])
	if e != nil {
		t.Fatal(e)
	}
	for k := range right[0] {
		if right[0][k].Cmp(left[1][k]) != 0 {
			t.Fatal("branch junction must match")
		}
	}
	if e = quadraticPlaneSeam(ctx, a, b, 0, .5, []float64{0, 1}, geom.SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("accepted discontinuous source through matching branch point")
	}
}

func TestPiecewisePolynomialPlaneFreeV(t *testing.T) {
	ctx := context.Background()
	source := creasedPolynomialClipStock(t)
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for _, f := range source.faces {
		d := f.Surface.(*geom.BSplineSurface).Data()
		p := make([]geom.Vec3, len(d.Poles))
		w := make([]float64, len(d.Weights))
		for i := 0; i < d.CountU; i++ {
			for j := 0; j < d.CountV; j++ {
				p[j*d.CountU+i] = d.Poles[i*d.CountV+j]
				if len(w) > 0 {
					w[j*d.CountU+i] = d.Weights[i*d.CountV+j]
				}
			}
		}
		s, e := geom.NewBSplineSurface(d.DegreeV, d.DegreeU, d.CountV, d.CountU, p, d.KnotsV, d.KnotsU, w)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
		reversed = append(reversed, !f.Reversed)
	}
	stock, _, e := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	r, e := CombinePiecewisePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, PolynomialPlaneSpanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, k := range r.Knots {
		if k.FreeAxis != 1 || k.Edges[0] == k.Edges[1] {
			t.Fatal("original V ownership")
		}
	}
	m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(m.Volume-4031./86016) > 1e-8 {
		t.Fatal("transposed crease integral", m, e)
	}
	// A second knot direction needs a different arrangement proof and is not
	// implicitly accepted by trying the other free coordinate.
	bad := stock
	bad.faces = append([]Face(nil), stock.faces...)
	s, e := surfaces[4].InsertUKnot(.5, 1)
	if e != nil {
		t.Fatal(e)
	}
	bad.faces[4].Surface = s
	if _, e = CertifyPiecewisePolynomialPlaneFaceSection(ctx, bad, 4, 3./16, 1, PolynomialPlaneSpanOptions{}); e == nil {
		t.Fatal("dependent-axis knot")
	}
}
