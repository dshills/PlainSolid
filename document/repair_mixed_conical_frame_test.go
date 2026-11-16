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
	"slices"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

var documentFrameRepairOptions = step.MixedConicalAP242FrameRepairOptions{OccurrenceID: 251, MaxAxisChord: 1e-12, MaxDisplacement: 1e-10}

const repairedFrameLeft = "step-product-5/occ-54/occ-85/occ-251"
const repairedFrameRight = "step-product-5/occ-466/occ-472/occ-251"

func TestRepairedMixedConicalAP242FrameDocument(t *testing.T) {
	raw, opened := openRepairedMixedConicalFrameDocument(t)
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	if strict, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}, 0); err == nil || strict != nil {
		t.Fatal("strict document reader silently repaired the original rounded frame", err)
	}
	base, _ := opened.Document.Snapshot()
	if opened.Repair.OriginalSTEP() != string(raw) || len(base.Nodes) != 12 || len(base.Trimmed) != 2 || base.LengthUnit != .001 ||
		base.Nodes[repairedFrameLeft].Asset != base.Nodes[repairedFrameRight].Asset {
		t.Fatal("lost the rounded source, shared material or occurrence expansion")
	}
	ledger := base.Labels[mixedConicalFrameRepairLabel]
	if ledger.Target != DocumentLabel || ledger.Attributes["step.source.relationship"] != conicalIntAttr(248) ||
		ledger.Attributes["step.source.nauo"] != conicalIntAttr(251) || ledger.Attributes["step.source.to-placement"] != conicalIntAttr(68) ||
		ledger.Attributes["step.source.x-direction"] != conicalIntAttr(71) || ledger.Attributes["step.source.z-direction"] != conicalIntAttr(70) ||
		ledger.Attributes["step.repair.displacement-upper"] != conicalStrAttr(opened.Repair.Evidence().DisplacementUpper) {
		t.Fatal("lost the original exact frame repair ledger", ledger)
	}
	for path, want := range map[string]string{
		repairedFrameLeft:  "78/95,54/95,1/19,649/5;-21/95,22/95,18/19,82/5;10/19,-15/19,6/19,30",
		repairedFrameRight: "-21/95,22/95,18/19,-598/5;-162/475,-441/475,14/95,-731/25;434/475,-138/475,27/95,1892/25",
	} {
		owner := base.Labels["source-node:"+path]
		if owner.Parent != mixedConicalFrameRepairLabel || owner.Target != NodeLabel || owner.TargetID != path ||
			owner.Attributes["step.source.world-pose"] != conicalStrAttr(want) {
			t.Fatal("the corrected frame did not retain an original exact expanded path", path, owner)
		}
	}
	var unchanged bytes.Buffer
	if err := base.WriteRepairedMixedConicalAP242FrameSTEP(ctx, &unchanged, opened.Repair, l); err != nil ||
		unchanged.String() != opened.Repair.CorrectedModelSTEP() {
		t.Fatal("unedited repaired document changed the strict carrier bytes", err)
	}
	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242FrameJSON(ctx, &receipt, opened.Repair); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRepairedMixedConicalAP242FrameReceipt(ctx, bytes.NewReader(receipt.Bytes()), l, 0, 1)
	if err != nil || replayed.Repair.OriginalSTEP() != string(raw) || replayed.Repair.CorrectedModelSTEP() != unchanged.String() {
		t.Fatal("receipt replay changed the exact source binding", err)
	}
	var initial, initialReplay bytes.Buffer
	if err := opened.Document.Write(&initial); err != nil {
		t.Fatal(err)
	}
	if err := replayed.Document.Write(&initialReplay); err != nil || !bytes.Equal(initial.Bytes(), initialReplay.Bytes()) {
		t.Fatal("receipt replay changed the initial native projection bytes", err)
	}

	tx := opened.Document.Begin()
	if err := tx.State.SetMixedConicalAP242OccurrenceRotation(251, conicalDocRotationAboutZ(5, 12, 13)); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{repairedFrameLeft, repairedFrameRight} {
		n := tx.State.Nodes[key]
		n.Name = "Moved original rounded-frame relationship"
		n.Transform[0][3] += 5.125
		tx.State.Nodes[key] = n
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var snapshot, repeated bytes.Buffer
	if err := opened.Document.Write(&snapshot); err != nil {
		t.Fatal(err)
	}
	native, err := Read(bytes.NewReader(snapshot.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := native.Write(&repeated); err != nil || !bytes.Equal(snapshot.Bytes(), repeated.Bytes()) {
		t.Fatal("native save/load changed the edited repaired document", err)
	}
	state, _ := native.Snapshot()
	for key, original := range base.Labels {
		if !reflect.DeepEqual(state.Labels[key], original) {
			t.Fatal("native edit changed an original source witness", key)
		}
	}
	var out bytes.Buffer
	if err := state.WriteRepairedMixedConicalAP242FrameSTEP(ctx, &out, replayed.Repair, l); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(out.Bytes()), l, geom.SurfacePatchOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := strict.Snapshot()
	if !reflect.DeepEqual(state.Nodes, after.Nodes) || len(after.Trimmed) != 2 {
		t.Fatal("strict reopen lost a supported edit or shared definition")
	}
	for path, want := range map[string]string{
		repairedFrameLeft:  "-4/13,48/65,3/5,1257/10;3/13,-36/65,4/5,779/40;12/13,5/13,0,30",
		repairedFrameRight: "3/13,-36/65,4/5,-4661/40;4/5,-9/25,-12/25,-649/25;36/65,244/325,9/25,3661/50",
	} {
		source := after.Labels["source-node:"+path].Attributes
		if source["step.source.world-pose"] != conicalStrAttr(want) || source["step.source.relationship"] != conicalIntAttr(248) {
			t.Fatal("strict reopen changed a shared source owner or exact edited pose", path, source)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_FRAME_REPAIR"); oracle != "" {
		checkRepairedFrameDocumentOracle(t, oracle, out.Bytes(), state)
	}
	var sameReceipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242FrameJSON(ctx, &sameReceipt, opened.Repair); err != nil ||
		!bytes.Equal(sameReceipt.Bytes(), receipt.Bytes()) || opened.Repair.OriginalSTEP() != string(raw) {
		t.Fatal("editing and exporting replaced the original frame repair receipt", err)
	}
}

func TestRepairedMixedConicalAP242FrameDocumentReject(t *testing.T) {
	raw, opened := openRepairedMixedConicalFrameDocument(t)
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	rejectRead := func(out RepairedMixedConicalAP242FrameDocument, err, cause error) {
		t.Helper()
		if !errors.Is(err, cause) || out.Document != nil || out.Repair.OriginalSTEP() != "" {
			t.Fatal("refused input exposed a partial document or repair", err)
		}
	}
	tight := documentFrameRepairOptions
	tight.MaxAxisChord = 3e-13
	out, err := ReadRoundedMixedConicalAP242FrameSTEP(ctx, bytes.NewReader(raw), l, tight, 0)
	rejectRead(out, err, geom.ErrConvergence)
	tight = documentFrameRepairOptions
	tight.MaxDisplacement = 1e-11
	out, err = ReadRoundedMixedConicalAP242FrameSTEP(ctx, bytes.NewReader(raw), l, tight, 0)
	rejectRead(out, err, geom.ErrConvergence)
	short := l
	short.MaxNodes--
	out, err = ReadRoundedMixedConicalAP242FrameSTEP(ctx, bytes.NewReader(raw), short, documentFrameRepairOptions, 0)
	rejectRead(out, err, geom.ErrConvergence)

	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242FrameJSON(ctx, &receipt, opened.Repair); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(receipt.Bytes(), []byte(`"DirectionIDs":[71,70]`)) != 1 {
		t.Fatal("missing exact receipt mutation anchor")
	}
	forged := bytes.Replace(receipt.Bytes(), []byte(`"DirectionIDs":[71,70]`), []byte(`"DirectionIDs":[70,71]`), 1)
	out, err = ReadRepairedMixedConicalAP242FrameReceipt(ctx, bytes.NewReader(forged), l, 0, 0)
	rejectRead(out, err, geom.ErrInvalid)
	out, err = ReadRepairedMixedConicalAP242FrameReceipt(ctx, bytes.NewReader(receipt.Bytes()), l, int64(receipt.Len()-1), 0)
	rejectRead(out, err, geom.ErrConvergence)

	// Another valid proof can normalize to the same corrected carrier. It
	// still belongs to a different byte sequence from this native snapshot.
	scoped := append(slices.Clone(raw), '\n')
	other, err := step.RepairMixedConicalAP242Frame(ctx, bytes.NewReader(scoped), l.Assembly, documentFrameRepairOptions)
	if err != nil || other.CorrectedModelSTEP() != opened.Repair.CorrectedModelSTEP() || other.OriginalSTEP() == opened.Repair.OriginalSTEP() {
		t.Fatal("replacement proof control changed the wrong source property", err)
	}
	s, _ := opened.Document.Snapshot()
	before := bytes.NewBufferString("untouched")
	if err := s.WriteRepairedMixedConicalAP242FrameSTEP(ctx, before, other, l); !errors.Is(err, geom.ErrUnrepresentable) || before.String() != "untouched" {
		t.Fatal("rebound the document to a different valid original source", err)
	}
	short = l
	short.Assembly.MaxLeafInstances--
	if err := s.WriteRepairedMixedConicalAP242FrameSTEP(ctx, before, opened.Repair, short); !errors.Is(err, geom.ErrConvergence) || before.String() != "untouched" {
		t.Fatal("writeback bypassed the strict occurrence budget", err)
	}
	for _, tc := range []struct {
		name string
		edit func(State)
	}{
		{"changed original relationship", func(s State) {
			s.Labels["source-node:"+repairedFrameRight].Attributes["step.source.relationship"] = conicalIntAttr(242)
		}},
		{"changed original path proof", func(s State) {
			s.Labels["source-node:"+repairedFrameRight].Attributes["step.source.path"] = conicalStrAttr(repairedFrameLeft)
		}},
		{"inconsistent shared translation", func(s State) {
			n := s.Nodes[repairedFrameRight]
			n.Transform[1][3]++
			s.Nodes[repairedFrameRight] = n
		}},
		{"paint without an original owner", func(s State) {
			s.Labels["paint-request"] = Label{Target: NodeLabel, TargetID: repairedFrameLeft, Attributes: colorAttributes(step.RGB{B: 1})}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := opened.Document.Snapshot()
			tc.edit(s)
			b := bytes.NewBufferString("untouched")
			if err := s.WriteRepairedMixedConicalAP242FrameSTEP(ctx, b, opened.Repair, l); !errors.Is(err, geom.ErrUnrepresentable) || b.String() != "untouched" {
				t.Fatal("invalid source ownership or unsupported edit changed STEP output", err)
			}
		})
	}
}

func openRepairedMixedConicalFrameDocument(t *testing.T) ([]byte, RepairedMixedConicalAP242FrameDocument) {
	t.Helper()
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-ap242-rounded-spatial-frame.step")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadRoundedMixedConicalAP242FrameSTEP(context.Background(), bytes.NewReader(raw),
		mixedConicalAppearanceLimits(), documentFrameRepairOptions, 1)
	if err != nil {
		t.Fatal(err)
	}
	return raw, m
}

func checkRepairedFrameDocumentOracle(t *testing.T, oracle string, raw []byte, s State) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "edited-frame.step")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(oracle, "read-dag", path).CombinedOutput()
	var hit struct {
		SharedAssemblies, MaterialDefinitions int
		Occurrences                           []struct {
			Path         []int
			Pose         [12]float64
			Valid        bool
			Volume, Area float64
			Topology     [4]int
		}
	}
	if err != nil || json.Unmarshal(out, &hit) != nil || hit.SharedAssemblies != 1 || hit.MaterialDefinitions != 2 || len(hit.Occurrences) != 7 {
		t.Fatalf("pinned XCAF failed to reopen the edited strict graph: %v\n%s", err, out)
	}
	keys := []string{
		"step-product-5/occ-54/occ-85/occ-245", repairedFrameLeft, "step-product-5/occ-54/occ-85/occ-443",
		"step-product-5/occ-466/occ-472/occ-245", repairedFrameRight, "step-product-5/occ-466/occ-472/occ-443",
		"step-product-5/occ-477",
	}
	paths := [][]int{{1, 1, 1}, {1, 1, 2}, {1, 1, 3}, {2, 1, 1}, {2, 1, 2}, {2, 1, 3}, {3}}
	for i, got := range hit.Occurrences {
		want, err := s.WorldTransform(keys[i])
		if err != nil || !slices.Equal(got.Path, paths[i]) || !got.Valid {
			t.Fatal("pinned XCAF changed an original expanded path", got, err)
		}
		topology, volume, area := [4]int{1, 6, 10, 6}, 2282.40140408289, 1182.6440976399515
		if s.Nodes[keys[i]].Asset == "step-product-254" {
			topology, volume, area = [4]int{1, 8, 14, 8}, 726.988268498145, 593.897326906695
		}
		if got.Topology != topology || math.Abs(got.Volume-volume) > 1e-8*volume || math.Abs(got.Area-area) > 1e-8*area {
			t.Fatal("pinned XCAF changed an original local part", got)
		}
		for row := range want {
			for col, value := range want[row] {
				if math.Abs(got.Pose[4*row+col]-value) > 1e-12 {
					t.Fatal("pinned XCAF changed the edited world frame", got.Path, got.Pose)
				}
			}
		}
	}
	t.Logf("pinned edited occurrence geometry: %s", out)
}
