package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

const (
	roundedAppearanceSelected = appearanceLeftCore + "/occ-254"
	roundedAppearanceSibling  = appearanceRoot + "/occ-487/occ-493/occ-254"
	roundedAppearanceFace     = "step-style:step-product-257:face-3"
	roundedAppearanceDirect   = appearanceRoot + "/occ-498"
)

func TestRoundedMixedConicalAP242DAGAppearanceDocument(t *testing.T) {
	slowtest.Skip(t)
	raw, opened := openRoundedMixedConicalDAGAppearanceDocument(t)
	ctx, limits := context.Background(), mixedConicalAppearanceLimits()
	base, _ := opened.Document.Snapshot()
	corrected := opened.Repair.CorrectedModelSTEP()
	if opened.Repair.OriginalSTEP() != string(raw) || corrected == string(raw) ||
		len(base.Trimmed) != 2 || len(base.Nodes) != 12 || base.LengthUnit != .001 ||
		base.Nodes[roundedAppearanceSelected].Asset != base.Nodes[roundedAppearanceSibling].Asset {
		t.Fatal("repair lost its original source, shared local geometry or expanded occurrences")
	}
	checkRoundedAppearanceDocumentSourceLabels(t, base, opened.Repair)
	checkConicalDocPaint(t, base, roundedAppearanceSelected, 0, step.RGB{B: 1})
	checkConicalDocPaint(t, base, roundedAppearanceSibling, 0, step.RGB{R: 1})
	checkConicalDocPaint(t, base, roundedAppearanceDirect, 3, step.RGB{R: 1, G: 1})
	var unchanged bytes.Buffer
	if err := base.WriteRepairedMixedConicalAP242DAGAppearanceSTEP(ctx, &unchanged, opened.Repair, limits); err != nil || unchanged.String() != corrected {
		t.Fatal("unedited repaired document changed the strict styled carrier", err)
	}

	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx, &receipt, opened.Repair); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRepairedMixedConicalAP242DAGAppearanceReceipt(ctx, bytes.NewReader(receipt.Bytes()), limits, 0, 1)
	if err != nil || replayed.Repair.OriginalSTEP() != string(raw) || replayed.Repair.CorrectedModelSTEP() != corrected {
		t.Fatal("original repair receipt failed to reconstruct the styled document", err)
	}
	var originalProjection, replayedProjection bytes.Buffer
	if err := opened.Document.Write(&originalProjection); err != nil {
		t.Fatal(err)
	}
	if err := replayed.Document.Write(&replayedProjection); err != nil || !bytes.Equal(originalProjection.Bytes(), replayedProjection.Bytes()) {
		t.Fatal("replaying the source receipt changed the initial native snapshot", err)
	}

	tx := opened.Document.Begin()
	cyan, magenta, orange := step.RGB{G: 1, B: 1}, step.RGB{R: 1, B: 1}, step.RGB{R: 1, G: .5}
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey("step-product-89"), cyan)
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey(roundedAppearanceSelected), magenta)
	repaintConicalDoc(t, tx.State, roundedAppearanceFace, orange)
	for _, key := range []string{roundedAppearanceSelected, roundedAppearanceSibling} {
		node := tx.State.Nodes[key]
		node.Name = "Repositioned repaired conical solid"
		node.Transform[0][3] += 5.125
		tx.State.Nodes[key] = node
	}
	key := appearanceRoot + "/occ-54"
	node := tx.State.Nodes[key]
	node.Transform[1][3] += 8
	tx.State.Nodes[key] = node
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var saved, savedAgain bytes.Buffer
	if err := opened.Document.Write(&saved); err != nil {
		t.Fatal(err)
	}
	native, err := Read(bytes.NewReader(saved.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := native.Write(&savedAgain); err != nil || !bytes.Equal(saved.Bytes(), savedAgain.Bytes()) {
		t.Fatal("edited native document did not reopen byte-identically", err)
	}
	state, _ := native.Snapshot()
	checkRoundedAppearanceDocumentSourceLabels(t, state, opened.Repair)
	checkConicalDocPaint(t, state, roundedAppearanceSelected, 0, magenta)
	checkConicalDocPaint(t, state, roundedAppearanceSibling, 0, cyan)
	checkConicalDocPaint(t, state, roundedAppearanceDirect, 3, orange)
	if !reflect.DeepEqual(state.Labels["source-node:"+roundedAppearanceSelected], base.Labels["source-node:"+roundedAppearanceSelected]) {
		t.Fatal("editing placement changed the original exact source node")
	}
	var output bytes.Buffer
	if err := state.WriteRepairedMixedConicalAP242DAGAppearanceSTEP(ctx, &output, replayed.Repair, limits); err != nil {
		t.Fatal(err)
	}
	post, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(output.Bytes()), limits, geom.SurfacePatchOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := post.Snapshot()
	owner := got.Labels[mixedConicalDAGStyleKey(roundedAppearanceSelected)]
	if !reflect.DeepEqual(got.Nodes, state.Nodes) || len(got.Trimmed) != 2 ||
		owner.TargetID != roundedAppearanceSelected || owner.Attributes["step.source.nauo"] != conicalIntAttr(254) ||
		owner.Attributes["step.source.styled-item"] != conicalIntAttr(513) || owner.Attributes["step.source.style-placement"] != conicalIntAttr(68) ||
		got.Labels[roundedAppearanceFace].Attributes["step.source.face"] != conicalIntAttr(348) {
		t.Fatal("strict corrected STEP reopen changed the edited nodes or paint owners")
	}
	checkConicalDocPaint(t, got, roundedAppearanceSelected, 0, magenta)
	checkConicalDocPaint(t, got, roundedAppearanceSibling, 0, cyan)
	checkConicalDocPaint(t, got, roundedAppearanceDirect, 3, orange)
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE"); oracle != "" {
		checkConicalDocumentAppearanceOracle(t, oracle, output.Bytes())
	}
	var sameReceipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx, &sameReceipt, opened.Repair); err != nil ||
		!bytes.Equal(receipt.Bytes(), sameReceipt.Bytes()) || opened.Repair.OriginalSTEP() != string(raw) {
		t.Fatal("native edits or STEP writeback changed the original receipt", err)
	}
}

