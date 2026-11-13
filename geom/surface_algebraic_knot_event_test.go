package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func algebraicKnotFixture(t *testing.T) (*ExactParameterKnotEvent, *SurfacePatch, *SurfacePatch) {
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	event, e := NewExactParameterKnotEvent(context.Background(), uv, 0, big.NewRat(1, 2), ExactParameterInterval{Lower: big.NewRat(2, 3), Upper: big.NewRat(3, 4)}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, _ := graphContactFixture(t, false, false)
	left, e := p.Trim(ParameterRectangle{UMin: 0, UMax: .5, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	right, e := p.Trim(ParameterRectangle{UMin: .5, UMax: 1, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	return event, left, right
}
func TestAlgebraicKnotEventModelAgreement(t *testing.T) {
	event, left, right := algebraicKnotFixture(t)
	// The right continuation differs away from the cut by u-1/2. Equality
	// must use the irrational event's polynomial, not a global curve identity.
	for i := range right.net {
		right.net[i][2].Add(right.net[i][2], new(big.Rat).Sub(right.net[i][0], new(big.Rat).Mul(big.NewRat(1, 2), right.net[i][3])))
	}
	for _, pair := range [][2]*SurfacePatch{{left, right}, {right, left}} {
		r, e := CompareSurfaceKnotEventModels(context.Background(), event, pair[0], pair[1], ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || !r.Equal {
			t.Fatal(r, e)
		}
	}
	bounds := event.Interval()
	bounds.Lower.SetInt64(10)
	if event.Multiplicity() != 1 {
		t.Fatal("multiplicity")
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for i := range right.net {
		right.net[i][2].Add(right.net[i][2], new(big.Rat).Mul(tiny, right.net[i][3]))
	}
	r, e := CompareSurfaceKnotEventModels(context.Background(), event, left, right, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Equal {
		t.Fatal("distinct near point", r, e)
	}
}
func TestAlgebraicKnotEventDomainAndLimits(t *testing.T) {
	event, left, right := algebraicKnotFixture(t)
	outside, e := right.Trim(ParameterRectangle{UMin: .5, UMax: 1, VMin: 0, VMax: .5})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareSurfaceKnotEventModels(context.Background(), event, left, outside, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDomain) || r.Proven {
		t.Fatal(r, e)
	}
	r, e = CompareSurfaceKnotEventModels(context.Background(), event, left, right, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CompareSurfaceKnotEventModels(ctx, event, left, right, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestAlgebraicKnotEventTangency(t *testing.T) {
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{big.NewRat(3, 4), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(3, 4), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	event, e := NewExactParameterKnotEvent(context.Background(), uv, 0, big.NewRat(1, 2), ExactParameterInterval{Lower: new(big.Rat), Upper: big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if e != nil || event.Multiplicity() != 2 {
		t.Fatal(event, e)
	}
}
