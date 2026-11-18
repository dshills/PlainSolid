package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeAcnodalCubicCut(t *testing.T) {
	ctx := context.Background()
	// For the authored law y(t)=(1/8+t²)*t and du=2t dt.
	// Integrating 6*(3/16)*(4/3)*y(t)^3 du from 0 to 1/4 gives
	// 4703/1614807040. A changed source reverses the law, doubles the
	// height gap and puts the cubic roof first on its own U clock.
	for i, fixture := range []struct {
		source    [3]*geom.BSplineSurface
		firstRoof int
		freeAxis  int
	}{
		{acnodalCubicStockFixture(t, true), 2, 1},
		{nodalCubicStockFixture(t, [4]float64{7605.0 / 32768, -1339.0 / 32768, 213.0 / 32768, -27.0 / 32768}, -3.0/32), 1, 0},
	} {
		source := fixture.source
		roofs := [2]*geom.BSplineSurface{source[fixture.firstRoof], source[3-fixture.firstRoof]}
		cut, err := CutAcnodalCubicSplineGraphs(ctx, source[0], roofs[0], roofs[1], fixture.freeAxis, 1e-9, AcnodalCubicCutOptions{})
		if err != nil {
			t.Fatal(i, err)
		}
		want := big.NewRat(int64(i+1)*4703, 1614807040)
		if cut.ExactVolume().Cmp(want) != 0 {
			t.Fatal("original integrated material differs", i, cut.ExactVolume(), want)
		}
		checkNativeAcnodalCut(t, cut, roofs)
		mass, err := cut.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		w, _ := want.Float64()
		if err != nil || !(math.Abs(mass.Volume-w) <= 1e-13) || mass.Area <= 0 {
			t.Fatal("native boundary differs from independent exact cap measure", i, mass, want, err)
		}
		t.Logf("source %d exact volume %s integrated %.16g area %.16g", i, want, mass.Volume, mass.Area)
	}
}

func checkNativeAcnodalCut(t *testing.T, cut *AcnodalCubicCut, roofs [2]*geom.BSplineSurface) {
	t.Helper()
	s, h := cut.Boundary(), cut.History()
	topo := s.Topology()
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 2 || topo.CollapsedEdges != 0 ||
		len(s.faces) != 4 || len(s.vertices) != 5 || len(s.edges) != 7 {
		t.Fatal("retained cap must be one closed Euler-2 shell", topo)
	}
	p, err := geom.RepresentedExactPoint(cut.IsolatedContact())
	if err != nil || p != (geom.Vec3{X: 39.0 / 16, Y: 1, Z: 2243.0 / 1024}) || slices.Contains(s.vertices, p) {
		t.Fatal("isolated contact became a boundary occurrence", p, err)
	}
	for owner, face := range h.RoofFaces {
		if s.faces[face].Surface != roofs[owner] {
			t.Fatal("cap substituted one of the original roof supports", owner)
		}
	}
	xy := [3]geom.Vec2{{X: 45.0 / 16, Y: 1}, {X: 3, Y: 29.0 / 32}, {X: 3, Y: 35.0 / 32}}
	for i, vertex := range h.ArcVertices {
		q := s.vertices[vertex]
		if q.X != xy[i].X || q.Y != xy[i].Y || q.Z != 2+q.X/16+q.X*q.Y/64 {
			t.Fatal("the regular arc moved from its original height and footprint", i, q)
		}
	}
	for i, id := range h.ArcEdges {
		e := s.edges[id]
		if !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) ||
			!slices.Contains([]int{e.Start, e.End}, h.ArcVertices[i+1]) {
			t.Fatal("regular arc lost an original coedge or terminal", i, e)
		}
		at := e.Range.Min
		if e.End == h.ArcVertices[0] {
			at = e.Range.Max
		} else if e.Start != h.ArcVertices[0] {
			t.Fatal("regular arc does not meet its native turn", i, e)
		}
		v, err := e.Curve.Derivative(at)
		if err != nil || v.X != 0 || math.Abs(v.Y) != .0625 {
			t.Fatal("the regular turn was treated as a singular cusp", i, v, err)
		}
	}
	for _, id := range h.WallFaces {
		face := s.faces[id]
		wall, ok := face.Surface.(splineGraphAffineWall)
		if !ok || wall.origin.X != 3 || wall.along.X != 0 {
			t.Fatal("closing wall left the original stock side", id)
		}
		seen := [2]bool{}
		for _, use := range face.Outer {
			for owner := range seen {
				trim, ok := graphWallSourceTrim(use.Curve, owner == 0, true)
				seen[owner] = seen[owner] || (ok && trim.surface == roofs[owner])
			}
		}
		if seen != ([2]bool{true, true}) {
			t.Fatal("closing wall lost an owner coedge", id, seen)
		}
	}
}

