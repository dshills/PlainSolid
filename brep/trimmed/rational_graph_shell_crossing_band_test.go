package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestClipCrossingRationalGraphSplineShell(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	makeCut := func(h, slope float64, positive bool) RationalGraphHalfspace {
		t.Helper()
		s, e := slabTool(t, true, h, 0).Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {slope, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		return RationalGraphHalfspace{
			Points: [3]geom.Vec3{{Z: h}, {X: 1, Z: h + slope}, {Y: 1, Z: h}}, KeepPositive: positive,
			Cap: s.faces[4].Surface, CapDomain: geom.ParameterRectangle{UMin: -1, VMin: -1, UMax: 2, VMax: 2},
		}
	}
	// Use an independently authored box for the first end-to-end crossing
	// case: the two chord endpoints lie strictly inside affine stock faces.
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	cuts := [2]RationalGraphHalfspace{makeCut(3./8, .25, true), makeCut(.5, 0, false)}
	for trial := 0; trial < 2; trial++ {
		r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(e)
		}
		got, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(got.Volume-1./32) > 1e-12 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("exact taper volume and shell topology", got, e)
		}
		if len(r.CapEdges[0]) != 3 || len(r.CapEdges[1]) != 3 || r.CapIntersection.Edge < 0 {
			t.Fatal("one cap seam must be separate from all stock/cap edges", r.CapEdges, r.CapIntersection)
		}
		for i, p := range r.CapIntersection.Points {
			if p[0].Cmp(ratInset(.5)) != 0 || p[2].Cmp(ratInset(.5)) != 0 || p[1].Cmp(ratInset(0)) != 0 && p[1].Cmp(ratInset(1)) != 0 {
				t.Fatal("wrong exact chord endpoint", i, p)
			}
		}
		for i, f := range r.CapIntersection.Faces {
			if r.Solid.faces[f].Surface != cuts[i].Cap {
				t.Fatal("lost source cap chart", i, f)
			}
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || reload.Topology().Edges != r.Solid.Topology().Edges {
			t.Fatal("native crossing-cap reload", e)
		}
		cuts[0], cuts[1] = cuts[1], cuts[0]
	}
	tangent := [2]RationalGraphHalfspace{makeCut(3./8, .25, true), makeCut(5./8, 0, false)}
	if r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, tangent, RationalGraphSeparatedBandOptions{}); !errors.Is(e, geom.ErrConvergence) || len(r.Solid.faces) != 0 || r.Sections[0].Complete {
		t.Fatal("coincident original section terminal rejects atomically", e)
	}
	stock, _ = polynomialClipStock(t)
	// The lower plane, z=3/16, and the upper plane z=5/32+x/16
	// both cross the cubic floor; their line runs across that floor and
	// the y=0 side. The retained material is on the x≥1/2 end.
	cuts = [2]RationalGraphHalfspace{makeCut(3./16, 0, true), makeCut(5./32, 1./16, false)}
	for trial := 0; trial < 2; trial++ {
		r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.SourceFaces[4]) != 1 || r.Solid.faces[r.SourceFaces[4][0]].Surface != stock.faces[4].Surface {
			t.Fatal("curved stock must retain its original surface", r.SourceFaces[4])
		}
		mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		// Integrate 4*z−x³/2 over x∈[.5,1] between the two cuts.
		if e != nil || math.Abs(mass.Volume-233./61440) > 1e-10 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("independent curved taper integral", mass, e)
		}
		if trial == 0 {
			many, e := stock.ClipIntersectingRationalGraphCaps(ctx, cuts, RationalGraphSeparatedBandOptions{})
			if e != nil || len(many.CapIntersections) != 1 || many.Solid.Topology().Edges != r.Solid.Topology().Edges {
				t.Fatal("rational arrangement must retain the same certified cubic solid", len(many.CapIntersections), e)
			}
			m, e := many.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(m.Volume-233./61440) > 1e-10 {
				t.Fatal("curved arrangement altered material", m, e)
			}
		}
		if path := os.Getenv("PLAINSOLID_OCCT_MULTI_PLANE_CUTS"); trial == 0 && path != "" {
			data, e := exec.CommandContext(ctx, path, "crossing-band").Output()
			if e != nil {
				t.Fatal(e)
			}
			var oracle []float64
			if e = json.Unmarshal(data, &oracle); e != nil || len(oracle) != 8 {
				t.Fatalf("invalid OCCT result %s: %v", data, e)
			}
			p := r.Solid.Topology()
			got := []float64{mass.Volume, mass.Area, mass.Centroid.X, mass.Centroid.Y, mass.Centroid.Z, float64(p.Vertices), float64(p.Edges), float64(p.Faces)}
			for i, tol := range []float64{4e-8, 4e-6, 3e-6, 3e-6, 3e-6, 0, 0, 0} {
				if math.Abs(got[i]-oracle[i]) > tol || math.IsNaN(got[i]) {
					t.Fatalf("measure %d: PlainSolid %.16g, OCCT %.16g", i, got[i], oracle[i])
				}
			}
		}
		for _, p := range r.CapIntersection.Points {
			if p[0].Cmp(ratInset(.5)) != 0 || p[2].Cmp(ratInset(3./16)) != 0 || p[1].Cmp(ratInset(0)) != 0 && p[1].Cmp(ratInset(11./16)) != 0 {
				t.Fatal("cap chord must end at the exact cubic floor", p)
			}
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || back.Topology().Faces != r.Solid.Topology().Faces {
			t.Fatal("curved taper reload", e)
		}
		cuts[0], cuts[1] = cuts[1], cuts[0]
	}
	for _, c := range []struct {
		above  bool
		volume float64
	}{
		// The upper envelope subtracts the gap integrated over x≥1/2
		// from z≥3/16; the lower envelope subtracts the gap over x≤1/2
		// from z≤3/16. Those gaps are 233/61440 and 337/61440.
		{true, 406369. / 430080},
		{false, 19721. / 430080},
	} {
		for i := range cuts {
			cuts[i].KeepPositive = c.above
		}
		r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(c.above, e)
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-c.volume) > 1e-9 || len(r.SourceFaces[4]) != 1 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("crossing graph envelope volume/topology", c.above, m, r.SourceFaces, e)
		}
		var boundaryPieces int
		for _, u := range r.Solid.faces[r.SourceFaces[4][0]].Outer {
			if u.Curve != nil {
				boundaryPieces++
			}
		}
		if boundaryPieces != 2 {
			t.Fatal("both graph pieces must survive in the curved stock envelope", c.above, boundaryPieces)
		}
	}
}

