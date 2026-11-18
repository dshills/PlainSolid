//go:build research

package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedPolygonFaceSeam(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	uv := []geom.Vec2{{X: -.25, Y: -1}, {X: .25, Y: 0}, {X: -.25, Y: 1}}
	for _, reversed := range []bool{false, true} {
		o := WrappedPolygonFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: reversed}
		r, e := NewWrappedPolygonFace(ctx, w, uv, o)
		if e != nil {
			t.Fatal(e)
		}
		f := r.NativeFace()
		if r.FundamentalPieces() != 2 || f.Face.Surface != w || f.Face.Reversed != reversed || len(f.Edges) != 3 {
			t.Fatal("lost original support")
		}
		open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
		proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal("native edge agreement", e)
		}
		// Independent exact polynomial cylinder profile on either side of seam.
		for i, edge := range f.Edges {
			a, b := uv[i], uv[(i+1)%3]
			side, e := r.OriginalSide(i)
			if e != nil || side[0][0].Cmp(ratInset(a.X)) != 0 || side[1][1].Cmp(ratInset(b.Y)) != 0 {
				t.Fatal("original side")
			}
			for _, s := range []float64{0, .25, .5, .75, 1} {
				u, v := a.X+s*(b.X-a.X), a.Y+s*(b.Y-a.Y)
				y := 4*math.Abs(u) - 2*u*u
				if u < 0 {
					y = -y
				}
				want := geom.Vec3{X: 2 - 2*u*u, Y: y, Z: v}
				got, e := edge.Curve.Eval(s)
				if e != nil || got != want {
					t.Fatal("independent model equation", i, s, got, want, e)
				}
			}
		}
		// Both diagonal edges cross the seam at an interior exact parameter.
		for _, i := range []int{0, 1} {
			p, e := f.Edges[i].Curve.Eval(.5)
			if e != nil || p.X != 2 || p.Y != 0 {
				t.Fatal("diagonal seam", e)
			}
		}
		f.Face.Outer[0].UVStart.X = 77
		f.Vertices[0].X = 77
		original := r.OriginalUV()
		original[0].X = 77
		side, _ := r.OriginalSide(0)
		side[0][0].SetInt64(77)
		if r.NativeFace().Vertices[0].X == 77 || r.OriginalUV()[0].X == 77 {
			t.Fatal("mutable face")
		}
		low := o
		low.MaxOperations = r.ExactOperations() - 1
		if q, e := NewWrappedPolygonFace(ctx, w, uv, low); e == nil || q != nil {
			t.Fatal("partial budget publication")
		}
		cancel, stop := context.WithCancel(ctx)
		stop()
		if q, e := NewWrappedPolygonFace(cancel, w, uv, o); e == nil || q != nil {
			t.Fatal("partial cancellation")
		}
	}
	// Exact same lifted triangle on a different integer-period occurrence.
	shifted := append([]geom.Vec2(nil), uv...)
	for i := range shifted {
		shifted[i].X += 4
	}
	r, e := NewWrappedPolygonFace(ctx, w, shifted, WrappedPolygonFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	if r.OriginalUV()[0].X != 3.75 {
		t.Fatal("collapsed owner lift")
	}
	for i, p := range r.NativeFace().Vertices {
		want := geom.Vec3{X: 15.0 / 8, Y: -7.0 / 8, Z: uv[i].Y}
		if i == 1 {
			want.Y = 7.0 / 8
		}
		if p != want {
			t.Fatal("lift changed geometry")
		}
	}
}

func TestWrappedPolygonFaceReject(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	o := WrappedPolygonFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	cases := [][]geom.Vec2{
		{{X: -.25, Y: -1}, {X: -.25, Y: 1}, {X: .25, Y: 0}},                                // clockwise
		{{X: -.25, Y: -1}, {X: .25, Y: -1}, {X: 0, Y: 0}, {X: .25, Y: 1}, {X: -.25, Y: 1}}, // concave
		{{X: -.25, Y: -1}, {X: .25, Y: 1}, {X: .25, Y: -1}, {X: -.25, Y: 1}},               // crossing
		{{X: 0, Y: -1}, {X: 4, Y: 0}, {X: 0, Y: 1}},                                        // duplicate quotient seam
		{{X: -1, Y: -1}, {X: 1, Y: 0}, {X: -1, Y: 1}},                                      // projected derivative singular
		{{X: -.25, Y: -1}, {X: .25, Y: -1}, {X: 0, Y: -1}},                                 // degenerate
	}
	for i, uv := range cases {
		if r, e := NewWrappedPolygonFace(ctx, w, uv, o); e == nil || r != nil {
			t.Fatal("unsupported accepted", i, e)
		}
	}
}
