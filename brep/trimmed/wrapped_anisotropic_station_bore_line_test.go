package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAnisotropicBoreLineMaterialAndCaps(t *testing.T) {
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	r := big.NewRat
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	caps, e := s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	capline := [3]*big.Rat{r(1, 4), r(-1, 2), r(-4, 1)}
	dx := [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}
	q, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, capline, dx, r(-4, 1), r(4, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Interior) != 0 || len(q.Boundary) != 2 || len(q.Hits) != 4 {
		t.Fatalf("annular cap %+v", q)
	}
	for i, want := range [][2]*big.Rat{{r(-3, 1), r(-1, 2)}, {r(1, 2), r(3, 1)}} {
		span := q.Boundary[i]
		if span.Start.Parameter.Min.Cmp(want[0]) != 0 || span.End.Parameter.Max.Cmp(want[1]) != 0 || len(span.Faces) != 1 || span.Faces[0] != caps.Caps[0] {
			t.Fatal("cap material interval", span)
		}
		for _, h := range []WrappedAnisotropicLineHit{span.Start, span.End} {
			if len(h.Faces) != 2 || len(h.RimEdges) != 2 || len(h.Vertices) != 1 || h.Parts != (geom.CylinderLateral|geom.CylinderLowerCap) {
				t.Fatal("cap corner incidence", h)
			}
		}
	}
	if q.ExactOperations <= 0 {
		t.Fatal("ledger")
	}
	n := q.ExactOperations
	o.MaxOperations = n
	if _, e = s.ClipWrappedAnisotropicStationBoreLineRational(ctx, capline, dx, r(-4, 1), r(4, 1), o); e != nil {
		t.Fatal("exact quota", e)
	}
	o.MaxOperations = n - 1
	bad, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, capline, dx, r(-4, 1), r(4, 1), o)
	if e == nil || len(bad.Hits)+len(bad.Interior)+len(bad.Boundary) != 0 || bad.ExactOperations != 0 {
		t.Fatal("atomic quota", e)
	}
	o.MaxOperations = 0
	clipped, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, capline, dx, r(3, 4), r(1, 1), o)
	if e != nil || len(clipped.Boundary) != 1 || len(clipped.Interior) != 0 || len(clipped.Hits) != 2 {
		t.Fatal("clipped annular cap", clipped, e)
	}
	for _, h := range clipped.Hits {
		if len(h.Faces) != 1 || h.Faces[0] != caps.Caps[0] || len(h.RimEdges) != 0 {
			t.Fatal("false clip rim")
		}
	}
	// Exact clips on opposite sides of the inner rim remain distinguishable
	// far below binary64 resolution. Only the annular side is cap boundary.
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int64{-1, 1} {
		a := new(big.Rat).Add(r(1, 2), new(big.Rat).Mul(r(sign, 1), epsilon))
		c := new(big.Rat).Add(r(1, 2), new(big.Rat).Mul(r(2*sign, 1), epsilon))
		if a.Cmp(c) > 0 {
			a, c = c, a
		}
		tiny, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, capline, dx, a, c, o)
		if e != nil {
			t.Fatal(e)
		}
		want := 0
		if sign > 0 {
			want = 1
		}
		if len(tiny.Boundary) != want || len(tiny.Interior) != 0 {
			t.Fatal("subfloat annulus", tiny)
		}
	}
	dz := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	for _, x := range []*big.Rat{r(1, 4), r(1, 1)} {
		line := [3]*big.Rat{x, r(-1, 2), r(0, 1)}
		q, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, line, dz, r(-5, 1), r(0, 1), o)
		if e != nil {
			t.Fatal(e)
		}
		if x.Cmp(r(1, 4)) == 0 {
			if len(q.Hits)+len(q.Boundary)+len(q.Interior) != 0 {
				t.Fatal("opening falsely material", q)
			}
		} else {
			if len(q.Hits) != 2 || len(q.Interior) != 1 || len(q.Boundary) != 0 || q.Interior[0].Start.Min.Cmp(r(-4, 1)) != 0 || q.Interior[0].End.Max.Cmp(r(-1, 1)) != 0 {
				t.Fatal("material cap section", q)
			}
		}
	}
	// A smooth cardinal inner tangent lies on the other family's genuine
	// original crease plane. Its multiplicity and original periodic seam survive.
	tangent := [3]*big.Rat{r(5, 8), r(-1, 2), r(-3, 1)}
	dy := [3]*big.Rat{r(0, 1), r(1, 1), r(0, 1)}
	q, e = s.ClipWrappedAnisotropicStationBoreLineRational(ctx, tangent, dy, r(-3, 1), r(3, 1), o)
	if e != nil {
		t.Fatal("tangent", e)
	}
	if len(q.Hits) != 3 || len(q.Interior) != 2 || len(q.Boundary) != 0 {
		t.Fatal("grazing inner wall", q)
	}
	h := q.Hits[1]
	if h.Parameter.Min.Sign() != 0 || h.Parameter.Max.Sign() != 0 || h.Parameter.Multiplicity != 2 || len(h.Faces) != 1 || h.Faces[0] != caps.InnerWalls[0] || len(h.Edges) != 1 {
		t.Fatal("tangent source", h)
	}
	for _, h := range []WrappedAnisotropicLineHit{q.Hits[0], q.Hits[2]} {
		if len(h.Faces) != 2 || len(h.RimEdges) != 1 {
			t.Fatal("algebraic outer crease owners", h)
		}
	}
	t.Log("cap query exact operations", n)
}

