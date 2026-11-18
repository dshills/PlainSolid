package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestMixedConicalAP242DAGDocument(t *testing.T) {
	slowtest.Skip(t)
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-curved-ap242-exact.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGDocumentLimits{Assembly: step.MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}, MaxNodes: 12}
	doc, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(raw), l, options, 1)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := doc.Snapshot()
	if s.LengthUnit != .001 || len(s.Nodes) != 12 || len(s.Trimmed) != 2 || len(s.Trimmed["step-product-89"].Faces()) != 6 || len(s.Trimmed["step-product-254"].Faces()) != 8 {
		t.Fatal("DAG document duplicated or lost a source definition or path")
	}
	const root = "step-product-5"
	const coreA, coreB = root + "/occ-54/occ-85", root + "/occ-466/occ-472"
	const leafA, leafB, direct = coreA + "/occ-443", coreB + "/occ-443", root + "/occ-477"
	for _, id := range []string{leafA, leafB, direct} {
		if s.Nodes[id].Asset != "step-product-254" {
			t.Fatal("lost shared material on a direct or nested path", id)
		}
	}
	for _, key := range []string{coreA, coreB} {
		if s.Nodes[key].Asset != "" || s.Labels["source-node:"+key].Attributes["step.source.definition"].Integer != 58 {
			t.Fatal("different occurrence paths did not retain their shared group definition", key)
		}
	}
	source := s.Labels["source-node:"+leafB]
	if source.Attributes["step.source.nauo"].Integer != 443 || source.Attributes["step.source.world-pose"].Text !=
		"9/25,-12/25,4/5,-488/5;108/125,-19/125,-12/25,-1121/25;44/125,108/125,9/25,497/25" {
		t.Fatal("exact depth-three source pose did not reach the document", source)
	}
	world := func(s State, key string) geom.Vec3 {
		t.Helper()
		a, err := s.WorldTransform(key)
		if err != nil {
			t.Fatal(err)
		}
		return a.Apply(geom.Vec3{X: 1.5, Y: 2, Z: 4})
	}
	want := geom.Vec3{X: -4741. / 50, Y: -5721. / 125, Z: 2947. / 125}
	if got := world(s, leafB); math.Abs(got.X-want.X)+math.Abs(got.Y-want.Y)+math.Abs(got.Z-want.Z) > 5e-13 {
		t.Fatal("document changed the independently authored world point", got)
	}
	tx := doc.Begin()
	id := root + "/occ-466"
	group := tx.State.Nodes[id]
	group.Transform[0][3] += 9
	tx.State.Nodes[id] = group
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	after, _ := doc.Snapshot()
	if world(s, leafA) != world(after, leafA) || world(s, direct) != world(after, direct) ||
		math.Abs(world(after, leafB).X-world(s, leafB).X-9) > 1e-13 || !reflect.DeepEqual(after.Labels["source-node:"+leafB], source) {
		t.Fatal("editing one parent changed a different shared path or its source provenance")
	}
	var native, replay bytes.Buffer
	if err := doc.Write(&native); err != nil {
		t.Fatal(err)
	}
	r, err := Read(bytes.NewReader(native.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Write(&replay); err != nil || !bytes.Equal(native.Bytes(), replay.Bytes()) {
		t.Fatal("depth-three document did not save and reopen byte-identically", err)
	}
	l.MaxNodes--
	if doc, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(raw), l, options, 1); !errors.Is(err, geom.ErrConvergence) || doc != nil {
		t.Fatal("a short total-node budget published an incomplete document", err)
	}
}

func TestMixedConicalAP242DAGDocumentWrite(t *testing.T) {
	slowtest.Skip(t)
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-curved-ap242-exact.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx, o := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGDocumentLimits{Assembly: step.MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}, MaxNodes: 12}
	d, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(raw), l, o, 0)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := d.Snapshot()
	const parent = "step-product-5/occ-466"
	const useA, useB = "step-product-5/occ-54/occ-85/occ-443", parent + "/occ-472/occ-443"
	a := s.Nodes[useA]
	a.Name = "Shared rim fixture"
	s.Nodes[useA] = a
	var out bytes.Buffer
	if err := s.WriteMixedConicalAP242DAGSTEP(ctx, &out, bytes.NewReader(raw), l, o); !errors.Is(err, geom.ErrUnrepresentable) ||
		!strings.Contains(err.Error(), "shared STEP occurrence 443") || out.Len() != 0 {
		t.Fatal("a shared relationship conflict published mixed STEP", err)
	}
	b := s.Nodes[useB]
	b.Name = a.Name
	s.Nodes[useB] = b
	p := s.Nodes[parent]
	p.Transform[0][3] += 5.125
	s.Nodes[parent] = p
	if err := s.WriteMixedConicalAP242DAGSTEP(ctx, &out, bytes.NewReader(raw), l, o); err != nil {
		t.Fatal(err)
	}
	readback, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(out.Bytes()), l, o, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := readback.Snapshot()
	if !reflect.DeepEqual(s.Nodes, got.Nodes) || len(got.Trimmed) != 2 || got.LengthUnit != .001 {
		t.Fatal("STEP writeback lost an edited relationship or duplicated local material")
	}
}
