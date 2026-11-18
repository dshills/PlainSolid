package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeLowerParabolaCircleGraphCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	src := parabolaCircleGraphStock(t, 1, big.NewRat(1, 8), big.NewRat(1, 256), false)
	disk := 5763.0 * math.Pi / 8589934592
	for order, volume := range [2][]float64{{121909.0/55050240 + disk}, {798593.0 / 11010048, disk}} {
		r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], 1, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		checkNativeParabolaCircleCutComponents(t, ctx, r, [2][]int{{0}, {1, 2}}[order], volume)
	}
}

func checkNativeParabolaCircleCutComponents(t *testing.T, ctx context.Context, cut *ParabolaCircleGraphCut, regions []int, volumes []float64) {
	t.Helper()
	parts, history := cut.Components(), cut.History()
	if len(parts) != len(volumes) || len(history) != len(volumes) {
		t.Fatal("wrong number of individually closed source components", len(parts))
	}
	for i, s := range parts {
		h := history[i]
		if h.Region != regions[i] || len(s.Topology().Shells) != 1 ||
			s.faces[h.RoofFaces[0]].Surface != cut.sources[1] || s.faces[h.RoofFaces[0]].Reversed ||
			s.faces[h.RoofFaces[1]].Surface != cut.sources[2] || !s.faces[h.RoofFaces[1]].Reversed {
			t.Fatal("lost source region or directed roofs", h, s.Topology())
		}
		m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if err != nil || !(m.Volume > 0) || !(math.Abs(m.Volume-volumes[i]) <= 1e-9) {
			t.Fatal("component is not positively oriented or differs from the independent signed source integral", m, volumes[i], err)
		}
		t.Logf("region %d has %d faces, %d edges, %d vertices, signed volume %.16g", h.Region, len(s.faces), len(s.edges), len(s.vertices), m.Volume)
	}
}