func TestNativeAcnodalCubicCutOtherStockSides(t *testing.T) {
	ctx := context.Background()
	stock := acnodalCubicStockFixture(t, true)
	o := AcnodalCubicCutOptions{}
	want := big.NewRat(4703, 1614807040)
	for _, fixture := range []struct {
		name  string
		place func(geom.Vec3) geom.Vec3
	}{
		{"left", func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: 3 - p.X, Y: p.Y, Z: p.Z} }},
		{"upper", func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: p.X, Z: p.Z} }},
		{"lower", func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: p.Y, Y: 3 - p.X, Z: p.Z} }},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var source [3]*geom.BSplineSurface
			for i, patch := range stock {
				d := patch.Data()
				for j, p := range d.Poles {
					d.Poles[j] = fixture.place(p)
				}
				var err error
				source[i], err = geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
				if err != nil {
					t.Fatal(err)
				}
			}
			r, err := CutAcnodalCubicSplineGraphs(ctx, source[0], source[2], source[1], 1, 1e-9, o)
			if err != nil || r.ExactVolume().Cmp(want) != 0 {
				t.Fatal("reflected or swapped original cap lost positive material", err)
			}
			s, h := r.Boundary(), r.History()
			if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 || len(h.WallFaces) != 2 {
				t.Fatal("reflected or swapped cap changed material topology", s.Topology())
			}
			for i, p := range [3]geom.Vec3{
				{X: 45.0 / 16, Y: 1, Z: 2273.0 / 1024},
				{X: 3, Y: 29.0 / 32, Z: 4567.0 / 2048},
				{X: 3, Y: 35.0 / 32, Z: 4585.0 / 2048},
			} {
				if s.vertices[h.ArcVertices[i]] != fixture.place(p) {
					t.Fatal("original signed arc order changed", i)
				}
			}
			p, q := fixture.place(geom.Vec3{X: 3}), fixture.place(geom.Vec3{X: 3, Y: 2})
			for _, id := range h.WallFaces {
				wall, ok := s.faces[id].Surface.(splineGraphAffineWall)
				if !ok || (p.X == q.X && (wall.origin.X != p.X || wall.along.X != 0)) ||
					(p.Y == q.Y && (wall.origin.Y != p.Y || wall.along.Y != 0)) {
					t.Fatal("wall left its original stock side", id, wall)
				}
			}
			w, _ := want.Float64()
			m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || !(math.Abs(m.Volume-w) <= 1e-13) {
				t.Fatal("physical placement changed signed cap mass", m.Volume, err)
			}
			if out, err := CutAcnodalCubicSplineGraphs(ctx, source[0], source[1], source[2], 0, 1e-9, o); !errors.Is(err, geom.ErrDegenerate) || out != nil {
				t.Fatal("reflected or swapped exterior still pinches", err)
			}
		})
	}
}

func TestNativeAcnodalCubicCutRefusals(t *testing.T) {
	ctx := context.Background()
	o := AcnodalCubicCutOptions{}
	for _, arc := range []bool{false, true} {
		src := acnodalCubicStockFixture(t, arc)
		if out, err := CutAcnodalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o); !errors.Is(err, geom.ErrDegenerate) || out != nil {
			t.Fatal("the exterior contact pinches one component", arc, err)
		}
		if !arc {
			if out, err := CutAcnodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o); !errors.Is(err, geom.ErrDomain) || out != nil {
				t.Fatal("a zero-thickness isolated contact does not make a positive Cut", err)
			}
		}
	}
	// The amplitude change makes the exact terminal clock 1/5. The
	// physical exit ordinate has denominator five, so no vertex can
	// be placed at that original point without rounding.
	nonbinary := nodalCubicStockFixture(t, [4]float64{-7605.0 / 65536, 1339.0 / 65536, -213.0 / 65536, 27.0 / 65536}, 75.0/1024)
	if out, err := CutAcnodalCubicSplineGraphs(ctx, nonbinary[0], nonbinary[2], nonbinary[1], 1, 1e-9, o); !errors.Is(err, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("nonbinary original arc exit was rounded", err)
	}
	src := acnodalCubicStockFixture(t, true)
	o.Proof.MaxOperations = 1
	if out, err := CutAcnodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o); err == nil || out != nil {
		t.Fatal("truncated source proof returned a partial regular cap", err)
	}
}