func TestAnisotropicBoreGeneratorAndCrease(t *testing.T) {
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	r := big.NewRat
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-9}}
	p, e := s.ValidateWrappedAnisotropicStationBore(ctx, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, inner := range []bool{false, true} {
		origin := [3]*big.Rat{r(13, 4), r(-1, 2), r(-4, 1)}
		d := [3]*big.Rat{r(-2, 1), r(0, 1), r(1, 1)}
		maxT := r(1, 1)
		walls := p.OuterWalls
		if inner {
			origin[0] = r(3, 4)
			d[0] = r(-1, 8)
			maxT = r(2, 1)
			walls = p.InnerWalls
		}
		q, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, origin, d, r(0, 1), maxT, o)
		if e != nil {
			t.Fatal("generator", inner, e)
		}
		if len(q.Boundary) != 1 || len(q.Interior) != 0 || len(q.Hits) != 2 {
			t.Fatal("generator spans", inner, q)
		}
		span := q.Boundary[0]
		if len(span.Faces) != 1 || span.Faces[0] != walls[0] || len(span.Edges) != 1 || len(span.Start.Faces) != 2 || len(span.End.Faces) != 2 || len(span.End.RimEdges) != 2 || len(span.End.Vertices) != 1 {
			t.Fatal("generator ownership", span)
		}
		for _, id := range walls {
			found := false
			for _, got := range span.End.Faces {
				found = found || id == got
			}
			if !found {
				t.Fatal("crease missing")
			}
		}
	}
	// The narrowest inner station touches this vertical line only at its
	// corner: no material interval can be fabricated on either side of it.
	origin := [3]*big.Rat{r(1, 2), r(-1, 2), r(0, 1)}
	d := [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}
	q, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, origin, d, r(-4, 1), r(-1, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) != 1 || len(q.Interior)+len(q.Boundary) != 0 || q.Hits[0].Parameter.Min.Cmp(r(-2, 1)) != 0 || len(q.Hits[0].Faces) != 2 {
		t.Fatal("isolated crease touch", q)
	}
	o.MaxEvents = 1
	bad, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, origin, d, r(-4, 1), r(-1, 1), o)
	if e == nil || len(bad.Hits) != 0 {
		t.Fatal("premerge event quota", e)
	}
}

