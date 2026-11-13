package document

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
	"plainsolid/scene"
)

func TestPickWrappedDocumentSharedOccurrence(t *testing.T) {
	ctx := context.Background()
	camera, e := scene.Orthographic(geom.Vec3{Z: 10}, geom.Vec3{}, geom.Vec3{Y: 1}, 8, 8, 1, 11)
	if e != nil {
		t.Fatal(e)
	}
	state := State{LengthUnit: 1,
		Features: map[string]Feature{"part": {Kind: WrappedCylinderFeature, Radius: 2, Offset: geom.Vec3{Z: 4}}},
		Nodes: map[string]Node{
			"lower": {Asset: "part", Transform: geom.Identity()},
			"group": {Transform: geom.Translation(geom.Vec3{Z: 1})},
			"upper": {Asset: "part", Parent: "group", Transform: geom.Translation(geom.Vec3{Z: 1})},
		}}
	h, hit, e := state.PickWrappedGeometry(ctx, camera, geom.Vec2{}, geom.ToroidLineOptions{})
	if e != nil || !hit || h.ID != "upper" || h.Part != geom.CylinderUpperCap || h.Distance.Min.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(h, hit, e)
	}
	n := state.Nodes["upper"]
	n.Transform = geom.Translation(geom.Vec3{X: 0x1p-60})
	state.Nodes["upper"] = n
	// On an off-center camera ray, this translation cannot be subtracted
	// without losing an exact model-space displacement.
	_, _, e = state.PickWrappedGeometry(ctx, camera, geom.Vec2{X: .25}, geom.ToroidLineOptions{})
	if !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded local ray origin", e)
	}
	n.Transform = geom.Translation(geom.Vec3{Z: 0x1p-60})
	state.Nodes["upper"] = n
	if _, _, e = state.PickWrappedGeometry(ctx, camera, geom.Vec2{}, geom.ToroidLineOptions{}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded parent and child translation", e)
	}
}
