package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestGraphBooleanCertifiedBoundaryDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	probe := [3]*big.Rat{big.NewRat(27, 16), big.NewRat(-4, 1), big.NewRat(9, 2)}
	// Under signed XZ placement material lies at Y>=-3. The plane
	// point at its lobe center has squared distance exactly one.
	r, e := q.BoundBoundaryDistance(ctx, probe, AlgebraicTubeLensDistanceOptions{SquaredGap: big.NewRat(1, 1000000)})
	if e != nil || !r.Converged || r.Face < 0 || r.RimEdge >= 0 || r.Squared.Lower.Cmp(big.NewRat(1, 1)) != 0 ||
		r.Squared.Upper.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("certified distance to the original retained cap", r, e)
	}
	f, e := s.OriginalFace(r.Face)
	if e != nil || f.Surface != q.faces[0].Surface {
		t.Fatal("lost retained cap owner", r.Face, e)
	}
	// Midway between the lobes the original support plane is buried.
	// Both disks have radius 5/64, centers 3/16 from this probe.
	// The exact nearest boundary is the nearer rim at gap 7/64.
	probe = [3]*big.Rat{big.NewRat(3, 2), big.NewRat(-3, 1), big.NewRat(9, 2)}
	want := big.NewRat(49, 4096)
	r, e = q.BoundBoundaryDistance(ctx, probe, AlgebraicTubeLensDistanceOptions{SquaredGap: big.NewRat(1, 1000), MaxCells: 64})
	if e != nil && !errors.Is(e, geom.ErrConvergence) || r.Squared.Lower == nil || r.Squared.Upper == nil ||
		r.Squared.Lower.Cmp(want) > 0 || r.Squared.Upper.Cmp(want) < 0 || r.Squared.Upper.Sign() == 0 {
		t.Fatal("buried original plane was used or the true rim was lost", r, e)
	}
	if r.RimEdge >= 0 {
		if _, _, e := s.SectionSegment(r.RimEdge); e != nil {
			t.Fatal("rim witness lacks original section ancestry", e)
		}
	}
	stop, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := q.BoundBoundaryDistance(stop, probe, AlgebraicTubeLensDistanceOptions{}); !errors.Is(e, context.Canceled) || bad.Squared.Upper != nil {
		t.Fatal("cancellation returned a stale witness", e)
	}
}
