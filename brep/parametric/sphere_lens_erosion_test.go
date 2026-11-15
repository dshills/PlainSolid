package parametric

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"plainsolid/geom"
)

func sphericalLensFixture(t *testing.T) SpherePairSet {
	t.Helper()
	a := Ball{Radius: 2}
	b := Ball{Center: geom.Vec3{Y: 2}, Radius: 2.5}
	source, err := NewSpherePairSet(context.Background(), a, b, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestSphericalLensErosion(t *testing.T) {
	ctx := context.Background()
	source := sphericalLensFixture(t)
	solid, history, err := source.ErodeSphericalLens(ctx, .25, 1e-9)
	if err != nil || history.SourceFaces != [2]int{0, 1} || history.ResultFaces != history.SourceFaces {
		t.Fatalf("lens ancestry = %+v, %v", history, err)
	}
	for i, face := range solid.Faces() {
		support, ok := face.Surface.(geom.SphereSurface)
		if !ok || face.Reversed {
			t.Fatalf("lens owner %d changed support or orientation", i)
		}
		frame, radius := support.Parameters()
		want := [2]float64{1.75, 2.25}[i]
		if frame.Origin() != (geom.Vec3{Y: float64(2 * i)}) || radius != want {
			t.Fatalf("lens owner %d at %v has radius %v", i, frame.Origin(), radius)
		}
	}
	// The intersection plane at y=1/2 cuts cap heights 5/4 and 3/4.
	const ra, rb, ha, hb = 1.75, 2.25, 1.25, .75
	volume := math.Pi * (ha*ha*(ra-ha/3) + hb*hb*(rb-hb/3))
	area := 2 * math.Pi * (ra*ha + rb*hb)
	properties, err := solid.MassProperties(ctx, IntegrationOptions{})
	if err != nil || math.Abs(properties.Volume-volume) > 1e-8 || math.Abs(properties.Area-area) > 1e-8 {
		t.Fatalf("eroded lens mass=%+v, want V=%v A=%v, error=%v", properties, volume, area, err)
	}
	for _, depth := range []float64{1.25, 1.5, 3} {
		empty, report, err := source.ErodeSphericalLens(ctx, depth, 1e-9)
		if err != nil || len(empty.Faces()) != 0 || report.ResultFaces != [2]int{-1, -1} || report.SourceFaces != history.SourceFaces {
			t.Fatalf("depth %v retained collapsed material: %+v, %v", depth, report, err)
		}
	}
	if _, report, err := source.ErodeSphericalLens(ctx, .1, 1e-9); !errors.Is(err, geom.ErrUnrepresentable) || report.Radii[0] != nil {
		t.Fatalf("unrepresentable result = %+v, %v", report, err)
	}
	if _, _, err := sphericalOffsetFixture(t).ErodeSphericalLens(ctx, .25, 1e-9); !errors.Is(err, geom.ErrDomain) {
		t.Fatalf("cavity was accepted as an intersection: %v", err)
	}
}

func TestSphericalLensErosionOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET to the optional OCCT comparator")
	}
	source := sphericalLensFixture(t)
	for _, row := range []struct {
		depth  float64
		solids int
	}{{.25, 1}, {1.25, 0}, {1.5, 0}} {
		solid, _, err := source.ErodeSphericalLens(context.Background(), row.depth, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		checkSphereOffsetAgainstOCCT(t, bin, "lens-supports", row.depth, row.solids, solid)
	}
}