func TestClipCrossingRationalGraphNonbinaryVertices(t *testing.T) {
	ctx := context.Background()
	stock, _ := quadraticClipStock(t)
	var cuts [2]RationalGraphHalfspace
	for i, slope := range []float64{0, 3. / 32} {
		h := (3. - float64(i)) / 16
		tool, e := slabTool(t, true, h, 0).Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {slope, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		cuts[i] = RationalGraphHalfspace{Points: [3]geom.Vec3{{Z: h}, {X: 1, Z: h + slope}, {Y: 1, Z: h}}, KeepPositive: i == 0,
			Cap: tool.faces[4].Surface, CapDomain: geom.ParameterRectangle{UMin: -1, VMin: -1, UMax: 2, VMax: 2}}
	}
	if out, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{}); e == nil || len(out.Solid.faces) != 0 {
		t.Fatal("default mode must reject nonbinary junctions atomically", e)
	}
	o := RationalGraphSeparatedBandOptions{Caps: RationalGraphSectionCapOptions{RationalEndpoints: true}}
	r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range r.CapIntersection.Points {
		if p[0].Cmp(big.NewRat(2, 3)) != 0 || p[2].Cmp(big.NewRat(3, 16)) != 0 || p[1].Sign() != 0 && p[1].Cmp(big.NewRat(19, 36)) != 0 {
			t.Fatal("nonbinary exact model endpoint", p)
		}
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, r.Solid); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &data, JSONLimits{})
	if e != nil {
		t.Fatal("reload", e)
	}
	exact, e := back.RationalVertices(ctx)
	if e != nil {
		t.Fatal("reload exact endpoints", e)
	}
	start, end := back.edges[r.CapIntersection.Edge].Start, back.edges[r.CapIntersection.Edge].End
	count := 0
	for _, v := range exact {
		if v.Vertex == start && rationalGraphSamePoint(v.Point, r.CapIntersection.Points[0]) || v.Vertex == end && rationalGraphSamePoint(v.Point, r.CapIntersection.Points[1]) {
			count++
		}
	}
	if count != 2 {
		t.Fatal("both exact seam vertices must reload", count, exact)
	}
}

