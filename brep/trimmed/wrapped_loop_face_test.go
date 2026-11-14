package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func TestWrappedLoopFaceSeamHole(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	outer := []geom.Vec2{{X: -.25, Y: -1}, {X: .25, Y: -1}, {X: .25, Y: 1}, {X: -.25, Y: 1}}
	hole := []geom.Vec2{{X: -.125, Y: 0}, {X: 0, Y: .5}, {X: .125, Y: 0}, {X: 0, Y: -.5}}
	for _, reverse := range []bool{false, true} {
		o := WrappedLoopFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: reverse}
		r, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{hole}, o)
		if e != nil {
			t.Fatal(e)
		}
		f := r.NativeFace()
		if r.FundamentalPieces() != 2 || f.Face.Reversed != reverse || f.Face.Surface != w || len(f.Face.Holes) != 1 || len(f.Edges) != 8 {
			t.Fatal("lost hole/support")
		}
		open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
		proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal("continuous edge proof", e)
		}
		loops := r.OriginalLoops()
		edgeID := 0
		for li, l := range loops {
			area := 0.0
			for i, a := range l {
				b := l[(i+1)%len(l)]
				area += a.X*b.Y - b.X*a.Y
				side, e := r.OriginalSide(li, i)
				if e != nil || side[0][0].Cmp(ratInset(a.X)) != 0 || side[1][1].Cmp(ratInset(b.Y)) != 0 {
					t.Fatal("lost original ancestry")
				}
				for _, s := range []float64{0, .25, .5, .75, 1} {
					u, v := a.X+s*(b.X-a.X), a.Y+s*(b.Y-a.Y)
					y := 4*math.Abs(u) - 2*u*u
					if u < 0 {
						y = -y
					}
					want := geom.Vec3{X: 2 - 2*u*u, Y: y, Z: v}
					p, e := f.Edges[edgeID].Curve.Eval(s)
					if e != nil || p != want {
						t.Fatal("independent model", li, i, s, p, want, e)
					}
				}
				edgeID++
			}
			if (li == 0 && area <= 0) || (li > 0 && area >= 0) {
				t.Fatal("changed winding")
			}
		}
		loops[1][0].X = 99
		f.Face.Holes[0][0].UVStart.X = 99
		if r.OriginalLoops()[1][0].X == 99 || r.NativeFace().Face.Holes[0][0].UVStart.X == 99 {
			t.Fatal("mutable hole")
		}
		low := o
		low.MaxOperations = r.ExactOperations() - 1
		if q, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{hole}, low); e == nil || q != nil {
			t.Fatal("partial quota")
		}
		cancel, stop := context.WithCancel(ctx)
		stop()
		if q, e := NewWrappedLoopFace(cancel, w, outer, [][]geom.Vec2{hole}, o); e == nil || q != nil {
			t.Fatal("partial cancellation")
		}
	}
}

func TestWrappedLoopFaceNonconvex(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	outer := []geom.Vec2{{X: -.25, Y: -1}, {X: .25, Y: -1}, {X: .25, Y: 0}, {X: .125, Y: 0}, {X: .125, Y: 1}, {X: -.25, Y: 1}}
	hole := []geom.Vec2{{X: -.0625, Y: 0}, {X: 0, Y: .25}, {X: .0625, Y: 0}, {X: 0, Y: -.25}}
	o := WrappedLoopFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	if r, e := NewWrappedPolygonFace(ctx, w, outer, WrappedPolygonFaceOptions{SurfaceInjectivityOptions: o.SurfaceInjectivityOptions}); e == nil || r != nil {
		t.Fatal("legacy convex scope widened")
	}
	r, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{hole}, o)
	if e != nil {
		t.Fatal(e)
	}
	f := r.NativeFace()
	if len(f.Face.Outer) != 6 || len(f.Face.Holes) != 1 {
		t.Fatal("nonconvex topology")
	}
	open := Solid{vertices: f.Vertices, edges: f.Edges, faces: []Face{f.Face}, tolerance: 1e-9}
	proof, e := open.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
}

func TestWrappedLoopFaceReject(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	outer := []geom.Vec2{{X: -.25, Y: -1}, {X: .25, Y: -1}, {X: .25, Y: 1}, {X: -.25, Y: 1}}
	hole := []geom.Vec2{{X: -.125, Y: 0}, {X: 0, Y: .5}, {X: .125, Y: 0}, {X: 0, Y: -.5}}
	o := WrappedLoopFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}}
	touch := append([]geom.Vec2(nil), hole...)
	touch[0].X = -.25
	outside := append([]geom.Vec2(nil), hole...)
	for i := range outside {
		outside[i].X += .5
	}
	small := append([]geom.Vec2(nil), hole...)
	for i := range small {
		small[i].X /= 2
		small[i].Y /= 2
	}
	wrong := append([]geom.Vec2(nil), hole...)
	for i, j := 0, len(wrong)-1; i < j; i, j = i+1, j-1 {
		wrong[i], wrong[j] = wrong[j], wrong[i]
	}
	cross := []geom.Vec2{hole[0], hole[2], hole[1], hole[3]}
	for i, holes := range [][][]geom.Vec2{{touch}, {outside}, {hole, small}, {hole, hole}, {wrong}, {cross}} {
		if r, e := NewWrappedLoopFace(ctx, w, outer, holes, o); e == nil || r != nil {
			t.Fatal("accepted invalid hole", i, e)
		}
	}
	near := append([]geom.Vec2(nil), touch...)
	near[0].X += 1.0 / 1048576
	if r, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{near}, o); e != nil || r == nil {
		t.Fatal("strict near-boundary hole", e)
	}
	near[0].X = -.25 - 1.0/1048576
	if r, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{near}, o); e == nil || r != nil {
		t.Fatal("near-boundary exit accepted")
	}
	limited := o
	limited.MaxVertices = 7
	if r, e := NewWrappedLoopFace(ctx, w, outer, [][]geom.Vec2{hole}, limited); e == nil || r != nil {
		t.Fatal("per-loop instead of total edge limit")
	}
}
