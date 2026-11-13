package document

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
)

func TestMeshSceneSharedAssetAndUnits(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 30, Z: 40})
	if e != nil {
		t.Fatal(e)
	}
	s := State{LengthUnit: .001, Planar: map[string]brep.Solid{"part": b}, Nodes: map[string]Node{
		"group": {Name: "Group", Transform: geom.Translation(geom.Vec3{X: 300})},
		"left":  {Name: "Left", Parent: "group", Asset: "part", Transform: geom.Translation(geom.Vec3{Y: 100})},
		"right": {Name: "Right", Parent: "group", Asset: "part", Transform: geom.Translation(geom.Vec3{Y: -100})},
	}}
	ctx := context.Background()
	m, e := s.MeshScene(ctx, "Fixture", parametric.TessellationOptions{}, 12)
	if e != nil || len(m.Meshes) != 1 || len(m.Nodes) != 3 || m.Nodes[1].Mesh != m.Nodes[2].Mesh || m.Nodes[1].World.Apply(geom.Vec3{}).Distance(geom.Vec3{X: .3, Y: .1}) > 1e-12 {
		t.Fatal(m, e)
	}
	d, e := FromMeshScene(m, 1)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := d.Snapshot()
	p, e := r.Flatten()
	if e != nil || len(r.Assets) != 1 || r.LengthUnit != 1 || math.Abs(p.SignedVolume()-2*.02*.03*.04) > 1e-14 {
		t.Fatal(r, p.SignedVolume(), e)
	}
	if _, e = s.MeshScene(ctx, "Fixture", parametric.TessellationOptions{}, 11); e == nil {
		t.Fatal("accepted definition over triangle limit")
	}
	for _, binary := range []bool{false, true} {
		var stream bytes.Buffer
		if e = s.WriteGLTFScene(ctx, &stream, "Fixture", parametric.TessellationOptions{}, 12, binary); e != nil {
			t.Fatal(e)
		}
		var r exchange.MeshScene
		if binary {
			r, e = exchange.ReadGLBScene(&stream, exchange.SceneLimits{})
		} else {
			r, e = exchange.ReadGLTFScene(&stream, exchange.SceneLimits{})
		}
		if e != nil || len(r.Meshes) != 1 || len(r.Nodes) != 3 || r.Name != "Fixture" || r.Nodes[1].Mesh != r.Nodes[2].Mesh {
			t.Fatal(r, e)
		}
	}
	m.Nodes[2].Children = []int{0}
	if _, e = FromMeshScene(m, 1); e == nil {
		t.Fatal("accepted a cycle")
	}
}
