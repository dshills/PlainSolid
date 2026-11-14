package step

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestIndependentSphericalHierarchyEditSharedDatum(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	// The torus use has the same identity datum in its child and parent;
	// the other uses use it as their child frame as well. It cannot be moved.
	const relationship, transformID, sharedDatum = 556, 558, 457
	pose := geom.Affine{{0, -1, 0, 0}, {1, 0, 0, 0}, {0, 0, 1, 2.5}}
	edits := []SphericalHierarchyOccurrenceEdit{{NAUOID: relationship, Name: "radial group", Transform: pose}}
	original, err := mappedPlanarSourceBytes(m.Assembly.Source)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteIndependentSphericalHierarchyEdits(ctx, &out, m.Assembly.Source, edits, Limits{}, 1e-9); err != nil {
		t.Fatal(err)
	}
	fresh, err := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	f := fresh.Assembly.Source
	base := m.Assembly.Source
	if len(f.Entities) != len(base.Entities)+4 || !reflect.DeepEqual(base.Entities[sharedDatum], f.Entities[sharedDatum]) {
		t.Fatal("shared source datum was changed or not cloned exactly once")
	}
	tr := f.Entities[transformID].Records[0]
	if tr.Args[2].Ref != sharedDatum || tr.Args[3].Ref == sharedDatum {
		t.Fatal("the wrong frame was changed", tr)
	}
	affected := 0
	for i, instance := range fresh.Instances {
		old := m.Instances[i]
		if !reflect.DeepEqual(instance.Path, old.Path) || instance.ProductID != old.ProductID {
			t.Fatal("source path identity changed", old, instance)
		}
		if instance.Path[len(instance.Path)-1] == relationship {
			affected++
			if instance.Transform != old.Transform.Mul(pose) {
				t.Fatal("shared-use edit lost a composed placement", instance)
			}
		} else if instance.Transform != old.Transform {
			t.Fatal("unselected occurrence changed", instance)
		}
	}
	if affected != 2 {
		t.Fatal("expected both paths through the shared relationship", affected)
	}
	after, err := mappedPlanarSourceBytes(base)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("editing mutated the caller's retained source", err)
	}
	for _, bad := range [][]SphericalHierarchyOccurrenceEdit{
		{edits[0], edits[0]},
		{{NAUOID: 9999, Transform: geom.Identity()}},
		{{NAUOID: relationship, Transform: geom.Affine{{-1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}}},
	} {
		var rejected bytes.Buffer
		if err := WriteIndependentSphericalHierarchyEdits(ctx, &rejected, base, bad, Limits{}, 1e-9); err == nil || rejected.Len() != 0 {
			t.Fatal("invalid edit left partial exchange output", bad, err)
		}
	}
	var rejected bytes.Buffer
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if err := WriteIndependentSphericalHierarchyEdits(cancelled, &rejected, base, edits, Limits{}, 1e-9); !errors.Is(err, context.Canceled) || rejected.Len() != 0 {
		t.Fatal("cancelled edit left exchange output", err)
	}
}
