//go:build research

package trimmed

import (
	"context"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactSewDistinguishesTouchingVertexFans(t *testing.T) {
	ctx := context.Background()
	var components []Solid
	for _, limits := range [][2]geom.Vec3{{{X: -1, Y: -1, Z: -1}, {}}, {{}, {X: 1, Y: 1, Z: 1}}} {
		planar, err := brep.Box(limits[0], limits[1])
		if err != nil {
			t.Fatal(err)
		}
		component, err := FromPlanar(planar, 1e-8)
		if err != nil {
			t.Fatal(err)
		}
		components = append(components, component)
	}
	var vertices []geom.Vec3
	var edges []parametric.Edge
	var faces []Face
	for _, component := range components {
		v, e, f := explodeExactSewSource(component)
		for i := range e {
			e[i].Start += len(vertices)
			e[i].End += len(vertices)
		}
		for i := range f {
			for j := range f[i].Outer {
				f[i].Outer[j].Edge += len(edges)
			}
		}
		vertices = append(vertices, v...)
		edges = append(edges, e...)
		faces = append(faces, f...)
	}
	out, _, err := SewExactTopology(ctx, vertices, edges, faces, ExactSewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if top := out.Topology(); len(top.Shells) != 2 || top.Vertices != 16 || top.Edges != 24 {
		t.Fatal("coincident independent vertex fans must retain separate identities", top)
	}
	split, report, err := SewExactStraightSubdivisions(ctx, vertices, edges, faces, ExactStraightSewOptions{})
	if err != nil || report.InsertedVertices != 0 || len(split.Topology().Shells) != 2 || split.Topology().Vertices != 16 {
		t.Fatal("straight subdivision changed pure point contact", split.Topology(), report.InsertedVertices, err)
	}
}
