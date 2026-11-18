package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
)

// The lower and upper graphs are Z=X^degree/16+(1+X)Y/4+W.
// Degree four control grids represent X^3 as well as X^4 exactly.
func rationalBezierGraphStock(t *testing.T, degree int) Solid {
	t.Helper()
	return rationalBezierGraphStockFrom(t, func(k int, y float64) float64 {
		xp := float64(polynomialChoose(k, degree)) / float64(polynomialChoose(4, degree))
		return xp/16 + (1+float64(k)/4)*y/4
	})
}

func rationalBezierGraphStockFrom(t *testing.T, height func(int, float64) float64) Solid {
	t.Helper()
	var faces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := range 5 {
				for j := range 5 {
					p := [3]float64{}
					p[axis], p[u], p[v] = float64(side), float64(i)/4, float64(j)/4
					k := int(4 * p[0])
					p[2] += height(k, p[1])
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
				}
			}
			s, err := geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
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

func TestRationalGraphPlaneQuadraticDenominator(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock := rationalBezierGraphStockFrom(t, func(k int, y float64) float64 {
		return (1 + float64(polynomialChoose(k, 2))/2) * y / 4
	})
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	// Lower graph Z=(1+3X²)Y/4 gives cutter Y=3/(4(1+3X²)).
	// Integrate the vertical wedge without using its curve or surface nodes.
	i := math.Pi / (3 * math.Sqrt(3))
	v := 9 * i / 128
	moment := geom.Vec3{X: 3 * math.Log(4) / 256, Y: 9 * (i + .25) / 1024, Z: v / 8}
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
		r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
		if e != nil || len(r.SectionEdges) != 4 {
			t.Fatal("quadratic-denominator section", e)
		}
		want, first := v, moment
		if op == parametric.Difference {
			want, first = 1-v, (geom.Vec3{X: .5, Y: .5, Z: .75}).Sub(moment)
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(m.Volume-want) > 1e-8 || m.Centroid.Sub(first.Scale(1/want)).Length() > 1e-8 {
			t.Fatal("independent quadratic-denominator integral", m, e, want, first.Scale(1/want))
		}
		found := false
		for _, u := range r.Solid.faces[r.StockFaces[4][0]].Outer {
			if q, ok := u.Curve.(ExactBezierUV); ok {
				p, _, e := q.EvaluateRational(big.NewRat(3, 11))
				if e != nil || len(q.control) != 4 {
					t.Fatal("expected exact rational cubic", e)
				}
				x2 := new(big.Rat).Mul(p[0], p[0])
				b := new(big.Rat).Add(big.NewRat(4, 1), new(big.Rat).Mul(big.NewRat(12, 1), x2))
				if new(big.Rat).Mul(b, p[1]).Cmp(big.NewRat(3, 1)) != 0 {
					t.Fatal("trim does not solve the exact quadratic denominator", p)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("native trim absent")
		}
	}
	// Same-sign endpoints do not establish the sign of a quadratic. This
	// chart has B=3(2X-1)^2/4, so it must fail before any UV trim is returned.
	singular := rationalBezierGraphStockFrom(t, func(k int, y float64) float64 {
		return (3 - 3*float64(k) + 2*float64(polynomialChoose(k, 2))) * y / 4
	})
	p, _, e := singular.splineEmbeddingFace(ctx, singular.faces[4], SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q, _, e := rationalGraphPlaneTrim(p.patch, horizontalPolynomialPlane(big.NewRat(3, 16)), 0); q != nil || e != geom.ErrConvergence {
		t.Fatal("accepted an interior double zero in the denominator", e)
	}
}

func TestRationalBezierGraphPlaneClip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			stock := rationalBezierGraphStock(t, n)
			// Integrate between the original lower graph and the cutter.
			// I and J integrate x^k/(1+x) and x^k/(1+x)^2 respectively.
			I, J := [14]float64{math.Log(2)}, [14]float64{.5}
			for k := 1; k < len(I); k++ {
				I[k] = 1/float64(k) - I[k-1]
				J[k] = I[k-1] - J[k-1]
			}
			v := (9*I[0] - 6*I[n] + I[2*n]) / 128
			first := geom.Vec3{
				X: (9*I[1] - 6*I[n+1] + I[2*n+1]) / 128,
				Y: (27*J[0] - 27*J[n] + 9*J[2*n] - J[3*n]) / 1536,
				Z: (3./16)*v - (27*I[0]-27*I[n]+9*I[2*n]-I[3*n])/6144,
			}
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Difference} {
				res, err := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, op, SplineRectangleEmbeddingOptions{})
				if err != nil {
					t.Fatal(op, err)
				}
				wantV, wantM := v, first
				if op == parametric.Difference {
					wantV, wantM = 1-v, (geom.Vec3{X: .5, Y: .5, Z: 11./16 + 1/float64(16*(n+1))}).Sub(first)
				}
				mass, err := res.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil || math.Abs(mass.Volume-wantV) > 1e-8 || mass.Centroid.Sub(wantM.Scale(1/wantV)).Length() > 1e-8 {
					t.Fatal("independent mass", mass, err, wantV, wantM.Scale(1/wantV))
				}
				if len(res.SectionEdges) != 4 || res.Solid.Topology().EulerCharacteristic != 2 {
					t.Fatal("section topology", res.Solid.Topology())
				}
				bottomID := res.StockFaces[4][0]
				bottom := res.Solid.faces[bottomID]
				if bottom.Surface != stock.faces[4].Surface || res.Solid.faces[res.ToolFaces[5][0]].Surface != tool.faces[5].Surface {
					t.Fatal("changed original support")
				}
				found := false
				for _, u := range bottom.Outer {
					q, ok := u.Curve.(ExactBezierUV)
					if !ok {
						continue
					}
					if found || len(q.control) != n+1 {
						t.Fatal("wrong section degree or duplicate section")
					}
					found = true
					uv, duv, err := q.EvaluateRational(big.NewRat(1, 7))
					if err != nil {
						t.Fatal(err)
					}
					x, y := uv[0], uv[1]
					xPow, xPrev := big.NewRat(1, 1), big.NewRat(1, 1)
					for range n - 1 {
						xPrev.Mul(xPrev, x)
					}
					xPow.Mul(xPrev, x)
					den := new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Add(big.NewRat(1, 1), x))
					if new(big.Rat).Mul(den, y).Cmp(new(big.Rat).Sub(big.NewRat(3, 1), xPow)) != 0 {
						t.Fatal("exact UV residual", uv)
					}
					dh := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(int64(n), 1), xPrev), new(big.Rat).Mul(big.NewRat(4, 1), y))
					if new(big.Rat).Add(new(big.Rat).Mul(dh, duv[0]), new(big.Rat).Mul(den, duv[1])).Sign() != 0 {
						t.Fatal("exact UV derivative residual", uv, duv)
					}
				}
				if !found {
					t.Fatal("missing graph trim")
				}
				var b bytes.Buffer
				if err = WriteJSON(&b, res.Solid); err != nil {
					t.Fatal(err)
				}
				loaded, err := ReadJSONContext(ctx, bytes.NewReader(b.Bytes()), JSONLimits{})
				if err != nil {
					t.Fatal(err)
				}
				m, err := loaded.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
				if err != nil || m.ValidateSolid() != nil {
					t.Fatal("native mesh closure", err)
				}
				c, err := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
				if err != nil || !c.Complete || len(c.Contacts) != 0 {
					t.Fatal("native mesh contacts", c, err)
				}
				if n == 4 && op == parametric.Intersection {
					var data nativeSolid
					if err = json.Unmarshal(b.Bytes(), &data); err != nil || data.Version != 6 {
						t.Fatal("native version", data.Version, err)
					}
					data.Version = 5
					prior, _ := json.Marshal(data)
					if _, err = ReadJSONContext(ctx, bytes.NewReader(prior), JSONLimits{}); err == nil {
						t.Fatal("older native schema accepted a new trim tag")
					}
					data.Version = 6
					for _, u := range data.Faces[bottomID].Outer {
						if len(u.ExactBezier) != 0 {
							p, _ := new(big.Rat).SetString(u.ExactBezier[1][1])
							u.ExactBezier[1][1] = p.Add(p, big.NewRat(1, 1<<45)).RatString()
						}
					}
					bad, _ := json.Marshal(data)
					if _, err = ReadJSONContext(ctx, bytes.NewReader(bad), JSONLimits{}); err == nil {
						t.Fatal("inexact coedge accepted after a sub-tolerance interior change")
					}
				}
			}
		})
	}
}

