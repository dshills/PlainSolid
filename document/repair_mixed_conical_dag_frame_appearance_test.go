package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

var documentCombinedRepairOptions = step.MixedConicalAP242DAGFrameAppearanceRepairOptions{
	Frame:    step.MixedConicalAP242FrameRepairOptions{OccurrenceID: 254, MaxAxisChord: 1e-12, MaxDisplacement: 1e-10},
	Geometry: step.MixedConicalAP242DAGRepairOptions{MaxDistance: 3e-12},
}

func TestRepairedMixedConicalAP242DAGFrameAppearanceDocument(t *testing.T) {
	raw, opened := openCombinedConicalRepairDocument(t)
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	m := opened.Repair
	base, _ := opened.Document.Snapshot()
	if m.OriginalSTEP() != string(raw) || len(base.Nodes) != 12 || len(base.Trimmed) != 2 || base.LengthUnit != .001 ||
		base.Nodes[roundedAppearanceSelected].Asset != base.Nodes[roundedAppearanceSibling].Asset {
		t.Fatal("document lost original source, shared geometry or expanded paths")
	}
	digest := func(raw string) Attribute {
		return conicalStrAttr(fmt.Sprintf("%x", sha256.Sum256([]byte(raw))))
	}
	for _, key := range []string{"source:step-repair", mixedConicalFrameRepairLabel} {
		a := base.Labels[key].Attributes
		if a["step.source.original-sha256"] != digest(m.OriginalSTEP()) ||
			a["step.source.corrected-sha256"] != digest(m.CorrectedModelSTEP()) {
			t.Fatal("repair ledger lost original or corrected hash", key, a)
		}
	}
	for path, pose := range map[string]string{
		roundedAppearanceSelected: "-15/19,-6/19,10/19,82;10/19,-15/19,6/19,48;6/19,10/19,15/19,9",
		roundedAppearanceSibling:  "-21/95,22/95,18/19,-598/5;-162/475,-441/475,14/95,-731/25;434/475,-138/475,27/95,1892/25",
	} {
		owner := base.Labels["source-node:"+path]
		if owner.Parent != mixedConicalFrameRepairLabel || owner.TargetID != path ||
			owner.Attributes["step.source.relationship"] != conicalIntAttr(251) ||
			owner.Attributes["step.source.world-pose"] != conicalStrAttr(pose) {
			t.Fatal("lost shared relationship or its original exact world frame", owner)
		}
	}
	for _, part := range m.PartRepairs() {
		key := "source:" + mixedConicalProductKey(part.DefinitionID)
		owner := base.Labels[key]
		if owner.Parent != "source:step-repair" || owner.Attributes["step.source.local-original-sha256"] != conicalStrAttr(part.LocalSourceSHA256) {
			t.Fatal("lost original local material owner", key, owner)
		}
	}
	checkConicalDocPaint(t, base, roundedAppearanceSelected, 0, step.RGB{B: 1})
	checkConicalDocPaint(t, base, roundedAppearanceSibling, 0, step.RGB{R: 1})
	checkConicalDocPaint(t, base, roundedAppearanceDirect, 3, step.RGB{R: 1, G: 1})
	var initialCarrier, receipt bytes.Buffer
	if err := base.WriteRepairedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, &initialCarrier, m, l); err != nil ||
		initialCarrier.String() != m.CorrectedModelSTEP() {
		t.Fatal("unchanged document changed the corrected carrier", err)
	}
	if err := step.WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, &receipt, m); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRepairedMixedConicalAP242DAGFrameAppearanceReceipt(ctx, bytes.NewReader(receipt.Bytes()), l, 0, 1)
	if err != nil || replayed.Repair.OriginalSTEP() != string(raw) {
		t.Fatal("source receipt did not restore the original before both repairs", err)
	}
	var initial, fromReceipt bytes.Buffer
	if err := opened.Document.Write(&initial); err != nil {
		t.Fatal(err)
	}
	if err := replayed.Document.Write(&fromReceipt); err != nil || !bytes.Equal(initial.Bytes(), fromReceipt.Bytes()) {
		t.Fatal("source replay did not restore the initial native document", err)
	}

	tx := opened.Document.Begin()
	if err := tx.State.SetMixedConicalAP242OccurrenceRotation(254, conicalDocRotationAboutZ(5, 12, 13)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{roundedAppearanceSelected, roundedAppearanceSibling} {
		node := tx.State.Nodes[path]
		node.Name = "Edited shared conical solid"
		node.Transform[0][3] += 5.125
		tx.State.Nodes[path] = node
	}
	cyan, magenta, orange := step.RGB{G: 1, B: 1}, step.RGB{R: 1, B: 1}, step.RGB{R: 1, G: .5}
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey("step-product-89"), cyan)
	repaintConicalDoc(t, tx.State, mixedConicalDAGStyleKey(roundedAppearanceSelected), magenta)
	repaintConicalDoc(t, tx.State, roundedAppearanceFace, orange)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var saved, repeated bytes.Buffer
	if err := opened.Document.Write(&saved); err != nil {
		t.Fatal(err)
	}
	native, err := Read(bytes.NewReader(saved.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := native.Write(&repeated); err != nil || !bytes.Equal(saved.Bytes(), repeated.Bytes()) {
		t.Fatal("edited native save/load changed bytes", err)
	}
	state, _ := native.Snapshot()
	for id, label := range base.Labels {
		if !identicalMixedConicalStyleLabels(label, state.Labels[id], strings.HasPrefix(id, "step-style:")) {
			t.Fatal("editing the document changed an original owner", id)
		}
	}
	checkConicalDocPaint(t, state, roundedAppearanceSelected, 0, magenta)
	checkConicalDocPaint(t, state, roundedAppearanceSibling, 0, cyan)
	checkConicalDocPaint(t, state, roundedAppearanceDirect, 3, orange)
	var corrected bytes.Buffer
	if err := state.WriteRepairedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, &corrected, replayed.Repair, l); err != nil {
		t.Fatal(err)
	}
	d, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(corrected.Bytes()), l, geom.SurfacePatchOptions{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := d.Snapshot()
	if !reflect.DeepEqual(state.Nodes, after.Nodes) || len(after.Trimmed) != 2 {
		t.Fatal("strict STEP reopen lost supported edits or shared geometry")
	}
	for path, world := range map[string]string{
		roundedAppearanceSelected: "5/13,-12/13,0,697/8;12/13,5/13,0,48;0,0,1,9",
		roundedAppearanceSibling:  "3/13,-36/65,4/5,-4661/40;4/5,-9/25,-12/25,-649/25;36/65,244/325,9/25,3661/50",
	} {
		owner := after.Labels["source-node:"+path].Attributes
		if owner["step.source.world-pose"] != conicalStrAttr(world) || owner["step.source.relationship"] != conicalIntAttr(251) {
			t.Fatal("strict reopen changed a shared original owner or exact edited world frame", path, owner)
		}
	}
	owner := after.Labels[mixedConicalDAGStyleKey(roundedAppearanceSelected)]
	if owner.TargetID != roundedAppearanceSelected || owner.Attributes["step.source.prefix-shuo"] != conicalIntAttr(500) ||
		owner.Attributes["step.source.terminal-shuo"] != conicalIntAttr(499) || owner.Attributes["step.source.styled-item"] != conicalIntAttr(530) ||
		owner.Attributes["step.source.style-placement"] != conicalIntAttr(68) ||
		after.Labels[roundedAppearanceFace].Attributes["step.source.face"] != conicalIntAttr(348) {
		t.Fatal("strict reopen changed original SHUO or cap paint owners", owner)
	}
	checkConicalDocPaint(t, after, roundedAppearanceSelected, 0, magenta)
	checkConicalDocPaint(t, after, roundedAppearanceSibling, 0, cyan)
	checkConicalDocPaint(t, after, roundedAppearanceDirect, 3, orange)
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_FRAME_APPEARANCE_REPAIR"); oracle != "" {
		checkConicalDocumentAppearanceOracle(t, oracle, corrected.Bytes())
		keys := []string{
			appearanceLeftCore + "/occ-248", roundedAppearanceSelected, appearanceLeftCore + "/occ-464",
			appearanceRoot + "/occ-487/occ-493/occ-248", roundedAppearanceSibling,
			appearanceRoot + "/occ-487/occ-493/occ-464", roundedAppearanceDirect,
		}
		checkFrameDocumentOracle(t, oracle, corrected.Bytes(), state, keys, "step-product-257")
	}
	var sameReceipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, &sameReceipt, m); err != nil ||
		!bytes.Equal(receipt.Bytes(), sameReceipt.Bytes()) || m.OriginalSTEP() != string(raw) {
		t.Fatal("editing and writeback changed the original repair receipt", err)
	}
}