func TestRoundedMixedConicalAP242DAGAppearanceDocumentReject(t *testing.T) {
	slowtest.Skip(t)
	raw, opened := openRoundedMixedConicalDAGAppearanceDocument(t)
	ctx, limits := context.Background(), mixedConicalAppearanceLimits()
	tight := step.MixedConicalAP242DAGRepairOptions{MaxDistance: 2.8e-12}
	m, err := ReadRoundedMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(raw), limits, tight, 0)
	if err == nil || !strings.Contains(err.Error(), "finite cone angle displacement") ||
		m.Document != nil || m.Repair.OriginalSTEP() != "" {
		t.Fatal("tight displacement budget published a partially repaired document", err)
	}
	short := limits
	short.MaxNodes--
	m, err = ReadRoundedMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(raw), short, step.MixedConicalAP242DAGRepairOptions{MaxDistance: 3e-12}, 0)
	if !errors.Is(err, geom.ErrConvergence) || m.Document != nil || m.Repair.OriginalSTEP() != "" {
		t.Fatal("short expanded node limit published a partially repaired document", err)
	}
	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGAppearanceJSON(ctx, &receipt, opened.Repair); err != nil {
		t.Fatal(err)
	}
	m, err = ReadRepairedMixedConicalAP242DAGAppearanceReceipt(ctx, bytes.NewReader(receipt.Bytes()), limits, int64(receipt.Len()-1), 0)
	if !errors.Is(err, geom.ErrConvergence) || m.Document != nil || m.Repair.OriginalSTEP() != "" {
		t.Fatal("short receipt decoding limit published a partial document", err)
	}
	for name, edit := range map[string]func(State){
		"changed original digest": func(s State) {
			s.Labels["source:step-repair"].Attributes["step.source.original-sha256"] = conicalStrAttr("unproved")
		},
		"changed local repair ledger": func(s State) {
			s.Labels["source:step-product-257"].Attributes["step.source.local-original-sha256"] = conicalStrAttr("unproved")
		},
		"conflicting path paints": func(s State) {
			s.Labels["duplicate:paint"] = Label{Target: NodeLabel, TargetID: roundedAppearanceSelected, Attributes: colorAttributes(step.RGB{R: 1})}
		},
		"changed path owner": func(s State) {
			k := mixedConicalDAGStyleKey(roundedAppearanceSelected)
			label := s.Labels[k]
			label.TargetID = roundedAppearanceSibling
			s.Labels[k] = label
		},
		"conflicting shared pose": func(s State) {
			node := s.Nodes[roundedAppearanceSelected]
			node.Transform[0][3]++
			s.Nodes[roundedAppearanceSelected] = node
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := opened.Document.Snapshot()
			edit(s)
			out := bytes.NewBufferString("unchanged")
			if err := s.WriteRepairedMixedConicalAP242DAGAppearanceSTEP(ctx, out, opened.Repair, limits); !errors.Is(err, geom.ErrUnrepresentable) || out.String() != "unchanged" {
				t.Fatal("invalid native repair provenance or style conflict changed output", err)
			}
		})
	}
}

