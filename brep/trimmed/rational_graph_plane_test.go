package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func rationalGraphClipStock(t *testing.T, dependentChange float64) Solid {
	t.Helper()
	var faces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	k := []float64{0, 0, 0, 1, 1, 1}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := range 3 {
				for j := range 3 {
					p := [3]float64{}
					p[axis], p[u], p[v] = float64(side), float64(i)/2, float64(j)/2
					x2 := p[0] * p[0]
					if u == 0 && i == 1 || v == 0 && j == 1 {
						x2 = 0
					}
					z := p[2] + p[0]/16 + x2/32 + (1+dependentChange*p[0])*p[1]/4
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: z})
				}
			}
			s, err := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
			if err != nil {
				t.Fatal(err)
			}
			faces = append(faces, s)
			domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
			reversed = append(reversed, side == 0)
		}
	}
	s, _, err := SewSplineRectangles(context.Background(), faces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

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

func TestRationalGraphPlaneRejectsSingularDenominator(t *testing.T) {
	stock := rationalGraphClipStock(t, -2)
	ctx := context.Background()
	face, _, err := stock.splineEmbeddingFace(ctx, stock.faces[4], SplineRectangleEmbeddingOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = rationalGraphPlaneTrim(face.patch, horizontalPolynomialPlane(big.NewRat(3, 16)), 0); !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("accepted an interior denominator zero", err)
	}
}