func TestRepairedMixedConicalAP242DAGFrameAppearanceDocumentReject(t *testing.T) {
	raw, opened := openCombinedConicalRepairDocument(t)
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	if strict, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}, 0); err == nil || strict != nil {
		t.Fatal("strict document projection repaired the original source", err)
	}
	reject := func(m RepairedMixedConicalAP242DAGFrameAppearanceDocument, err error) {
		t.Helper()
		if err == nil || m.Document != nil || m.Repair.OriginalSTEP() != "" {
			t.Fatal("failed proof exposed a partial document", err)
		}
	}
	o := documentCombinedRepairOptions
	o.Frame.MaxDisplacement = 1e-11
	m, err := ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, bytes.NewReader(raw), l, o, 0)
	reject(m, err)
	o = documentCombinedRepairOptions
	o.Geometry.MaxDistance = 2.8e-12
	m, err = ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, bytes.NewReader(raw), l, o, 0)
	reject(m, err)
	short := l
	short.MaxNodes--
	m, err = ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, bytes.NewReader(raw), short, documentCombinedRepairOptions, 0)
	reject(m, err)

	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGFrameAppearanceJSON(ctx, &receipt, opened.Repair); err != nil {
		t.Fatal(err)
	}
	fake, replacement := []byte(`"PrefixSHUOID":500`), []byte(`"PrefixSHUOID":499`)
	if bytes.Count(receipt.Bytes(), fake) != 1 {
		t.Fatal("missing original receipt claim to forge")
	}
	forged := bytes.Replace(receipt.Bytes(), fake, replacement, 1)
	m, err = ReadRepairedMixedConicalAP242DAGFrameAppearanceReceipt(ctx, bytes.NewReader(forged), l, 0, 0)
	reject(m, err)
	m, err = ReadRepairedMixedConicalAP242DAGFrameAppearanceReceipt(ctx, bytes.NewReader(receipt.Bytes()), l, int64(receipt.Len()-1), 0)
	reject(m, err)
	for name, edit := range map[string]func(State){
		"wrong frame relationship": func(s State) {
			s.Labels["source-node:"+roundedAppearanceSibling].Attributes["step.source.relationship"] = conicalIntAttr(248)
		},
		"wrong raw source": func(s State) {
			s.Labels["source:step-repair"].Attributes["step.source.original-sha256"] = conicalStrAttr("wrong")
		},
		"wrong material proof": func(s State) {
			s.Labels["source:step-product-257"].Attributes["step.source.local-corrected-sha256"] = conicalStrAttr("wrong")
		},
		"wrong face owner": func(s State) {
			s.Labels[roundedAppearanceFace].Attributes["step.source.face"] = conicalIntAttr(97)
		},
		"extra path paint": func(s State) {
			s.Labels["new-paint"] = Label{Target: NodeLabel, TargetID: roundedAppearanceSibling, Attributes: colorAttributes(step.RGB{G: 1})}
		},
		"conflicting shared placement": func(s State) {
			node := s.Nodes[roundedAppearanceSibling]
			node.Transform[0][3]++
			s.Nodes[roundedAppearanceSibling] = node
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := opened.Document.Snapshot()
			edit(s)
			out := bytes.NewBufferString("unchanged")
			if err := s.WriteRepairedMixedConicalAP242DAGFrameAppearanceSTEP(ctx, out, opened.Repair, l); !errors.Is(err, geom.ErrUnrepresentable) || out.String() != "unchanged" {
				t.Fatal("invalid state modified writeback output", err)
			}
		})
	}
}

func openCombinedConicalRepairDocument(t *testing.T) ([]byte, RepairedMixedConicalAP242DAGFrameAppearanceDocument) {
	t.Helper()
	raw, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-ap242-rounded-frame-appearance.step")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadRoundedMixedConicalAP242DAGFrameAppearanceSTEP(context.Background(), bytes.NewReader(raw),
		mixedConicalAppearanceLimits(), documentCombinedRepairOptions, 1)
	if err != nil {
		t.Fatal(err)
	}
	return raw, m
}
