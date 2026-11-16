package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestMappedChainKnotTrimCutsRequireExactRationalStations(t *testing.T) {
	ctx := context.Background()
	p, err := geom.NewSurfacePatch(mappedChainCreasedSupport(t, false, .5, .5), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	line := mappedChainFixtureLine(t, geom.Vec2{Y: .25}, geom.Vec2{X: .75, Y: .25})
	prescribed := [][]*big.Rat{{new(big.Rat), big.NewRat(1, 10), big.NewRat(1, 1)}}
	cuts, err := mappedChainKnotTrimCuts(ctx, p, []PCurve{line}, prescribed, geom.SurfaceCurveAgreementOptions{})
	if err != nil || len(cuts) != 1 || len(cuts[0]) != 4 ||
		cuts[0][1].Cmp(big.NewRat(1, 10)) != 0 || cuts[0][2].Cmp(big.NewRat(2, 3)) != 0 || len(prescribed[0]) != 3 {
		t.Fatal("original 2/3 trim parameter should be inserted with all prescribed samples retained", cuts, err)
	}
	// u=t² meets the unchanged support knot u=1/2 at an irrational station.
	irrational, err := NewExactQuadraticUV([3][3]*big.Rat{
		{new(big.Rat), big.NewRat(1, 4), big.NewRat(1, 1)},
		{new(big.Rat), big.NewRat(1, 4), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 4), big.NewRat(1, 1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	cuts, err = mappedChainKnotTrimCuts(ctx, p, []PCurve{irrational}, prescribed, geom.SurfaceCurveAgreementOptions{})
	if cuts != nil || (!errors.Is(err, geom.ErrUnrepresentable) && !errors.Is(err, geom.ErrConvergence)) {
		t.Fatal("knot subdivision must reject an irrational crossing without emitting a rounded sample", cuts, err)
	}
}
