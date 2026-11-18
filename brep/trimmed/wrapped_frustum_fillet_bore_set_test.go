package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestMixedConicalFilletBores(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := FilletWrappedFrustumRims(ctx, source, 15./16, 15./8, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}
	result, e := FilletAndBoreWrappedFrustumSet(ctx, source, 15./16, 15./8, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	out := result.Solid
	if out.Topology().Faces != 10 || out.Topology().EulerCharacteristic != 0 || len(out.Topology().Shells) != 1 {
		t.Fatal(out.Topology())
	}
	if !reflect.DeepEqual(stock.Vertices(), out.Vertices()[:len(stock.Vertices())]) {
		t.Fatal("original vertices")
	}
	for i, edge := range stock.Edges() {
		if !reflect.DeepEqual(edge, out.Edges()[i]) {
			t.Fatal("original edge", i)
		}
	}
	for i, f := range stock.Faces() {
		g := out.Faces()[i]
		if !reflect.DeepEqual(f.Surface, g.Surface) || !reflect.DeepEqual(f.Outer, g.Outer) || !reflect.DeepEqual(result.SourceFaces[i], []int{i}) {
			t.Fatal("stock owner", i)
		}
	}
	for _, cap := range result.Fillets.Caps {
		if len(out.Faces()[cap].Holes) != 2 {
			t.Fatal("holes")
		}
	}
	for i, h := range result.Cuts {
		if len(result.SourceFaces[h.BoreFace]) != 0 {
			t.Fatal("generated owner")
		}
		if i == 0 {
			if h.FloorFace != -1 || h.FloorZ != nil {
				t.Fatal(h)
			}
		} else {
			if h.FloorFace < 0 || h.FloorZ == nil || len(result.SourceFaces[h.FloorFace]) != 0 {
				t.Fatal(h)
			}
			floor := out.Faces()[h.FloorFace]
			plane, ok := floor.Surface.(geom.PlaneSurface)
			if !ok {
				t.Fatal("floor support")
			}
			normal := plane.Frame().ZAxis()
			if floor.Reversed {
				normal = normal.Scale(-1)
			}
			want := -1.
			if cuts[i].FromUpper {
				want = 1
			}
			if normal.Z != want {
				t.Fatal("floor outward normal", i, normal)
			}
		}
	}
	a, e := stock.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	b, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(a.Volume-b.Volume-11*math.Pi/4) > 1e-7 || math.Abs(b.Area-a.Area-21*math.Pi/2) > 1e-7 {
		t.Fatal(a, b)
	}
	var buf bytes.Buffer
	if e = WriteJSON(&buf, out); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil || back.Topology().Faces != 10 {
		t.Fatal(e)
	}
	for _, badCuts := range [][]FilletedFrustumBoreCut{{cuts[0], cuts[0]}, {{Center: geom.Vec2{}, Radius: .5}, {Center: geom.Vec2{X: 1}, Radius: .5}}, {{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 2}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, {{Radius: .5, Depth: 6}}, make([]FilletedFrustumBoreCut, 17)} {
		bad, e := FilletAndBoreWrappedFrustumSet(ctx, source, 15./16, 15./8, badCuts, o)
		if e == nil || bad.Solid.Topology().Faces != 0 || bad.Cuts != nil {
			t.Fatal("atomic failure", e)
		}
	}
}