func TestClipCrossingRationalGraphThroughHole(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock := makeSquareRationalGraphTube(t)
	makeCut := func(h, slope float64, positive bool) RationalGraphHalfspace {
		t.Helper()
		const lo, hi = -1., 4.
		k := []float64{lo, lo, hi, hi}
		p := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: h + slope*x} }
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p(lo, lo), p(lo, hi), p(hi, lo), p(hi, hi)}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return RationalGraphHalfspace{Points: [3]geom.Vec3{p(0, 0), p(1, 0), p(0, 1)}, KeepPositive: positive,
			Cap: s, CapDomain: geom.ParameterRectangle{UMin: lo, VMin: lo, UMax: hi, VMax: hi}}
	}
	for _, fixture := range []struct {
		x, volume float64
		holes     int
	}{{.5, 67. / 64, 1}, {2.5, 3. / 64, 0}} {
		cuts := [2]RationalGraphHalfspace{makeCut(.25, 0, true), makeCut(.25-fixture.x/8, .125, false)}
		if fixture.x == 2.5 {
			// Shift both planes to keep each original section wholly within
			// the wall height. The thickness is unchanged.
			cuts = [2]RationalGraphHalfspace{makeCut(.5, 0, true), makeCut(.5-fixture.x/8, .125, false)}
		}
		r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(fixture, e)
		}
		for _, id := range r.CapIntersection.Faces {
			if len(r.Solid.faces[id].Holes) != fixture.holes {
				t.Fatal("wrong cap hole ancestry", fixture, id)
			}
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-fixture.volume) > 1e-11 || r.Solid.Topology().EulerCharacteristic != 2-2*fixture.holes {
			t.Fatal("square sleeve exact volume/genus", fixture, m, r.Solid.Topology(), e)
		}
	}
	cuts := [2]RationalGraphHalfspace{makeCut(.5, 0, true), makeCut(.3125, .125, false)}
	if r, e := stock.ClipCrossingRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{}); !errors.Is(e, geom.ErrConvergence) || len(r.Solid.faces) != 0 {
		t.Fatal("a plane crossing the inner loop must reject atomically", e)
	}
	for trial := 0; trial < 2; trial++ {
		r, e := stock.ClipIntersectingPolygonalGraphCaps(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		// Full wedge 27/64 minus 1/64 of the original void.
		if e != nil || math.Abs(m.Volume-13./32) > 1e-11 || r.Solid.Topology().EulerCharacteristic != 2 || len(r.CapIntersections) != 2 {
			t.Fatal("the cut void opens and produces two exact cap seams", m, r.Solid.Topology(), len(r.CapIntersections), e)
		}
		if path := os.Getenv("PLAINSOLID_OCCT_MULTI_PLANE_CUTS"); trial == 0 && path != "" {
			data, e := exec.CommandContext(ctx, path, "sleeve").Output()
			if e != nil {
				t.Fatal(e)
			}
			var oracle []float64
			if e = json.Unmarshal(data, &oracle); e != nil || len(oracle) != 8 {
				t.Fatalf("invalid OCCT result %s: %v", data, e)
			}
			// The Go source has split rectangular charts along the side
			// walls, so compare material measures and Euler, not raw IDs.
			for i, got := range []float64{m.Volume, m.Area, m.Centroid.X, m.Centroid.Y, m.Centroid.Z, float64(r.Solid.Topology().EulerCharacteristic)} {
				want := oracle[i]
				if i == 5 {
					want = oracle[5] - oracle[6] + oracle[7]
				}
				if math.Abs(got-want) > 1e-11 || math.IsNaN(got) {
					t.Fatalf("measure %d: PlainSolid %.16g, OCCT %.16g", i, got, want)
				}
			}
		}
		claimed := make([]int, len(r.Solid.edges))
		for _, group := range append(append([][]int(nil), r.SourceEdges...), r.CapEdges[:]...) {
			for _, id := range group {
				claimed[id]++
			}
		}
		for _, seam := range r.CapIntersections {
			claimed[seam.Edge]++
			for _, p := range seam.Points {
				if p[0].Cmp(ratInset(1.5)) != 0 || p[2].Cmp(ratInset(.5)) != 0 {
					t.Fatal("unproved exact shared cap endpoint", p)
				}
			}
			for i, f := range seam.Faces {
				if r.Solid.faces[f].Surface != cuts[i].Cap || len(r.Solid.faces[f].Holes) != 0 {
					t.Fatal("lost original cap or uncut void", i, f)
				}
			}
		}
		for i, n := range claimed {
			if n != 1 {
				t.Fatal("one complete ownership row per output edge", i, n)
			}
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil || back.Topology().Edges != r.Solid.Topology().Edges {
			t.Fatal("multi-seam native roundtrip", e)
		}
		cuts[0], cuts[1] = cuts[1], cuts[0]
	}
}