func TestAnisotropicBoreObliqueLine(t *testing.T) {
	ctx := context.Background()
	s := anisotropicMeasuredBore(t)
	r := big.NewRat
	o := WrappedAnisotropicMultiBandLineOptions{Geometry: geom.ToroidLineOptions{ParameterTolerance: 1e-10}}
	origin := [3]*big.Rat{r(-7, 4), r(-3, 8), r(-4, 1)}
	direction := [3]*big.Rat{r(1, 1), r(0, 1), r(1, 1)}
	q, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, origin, direction, r(0, 1), r(3, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Hits) < 6 || len(q.Interior) < 3 || len(q.Boundary) != 0 {
		t.Fatal("nontrivial oblique section", q)
	}
	// Independent authored quadrant equation and independent affine station
	// interpolation classify a dense set away from certified root brackets.
	profileInside := func(x, y float64) bool {
		weights := [4]float64{.5, .75, .75, .75}
		quadrant := 0
		if x < 0 {
			if y >= 0 {
				quadrant = 1
			} else {
				quadrant = 2
			}
		} else if y < 0 {
			quadrant = 3
		}
		for j := 0; j < quadrant; j++ {
			x, y = y, -x
		}
		if x > 1 || y > 1 {
			return false
		}
		if x+y <= 1 {
			return true
		}
		return (x+y-1)*(x+y-1) < 4*weights[quadrant]*weights[quadrant]*(1-x)*(1-y)
	}
	scale := func(stations [][3]float64, z float64) (float64, float64) {
		for i := 0; i+1 < len(stations); i++ {
			a, b := stations[i], stations[i+1]
			if z <= b[0] {
				u := (z - a[0]) / (b[0] - a[0])
				return a[1] + u*(b[1]-a[1]), a[2] + u*(b[2]-a[2])
			}
		}
		panic("station")
	}
	for k := 1; k < 300; k++ {
		tval := float64(k) / 100
		param := r(int64(k), 100)
		x, y := -2+tval, .125
		ox, oy := scale([][3]float64{{0, 3, 1}, {1, 1, 2}, {3, 2, 1}}, tval)
		ix, iy := scale([][3]float64{{0, .5, .25}, {2, .25, .5}, {3, .375, .25}}, tval)
		want := profileInside(x/ox, y/oy) && !profileInside(x/ix, y/iy)
		uncertain := false
		for _, h := range q.Hits {
			if param.Cmp(h.Parameter.Min) >= 0 && param.Cmp(h.Parameter.Max) <= 0 {
				uncertain = true
			}
		}
		if uncertain {
			continue
		}
		got := false
		for _, span := range q.Interior {
			got = got || (param.Cmp(span.Start.Max) > 0 && param.Cmp(span.End.Min) < 0)
		}
		if got != want {
			t.Fatal("independent material", tval, got, want)
		}
	}
	reverse := [3]*big.Rat{r(-1, 1), r(0, 1), r(-1, 1)}
	rev, e := s.ClipWrappedAnisotropicStationBoreLineRational(ctx, origin, reverse, r(-3, 1), r(0, 1), o)
	if e != nil {
		t.Fatal(e)
	}
	if len(rev.Hits) != len(q.Hits) || len(rev.Interior) != len(q.Interior) {
		t.Fatal("reversed unnormalized line")
	}
	for i, h := range q.Hits {
		c := rev.Hits[len(q.Hits)-1-i]
		lo := new(big.Rat).Neg(c.Parameter.Max)
		hi := new(big.Rat).Neg(c.Parameter.Min)
		if lo.Cmp(h.Parameter.Max) > 0 || hi.Cmp(h.Parameter.Min) < 0 {
			t.Fatal("parameter changed")
		}
		if len(h.Faces) != len(c.Faces) {
			t.Fatal("reverse owners")
		}
		for j, id := range h.Faces {
			if id != c.Faces[j] {
				t.Fatal("reverse face")
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := s.ClipWrappedAnisotropicStationBoreLineRational(cancelled, origin, direction, r(0, 1), r(3, 1), o)
	if e == nil || len(bad.Hits)+len(bad.Interior)+len(bad.Boundary) != 0 {
		t.Fatal("cancelled result", e)
	}
	t.Log("oblique hits/intervals/operations", len(q.Hits), len(q.Interior), q.ExactOperations)
}
