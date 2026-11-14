package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestRefinedWrappedSeamAgreement(t *testing.T) {
	ctx := context.Background()
	a := independentWrappedToroid(t)
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e = b.InsertKnot(ctx, 1, .375, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if geom.SameWrappedSurfaceRecipe(a, b) {
		t.Fatal("fixture must differ in basis")
	}
	f, e := WrappedSplineFace(ctx, a, false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	f.Face.Surface = b // model curves retain original a; actual face independently refined
	same, e := compareWrappedEdgeUse(ctx, f.Edges[0], b, f.Face.Outer[0], edgecheck.Options{})
	if e != nil || !same {
		t.Fatal(same, e)
	}
	if sameAnalyticEdgeUse(f.Edges[0], b, f.Face.Outer[0]) {
		t.Fatal("recipe shortcut unexpectedly qualifies fixture")
	}
	solid, _, e := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	proof, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete || solid.Topology().Edges != 2 {
		t.Fatal(proof, e)
	}
	rejected, _, err := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9, Proof: edgecheck.Options{MaxWork: 1}})
	if err == nil || len(rejected.Faces()) != 0 {
		t.Fatal("sewing budget published geometry", err)
	}
	// An affine diagonal, not an iso curve, has exact constant integer-period
	// agreement; varying or fractional offsets must not infer equal occurrences.
	model := parametric.Edge{Curve: surfaceTrimCurve{surface: a, a: geom.Vec2{}, b: geom.Vec2{X: 1, Y: 1}}, Range: parametric.Interval{Max: 1}}
	use := Use{UVStart: geom.Vec2{X: 4}, UVEnd: geom.Vec2{X: 5, Y: 1}}
	same, e = compareWrappedEdgeUse(ctx, model, b, use, edgecheck.Options{})
	if e != nil || !same {
		t.Fatal("diagonal", same, e)
	}
	use.UVEnd.X = 9
	same, e = compareWrappedEdgeUse(ctx, model, b, use, edgecheck.Options{})
	if e != nil || same {
		t.Fatal("varying offset", same, e)
	}
	use.UVStart.X = .5
	use.UVEnd.X = 1.5
	same, e = compareWrappedEdgeUse(ctx, model, b, use, edgecheck.Options{})
	if e != nil || same {
		t.Fatal("fractional offset", same, e)
	}
	use.UVStart.X = 4
	use.UVEnd.X = 5
	same, e = compareWrappedEdgeUse(ctx, model, b, use, edgecheck.Options{MaxWork: 1})
	if e == nil || same {
		t.Fatal("budget", same, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	same, e = compareWrappedEdgeUse(cancelled, model, b, use, edgecheck.Options{})
	if e == nil || same {
		t.Fatal("cancel", same, e)
	}
}
