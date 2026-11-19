//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func wrappedOverlapFace(t *testing.T, w *geom.WrappedSurfacePatch, d geom.ParameterRectangle) SewFace {
	t.Helper()
	f := SewFace{Face: Face{Surface: w}}
	corners := insetCorners(d)
	for _, uv := range corners {
		jet, e := w.EvaluateRational(context.Background(), ratInset(uv.X), ratInset(uv.Y), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.RepresentedExactPoint(jet.Point)
		if e != nil {
			t.Fatal(e)
		}
		f.Vertices = append(f.Vertices, p)
	}
	for i, a := range corners {
		j := (i + 1) % 4
		b := corners[j]
		f.Edges = append(f.Edges, parametric.Edge{Start: i, End: j, Curve: surfaceTrimCurve{surface: w, a: a, b: b}, Range: parametric.Interval{Max: 1}})
		f.Face.Outer = append(f.Face.Outer, Use{Edge: i, UVStart: a, UVEnd: b})
	}
	return f
}

func TestWrappedRectangleFaceOverlap(t *testing.T) {
	ctx := context.Background()
	a := wrappedOverlapSupport(t)
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	makeFaces := func() (SewFace, SewFace) {
		return wrappedOverlapFace(t, a, geom.ParameterRectangle{UMin: 4.125, UMax: 4.375, VMin: -1, VMax: 1}), wrappedOverlapFace(t, b, geom.ParameterRectangle{UMin: .25, UMax: .5, VMin: 0, VMax: 2})
	}
	x, y := makeFaces()
	y.Face.Reversed = true
	// Reverse one original model edge and use a strict subrange with identical
	// continuous trace, so ancestry must report its true parameter units.
	u := &x.Face.Outer[0]
	edge := &x.Edges[u.Edge]
	delta := geom.Vec2{X: u.UVEnd.X - u.UVStart.X, Y: u.UVEnd.Y - u.UVStart.Y}
	edge.Curve = surfaceTrimCurve{surface: a, a: geom.Vec2{X: u.UVEnd.X + delta.X*.5, Y: u.UVEnd.Y + delta.Y*.5}, b: geom.Vec2{X: u.UVStart.X - delta.X*.5, Y: u.UVStart.Y - delta.Y*.5}}
	edge.Range = parametric.Interval{Min: .25, Max: .75}
	edge.Start, edge.End = edge.End, edge.Start
	u.Reversed = true
	opts := WrappedRectangleFaceOverlapOptions{ProjectionAxes: [2]int{1, 2}}
	r, e := CertifyWrappedRectangleFaceOverlap(ctx, x, y, opts)
	if e != nil || !r.Complete || !r.FaceReversed[1] || len(r.Union) != 12 || len(r.Intersection) != 4 {
		t.Fatal(r, e)
	}
	checked := 0
	for _, cycle := range [][]WrappedFaceRegionFragment{r.Union, r.Intersection} {
		for i, f := range cycle {
			next := cycle[(i+1)%len(cycle)]
			for axis := 0; axis < 3; axis++ {
				if f.End[axis].Cmp(next.Start[axis]) != 0 {
					t.Fatal("open physical cycle")
				}
			}
			for _, owner := range f.Owners {
				for end, p := range [2][3]*big.Rat{f.Start, f.End} {
					w := a
					if owner.Face == 1 {
						w = b
					}
					jet, e := w.EvaluateRational(ctx, owner.UV[end][0], owner.UV[end][1], geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					// Independent polynomial oracle for this first curved span:
					// X=2-2u², Y=4u-2u², Z=v, with A lifted by four.
					canonical := new(big.Rat).Set(owner.UV[end][0])
					if owner.Face == 0 {
						canonical.Sub(canonical, big.NewRat(4, 1))
					}
					square := new(big.Rat).Mul(canonical, canonical)
					twice := new(big.Rat).Mul(big.NewRat(2, 1), square)
					want := [3]*big.Rat{new(big.Rat).Sub(big.NewRat(2, 1), twice), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), canonical), twice), owner.UV[end][1]}
					for axis := 0; axis < 3; axis++ {
						if p[axis].Cmp(want[axis]) != 0 {
							t.Fatal("independent model oracle")
						}
						if p[axis].Cmp(jet.Point[axis]) != 0 {
							t.Fatal("owner point")
						}
					}
				}
				if owner.Face == 0 && owner.Use == 0 {
					checked++
					if owner.Parameters[0].Cmp(big.NewRat(int64(4-checked), 4)) != 0 || owner.Parameters[1].Cmp(big.NewRat(int64(3-checked), 4)) != 0 {
						t.Fatal("original reversed range", owner)
					}
				}
			}
		}
	}
	if checked != 2 {
		t.Fatal("missing original edge")
	}
	reject := func(x, y SewFace, o WrappedRectangleFaceOverlapOptions) {
		t.Helper()
		out, e := CertifyWrappedRectangleFaceOverlap(ctx, x, y, o)
		if e == nil || out.Complete || len(out.Union) != 0 {
			t.Fatal("non-atomic rejection", out, e)
		}
	}
	x, y = makeFaces()
	original := x.Edges[0]
	line, e := geom.NewBSpline(1, []geom.Vec3{x.Vertices[original.Start], x.Vertices[original.End]}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, parameter := range []float64{0, 1} {
		p, err := line.Eval(parameter)
		q, other := original.Curve.Eval(parameter)
		if err != nil || other != nil || p != q {
			t.Fatal("impostor endpoints do not match")
		}
	}
	p, _ := line.Eval(.5)
	q, _ := original.Curve.Eval(.5)
	if p == q {
		t.Fatal("impostor is not curved-distinct")
	}
	x.Edges[0].Curve = line
	reject(x, y, opts) // Matching endpoints, bowed continuous model mismatch.
	x, y = makeFaces()
	x.Vertices[0].X += .125
	reject(x, y, opts)
	x, y = makeFaces()
	bad := opts
	bad.MaxOperations = 1
	reject(x, y, bad)
	x, y = makeFaces()
	x.Face.Outer[2].Edge = 99
	reject(x, y, opts)
	x, y = makeFaces()
	x.Face.Holes = [][]Use{x.Face.Outer}
	reject(x, y, opts)
	x, y = makeFaces()
	complete, err := CertifyWrappedRectangleFaceOverlap(ctx, x, y, opts)
	if err != nil {
		t.Fatal(err)
	}
	bad = opts
	bad.MaxOperations = complete.ExactOperations - 1
	reject(x, y, bad)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	out, e := CertifyWrappedRectangleFaceOverlap(canceled, x, y, opts)
	if e == nil || out.Complete {
		t.Fatal(out, e)
	}
}
