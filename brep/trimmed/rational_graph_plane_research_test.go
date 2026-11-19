//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestRationalGraphSplinePlaneClip(t *testing.T) {
	ctx := context.Background()
	stock := rationalGraphClipStock(t, 1)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	if r, err := CombinePolynomialSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{}); err == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("polynomial-only operation accepted a rational graph", err)
	}
	// Independently integrate vertical material between the original surface
	// and Z=3/16 over 0<Y<q(X), where q=(6−2X−X²)/(8(1+X)).
	l := math.Log(2)
	volume := -69./2048 + 49*l/512
	first := geom.Vec3{X: 2387./30720 - 49*l/512, Y: 673. / 122880, Z: -211./65536 + 539*l/49152}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, err := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if err != nil {
			t.Fatal(op, err)
		}
		wantV, wantM := volume, first
		if op == parametric.Difference {
			wantV, wantM = 1-volume, (geom.Vec3{X: .5, Y: .5, Z: 35. / 48}).Sub(first)
		}
		mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(mass.Volume-wantV) > 1e-8 || mass.Centroid.Sub(wantM.Scale(1/wantV)).Length() > 1e-8 {
			t.Fatal("independent rational-region mass", mass, err)
		}
		if len(r.SectionEdges) != 4 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("rational section topology", r.Solid.Topology())
		}
		cap := r.Solid.faces[r.ToolFaces[5][0]]
		if cap.Surface != tool.faces[5].Surface || r.Solid.faces[r.StockFaces[4][0]].Surface != stock.faces[4].Surface {
			t.Fatal("original spline or cutter support was replaced")
		}
		weighted := 0
		for _, use := range cap.Outer {
			q, ok := use.Curve.(ExactQuadraticUV)
			if !ok {
				continue
			}
			h := q.HomogeneousControls()
			if h[0][2].Cmp(h[2][2]) == 0 {
				continue
			}
			weighted++
			p, err := r.Solid.edges[use.Edge].Curve.Eval(.375)
			if err != nil || math.Abs(p.Z-3./16) > 1e-13 {
				t.Fatal("exact curve leaves cutter plane", p, err)
			}
			if op == parametric.Intersection {
				h[1][0].Add(h[1][0], big.NewRat(1, 512))
				changed, err := NewExactQuadraticUV(h)
				if err != nil {
					t.Fatal(err)
				}
				other := use
				other.Curve = changed
				match, err := comparePolynomialTrimUse(ctx, r.Solid.edges[use.Edge], cap.Surface, other, edgecheck.Options{})
				if err != nil || match {
					t.Fatal("accepted a changed rational interior with unchanged endpoints", match, err)
				}
			}
		}
		if weighted != 1 {
			t.Fatal("expected one nonpolynomial section on the cutter", weighted)
		}
		// The lower chart uses X,Y directly. Check its complete rational
		// correspondence independently at a nonbinary parameter.
		bottom := r.Solid.faces[r.StockFaces[4][0]]
		found := false
		for _, use := range bottom.Outer {
			q, ok := use.Curve.(ExactQuadraticUV)
			if !ok || q.control[0][2].Cmp(q.control[2][2]) == 0 {
				continue
			}
			if found {
				t.Fatal("duplicate original rational section")
			}
			found = true
			uv, _, err := q.EvaluateRational(big.NewRat(1, 7))
			if err != nil {
				t.Fatal(err)
			}
			x := uv[0]
			num := new(big.Rat).Sub(big.NewRat(6, 1), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), x), new(big.Rat).Mul(x, x)))
			den := new(big.Rat).Mul(big.NewRat(8, 1), new(big.Rat).Add(big.NewRat(1, 1), x))
			if uv[1].Cmp(num.Quo(num, den)) != 0 {
				t.Fatal("original UV is not on the exact rational graph", uv)
			}
		}
		if !found {
			t.Fatal("missing original rational section")
		}
		var b bytes.Buffer
		if err = WriteJSON(&b, r.Solid); err != nil {
			t.Fatal(err)
		}
		loaded, err := ReadJSONContext(ctx, &b, JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		display, err := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if err != nil {
			t.Fatal(err)
		}
		if err = display.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		contacts, err := collision.SelfIntersectionsBVHContext(ctx, display, collision.Options{})
		if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("rational-section display contacts", contacts, err)
		}
	}
}

