package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRationalGraphTwoRetainedRunsOnOneStockFace(t *testing.T) {
	ctx := context.Background()
	stock := makeGraphDisconnectedStripStock(t)
	makeCut := func(h, slopeY float64, keep bool) RationalGraphHalfspace {
		t.Helper()
		const lo, hi = -1., 2.
		k := []float64{lo, lo, hi, hi}
		p := func(x, y float64) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: h + slopeY*y} }
		q, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{p(lo, lo), p(lo, hi), p(hi, lo), p(hi, hi)}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return RationalGraphHalfspace{Points: [3]geom.Vec3{p(0, 0), p(1, 0), p(0, 1)}, KeepPositive: keep, Cap: q,
			CapDomain: geom.ParameterRectangle{UMin: lo, VMin: lo, UMax: hi, VMax: hi}}
	}
	// Both planes cross the floor Z=(x−1/2)²/8+y/16. Their line
	// lies at y=1/2,z=5/128 and meets the original floor twice,
	// at x=1/4 and 3/4. The kept z-band crosses the floor in two
	// disconnected runs, while the material remains one connected shell.
	cuts := [2]RationalGraphHalfspace{makeCut(5./128, 0, true), makeCut(9./128, -.0625, false)}
	for trial := 0; trial < 2; trial++ {
		r, e := stock.ClipIntersectingRationalGraphCaps(ctx, cuts, RationalGraphSeparatedBandOptions{Sections: RationalGraphShellSectionOptions{ComposeSectionCurves: true}})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.SourceFaces[4]) != 2 || len(r.CapIntersections) != 1 || r.Solid.Topology().EulerCharacteristic != 2 || len(r.Solid.Topology().Shells) != 1 {
			t.Fatal("two source fragments, one cap chord and one exterior shell required", r.SourceFaces, r.CapIntersections, r.Solid.Topology())
		}
		for _, id := range r.SourceFaces[4] {
			if r.Solid.faces[id].Surface != stock.faces[4].Surface {
				t.Fatal("retained fragments must share the original curved support", id)
			}
		}
		// Subtract from the full wedge of volume 1/128 the two
		// triangular runs removed by the floor. Their integrated area
		// is ∫8*(f−1/128)² dx = 19/61440, f=(x−1/2)²/8.
		m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-461./61440) > 1e-12 {
			t.Fatal("independent disconnected-strip volume", m, e)
		}
		if path := os.Getenv("PLAINSOLID_OCCT_MULTI_PLANE_CUTS"); trial == 0 && path != "" {
			data, e := exec.CommandContext(ctx, path, "two-strips").Output()
			if e != nil {
				t.Fatal(e)
			}
			var oracle []float64
			if e = json.Unmarshal(data, &oracle); e != nil || len(oracle) != 8 {
				t.Fatalf("invalid OCCT result %s: %v", data, e)
			}
			p := r.Solid.Topology()
			got := []float64{m.Volume, m.Area, m.Centroid.X, m.Centroid.Y, m.Centroid.Z, float64(p.Vertices), float64(p.Edges), float64(p.Faces)}
			for i, tol := range []float64{5e-8, 3e-7, 2e-6, 5e-8, 5e-8, 0, 0, 0} {
				if math.Abs(got[i]-oracle[i]) > tol || math.IsNaN(got[i]) {
					t.Fatalf("measure %d: PlainSolid %.16g, OCCT %.16g", i, got[i], oracle[i])
				}
			}
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || len(back.Topology().Shells) != 1 || back.Topology().Faces != r.Solid.Topology().Faces {
			t.Fatal("disjoint fragments through native reload", e)
		}
		cuts[0], cuts[1] = cuts[1], cuts[0]
	}
}

func makeGraphDisconnectedStripStock(t *testing.T) Solid {
	t.Helper()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	k := []float64{0, 0, 0, 1, 1, 1}
	for fixed := range 3 {
		u, v := (fixed+1)%3, (fixed+2)%3
		for side := range 2 {
			var poles []geom.Vec3
			for i := range 3 {
				for j := range 3 {
					var p [3]float64
					p[fixed], p[u], p[v] = float64(side), float64(i)/2, float64(j)/2
					f := (p[0] - .5) * (p[0] - .5) / 8
					if fixed != 0 && p[0] == .5 {
						f = -1. / 32
					}
					poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2] + p[1]/16 + f})
				}
			}
			q, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, domains, reversed = append(surfaces, q), append(domains, geom.ParameterRectangle{UMax: 1, VMax: 1}), append(reversed, side == 0)
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