func openRoundedMixedConicalDAGAppearanceDocument(t *testing.T) ([]byte, RepairedMixedConicalAP242DAGAppearanceDocument) {
	t.Helper()
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-ap242-rounded-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	repaired, err := ReadRoundedMixedConicalAP242DAGAppearanceSTEP(context.Background(), bytes.NewReader(raw),
		mixedConicalAppearanceLimits(), step.MixedConicalAP242DAGRepairOptions{MaxDistance: 3e-12}, 1)
	if err != nil {
		t.Fatal(err)
	}
	return raw, repaired
}

func checkRoundedAppearanceDocumentSourceLabels(t *testing.T, s State, m step.RepairedMixedConicalAP242DAGAppearance) {
	t.Helper()
	digest := func(text string) Attribute {
		return conicalStrAttr(fmt.Sprintf("%x", sha256.Sum256([]byte(text))))
	}
	repair := s.Labels["source:step-repair"].Attributes
	if repair["step.source.original-sha256"] != digest(m.OriginalSTEP()) ||
		repair["step.source.corrected-sha256"] != digest(m.CorrectedModelSTEP()) ||
		s.Labels[mixedConicalAppearanceSourceLabel].Attributes["step.source.original-sha256"] != repair["step.source.corrected-sha256"] {
		t.Fatal("document did not retain separate exact source and corrected carrier fingerprints")
	}
	for _, p := range m.PartRepairs() {
		id := "source:" + mixedConicalProductKey(p.DefinitionID)
		label := s.Labels[id]
		if label.Parent != "source:step-repair" || label.Attributes["step.source.local-original-sha256"] != conicalStrAttr(p.LocalSourceSHA256) ||
			label.Attributes["step.source.local-corrected-sha256"] != conicalStrAttr(p.LocalCorrectedSHA256) ||
			label.Attributes["step.source.native-sha256"] != conicalStrAttr(p.NativeSHA256) {
			t.Fatal("document lost one original local repair ledger", id)
		}
	}
	override := s.Labels[mixedConicalDAGStyleKey(roundedAppearanceSelected)]
	if override.TargetID != roundedAppearanceSelected ||
		override.Attributes["step.source.prefix-shuo"] != conicalIntAttr(500) ||
		override.Attributes["step.source.terminal-shuo"] != conicalIntAttr(499) ||
		override.Attributes["step.source.outer-nauo"] != conicalIntAttr(54) ||
		override.Attributes["step.source.middle-nauo"] != conicalIntAttr(85) ||
		override.Attributes["step.source.nauo"] != conicalIntAttr(254) ||
		override.Attributes["step.source.styled-item"] != conicalIntAttr(513) ||
		override.Attributes["step.source.srgb.blue"] != (Attribute{Kind: "number", Number: 1}) {
		t.Fatal("document lost its original typed contextual owner or paint", override)
	}
}
