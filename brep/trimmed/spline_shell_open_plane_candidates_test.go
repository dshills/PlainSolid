package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/intersect"
)

func TestOpenPlaneShellConservativeCandidatePairs(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, .5, -.5}, [3]float64{1.5, 1.5, .5}, .125)
	var pairs [][2]int
	for i := range a.faces {
		for j := range b.faces {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	options := SplineShellOpenOptions{}
	options.Shells.Separation.SeparatingProjections = true
	options.Shells.Separation.MaxPairs = 256
	s, h, e := CombineOpenPlaneSplineShells(ctx, a, b, parametric.Intersection, pairs, intersect.SplinePlanePairDiscoveryOptions{}, options)
	if e != nil || len(h.FacePairs) != 6 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(e, len(h.FacePairs))
	}
	p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-.125) > 1e-10 {
		t.Fatal(p, e)
	}
	// Same boundary pairs cannot be thrown away by the separation fallback.
	bad, _, e := CombineOpenPlaneSplineShells(ctx, a, a, parametric.Union, [][2]int{{0, 0}}, intersect.SplinePlanePairDiscoveryOptions{}, options)
	if e == nil || len(bad.Faces()) != 0 {
		t.Fatal("coincidence discarded", e)
	}
}
