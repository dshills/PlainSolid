package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestRationalStationNativeSplitAtHeight(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	bands := []*geom.WrappedSurfacePatch{rationalStationBand(t, 0, 1, 3, 1, .75), rationalStationBand(t, 1, 3, 1, 2, .75)}
	body, _, err := NewWrappedRationalStationLoft(ctx, bands, 1e-8, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range []float64{.5, 1} {
		lo, hi, h, err := body.SplitWrappedRationalStationLoftAtZ(ctx, z, o)
		if err != nil || lo.Topology().EulerCharacteristic != 2 || hi.Topology().EulerCharacteristic != 2 || h.ExistingStation != (z == 1) {
			t.Fatal("split must return two exact closed components", z, lo.Topology(), hi.Topology(), h, err)
		}
		if h.LowerWall[0] < 0 || h.LowerWall[1] != -1 || h.UpperWall[1] < 0 ||
			(z == 1) != (h.UpperWall[0] == -1) ||
			hi.faces[h.UpperWall[1]].Surface != body.faces[h.OriginalWall[1]].Surface ||
			(z == 1) != (lo.faces[h.LowerWall[0]].Surface == body.faces[h.OriginalWall[0]].Surface) {
			t.Fatal("split must report and reuse original wall ownership", z, h)
		}
		height := new(big.Rat).SetFloat64(z)
		a, err := lo.SectionWrappedRationalStationLoftRational(ctx, height, o)
		b, err2 := hi.SectionWrappedRationalStationLoftRational(ctx, height, o)
		if err != nil || err2 != nil || a.Cap != h.LowerCut || b.Cap != h.UpperCut ||
			lo.faces[a.Cap].Reversed == hi.faces[b.Cap].Reversed ||
			a.Outer.Section.Radius.Cmp(b.Outer.Section.Radius) != 0 {
			t.Fatal("the two exact new cut caps have opposite orientation", a, b, err, err2)
		}
		for _, u := range []float64{0, .5, 1, 2.375, 3.25, 4} {
			p, pe := a.Outer.Section.Curve.Eval(u)
			q, qe := b.Outer.Section.Curve.Eval(u)
			if pe != nil || qe != nil || p != q {
				t.Fatal("opposed caps must share the complete original weighted rim", u, p, q, pe, qe)
			}
		}
		taper := int64(1)
		if z == .5 {
			taper = 2
		}
		if a.Outer.Section.Radius.Cmp(big.NewRat(taper, 1)) != 0 {
			t.Fatal("cut profile scale must follow the original band", a.Outer.Section.Radius)
		}
	}
	if a, b, _, err := body.SplitWrappedCircularStationLoftAtZ(ctx, .5, o); err == nil || len(a.faces)+len(b.faces) != 0 {
		t.Fatal("circular entry point must retain its original support restriction", err)
	}
	for _, z := range []float64{0, 3} {
		if a, b, h, err := body.SplitWrappedRationalStationLoftAtZ(ctx, z, o); !errors.Is(err, geom.ErrDomain) || len(a.faces)+len(b.faces) != 0 || h.LowerCut != -1 || h.UpperCut != -1 {
			t.Fatal("terminal station must not produce a degenerate shell", z, h, err)
		}
	}
	// A sub-ULP displacement at the waist in this taper moves the new
	// first rim vertex off the native binary64 grid.
	smallest := 0x1.0000000000001p0
	if a, b, h, err := body.SplitWrappedRationalStationLoftAtZ(ctx, smallest, o); !errors.Is(err, geom.ErrUnrepresentable) || len(a.faces)+len(b.faces) != 0 || h.LowerCut != -1 || h.UpperCut != -1 {
		t.Fatal("nonrepresentable cut vertices must reject atomically", h, err)
	}
}
