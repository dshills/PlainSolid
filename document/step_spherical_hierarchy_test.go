package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
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
		for id, body := range proof.Products {
			key := "step-product-" + strconv.FormatInt(id, 10)
			sourceIDs, err := step.SphericalHierarchyNativeFaceSources(body)
			if err != nil {
				t.Fatal(err)
			}
			for index, original := range sourceIDs {
				entityID, faceIndex := strconv.FormatInt(original, 10), strconv.Itoa(index)
				f := s.Labels["source-face:"+key+":"+entityID+":"+faceIndex]
				if f.Face == nil || *f.Face != index || f.Parent != "source:"+key || f.Target != FaceLabel || f.TargetID != key || f.Attributes["step.source.face"].Integer != original {
					t.Fatal("lost original face label after materialization or reload", id, index, original, f)
				}
			}
			if body.Sphere != nil {
				for _, original := range sourceIDs {
					if original != body.Sphere.SourceFaceID {
						t.Fatal("sphere chart has wrong original source face", original, body.Sphere.SourceFaceID)
					}
				}
			}
			if body.Torus != nil {
				for _, original := range sourceIDs {
					if original != body.Torus.SourceFaceID {
						t.Fatal("torus chart has wrong original source face", original, body.Torus.SourceFaceID)
					}
				}
			}
			if body.Cylinder != nil {
				if !reflect.DeepEqual(sourceIDs, body.Cylinder.NativeFaceSourceIDs[:]) {
					t.Fatal("cylinder source faces must retain native cap and wall order", sourceIDs)
				}
			}
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
	sort.Strings(sphereNodes)
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
	n.Name = "moved sphere's placement"
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
	var exported bytes.Buffer
	if e = changed.WriteSphericalHierarchySTEP(ctx, &exported, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9); e != nil {
		t.Fatal("edited analytic STEP", e)
	}
	xp, e := step.ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(exported.Bytes()), step.Limits{}, 1e-9)
	if e != nil || len(xp.Products) != 4 || len(xp.Instances) != 7 {
		t.Fatal("exported product structure", xp, e)
	}
	if !reflect.DeepEqual(proof.Assembly.Products, xp.Assembly.Products) || !reflect.DeepEqual(proof.Assembly.Source.Order, xp.Assembly.Source.Order) {
		t.Fatal("an exclusive occurrence edit changed its original entity IDs")
	}
	for id, p := range proof.Products {
		q := xp.Products[id]
		if !reflect.DeepEqual(p.Source, q.Source) {
			t.Fatal("editing a use changed the original local body graph", id)
		}
	}
	xdoc, e := ReadSphericalHierarchySTEP(ctx, bytes.NewReader(exported.Bytes()), STEPAssemblyOptions{}, 1e-9, 0)
	if e != nil {
		t.Fatal(e)
	}
	xstate, _ := xdoc.Snapshot()
	if !reflect.DeepEqual(xstate.Nodes, changed.Nodes) {
		t.Fatal("reimport did not preserve the edited name and pose")
	}
	want := map[geom.Affine]bool{changedX: true, changedY: true}
	for _, v := range xp.Instances {
		if xp.Products[v.ProductID].Sphere != nil {
			if !want[v.Transform] {
				t.Fatal("edited STEP changed the exact sphere transform", v)
			}
			delete(want, v.Transform)
		}
	}
	if len(want) != 0 {
		t.Fatal("edited STEP omitted a sphere", want)
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); bin != "" {
		path := filepath.Join(t.TempDir(), "edited-hierarchy.step")
		if e := os.WriteFile(path, exported.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		data, e := exec.Command(bin, "read", path).CombinedOutput()
		at := bytes.LastIndexByte(data, '{')
		var got struct {
			Volume                  float64
			Solids, Faces, EdgeUses int
			Bounds                  [6]float64
		}
		if e != nil || at < 0 || json.Unmarshal(data[at:], &got) != nil {
			t.Fatalf("OCCT readback: %v %s", e, data)
		}
		if got.Solids != 7 || got.Faces != 22 || got.EdgeUses != 88 || math.Abs(got.Volume-(24+12*math.Pi*math.Pi+(24+64./3)*math.Pi)) > 1e-7 {
			t.Fatal("OCCT topology and volume after moving the sphere", got)
		}
		for i, v := range [6]float64{-20, -20, -2, 30, 30, 15} {
			if math.Abs(got.Bounds[i]-v) > 2e-7 {
				t.Fatal("OCCT bounds after moving the sphere", got.Bounds)
			}
		}
	}
	stale := d.Begin().State
	for key, node := range stale.Nodes {
		if node.Asset != "" && stale.Labels["source:"+node.Asset].Attributes["step.material.family"].Text == "torus" {
			node.Transform[1][3] += 1
			stale.Nodes[key] = node
			break
		}
	}
	var reject bytes.Buffer
	if e = stale.WriteSphericalHierarchySTEP(ctx, &reject, bytes.NewReader(raw), STEPAssemblyOptions{}, 1e-9); !errors.Is(e, geom.ErrUnrepresentable) || reject.Len() != 0 {
		t.Fatal("a single expansion of one shared relationship cannot be exported", e, reject.Len())
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
