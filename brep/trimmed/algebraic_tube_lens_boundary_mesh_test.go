package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedAlgebraicTubeLensBoundary(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := algebraicRunMeshFixture(t)
	lens, e := r.SealAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := AlgebraicTubeLensBoundaryOptions{Run: AlgebraicTubeRunMeshOptions{Display: RationalTubePolylineOptions{Deflection: .08}}}
	m, e := lens.TessellateBoundaryCertified(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	for i, a := range m.Owners[0] {
		b := m.Owners[1][i]
		if a.Reversed == b.Reversed || len(a.Vertices) != len(b.Vertices) {
			t.Fatal("incidence")
		}
		for j, id := range a.Vertices {
			if id != b.Vertices[len(b.Vertices)-1-j] {
				t.Fatal("shared owner")
			}
			for axis := 0; axis < 2; axis++ {
				comparison, e := a.UV[j][axis].Compare(ctx, b.UV[len(b.UV)-1-j][axis], geom.ExactCurvePlaneEventOptions{})
				if e != nil || !comparison.Proven || comparison.Order != 0 {
					t.Fatal("independent identity XY charts", comparison, e)
				}
				bounds, e := a.UV[j][axis].Enclose(ctx, big.NewRat(1, 100000000), geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					t.Fatal(e)
				}
				q, _ := bounds.Interval.Lower.Float64()
				want := m.Run.Vertices[id].X
				if axis == 1 {
					want = m.Run.Vertices[id].Y
				}
				if math.Abs(q-want) > .005 {
					t.Fatal(q, want)
				}
			}
		}
	}
	t.Log(len(m.Run.Vertices), m.OwnerOperations)
	options.MaxOwnerOperations = 1
	if m, e := lens.TessellateBoundaryCertified(ctx, options); e == nil || len(m.Run.Vertices) > 0 {
		t.Fatal("partial owner proof", e)
	}
}
