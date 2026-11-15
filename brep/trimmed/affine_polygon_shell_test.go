package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func convexPolygonShellFixture(t *testing.T, polygon []geom.Vec2) Solid {
	t.Helper()
	return convexPolygonShellMappedFixture(t, polygon, func(x, y, z float64) geom.Vec3 { return geom.Vec3{X: x + z/4, Y: y + x/8, Z: z + y/4} })
}
func convexPolygonShellMappedFixture(t *testing.T, polygon []geom.Vec2, world func(float64, float64, float64) geom.Vec3) Solid {
	return convexPolygonShellUnitsFixture(t, polygon, world, false)
}
func convexPolygonShellUnitsFixture(t *testing.T, polygon []geom.Vec2, world func(float64, float64, float64) geom.Vec3, physicalUnits bool) Solid {

	t.Helper()
	ctx := context.Background()
	parts := []SewFace{}
	add := func(uv []geom.Vec2, eval func(float64, float64) geom.Vec3, reverse bool, umax float64) {
		poles := []geom.Vec3{eval(0, 0), eval(0, 1), eval(umax, 0), eval(umax, 1)}
		surf, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, umax, umax}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		part := SewFace{Face: Face{Surface: surf, Reversed: reverse}}
		for i, p := range uv {
			n := (i + 1) % len(uv)
			part.Vertices = append(part.Vertices, eval(p.X, p.Y))
			part.Edges = append(part.Edges, parametric.Edge{Start: i, End: n, Curve: surfaceTrimCurve{surface: surf, a: p, b: uv[n]}, Range: parametric.Interval{Min: 0, Max: 1}})
			part.Face.Outer = append(part.Face.Outer, Use{Edge: i, UVStart: p, UVEnd: uv[n]})
		}
		parts = append(parts, part)
	}
	uv := make([]geom.Vec2, len(polygon))
	for i, p := range polygon {
		uv[i] = geom.Vec2{X: p.X / 4, Y: p.Y / 4}
	}
	add(uv, func(u, v float64) geom.Vec3 { return world(4*u, 4*v, 0) }, true, 1)
	add(uv, func(u, v float64) geom.Vec3 { return world(4*u, 4*v, 2) }, false, 1)
	for i, p := range polygon {
		q := polygon[(i+1)%len(polygon)]
		length := 1.0
		if physicalUnits {
			length = math.Max(math.Abs(q.X-p.X), math.Abs(q.Y-p.Y))
		}
		add([]geom.Vec2{{}, {X: length}, {X: length, Y: 1}, {Y: 1}}, func(u, v float64) geom.Vec3 { return world(p.X+(u/length)*(q.X-p.X), p.Y+(u/length)*(q.Y-p.Y), 2*v) }, false, length)
	}
	s, _, e := Sew(ctx, parts, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestConvexAffinePolygonShell(t *testing.T) {
	for _, tc := range []struct {
		name   string
		p      []geom.Vec2
		volume *big.Rat
	}{
		{"triangle", []geom.Vec2{{}, {X: 2}, {Y: 2}}, big.NewRat(129, 32)},
		{"pentagon", []geom.Vec2{{}, {X: 2}, {X: 3, Y: 1}, {X: 1.5, Y: 2}, {Y: 1}}, big.NewRat(129, 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := convexPolygonShellFixture(t, tc.p)
			r, e := CertifyConvexAffinePolygonShell(context.Background(), s, ConvexAffinePolygonShellOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if r.ExactVolume().Cmp(tc.volume) != 0 {
				t.Fatal("volume", r.ExactVolume(), tc.volume)
			}
			if r.CompleteFacePairs() != len(s.faces)*(len(s.faces)-1)/2 || r.EdgeUses() != 2*len(s.edges) {
				t.Fatal("incomplete")
			}

			// Every original adjacent face has one complete source-owned segment;
			// cap pairs are empty and non-adjacent side faces meet at most in a point.
			for i := range s.faces {
				for j := i + 1; j < len(s.faces); j++ {
					pair, e := r.PairContact(i, j)
					if e != nil {
						t.Fatal(e)
					}
					kind, e := pair.Contact().Kind()
					if e != nil {
						t.Fatal(e)
					}
					if kind == intersect.AffinePatchContactSegment {
						sides := pair.EdgeTrimSides()
						if len(sides) != 1 || len(sides[0][0]) != 1 || len(sides[0][1]) != 1 {
							t.Fatal("original trim ownership", sides)
						}
					}
				}
			}
			v := r.ExactVolume()
			v.SetInt64(0)
			if r.ExactVolume().Sign() != 1 {
				t.Fatal("mutable")
			}
			if q, e := CertifyConvexAffinePolygonShell(context.Background(), s, ConvexAffinePolygonShellOptions{MaxOperations: r.ExactOperations()}); e != nil || q == nil {
				t.Fatal("exact budget", e)
			}
			if q, e := CertifyConvexAffinePolygonShell(context.Background(), s, ConvexAffinePolygonShellOptions{MaxOperations: r.ExactOperations() - 1}); e == nil || q != nil {
				t.Fatal("short budget")
			}
		})
	}
}
func TestConvexAffinePolygonShellRejects(t *testing.T) {
	s := convexPolygonShellFixture(t, []geom.Vec2{{}, {X: 2}, {Y: 2}})
	for _, o := range []ConvexAffinePolygonShellOptions{{MaxFaces: 4}, {MaxUses: 12}, {MaxOperations: 128000001}} {
		if r, e := CertifyConvexAffinePolygonShell(context.Background(), s, o); e == nil || r != nil {
			t.Fatal("bounded rejection", o)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := CertifyConvexAffinePolygonShell(ctx, s, ConvexAffinePolygonShellOptions{}); e == nil || r != nil {
		t.Fatal("cancel")
	}
	bad := s
	bad.faces = append([]Face(nil), s.faces...)
	bad.faces[0].Reversed = !bad.faces[0].Reversed
	if r, e := CertifyConvexAffinePolygonShell(context.Background(), bad, ConvexAffinePolygonShellOptions{}); e == nil || r != nil {
		t.Fatal("inward")
	}
	bad = s
	bad.faces = append([]Face(nil), s.faces...)
	bad.faces[0].Outer = append([]Use(nil), s.faces[0].Outer...)
	bad.faces[0].Outer[0].UVStart.X += .125
	if r, e := CertifyConvexAffinePolygonShell(context.Background(), bad, ConvexAffinePolygonShellOptions{}); e == nil || r != nil {
		t.Fatal("stale chart")
	}
	concave := convexPolygonShellFixture(t, []geom.Vec2{{}, {X: 2}, {X: 1, Y: .5}, {Y: 2}})
	if r, e := CertifyConvexAffinePolygonShell(context.Background(), concave, ConvexAffinePolygonShellOptions{}); e == nil || r != nil {
		t.Fatal("nonconvex")
	}
}

// Native tolerance permits this small bow with unchanged endpoints. A convex
// support certificate must independently reject its nonzero continuous error.
func TestConvexAffinePolygonShellRejectsEndpointOnlyAgreement(t *testing.T) {
	s := convexPolygonShellFixture(t, []geom.Vec2{{}, {X: 2}, {Y: 2}})
	edges, vertices := s.Edges(), s.Vertices()
	a, c := vertices[edges[0].Start], vertices[edges[0].End]
	middle := geom.Vec3{X: (a.X + c.X) / 2, Y: (a.Y + c.Y) / 2, Z: (a.Z+c.Z)/2 + 1.0/(1<<40)}
	curve, e := geom.NewBezier([]geom.Vec3{a, middle, c}, nil)
	if e != nil {
		t.Fatal(e)
	}
	edges[0].Curve = curve
	edges[0].Range = parametric.Interval{Max: 1}
	bowed, e := NewSolid(vertices, edges, s.Faces(), 1e-10)
	if e != nil {
		t.Fatal("fixture native tolerance", e)
	}
	report, e := edgecheck.Check(context.Background(), edgecheck.Curve{Poles: []geom.Vec3{a, middle, c}}, edgecheck.Curve{Poles: []geom.Vec3{a, c}}, 1e-10, edgecheck.Options{})
	if e != nil || report.Status != edgecheck.Certified || report.SquaredUpperBound == nil || report.SquaredUpperBound.Sign() <= 0 {
		t.Fatal("independent tolerant continuous bound", report, e)
	}
	if r, e := CertifyConvexAffinePolygonShell(context.Background(), bowed, ConvexAffinePolygonShellOptions{}); e == nil || r != nil {
		t.Fatal("endpoint-only identity accepted")
	}
}
