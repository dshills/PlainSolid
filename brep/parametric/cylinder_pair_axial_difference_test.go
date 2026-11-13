package parametric

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestFiniteCylinderAxialCrossSectionCuts(t *testing.T) {
	ctx := context.Background()
	stock := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: -2, Z: 5}, Axis: geom.Vec3{Z: -3}, Min: 0, Max: 3, Radius: 2}
	// The eccentric wider cutter is exactly internally tangent to the stock
	// disk; in a fully removed axial slab that tangent does not survive.
	cutter := geom.RawCylinder{Origin: geom.Vec3{X: 2, Y: -2}, Axis: geom.Vec3{Z: 2}, Min: -.5, Max: .5, Radius: 3}
	for _, tc := range []struct {
		name        string
		min, max    float64
		cap         [2]*FiniteCylinderCutCap
		wantHeights []float64
		wantZ       []float64
		gap         string
	}{
		{"split", -.5, .5, [2]*FiniteCylinderCutCap{{1, 1}, {0, 2}}, []float64{4, 3}, []float64{3, -2.5}, "4"},
		{"first only", -3, .5, [2]*FiniteCylinderCutCap{nil, {0, 2}}, []float64{4}, []float64{3}, ""},
		{"second only", -.5, 3, [2]*FiniteCylinderCutCap{{0, 1}, nil}, []float64{3}, []float64{-2.5}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cutter.Min, cutter.Max = tc.min, tc.max
			r, err := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range tc.cap {
				got := r.AxialCutCaps[i]
				if (got == nil) != (want == nil) || got != nil && *got != *want {
					t.Fatalf("cap %d: got %v, want %v", i, got, want)
				}
				if got != nil {
					f, ok := PolarPlaneFrame(r.Solids[got.Solid].Faces()[got.Face].Surface)
					if !ok || f.Origin().Z != 2*[2]float64{tc.min, tc.max}[i] {
						t.Fatal("original cutter cap plane", i, f, ok)
					}
				}
			}
			if tc.gap == "" && r.MinimumGapSquared != nil || tc.gap != "" && (r.MinimumGapSquared == nil || r.MinimumGapSquared.RatString() != tc.gap) || r.ContactNodes != nil {
				t.Fatal("component separation", r.MinimumGapSquared, r.ContactNodes)
			}
			for i, s := range r.Solids {
				if i == len(tc.wantHeights) {
					if len(s.Faces()) != 0 {
						t.Fatal("unexpected second body")
					}
					break
				}
				m, err := s.MassProperties(ctx, IntegrationOptions{})
				if err != nil || math.Abs(m.Volume-4*math.Pi*tc.wantHeights[i]) > 1e-8 || m.Centroid.Distance(geom.Vec3{X: 1, Y: -2, Z: tc.wantZ[i]}) > 1e-8 {
					t.Fatal("retained cylinder", i, m, err)
				}
				if len(r.StockWallFaces[i]) != 1 || len(r.StockCapFaces[i]) != 1 || len(r.CutterCapFaces[i]) != 1 || len(r.CutterWallFaces[i]) != 0 {
					t.Fatal("retained boundary ownership")
				}
				mesh, err := s.Tessellate(ctx, TessellationOptions{Deflection: .1})
				if err != nil || mesh.ValidateSolid() != nil {
					t.Fatal("closed retained cylinder mesh", err)
				}
			}
		})
	}
	cutter.Min, cutter.Max = -.5, .5
	cutter.Origin.X = math.Nextafter(2, 3) // Exact proof must see the uncovered sliver.
	if r, err := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9); err == nil || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("partial disk must not use the full-section path", err)
	}
	cutter.Origin.X = 2
	cutter.Min, cutter.Max = 2.5, 3.5
	if r, err := FiniteCylinderPairDifference(ctx, stock, cutter, 1e-9); err != nil || len(r.StockCapFaces[0]) != 2 || r.AxialCutCaps != [2]*FiniteCylinderCutCap{} {
		t.Fatal("cap-only contact must retain the original cylinder", err)
	}
	cutter.Min, cutter.Max = -.5, .5
	if r, err := FiniteCylinderPairDifference(ctx, stock, cutter, .3); !errors.Is(err, geom.ErrDomain) || len(r.Solids[0].Faces()) != 0 {
		t.Fatal("requested clearance exceeds the removed gap", err)
	}
}
