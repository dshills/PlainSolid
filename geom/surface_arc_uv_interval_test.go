package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfaceArcUVResidualEnclosure(t *testing.T) {
	ctx := context.Background()
	p, _ := NewSurfacePatch(quadraticContactSurface(t, -.125), SurfacePatchOptions{})
	pl, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, pl, SurfacePlaneArcOptions{})
	if e != nil && !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	checked := 0
	for _, arc := range result.Arcs {
		q, e := arc.NewSurfaceArcUVEncloser(ctx, SurfaceArcUVOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, p := range []Vec2{{.25, .25}, {.25, .75}, {.75, .25}, {.75, .75}} {
			d := arc.source.Domain()
			if p.X < d.UMin || p.X > d.UMax || p.Y < d.VMin || p.Y > d.VMax {
				continue
			}
			free := p.Y
			if q.dep == 1 {
				free = p.X
			}
			for _, interval := range [][2]float64{{free - .011, free + .019}, {free - .0001, free}, {free, free + .001}} {
				fast, has, e := q.Enclose(ctx, interval[0], interval[1])
				if e != nil || !has || p.X < fast.Min.X || p.X > fast.Max.X || p.Y < fast.Min.Y || p.Y > fast.Max.Y {
					t.Fatal("lost an independent exact circle point", p, fast, has, e)
				}
				full, e := arc.EncloseInterval(ctx, interval[0], interval[1], SurfaceArcIntervalOptions{MaxIterations: 4})
				if e != nil && !errors.Is(e, ErrConvergence) || !full.HasBounds || full.Parameters != fast {
					t.Fatal("scalar residual contraction differs from full patch", fast, full, e)
				}
				checked++
			}
		}
	}
	if checked < 12 {
		t.Fatal("did not exercise every exact point", checked)
	}
}

func TestSurfaceArcUVResidualAtomicOptions(t *testing.T) {
	ctx := context.Background()
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u - .375} })
	pl, _ := NewPlane(Vec3{}, Vec3{Z: 1})
	result, e := CertifySurfacePlaneArcs(p, pl, SurfacePlaneArcOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := result.Arcs[0]
	if _, e = a.NewSurfaceArcUVEncloser(ctx, SurfaceArcUVOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal("construction ignored exact work limit", e)
	}
	q, e := a.NewSurfaceArcUVEncloser(ctx, SurfaceArcUVOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, has, e := q.Enclose(ctx, 2, 3); e != nil || has {
		t.Fatal("slab outside support", has, e)
	}
	if actual, has, e := q.Enclose(ctx, -.5, 0); e != nil || !has || actual.Min.Y != 0 || actual.Max.Y != 0 {
		t.Fatal("singleton slab was discarded", actual, has, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := q.Enclose(canceled, .2, .5); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestSurfaceArcUVResidualOnWeightedObliqueChart(t *testing.T) {
	// W=1-u/2 with X=3u, Y=v and Z=u²/(32W)+v/4. Intersect the
	// represented raw plane Z=3/16+X/32. The independent rational result
	// gives v directly for each exact free u.
	knots := []float64{0, 0, 0, 1, 1, 1}
	var poles []Vec3
	var weights []float64
	for i, x := range []float64{0, 2, 3} {
		for j := range 3 {
			y := float64(j) / 2
			z := y / 4
			if i == 2 {
				z += 1. / 16
			}
			poles = append(poles, Vec3{x, y, z})
			weights = append(weights, []float64{1, .75, .5}[i])
		}
	}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane, e := NewRawSupportPlane(Vec3{Z: 3. / 16}, Vec3{X: 1, Z: 7. / 32}, Vec3{Y: 1, Z: 3. / 16}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cert, e := CertifySurfaceRawPlaneArcs(p, plane, SurfacePlaneArcOptions{})
	if e != nil || !cert.Complete || len(cert.Arcs) == 0 {
		t.Fatal(cert.Complete, e)
	}
	checked := 0
	for _, a := range cert.Arcs {
		q, e := a.NewSurfaceArcUVEncloser(context.Background(), SurfaceArcUVOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i := 1; i < 16; i++ {
			u := big.NewRat(int64(i), 16)
			w := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Quo(u, big.NewRat(2, 1)))
			v := new(big.Rat).Sub(new(big.Rat).Add(big.NewRat(3, 4), new(big.Rat).Mul(big.NewRat(3, 8), u)), new(big.Rat).Quo(new(big.Rat).Mul(u, u), w.Mul(w, big.NewRat(8, 1))))
			axis := [2]*big.Rat{u, v}
			d := a.source.Domain()
			intervals := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
			in := func(x *big.Rat, lo, hi float64) bool {
				return x.Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && x.Cmp(new(big.Rat).SetFloat64(hi)) <= 0
			}
			if !in(u, d.UMin, d.UMax) || !in(v, d.VMin, d.VMax) {
				continue
			}
			free := 1 - q.dep
			f, _ := axis[free].Float64()
			uv, has, e := q.Enclose(context.Background(), max(f-.001, intervals[free][0]-.001), min(f+.003, intervals[free][1]+.001))
			if e != nil || !has || !in(u, uv.Min.X, uv.Max.X) || !in(v, uv.Min.Y, uv.Max.Y) {
				t.Fatal("lost exact weighted oblique section", u, v, uv, has, e)
			}
			checked++
		}
	}
	if checked < 15 {
		t.Fatal("missed independently known points", checked)
	}
}
