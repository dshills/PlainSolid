package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestMultiSpanExactAffineResidual(t *testing.T) {
	var poles []geom.Vec3
	for _, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, .5, 1} {
			poles = append(poles, geom.Vec3{X: u, Y: v, Z: 2*u + 3*v})
		}
	}
	surface, e := geom.NewBSplineSurface(1, 1, 3, 3, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, .5, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3]geom.Vec2{{}, {X: 1, Y: .25}, {X: .375, Y: 1}}
	var model [3]geom.Vec3
	for i, p := range uv {
		model[i] = geom.Vec3{X: p.X, Y: p.Y, Z: 2*p.X + 3*p.Y}
	}
	result, e := BoundMultiSpanSplineTriangleDeviation(context.Background(), patch, uv, model, MultiSpanTriangleBoundOptions{})
	if e != nil || !result.BothDirections || result.SquaredDistance.Sign() != 0 || result.Cells != 4 {
		t.Fatal(result, e)
	}
	nonbinary := false
	for _, piece := range result.Pieces {
		for _, p := range piece.UV {
			for _, q := range p {
				if _, exact := q.Float64(); !exact {
					nonbinary = true
				}
			}
		}
	}
	if !nonbinary {
		t.Fatal("fixture did not retain rational knot intersections")
	}
	for _, options := range []MultiSpanTriangleBoundOptions{{MaxCells: 1}, {MaxPieces: 1}, {MaxInputBits: 1}} {
		bad, e := BoundMultiSpanSplineTriangleDeviation(context.Background(), patch, uv, model, options)
		if e == nil || bad.SquaredDistance != nil || len(bad.Pieces) != 0 {
			t.Fatal("partial certificate after limit", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = BoundMultiSpanSplineTriangleDeviation(ctx, patch, uv, model, MultiSpanTriangleBoundOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = BoundMultiSpanSplineTriangleDeviation(context.Background(), &geom.SurfacePatch{}, uv, model, MultiSpanTriangleBoundOptions{})
	if e == nil {
		t.Fatal("zero patch accepted")
	}
	uv[1], uv[2] = uv[2], uv[1]
	model[1], model[2] = model[2], model[1]
	reverse, e := BoundMultiSpanSplineTriangleDeviation(context.Background(), patch, uv, model, MultiSpanTriangleBoundOptions{})
	if e != nil || reverse.SquaredDistance.Sign() != 0 {
		t.Fatal("reversed triangle", e)
	}
}

func TestMultiSpanRationalC0Residual(t *testing.T) {
	for _, rational := range []bool{false, true} {
		xs, zs := []float64{0, .25, .5, .75, 1}, []float64{0, 0, .25, .25, .5}
		var poles []geom.Vec3
		var weights []float64
		for i, u := range xs {
			for j, v := range []float64{0, 1} {
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: zs[i]})
				w := 1.
				if rational && (i+j)%3 == 0 {
					w = .75
				}
				weights = append(weights, w)
			}
		}
		surface, e := geom.NewBSplineSurface(2, 1, 5, 2, poles, []float64{0, 0, 0, .5, .5, 1, 1, 1}, []float64{0, 0, 1, 1}, weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		independent := func(u, v float64) geom.Vec3 {
			offset := 0
			t0 := 2 * u
			if u > .5 {
				offset = 2
				t0 = 2*u - 1
			}
			bu := []float64{(1 - t0) * (1 - t0), 2 * t0 * (1 - t0), t0 * t0}
			bv := []float64{1 - v, v}
			p := geom.Vec3{}
			w := 0.
			for i, a := range bu {
				for j, b := range bv {
					index := (offset+i)*2 + j
					value := a * b * weights[index]
					p = p.Add(poles[index].Scale(value))
					w += value
				}
			}
			return p.Scale(1 / w)
		}
		uv := [3]geom.Vec2{{}, {X: 1, Y: .25}, {X: .375, Y: 1}}
		var model [3]geom.Vec3
		for i, p := range uv {
			model[i] = independent(p.X, p.Y)
			model[i].Z += .03125
		}
		result, e := BoundMultiSpanSplineTriangleDeviation(context.Background(), patch, uv, model, MultiSpanTriangleBoundOptions{})
		if e != nil || !result.BothDirections || result.Cells != 2 {
			t.Fatal(result, e)
		}
		for _, piece := range result.Pieces {
			u, v := new(big.Rat), new(big.Rat)
			var target [3]*big.Rat
			for k := range target {
				target[k] = new(big.Rat)
			}
			for i, p := range piece.UV {
				u.Add(u, p[0])
				v.Add(v, p[1])
				for k := range target {
					target[k].Add(target[k], piece.Model[i][k])
				}
			}
			u.Quo(u, big.NewRat(3, 1))
			v.Quo(v, big.NewRat(3, 1))
			uf, _ := u.Float64()
			vf, _ := v.Float64()
			point := independent(uf, vf)
			values := []float64{point.X, point.Y, point.Z}
			distance := 0.
			for k := range target {
				target[k].Quo(target[k], big.NewRat(3, 1))
				f, _ := target[k].Float64()
				delta := values[k] - f
				distance += delta * delta
			}
			bound, _ := piece.SquaredDistance.Float64()
			if distance > bound+1e-14 {
				t.Fatal("independent span residual", rational, distance, bound)
			}
		}
		t.Logf("rational=%v: %d knot-clipped pieces", rational, len(result.Pieces))
	}
}
