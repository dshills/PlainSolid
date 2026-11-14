package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestAnisotropicHorizontalLine(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-10}}
	p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{r(1, 4), r(-1, 2), r(1, 3)}
	direction := [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}
	q, e := s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Interior) != 1 {
		t.Fatalf("%+v", q)
	}
	for i, want := range []*big.Rat{r(-7, 6), r(7, 6)} {
		h := q.Hits[i]
		if h.Parameter.Min.Cmp(want) > 0 || h.Parameter.Max.Cmp(want) < 0 || !reflect.DeepEqual(h.Faces, []int{p.History.WallFaces[0]}) {
			t.Fatalf("hit %+v want %s", h, want)
		}
	}
	// Independent exact weighted midpoint: x/sx=y/sy=5/7. A diagonal
	// through the center has these two rational feet, not ellipse feet.
	direction = [3]*big.Rat{r(7, 3), r(4, 3), r(0, 1)}
	q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-1, 1), r(1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Interior) != 1 {
		t.Fatalf("diagonal %+v", q)
	}
	for i, want := range []*big.Rat{r(-5, 7), r(5, 7)} {
		h := q.Hits[i].Parameter
		if h.Min.Cmp(want) > 0 || h.Max.Cmp(want) < 0 {
			t.Fatalf("%+v want %s", h, want)
		}
	}
	direction = [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}
	origin[1] = r(5, 6)
	q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 1 || len(q.Interior) != 0 || q.Hits[0].Parameter.Min.Sign() != 0 || q.Hits[0].Parameter.Max.Sign() != 0 {
		t.Fatalf("tangent %+v", q)
	}
	// The exact exterior probe is sub-ULP at this height; it must not
	// become a floating-point tangent.
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	origin[1] = new(big.Rat).Add(r(5, 6), epsilon)
	q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), o)
	if e != nil || len(q.Hits) != 0 || len(q.Interior) != 0 {
		t.Fatalf("sub-ULP exterior %+v %v", q, e)
	}
	origin[1] = r(-1, 2)
	q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-1, 10), r(1, 10), o)
	if e != nil || len(q.Hits) != 0 || len(q.Interior) != 1 {
		t.Fatalf("clipped %+v %v", q, e)
	}
	for _, z := range []*big.Rat{r(0, 1), r(1, 1), r(3, 1)} {
		origin[2] = z
		q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), o)
		if e == nil || len(q.Hits) != 0 || len(q.Interior) != 0 {
			t.Fatal("station accepted")
		}
	}
	origin[2] = r(1, 3)
	direction[2] = r(1, 1)
	if _, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("oblique accepted")
	}
	direction[2] = r(0, 1)
	for _, budget := range []int{1, 10} {
		bad := o
		bad.Geometry.Surface.MaxOperations = budget
		q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), bad)
		if e == nil || len(q.Hits) != 0 || len(q.Interior) != 0 {
			t.Fatal("budget")
		}
	}
	for _, bad := range []WrappedRationalStationLineOptions{
		{Geometry: o.Geometry, MaxEvents: 1}, {Geometry: o.Geometry, MaxPointOperations: 1},
	} {
		q, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, origin, direction, r(-2, 1), r(2, 1), bad)
		if e == nil || len(q.Hits) != 0 || len(q.Interior) != 0 {
			t.Fatal("ledger quota")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ClipWrappedAnisotropicStationHorizontalLineRational(c, origin, direction, r(-2, 1), r(2, 1), o); e == nil {
		t.Fatal("cancel")
	}
}
