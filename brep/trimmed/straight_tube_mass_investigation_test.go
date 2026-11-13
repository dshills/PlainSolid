package trimmed

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// Isolated opt-in investigation, not a production optimization. The only
// removed integration strips are certified affine UV paths; model-space
// collinearity alone is deliberately insufficient.
func TestStraightTubeMassInvestigation(t *testing.T) {
	if os.Getenv("PLAINSOLID_PROFILE_STRAIGHT_TUBES") != "1" {
		t.Skip("opt-in exact straight-tube mass comparison")
	}
	ctx := context.Background()
	a, b, sections := multipleOpenSplineFixture(t)
	original, _, e := CombineOpenSplineShells(ctx, a, b, parametric.Intersection, sections, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal(e)
	}
	started := time.Now()
	before, e := original.MassProperties(ctx, parametric.IntegrationOptions{})
	slow := time.Since(started)
	if e != nil {
		t.Fatal(e)
	}
	faces := original.Faces()
	cache := map[intersect.TubeRecipe][]intersect.TubeSegment{}
	count := 0
	started = time.Now()
	for i := range faces {
		for _, loop := range append([][]Use{faces[i].Outer}, faces[i].Holes...) {
			for j := range loop {
				trim, _, ok := trimTube(loop[j].Curve)
				if !ok {
					continue
				}
				recipe, index, _ := trim.Recipe()
				segments, ok := cache[recipe]
				if !ok {
					segments, e = intersect.RebuildTubeCurves(ctx, recipe.Data())
					if e != nil {
						t.Fatal(e)
					}
					cache[recipe] = segments
				}
				segment := segments[index]
				start, e := segment.EvaluatePair(0)
				if e != nil {
					t.Fatal(e)
				}
				end, e := segment.EvaluatePair(1)
				if e != nil {
					t.Fatal(e)
				}
				certificate, e := segment.CertifyAffineLine(ctx, intersect.Parameters{A: start.A, B: start.B}, intersect.Parameters{A: end.A, B: end.B}, geom.TubeAffineLineOptions{})
				if e != nil || !certificate.Proven {
					t.Fatal("straight UV identity unproved", e)
				}
				loop[j].Curve = nil
				count++
			}
		}
	}
	proofTime := time.Since(started)
	simplified, e := NewSolidContext(ctx, original.Vertices(), original.Edges(), faces, original.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	started = time.Now()
	after, e := simplified.MassProperties(ctx, parametric.IntegrationOptions{})
	fast := time.Since(started)
	if e != nil {
		t.Fatal(e)
	}
	if count != 48 || math.Abs(before.Volume-.25) > 1e-10 || math.Abs(before.Area-2.5) > 1e-10 || math.Abs(after.Volume-before.Volume) > 1e-12 || math.Abs(after.Area-before.Area) > 1e-12 || after.Centroid.Sub(before.Centroid).Length() > 1e-12 {
		t.Fatal("mass changed", count, before, after)
	}
	t.Logf("certified zero-width strips=%d original=%s proof+recipe-rebuild=%s strip-free=%s", count, slow, proofTime, fast)
}
