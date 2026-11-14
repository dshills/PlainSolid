package document

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestSphericalHierarchyDocument(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile("../exchange/step/testdata/independent-spherical-hierarchy.step")
	if err != nil {
		t.Fatal(err)
	}
	proof, err := step.ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), step.Limits{}, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ReadSphericalHierarchySTEP(ctx, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9, 2)
	if err != nil {
		t.Fatal(err)
	}
	var boundSource bytes.Buffer
	if err = step.WriteIndependentSphericalHierarchyJSON(ctx, &boundSource, proof); err != nil {
		t.Fatal(err)
	}
	fromNative, err := ReadSphericalHierarchyNative(ctx, bytes.NewReader(boundSource.Bytes()), step.IndependentMaterialJSONLimits{}, STEPAssemblyOptions{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	var stored bytes.Buffer
	if err = d.Write(&stored); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(stored.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	var again bytes.Buffer
	if err = reopened.Write(&again); err != nil || !bytes.Equal(stored.Bytes(), again.Bytes()) {
		t.Fatal("editable native snapshot is not stable", err)
	}
	for _, doc := range []*Document{d, fromNative, reopened} {
		s, _ := doc.Snapshot()
		if len(s.Planar) != 1 || len(s.Trimmed) != 3 || len(s.Assets) != 0 || s.LengthUnit != .001 {
			t.Fatal("shared exact assets and units", len(s.Planar), len(s.Trimmed), s.LengthUnit)
		}
		uses := map[string]int{}
		for id, n := range s.Nodes {
			if n.Asset == "" {
				continue
			}
			l := s.Labels["source-node:"+id]
			if l.TargetID != id || l.Attributes["step.source.path"].Text != id || l.Attributes["step.source.nauo"].Integer == 0 {
				returnUnexpectedNode(t, id, n)
			}
			uses[n.Asset]++
		}
		root := "step-product-" + strconv.FormatInt(proof.Assembly.Roots[0], 10)
		var spheres []geom.Vec3
		for _, leaf := range proof.Instances {
			key := root
			for _, id := range leaf.Path {
				key += "/occ-" + strconv.FormatInt(id, 10)
			}
			n, ok := s.Nodes[key]
			pose, err := s.WorldTransform(key)
			if !ok || err != nil || pose != leaf.Transform {
				t.Fatal("original path and composed pose", key, pose, err)
			}
			label := s.Labels["source:"+n.Asset]
			if label.Attributes["step.source.definition"].Integer != leaf.ProductID {
				returnUnexpectedNode(t, key, n)
			}
			if label.Attributes["step.material.family"].Text == "sphere" {
				if uses[n.Asset] != 2 {
					t.Fatal("sphere definitions were duplicated", uses)
				}
				spheres = append(spheres, pose.Apply(geom.Vec3{X: 1, Y: 2, Z: 3}))
			}
		}
		if len(spheres) != 2 || spheres[0] == spheres[1] || len(uses) != 4 {
			t.Fatal("two distinct uses of one sphere", spheres, uses)
		}
	}
	state0, _ := d.Snapshot()
	state1, _ := fromNative.Snapshot()
	if !reflect.DeepEqual(state0.Nodes, state1.Nodes) || !reflect.DeepEqual(state0.Labels, state1.Labels) {
		t.Fatal("source-bound and editable document views disagree")
	}
	var sphereNodes []string
	for id, n := range state0.Nodes {
		if state0.Labels["source:"+n.Asset].Attributes["step.material.family"].Text == "sphere" {
			sphereNodes = append(sphereNodes, id)
		}
	}
	if len(sphereNodes) != 2 {
		t.Fatal(sphereNodes)
	}
	x, y := sphereNodes[0], sphereNodes[1]
	beforeX, e := state0.WorldTransform(x)
	if e != nil {
		t.Fatal(e)
	}
	beforeY, e := state0.WorldTransform(y)
	if e != nil {
		t.Fatal(e)
	}
	tx := d.Begin()
	n := tx.State.Nodes[x]
	n.Transform[0][3] += 1
	tx.State.Nodes[x] = n
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	changed, _ := d.Snapshot()
	changedX, e := changed.WorldTransform(x)
	if e != nil {
		t.Fatal(e)
	}
	changedY, e := changed.WorldTransform(y)
	if e != nil {
		t.Fatal(e)
	}
	if changedX == beforeX || changedY != beforeY || changed.Nodes[x].Asset != changed.Nodes[y].Asset {
		t.Fatal("editing one occurrence changed a shared product or sibling placement")
	}
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	undone, _ := d.Snapshot()
	if !reflect.DeepEqual(undone.Nodes, state0.Nodes) {
		t.Fatal("original path placement did not survive Undo")
	}
	for _, lim := range []STEPAssemblyOptions{{MaxNodes: 2}, {MaxDepth: 2}} {
		if d, err := ReadSphericalHierarchySTEP(ctx, bytes.NewReader(raw), lim, 1e-9, 0); d != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("document expansion should reject atomically", d, err)
		}
	}
	bad := strings.Replace(boundSource.String(), "plainsolid-step-independent-spherical-hierarchy", "plainsolid-step-independent-analytic-hierarchy", 1)
	if d, err := ReadSphericalHierarchyNative(ctx, strings.NewReader(bad), step.IndependentMaterialJSONLimits{}, STEPAssemblyOptions{}, 0); d != nil || err == nil {
		t.Fatal("different source proof tag accepted", d, err)
	}
}

func returnUnexpectedNode(t *testing.T, id string, n Node) {
	t.Helper()
	t.Fatal("missing original node or product provenance", id, n)
}