func TestPiecewiseRationalGraphSplinePlaneClip(t *testing.T) {
	ctx := context.Background()
	base := rationalGraphClipStock(t, 1)
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for i, f := range base.faces {
		s := f.Surface.(*geom.BSplineSurface)
		var err error
		if i >= 2 {
			if i == 2 || i == 3 {
				s, err = s.InsertVKnot(.5, 2)
			} else {
				s, err = s.InsertUKnot(.5, 2)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		d := s.Data()
		for j := range d.Poles {
			d.Poles[j].Z += math.Abs(d.Poles[j].X-.5) / 64
		}
		s, err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if err != nil {
			t.Fatal(err)
		}
		surfaces = append(surfaces, s)
		domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
		reversed = append(reversed, f.Reversed)
	}
	stock, _, err := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 23. / 128}, 0)
	face, pieces, err := certifyPiecewiseGraphPlaneFaceSection(ctx, stock, 4, horizontalPolynomialPlane(big.NewRat(23, 128)), 0, PolynomialPlaneSpanOptions{}, true)
	if err != nil || len(pieces) != 2 || len(face.Knots()) != 1 {
		t.Fatal("expected one exact original knot", err)
	}
	for _, p := range pieces {
		q, ok := p.(ExactQuadraticUV)
		if !ok || q.control[0][2].Cmp(q.control[2][2]) == 0 {
			t.Fatal("expected a rational section on each side of the crease")
		}
	}
	// Exact antiderivatives of both rational strips for the known profile:
	// X/16+X²/32+(1+X)Y/4+|X−1/2|/64.
	l, h := math.Log(1.5), math.Log(4./3)
	v := -33./1024 + 9*l/128 + 225*h/2048
	first := geom.Vec3{
		X: 4049./122880 + 3011./81920 - 9*l/128 - 225*h/2048,
		Y: (14681.+21041)/7864320 + 9*l/2048 - 225*h/32768,
		Z: -1973./2621440 - 22723./10485760 + 135*l/16384 + 2925*h/262144,
	}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, err := CombinePiecewiseRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, PolynomialPlaneSpanOptions{})
		if err != nil {
			t.Fatal(op, err)
		}
		if len(r.Knots) != 2 || len(r.SectionEdges) != 6 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("missing original junctions or closed topology")
		}
		for _, k := range r.Knots {
			if k.Point[0].Cmp(big.NewRat(1, 2)) != 0 || k.Point[2].Cmp(big.NewRat(23, 128)) != 0 ||
				k.Edges[0] == k.Edges[1] || k.Pieces != [2]int{0, 1} {
				t.Fatal("wrong original knot ownership", k)
			}
		}
		wv, wm := v, first
		if op == parametric.Difference {
			wv, wm = 1-v, (geom.Vec3{X: .5, Y: .5, Z: 35./48 + 1./256}).Sub(first)
		}
		m, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(m.Volume-wv) > 1e-8 || m.Centroid.Sub(wm.Scale(1/wv)).Length() > 1e-8 {
			t.Fatal("independent strip integrals", m, err)
		}
		var b bytes.Buffer
		if err = WriteJSON(&b, r.Solid); err != nil {
			t.Fatal(err)
		}
		loaded, err := ReadJSONContext(ctx, &b, JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		mesh, err := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if err != nil || mesh.ValidateSolid() != nil {
			t.Fatal("native mesh closure", err)
		}
		contacts, err := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("native mesh embedding", err)
		}
	}
	if r, err := CombinePiecewiseRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, PolynomialPlaneSpanOptions{MaxSpans: 1}); err == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("span limit accepted", err)
	}
}
