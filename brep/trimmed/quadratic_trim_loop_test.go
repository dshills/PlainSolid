package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedQuadraticClosedCircleLoop(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	controls := [][3]geom.Vec3{
		{{X: .75, Y: .5}, {X: .75, Y: .75}, {X: .5, Y: .75}},
		{{X: .5, Y: .75}, {X: .25, Y: .75}, {X: .25, Y: .5}},
		{{X: .25, Y: .5}, {X: .25, Y: .25}, {X: .5, Y: .25}},
		{{X: .5, Y: .25}, {X: .75, Y: .25}, {X: .75, Y: .5}},
	}
	var trims []SplineUV
	for i, p := range controls {
		w := []float64{.5, .5, 1}
		reverse := i == 1
		if reverse {
			p[0], p[2] = p[2], p[0]
			w[0], w[2] = w[2], w[0]
		}
		curve, e := geom.NewBSpline(2, p[:], []float64{2, 2, 2, 6, 6, 6}, w)
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			trims = append(trims, SplineUV{Curve: curve, Range: UVInterval{Min: 2, Max: 4}}, SplineUV{Curve: curve, Range: UVInterval{Min: 4, Max: 6}})
		} else {
			trims = append(trims, SplineUV{Curve: curve, Range: UVInterval{Min: 2, Max: 6}, Reversed: reverse})
		}
	}
	o := QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .03}}
	result, e := TessellateQuadraticSplineTrimLoopCertified(context.Background(), patch, trims, o)
	if e != nil {
		t.Fatal(e)
	}
	if result.SourceProof.Status != trimvalidate.Certified || result.PolygonProof.Status != trimvalidate.Certified {
		t.Fatal("missing topology proof")
	}
	sources := map[int]bool{}
	for i, s := range result.Segments {
		if s.Vertices != [2]int{i, (i + 1) % len(result.Points)} {
			t.Fatal("noncyclic identity")
		}
		sources[s.Source] = true
		if trims[s.Source].Reversed != (s.Start > s.End) {
			t.Fatal("directed source ancestry", s)
		}
		// Independent exact circle equation in UV and exact bilinear support.
		p := result.ExactUV[i]
		x := new(big.Rat).Sub(p[0], big.NewRat(1, 2))
		y := new(big.Rat).Sub(p[1], big.NewRat(1, 2))
		radius := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		if radius.Cmp(big.NewRat(1, 16)) != 0 {
			t.Fatal("not exact circle", p)
		}
		z := new(big.Rat).Mul(p[0], p[1])
		if z.Cmp(result.ExactPoints[i][2]) != 0 {
			t.Fatal("not exact composed support")
		}
	}
	if len(sources) != len(trims) {
		t.Fatal("lost source pieces")
	}
	broken := append([]SplineUV(nil), trims...)
	broken[0].Range.Min = 2.25
	bad, e := TessellateQuadraticSplineTrimLoopCertified(context.Background(), patch, broken, o)
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("gap accepted")
	}
	o.Polygon.MaxEdges = 1
	bad, e = TessellateQuadraticSplineTrimLoopCertified(context.Background(), patch, trims, o)
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("partial topology budget result")
	}
	o.Polygon.MaxEdges = 0
	o.Tessellation.MaxSegments = len(trims)
	bad, e = TessellateQuadraticSplineTrimLoopCertified(context.Background(), patch, trims, o)
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("partial global chord budget")
	}

	// A crossed line loop is rejected by source topology before approximation.
	corners := []geom.Vec3{{X: .25, Y: .25}, {X: .75, Y: .75}, {X: .25, Y: .75}, {X: .75, Y: .25}}
	var crossed []SplineUV
	for i, p := range corners {
		c, e := geom.NewBSpline(1, []geom.Vec3{p, corners[(i+1)%4]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		crossed = append(crossed, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	bad, e = TessellateQuadraticSplineTrimLoopCertified(context.Background(), patch, crossed, QuadraticTrimLoopOptions{Tessellation: QuadraticTrimTessellationOptions{Deflection: .03}})
	if e == nil || len(bad.Points) != 0 {
		t.Fatal("crossed source accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = TessellateQuadraticSplineTrimLoopCertified(ctx, patch, trims, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	t.Logf("%d exact cyclic vertices, %d source trims", len(result.Points), len(trims))
}
