package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestRepairMixedConicalAP242DAG(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := mixedConicalAP242DAGFixture(t, "rounded")
	limits := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	m, err := RepairMixedConicalAP242DAG(ctx, bytes.NewReader(raw), limits, MixedConicalAP242DAGRepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	a := m.Assembly()
	if m.OriginalSTEP() != string(raw) || len(a.Parts) != 2 || len(a.Occurrences) != 8 || len(a.Instances) != 7 {
		t.Fatal("lost original bytes or DAG ownership")
	}
	wantPaths := [][]int64{{54, 85, 248}, {54, 85, 254}, {54, 85, 464}, {487, 493, 248}, {487, 493, 254}, {487, 493, 464}, {498}}
	exact, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(mixedConicalAP242DAGFixture(t, "exact")), limits, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, u := range a.Instances {
		if !slices.Equal(u.Path, wantPaths[i]) || u.ProductID != [7]int64{89, 89, 257, 89, 89, 257, 257}[i] ||
			!reflect.DeepEqual(u.Matrix(), exact.Instances[i].Matrix()) {
			t.Fatalf("changed exact world or original path %v", u.Path)
		}
	}
	first, second := m.PartRepairs()[0], m.PartRepairs()[1]
	if first.DefinitionID != 89 || first.RollingRim == nil || len(first.GeneratedEntityIDs) != 2 ||
		second.DefinitionID != 257 || len(second.ChamferWalls) != 6 || len(second.GeneratedEntityIDs) != 6 {
		t.Fatal("incorrect local repair ownership")
	}
	local := map[int64]trimmed.Solid{89: a.Parts[89].Rounded.Solid, 257: a.Parts[257].Chamfered.Solid}
	for i, part := range []trimmed.Solid{exact.Parts[89].Rounded.Solid, exact.Parts[254].Chamfered.Solid} {
		native, err := prismNativeBytes(part)
		if err != nil || rationalPosedRepairDigest(native) != m.PartRepairs()[i].NativeSHA256 {
			t.Fatalf("changed independent local proof %d: %v", i, err)
		}
	}
	original, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	fresh := a.Assembly.Source
	owned := map[int64]bool{}
	added := map[int64]bool{}
	for _, p := range m.PartRepairs() {
		closure, err := nauoClosure(original, []int64{p.BodyID}, 16)
		if err != nil {
			t.Fatal(err)
		}
		for id := range closure {
			owned[id] = true
		}
		for _, id := range p.GeneratedEntityIDs {
			if id <= 498 || added[id] || fresh.Entities[id].ID != id {
				t.Fatalf("invalid new support ID #%d", id)
			}
			added[id] = true
		}
	}
	for _, id := range original.Order {
		if !owned[id] && !reflect.DeepEqual(original.Entities[id], fresh.Entities[id]) {
			t.Fatalf("changed assembly relationship or shared definition record #%d", id)
		}
	}
	visit := m.Assembly()
	visit.Instances[0].Path[0] = -1
	if m.Assembly().Instances[0].Path[0] != 54 {
		t.Fatal("caller-owned path aliases the receipt proof")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		for _, src := range [][]byte{raw, []byte(m.CorrectedModelSTEP())} {
			checkMixedConicalAP242DAGOracle(t, oracle, src, a.Instances, local, 257)
		}
	}
}

func TestRepairMixedConicalAP242DAGReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := mixedConicalAP242DAGFixture(t, "rounded")
	check := func(input []byte, l MixedConicalAP242DAGLimits, budget float64, target error, detail string) {
		t.Helper()
		m, err := RepairMixedConicalAP242DAG(ctx, bytes.NewReader(input), l, MixedConicalAP242DAGRepairOptions{budget})
		if err == nil || m.OriginalSTEP() != "" || m.CorrectedModelSTEP() != "" || m.Assembly().Assembly.Source != nil ||
			target != nil && !errors.Is(err, target) || !strings.Contains(err.Error(), detail) {
			t.Fatalf("failed to reject DAG without exposing material: %v", err)
		}
	}
	check(raw, MixedConicalAP242DAGLimits{}, 2.8e-12, nil, "part #89")
	check(raw, MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 2}, 3e-12, geom.ErrConvergence, "")
	check(raw, MixedConicalAP242DAGLimits{MaxLeafInstances: 6}, 3e-12, geom.ErrConvergence, "")
	cycle := bytes.Replace(raw, []byte("#82 = ( REPRESENTATION_RELATIONSHIP('','',#63,#40)"), []byte("#82 = ( REPRESENTATION_RELATIONSHIP('','',#10,#40)"), 1)
	cycle = bytes.Replace(cycle, []byte("'4','=>[0:1:1:4]','',#35,#58,$"), []byte("'4','=>[0:1:1:4]','',#35,#5,$"), 1)
	check(cycle, MixedConicalAP242DAGLimits{}, 3e-12, nil, "authenticate rounded AP242 DAG")
	bad := bytes.Replace(raw, []byte("#294,5.,0.244978663127"), []byte("#294,5.,0.244978663227"), 1)
	check(bad, MixedConicalAP242DAGLimits{}, 3e-12, nil, "part #257")
}

func TestRepairedMixedConicalAP242DAGReceipt(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := mixedConicalAP242DAGFixture(t, "rounded")
	m, err := RepairMixedConicalAP242DAG(ctx, bytes.NewReader(raw), MixedConicalAP242DAGLimits{}, MixedConicalAP242DAGRepairOptions{3e-12})
	if err != nil {
		t.Fatal(err)
	}
	var first, repeated bytes.Buffer
	if err := WriteRepairedMixedConicalAP242DAGJSON(ctx, &first, m); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRepairedMixedConicalAP242DAGJSON(ctx, bytes.NewReader(first.Bytes()), MixedConicalAP242DAGRepairJSONLimits{})
	if err != nil || got.OriginalSTEP() != string(raw) || got.CorrectedModelSTEP() != m.CorrectedModelSTEP() ||
		!reflect.DeepEqual(got.PartRepairs(), m.PartRepairs()) {
		t.Fatalf("source receipt failed to replay: %v", err)
	}
	if err := WriteRepairedMixedConicalAP242DAGJSON(ctx, &repeated, got); err != nil || !bytes.Equal(first.Bytes(), repeated.Bytes()) {
		t.Fatalf("receipt did not replay byte-for-byte: %v", err)
	}
	limits := MixedConicalAP242DAGRepairJSONLimits{Source: MixedConicalAP242DAGLimits{MaxLeafInstances: 6}}
	if p, err := ReadRepairedMixedConicalAP242DAGJSON(ctx, bytes.NewReader(first.Bytes()), limits); !errors.Is(err, geom.ErrConvergence) || p.OriginalSTEP() != "" {
		t.Fatalf("receipt bypassed its occurrence budget: %v", err)
	}
	q, err := readRationalPosedPCurveRepairEnvelope(first.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	var claims mixedConicalAP242DAGRepairClaim
	if err := json.Unmarshal(q.Claims, &claims); err != nil {
		t.Fatal(err)
	}
	claims.Instances[0].Path[1] = 493
	q.Claims, err = json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if altered, err := ReadRepairedMixedConicalAP242DAGJSON(ctx, bytes.NewReader(changed), MixedConicalAP242DAGRepairJSONLimits{}); err == nil || altered.OriginalSTEP() != "" {
		t.Fatalf("receipt with altered occurrence relationship returned material: %v", err)
	}
}
