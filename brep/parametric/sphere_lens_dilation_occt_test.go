package parametric

import (
	"context"
	"os"
	"testing"

	"plainsolid/geom"
)

func TestSphericalLensDilationOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERICAL_CAVITY_OFFSET to the optional OCCT comparator")
	}
	ctx := context.Background()
	source, err := NewSpherePairSet(ctx, Ball{Radius: 5}, Ball{Center: geom.Vec3{Z: 6}, Radius: 5}, Intersection, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	for _, distance := range []float64{.5, 4, 12} {
		result, _, err := source.DilateSphericalLens(ctx, distance, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		checkSphereOffsetAgainstOCCT(t, bin, "lens-joined-dilation", distance, 1, result)
	}
}
