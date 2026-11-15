package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestVertexContactAssemblyNativeKeepsSeparateIndexedSolids(t *testing.T) {
	ctx := context.Background()
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	var source VertexContactAssembly
	for _, span := range [][2]float64{{0, 1}, {1, 2}, {5, 6}} {
		s, err := brep.Box(xyz(span[0]), xyz(span[1]))
		if err != nil {
			t.Fatal(err)
		}
		source.Components = append(source.Components, s)
	}
	var ids [2]int
	for i, s := range source.Components[:2] {
		for j, v := range s.Vertices() {
			if v == xyz(1) {
				ids[i] = j
			}
		}
	}
	source.Contacts = []collision.AssemblyVertexContact{{A: collision.AssemblyVertex{Solid: 1, Vertex: ids[1]}, B: collision.AssemblyVertex{Solid: 0, Vertex: ids[0]}}}
	var native, reemit bytes.Buffer
	if err := WriteVertexContactAssemblyJSON(ctx, &native, source, VertexContactAssemblyJSONOptions{}); err != nil {
		t.Fatal(err)
	}
	restored, report, err := ReadVertexContactAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), VertexContactAssemblyJSONOptions{})
	if err != nil || len(restored.Components) != 3 || len(restored.Contacts) != 1 || report.SignedVolume.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatalf("restored assembly: %+v, %+v, %v", restored, report, err)
	}
	want := collision.AssemblyVertexContact{A: source.Contacts[0].B, B: source.Contacts[0].A}
	if restored.Contacts[0] != want || report.Contacts[0] != want {
		t.Fatal("contact vertex IDs changed or were welded")
	}
	for i, c := range restored.Components {
		b := source.Components[i]
		if !reflect.DeepEqual(c.Faces(), b.Faces()) || !reflect.DeepEqual(c.Edges(), b.Edges()) || !reflect.DeepEqual(c.Vertices(), b.Vertices()) {
			t.Fatalf("original component %d changed", i)
		}
	}
	if err := WriteVertexContactAssemblyJSON(ctx, &reemit, restored, VertexContactAssemblyJSONOptions{}); err != nil || !bytes.Equal(reemit.Bytes(), native.Bytes()) {
		t.Fatal("native assembly did not re-emit deterministically", err)
	}
	if _, err := ReadBRepJSON(bytes.NewReader(native.Bytes()), BRepLimits{}); err == nil {
		t.Fatal("ordinary planar BRep accepted a component assembly")
	}
	for _, mutate := range []func(*vertexContactAssemblyDocument){
		func(d *vertexContactAssemblyDocument) { d.Contacts = d.Contacts[:0] },
		func(d *vertexContactAssemblyDocument) {
			d.Contacts[0].A.Vertex = (d.Contacts[0].A.Vertex + 1) % len(d.Components[0].Vertices)
		},
		func(d *vertexContactAssemblyDocument) {
			// Move a whole component, retaining its valid individual topology.
			for i := range d.Components[1].Vertices {
				d.Components[1].Vertices[i].X += .25
			}
		},
	} {
		var edited vertexContactAssemblyDocument
		if err := json.Unmarshal(native.Bytes(), &edited); err != nil {
			t.Fatal(err)
		}
		mutate(&edited)
		raw, err := json.Marshal(edited)
		if err != nil {
			t.Fatal(err)
		}
		if a, r, err := ReadVertexContactAssemblyJSON(ctx, bytes.NewReader(raw), VertexContactAssemblyJSONOptions{}); !errors.Is(err, collision.ErrInvalid) || a.Components != nil || r.SignedVolume != nil {
			t.Fatalf("altered contact admitted: %+v, %+v, %v", a, r, err)
		}
	}
	for _, o := range []VertexContactAssemblyJSONOptions{{MaxBytes: 80}, {Component: BRepLimits{MaxBytes: 80}}, {Proof: collision.VertexContactAssemblyOptions{MaxCrossPairs: 1}}} {
		var rejected bytes.Buffer
		if err := WriteVertexContactAssemblyJSON(ctx, &rejected, source, o); !errors.Is(err, collision.ErrLimit) || rejected.Len() != 0 {
			t.Fatalf("unsupported budget wrote a prefix: %v", err)
		}
		if a, r, err := ReadVertexContactAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), o); !errors.Is(err, collision.ErrLimit) || a.Components != nil || r.SignedVolume != nil {
			t.Fatalf("unsupported budget read assembly: %+v, %+v, %v", a, r, err)
		}
	}
}
