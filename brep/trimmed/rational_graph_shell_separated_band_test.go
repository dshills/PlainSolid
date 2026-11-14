package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestClipSeparatedRationalGraphSplineShell(t *testing.T) {
	ctx := context.Background()
	stock, _ := polynomialClipStock(t)
	// Both cuts cross the curved lower stock face y = 4z−x³/2 in
	// the same exact chart direction. The slopes in X differ; their
	// intersection line lies outside the stock for the first two cuts.
	makeCut := func(z, slope float64, keep bool) RationalGraphHalfspace {
		t.Helper()
		tool := slabTool(t, true, z, 0)
		tool, e := tool.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {slope, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		return RationalGraphHalfspace{
			Points: [3]geom.Vec3{{Z: z}, {X: 1, Z: z + slope}, {Y: 1, Z: z}}, KeepPositive: keep,
			Cap: tool.faces[4].Surface, CapDomain: geom.ParameterRectangle{UMin: -1, VMin: -1, UMax: 2, VMax: 2},
		}
	}
	cuts := [2]RationalGraphHalfspace{makeCut(3./16, 0, true), makeCut(7./32, 1./128, false)}
	var oracle []float64
	if path := os.Getenv("PLAINSOLID_OCCT_MULTI_PLANE_CUTS"); path != "" {
		data, e := exec.CommandContext(ctx, path, "curved-band").Output()
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(data, &oracle); e != nil || len(oracle) != 8 {
			t.Fatalf("invalid OCCT result %s: %v", data, e)
		}
	}
	for trial := 0; trial < 2; trial++ {
		r, e := stock.ClipSeparatedRationalGraphSplineShell(ctx, cuts, RationalGraphSeparatedBandOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.SourceFaces[4]) != 1 || r.Solid.faces[r.SourceFaces[4][0]].Surface != stock.faces[4].Surface || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("clipped lower cubic stock face or shell was lost", r.SourceFaces, r.Solid.Topology())
		}
		seen := make([]int, len(r.Solid.edges))
		for _, row := range r.SourceEdges {
			for _, id := range row {
				seen[id]++
			}
		}
		for i, sec := range r.Sections {
			if !sec.Complete || len(sec.Wires) != 1 || len(r.CapFaces[i]) != 1 || r.Solid.faces[r.CapFaces[i][0]].Surface != cuts[i].Cap {
				t.Fatal("missing original cap or complete section", i)
			}
			for _, id := range r.CapEdges[i] {
				seen[id]++
			}
		}
		for id, count := range seen {
			if count != 1 {
				t.Fatal("edge without unique ancestry", id, count)
			}
		}
		// Integrate lower-to-upper over 0≤x≤1, with cross section width
		// 4*z−x³/2 and z∈[3/16,7/32+x/128].
		mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-2969./122880) > 1e-9 {
			t.Fatal("independent curved volume", mass, e)
		}
		// Derive the five flat areas from the closed-form widths and use
		// an independent composite midpoint integral for the curved floor.
		const a, b, c = 3. / 16, 7. / 32, 1. / 128
		area := (4*a - 1./8) + (4*b+2*c-1./8)*math.Sqrt(1+c*c) +
			2*(b*b-a*a) + 2*((b+c)*(b+c)-a*a) - (b+c-a)/2 + b + c/2 - a
		const n = 8192
		for j := 0; j < n; j++ {
			x := (float64(j) + .5) / n
			area += 4 * (b + c*x - a) * math.Sqrt(1+1./16+9*x*x*x*x/64) / n
		}
		if math.Abs(mass.Area-area) > 1e-9 {
			t.Fatal("independent curved area", mass.Area, area)
		}
		if len(oracle) > 0 {
			p := r.Solid.Topology()
			got := []float64{mass.Volume, mass.Area, mass.Centroid.X, mass.Centroid.Y, mass.Centroid.Z, float64(p.Vertices), float64(p.Edges), float64(p.Faces)}
			// OCCT's intersected trims differ from the independent analytic
			// volume by 5.5e-8 and the area by about 2.7e-6.
			for i, tol := range []float64{8e-8, 4e-6, 1e-6, 1e-6, 1e-6, 0, 0, 0} {
				if math.Abs(got[i]-oracle[i]) > tol || math.IsNaN(got[i]) {
					t.Fatalf("measure %d: PlainSolid %.16g, OCCT %.16g", i, got[i], oracle[i])
				}
			}
			oracle = nil
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &native, JSONLimits{})
		if e != nil || back.Topology().Edges != r.Solid.Topology().Edges {
			t.Fatal("native nonparallel band round trip", e)
		}
		// Change order and reverse one chart normal with its keep sign.
		cuts[0], cuts[1] = cuts[1], cuts[0]
		cuts[0].Points[1], cuts[0].Points[2] = cuts[0].Points[2], cuts[0].Points[1]
		cuts[0].KeepPositive = !cuts[0].KeepPositive
	}
	for _, slope := range []float64{1. / 16, 1. / 32} {
		bad := [2]RationalGraphHalfspace{makeCut(3./16, 0, true), makeCut(5./32, slope, true)}
		bad[1].KeepPositive = false
		if out, e := stock.ClipSeparatedRationalGraphSplineShell(ctx, bad, RationalGraphSeparatedBandOptions{}); !errors.Is(e, geom.ErrConvergence) || out.Sections[0].Complete || len(out.Solid.edges) != 0 {
			t.Fatal("crossing or touching caps must reject atomically", slope, e)
		}
	}
	bad := [2]RationalGraphHalfspace{makeCut(3./16, 0, true), makeCut(7./32, 1./128, false)}
	bad[1].Cap = parametric.MapSurface(bad[1].Cap, geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 1. / 1024}})
	if out, e := stock.ClipSeparatedRationalGraphSplineShell(ctx, bad, RationalGraphSeparatedBandOptions{}); !errors.Is(e, geom.ErrDomain) || out.Sections[0].Complete {
		t.Fatal("independently shifted cap support must reject", e)
	}
}
