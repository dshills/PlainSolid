//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestAnisotropicBandLine(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	s := anisotropicMassFixture(t, false, geom.Vec3{})
	o := WrappedRationalStationLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	origin := [3]*big.Rat{r(9, 56), r(-51, 112), r(1, 2)}
	d := [3]*big.Rat{r(20, 7), r(15, 7), r(1, 8)}
	p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := s.ClipWrappedAnisotropicStationBandLineRational(ctx, origin, d, r(-1, 1), r(1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Interior) != 1 {
		t.Fatalf("%+v", q)
	}
	// Independently authored feet: at t=±1/2 each normalized weighted
	// quadrant coordinate is ±5/7, the exact u=1/2 profile point.
	for i, want := range []*big.Rat{r(-1, 2), r(1, 2)} {
		h := q.Hits[i]
		if h.Parameter.Min.Cmp(want) > 0 || h.Parameter.Max.Cmp(want) < 0 || !reflect.DeepEqual(h.Faces, []int{p.History.WallFaces[0]}) {
			t.Fatalf("%+v want %s", h, want)
		}
		var foot [3]*big.Rat
		for a := range foot {
			foot[a] = new(big.Rat).Add(origin[a], new(big.Rat).Mul(d[a], want))
		}
		v, e := s.LocateWrappedAnisotropicStationRational(ctx, foot, WrappedRationalStationPointOptions{})
		if e != nil || v.Location != parametric.Boundary {
			t.Fatalf("foot %+v %v", v, e)
		}
	}
	// Reversing the unnormalized authored direction preserves physical feet.
	rev := [3]*big.Rat{}
	for a := range d {
		rev[a] = new(big.Rat).Neg(d[a])
	}
	v, e := s.ClipWrappedAnisotropicStationBandLineRational(ctx, origin, rev, r(-1, 1), r(1, 1), o)
	if e != nil || len(v.Hits) != 2 || len(v.Interior) != 1 {
		t.Fatalf("reverse %+v %v", v, e)
	}
	// An offset removes the authored root coincidence. Prove strict material
	// signs at both rational ends of each nonexact algebraic enclosure.
	origin[0] = new(big.Rat).Add(origin[0], r(1, 1000))
	q, e = s.ClipWrappedAnisotropicStationBandLineRational(ctx, origin, d, r(-1, 1), r(1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 2 || len(q.Interior) != 1 {
		t.Fatalf("offset %+v", q)
	}
	for i, h := range q.Hits {
		if h.Parameter.Min.Cmp(h.Parameter.Max) == 0 {
			t.Fatal("expected nonexact enclosure")
		}
		for j, tt := range []*big.Rat{h.Parameter.Min, h.Parameter.Max} {
			var foot [3]*big.Rat
			for a := range foot {
				foot[a] = new(big.Rat).Add(origin[a], new(big.Rat).Mul(d[a], tt))
			}
			v, e := s.LocateWrappedAnisotropicStationRational(ctx, foot, WrappedRationalStationPointOptions{})
			want := parametric.Outside
			if i+j == 1 {
				want = parametric.Inside
			}
			if e != nil || v.Location != want {
				t.Fatalf("bracket %d/%d %+v %v want %v", i, j, v, e, want)
			}
		}
	}
	// Finite range must be wholly inside one original band, even though the
	// infinite affine line may intersect other native walls.
	for _, clip := range [][2]*big.Rat{{r(-4, 1), r(1, 1)}, {r(-1, 1), r(5, 1)}} {
		v, e = s.ClipWrappedAnisotropicStationBandLineRational(ctx, origin, d, clip[0], clip[1], o)
		if e == nil || len(v.Hits) != 0 || len(v.Interior) != 0 {
			t.Fatal("cross-station accepted")
		}
	}
	for _, bad := range []WrappedRationalStationLineOptions{{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9, Surface: geom.SurfacePatchOptions{MaxOperations: 1}}}, {Geometry: o.Geometry, MaxEvents: 1}, {Geometry: o.Geometry, MaxPointOperations: 1}} {
		v, e = s.ClipWrappedAnisotropicStationBandLineRational(ctx, origin, d, r(-1, 1), r(1, 1), bad)
		if e == nil || len(v.Hits) != 0 || len(v.Interior) != 0 {
			t.Fatal("partial quota result")
		}
	}
	// This exact affine line is an original weighted wall generator,
	// not a finite collection of isolated quartic roots.
	goOrigin := [3]*big.Rat{r(47, 28), r(4, 7), r(1, 2)}
	goDirection := [3]*big.Rat{r(-5, 28), r(5, 56), r(1, 8)}
	v, e = s.ClipWrappedAnisotropicStationBandLineRational(ctx, goOrigin, goDirection, r(-1, 1), r(1, 1), o)
	if e == nil || len(v.Hits) != 0 || len(v.Interior) != 0 {
		t.Fatal("generator accepted as isolated")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.ClipWrappedAnisotropicStationBandLineRational(c, origin, d, r(-1, 1), r(1, 1), o); e == nil {
		t.Fatal("cancel")
	}
}
