package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestCoaxialNoncircularStationLoftBooleanCrossing(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	parts := [2]*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 2, 3, 1, .75), rationalStationBand(t, 0, 2, 1, 3, .75)}
	var inputs [2]Solid
	for i, band := range parts {
		var err error
		inputs[i], _, err = NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{band}, 1e-8, o)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], inputs[1], op, o)
		if err != nil || out.Topology().EulerCharacteristic != 2 || len(h.Bands) != 2 {
			t.Fatal("the common profile must preserve an exact crossing rim", out.Topology(), h, err)
		}
		wantCaps := [2][2]bool{{i == 1, i == 0}, {i == 0, i == 1}}
		if wantCaps != h.CapSources {
			t.Fatal("incorrect original cap sources", h.CapSources, wantCaps)
		}
		for j, band := range h.Bands {
			owner := (i + j + 1) % 2
			if !band.BasisPreserved || band.SourceWalls[owner] != 0 || band.SourceWalls[1-owner] != -1 ||
				band.LowerZ.Cmp(big.NewRat(int64(j), 1)) != 0 || band.UpperZ.Cmp(big.NewRat(int64(j+1), 1)) != 0 {
				t.Fatal("result wall must retain its original source and exact signed band", i, j, band)
			}
		}
		proof, err := out.ValidateWrappedRationalStationLoft(ctx, o)
		if err != nil || proof.Scales[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal("incorrect shared original-profile rim", proof, err)
		}
		mass, err := out.BoundWrappedRationalStationMass(ctx, WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 100)})
		// Integrate the two linear radius ramps, times the independently
		// integrated area of the normalized .75-weight profile.
		want := 3.1748736333785317 * [2]float64{14. / 3, 38. / 3}[i]
		if err != nil {
			t.Fatal(err)
		}
		low, _ := mass.Volume.Lower.Float64()
		high, _ := mass.Volume.Upper.Float64()
		if low > want || high < want || math.Abs(high-low) > .01 {
			t.Fatal("independent weighted profile volume", low, high, want)
		}
	}
	other, _, err := NewWrappedRationalStationLoft(ctx, []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 2, 1, 3, .5)}, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], other, parametric.Union, o); !errors.Is(err, geom.ErrUnrepresentable) || len(out.faces) != 0 || len(h.Bands) != 0 {
		t.Fatal("equal control-polygon radii alone must not admit different rational profiles", h, err)
	}
	if out, h, err := CombineCoaxialWrappedRationalStationLofts(ctx, inputs[0], inputs[1], parametric.Difference, o); !errors.Is(err, geom.ErrUnrepresentable) || len(out.faces) != 0 || len(h.Bands) != 0 {
		t.Fatal("crossing noncircular difference leaked partial output", h, err)
	}
}
