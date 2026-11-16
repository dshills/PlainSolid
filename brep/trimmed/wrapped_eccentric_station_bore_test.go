package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestEccentricCircularStationBoreChecksInteriorStations(t *testing.T) {
	ctx := context.Background()
	opts := geom.SurfacePatchOptions{}
	outer, _, err := LoftWrappedCircularStations(ctx, geom.Vec2{}, []WrappedLoftStation{
		{Z: 0, Radius: 5}, {Z: 3, Radius: 4.5}, {Z: 8, Radius: 5},
	}, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, center := range []geom.Vec2{{X: 1, Y: .5}, {X: 2.5}} {
		inner, _, err := LoftWrappedCircularStations(ctx, center, []WrappedLoftStation{
			{Z: 0, Radius: 2}, {Z: 2, Radius: 2}, {Z: 8, Radius: 2},
		}, 1e-9, opts)
		if err != nil {
			t.Fatal(err)
		}
		out, history, err := CutWrappedEccentricCircularStationLofts(ctx, outer, inner, opts)
		if center.X == 2.5 {
			// The cap gaps are both 3, but the interior gap is exactly
			// the 2.5 axis separation, despite mismatched station grids.
			if !errors.Is(err, geom.ErrDomain) || len(out.Faces()) != 0 || len(history.InnerBands) != 0 {
				t.Fatalf("an interior tangent station returned material: %v", err)
			}
			continue
		}
		if err != nil || len(history.Bands) != 2 || len(history.InnerBands) != 2 {
			t.Fatalf("strict nesting with irrational axis separation sqrt(5)/2: %v", err)
		}
		p, err := out.ValidateWrappedEccentricCircularStationBore(ctx, opts)
		if err != nil || !p.Complete || p.Inner.Center[0].Cmp(big.NewRat(1, 1)) != 0 || p.Inner.Center[1].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatalf("original axes unavailable after independent reproof: %v", err)
		}
	}
}