func TestNativeExactTrimCoefficientBounds(t *testing.T) {
	for _, raw := range []string{"1e999999999999", "0x10", "01/2", "2/4", "-0", "1/0", "+1"} {
		if _, e := parseNativeExactTrimCoefficient(raw); e == nil {
			t.Errorf("accepted noncanonical or oversized coefficient %q", raw)
		}
	}
	if r, e := parseNativeExactTrimCoefficient("-2/3"); e != nil || r.Cmp(big.NewRat(-2, 3)) != 0 {
		t.Fatal("canonical signed coordinate", r, e)
	}
}

func TestRationalBezierGraphTrimRay(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStock(t, 4)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	face := r.StockFaces[4][0]
	// At X=1/2 the exact section has Y=47/96. Both query boxes are in
	// the complete curved control hull, so the box proof must subdivide.
	x := 0.5
	for _, q := range []struct {
		y    float64
		want trimvalidate.Location
	}{{.47, trimvalidate.Inside}, {.51, trimvalidate.Outside}} {
		box := geom.UVBox{Min: geom.Vec2{X: x - .001, Y: q.y - .001}, Max: geom.Vec2{X: x + .001, Y: q.y + .001}}
		loc, known, e := classifySplineFaceRootBox(ctx, r.Solid.faces[face], box, trimvalidate.Options{}, 256)
		if e != nil || !known || loc != q.want {
			t.Fatal(loc, known, e)
		}
		if _, known, e = classifySplineFaceRootBox(ctx, r.Solid.faces[face], box, trimvalidate.Options{}, 1); e != nil || known {
			t.Fatal("one hull is insufficient", known, e)
		}
		line, e := geom.NewLine(geom.Vec3{X: x, Y: q.y, Z: 1}, geom.Vec3{Z: -1})
		if e != nil {
			t.Fatal(e)
		}
		hit, e := r.Solid.IntersectSplineFaceLine(ctx, face, line, SplineFaceLineOptions{Support: geom.LineSplineSurfaceOptions{MaxParameter: 2}})
		if e != nil || !hit.Complete || (len(hit.Hits) == 1) != (q.want == trimvalidate.Inside) || len(hit.Hits)+len(hit.Excluded) != 1 {
			t.Fatal(hit, e)
		}
	}
	o := SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MaxParameter: 2}}
	for _, item := range []struct {
		x, y  float64
		count int
	}{{.5, .47, 2}, {.5, .51, 0}} {
		h, e := r.Solid.SplineRay(ctx, geom.Vec3{X: item.x, Y: item.y, Z: 1}, geom.Vec3{Z: -1}, o)
		if e != nil || !h.Complete || len(h.Hits) != item.count {
			t.Fatal(item, h, e)
		}
	}
	// A box straddling the curved edge must remain unresolved.
	y := 47. / 96
	box := geom.UVBox{Min: geom.Vec2{X: x - .001, Y: y - .001}, Max: geom.Vec2{X: x + .001, Y: y + .001}}
	if _, known, e := classifySplineFaceRootBox(ctx, r.Solid.faces[face], box, trimvalidate.Options{}, 256); e != nil || known {
		t.Fatal("claimed box overlapping exact trim", known, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := r.Solid.SplineRay(canceled, geom.Vec3{Z: 1}, geom.Vec3{Z: -1}, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestRationalBezierGraphBoundaryDistance(t *testing.T) {
	ctx := context.Background()
	stock := rationalBezierGraphStock(t, 4)
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 3. / 16}, 0)
	r, e := CombineRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Every point of the common is at or below Z=3/16. Its projection
	// on the trimmed cap lies strictly inside, attaining the plane bound.
	o := SplineBoundaryDistanceOptions{SquaredTolerance: 1e-9, MaxCells: 64}
	q := geom.Vec3{X: .5, Y: .25, Z: 7. / 16}
	got, e := r.Solid.BoundSplineBoundaryDistance(ctx, q, o)
	if e != nil || !got.Complete || !got.HasWitness || got.Face != r.ToolFaces[5][0] || got.SquaredMin != .0625 || got.SquaredMax != .0625 {
		t.Fatal("exact cap projection", got, e)
	}
	// At parameter 1/2 on the quartic edge, its nonbinary Y value is a true
	// attainable point. The query uses that nearest display coordinate while
	// constructing a rational witness independently of the rounded float.
	edgeY := 47. / 96
	q.Y = edgeY + 1./128
	o.SquaredTolerance = 4e-4
	o.MaxCells = 4096
	got, e = r.Solid.BoundSplineBoundaryDistance(ctx, q, o)
	if e != nil || !got.Complete || !got.HasWitness || got.SquaredMin > .0625+1./(128*128) ||
		got.SquaredMax < .0625 || got.SquaredMax > .0625+1./(128*128)+o.SquaredTolerance {
		t.Fatal("curved-edge witness or distance bound", got, e)
	}
}

func TestPiecewiseRationalBezierGraphPlaneClip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base := rationalBezierGraphStock(t, 3)
	var faces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for i, f := range base.faces {
		s := f.Surface.(*geom.BSplineSurface)
		var e error
		switch i {
		case 2, 3:
			s, e = s.InsertVKnot(.5, 4)
		case 4, 5:
			s, e = s.InsertUKnot(.5, 4)
		}
		if e != nil {
			t.Fatal(e)
		}
		d := s.Data()
		for j := range d.Poles {
			d.Poles[j].Z += math.Abs(d.Poles[j].X-.5) / 64
		}
		s, e = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		faces = append(faces, s)
		domains = append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1})
		reversed = append(reversed, f.Reversed)
	}
	stock, _, e := SewSplineRectangles(ctx, faces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	tool, _ := makeOpenSplineBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 1. / 8}, 0)
	r, e := CombinePiecewiseRationalGraphSplinePlaneClip(ctx, stock, tool, 5, parametric.Intersection, PolynomialPlaneSpanOptions{})
	if e != nil || len(r.Knots) != 2 || len(r.SectionEdges) != 6 {
		t.Fatal("piecewise cubic result", e)
	}
	for _, k := range r.Knots {
		if k.Edges[0] == k.Edges[1] || k.Point[0].Cmp(big.NewRat(1, 2)) != 0 || k.Point[2].Cmp(big.NewRat(1, 8)) != 0 {
			t.Fatal("invalid cubic knot ownership", k)
		}
		if k.Face == 4 && k.Point[1].Cmp(big.NewRat(5, 16)) != 0 {
			t.Fatal("incorrect exact cubic junction", k)
		}
	}
	pieces := 0
	for _, u := range r.Solid.faces[r.StockFaces[4][0]].Outer {
		if q, ok := u.Curve.(ExactBezierUV); ok && len(q.control) == 4 {
			pieces++
		}
	}
	if pieces != 2 {
		t.Fatal("source knot should retain one cubic trim on either side", pieces)
	}
	plane := [3]geom.Vec3{{Z: 1. / 8}, {X: 1, Z: 1. / 8}, {Y: 1, Z: 1. / 8}}
	section, e := CertifyRationalGraphPlaneFaceSection(ctx, stock, 4, plane, 0, PolynomialPlaneSpanOptions{})
	if e != nil || len(section.Pieces()) != 2 || len(section.Knots()) != 1 || section.Knots()[0].Point[1].Cmp(big.NewRat(5, 16)) != 0 {
		t.Fatal("face-only certificate loses original piecewise junction", section, e)
	}
	if bad, e := CertifyRationalGraphPlaneFaceSection(ctx, stock, 4, plane, 0, PolynomialPlaneSpanOptions{MaxSpans: 1}); e == nil || bad != nil {
		t.Fatal("face-only certificate ignored span budget", e)
	}
	wire, e := stock.SectionRationalGraphSplineShell(ctx, plane, RationalGraphShellSectionOptions{})
	if e != nil || !wire.Complete || len(wire.Wires) != 1 || len(wire.Wires[0]) != 4 {
		t.Fatal("piecewise shell section", wire, e)
	}
	pieces, junction := 0, false
	for _, b := range wire.Branches {
		pieces += len(b.Pieces)
		if b.Face == 4 {
			if len(b.Pieces) != 2 {
				t.Fatal("lost original bottom-face knot")
			}
			left, _, e1 := polynomialPlaneEndpoints(b.Pieces[0])
			right, _, e2 := polynomialPlaneEndpoints(b.Pieces[1])
			k := section.Knots()[0].UV
			junction = e1 == nil && e2 == nil && left[1][0].Cmp(k[0]) == 0 && left[1][1].Cmp(k[1]) == 0 &&
				right[0][0].Cmp(k[0]) == 0 && right[0][1].Cmp(k[1]) == 0
		}
	}
	if pieces != 6 || !junction {
		t.Fatal("composed wire lost original piecewise branches", pieces, junction)
	}
	cap, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{
		{Z: 1. / 8}, {Y: 1, Z: 1. / 8}, {X: 1, Z: 1. / 8}, {X: 1, Y: 1, Z: 1. / 8},
	}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	split, e := stock.SplitRationalGraphSplineShell(ctx, plane, cap,
		geom.ParameterRectangle{UMax: 1, VMax: 1}, RationalGraphShellSplitOptions{})
	if e != nil {
		t.Fatal("direct piecewise split", e)
	}
	for side, h := range split.Halves {
		if len(h.CapFaces) != 1 || len(h.CapEdges[0]) != 6 {
			t.Fatal("piecewise cap lost original knot splits", side)
		}
		var stream bytes.Buffer
		if e := WriteJSON(&stream, h.Solid); e != nil {
			t.Fatal(e)
		}
		reloaded, e := ReadJSONContext(ctx, &stream, JSONLimits{})
		if e != nil {
			t.Fatal("exact piecewise split persistence", side, e)
		}
		m, e := reloaded.TessellateEmbeddedDisplay(ctx, EmbeddedDisplayOptions{Tessellation: parametric.TessellationOptions{Deflection: .05}})
		if e != nil || m.SignedVolume.Sign() <= 0 {
			t.Fatal("reloaded split half does not have an embedded positive display", side, e)
		}
		n := 0
		for _, u := range reloaded.faces[h.CapFaces[0]].Outer {
			if q, ok := u.Curve.(ExactBezierUV); ok && len(q.HomogeneousControls()) == 4 {
				n++
			}
		}
		if n != 2 {
			t.Fatal("native cap lost cubic section pieces", side, n)
		}
	}
}
