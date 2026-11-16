package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func TestMappedSplineChainOrderedSupportKnotBounds(t *testing.T) {
	ctx := context.Background()
	for _, cells := range []int{5, MaxJoinedMappedSplineCells, MaxJoinedMappedSplineCells + 1} {
		var chains [2]MappedSplineChain
		var supports [2]*geom.BSplineSurface
		want := make([]MappedSplineChainSupportEvent, 2*cells-1)
		for i := range cells {
			want[2*i].ModelStation = big.NewRat(int64(2*i+1), int64(2*cells))
			want[2*i].OwnersChangingCells[i%2] = true
			if i+1 < cells {
				event := &want[2*i+1]
				event.ModelStation, event.OriginalJoint = big.NewRat(int64(i+1), int64(cells)), true
				event.OwnersChangingCells[i%2] = true
				if i%3 == 0 {
					event.OwnersChangingCells = [2]bool{true, true}
				}
			}
		}
		var parts []MappedSplineUV
		var stations []*big.Rat
		for i := range cells {
			x := float64(3*i) / 8
			basis, err := geom.NewBSpline(3, []geom.Vec3{
				{X: x, Y: .25}, {X: x + .125, Y: .25}, {X: x + .25, Y: .25}, {X: x + .375, Y: .25},
			}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
			if err != nil {
				t.Fatal(err)
			}
			part, err := NewMappedSplineUV(ctx, SplineUV{Curve: basis, Range: UVInterval{0, 1}}, big.NewRat(1, 1), geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts = append(parts, part)
			if i+1 < cells {
				stations = append(stations, want[2*i+1].ModelStation)
			}
		}
		for owner := range chains {
			var err error
			chains[owner], err = NewMappedSplineEdgeChain(ctx, parts, stations, geom.SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			breaks := []float64{0}
			for i, event := range want {
				if event.OwnersChangingCells[owner] {
					breaks = append(breaks, float64(3*(i+1))/16)
				}
			}
			breaks = append(breaks, float64(3*cells)/8)
			poles := make([]geom.Vec3, 0, 2*len(breaks))
			for i, x := range breaks {
				amplitude := float64(i%2) / 8
				poles = append(poles, geom.Vec3{X: x, Z: -amplitude / 4}, geom.Vec3{X: x, Y: 1, Z: 3 * amplitude / 4})
			}
			knots := append([]float64{0}, breaks...)
			knots = append(knots, breaks[len(breaks)-1])
			supports[owner], err = geom.NewBSplineSurface(1, 1, len(breaks), 2, poles, knots, []float64{0, 0, 1, 1}, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		r, err := CertifyMappedSplineChainOrderedJoinedSupportKnotsAgreement(ctx, supports[0], chains[0], supports[1], chains[1], edgecheck.Options{})
		if cells > MaxJoinedMappedSplineCells {
			if !errors.Is(err, geom.ErrConvergence) || len(r.JoinedSupportEvents()) != 0 {
				t.Fatal("exceeding the ordered chain bound must reject atomically", cells, err)
			}
			continue
		}
		if err != nil || len(r.ModelCuts()) != 2*cells+1 {
			t.Fatal("the bounded certificate must derive one exact interior cut per original cell", cells, err)
		}
		o := MappedChainNativeOptions{}
		var recipe bytes.Buffer
		if err := r.WriteJSON(ctx, &recipe, o); err != nil {
			t.Fatal(err)
		}
		reopened, err := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(recipe.Bytes()), o)
		if err != nil {
			t.Fatal(err)
		}
		for i, event := range reopened.JoinedSupportEvents() {
			expected := want[i]
			if event.OriginalJoint != expected.OriginalJoint || event.OwnersChangingCells != expected.OwnersChangingCells ||
				event.ModelStation.Cmp(expected.ModelStation) != 0 {
				t.Fatal("v13 must rederive each exact station, event kind and owner", cells, i, event)
			}
		}
	}
}
