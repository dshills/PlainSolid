package document

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestMixedConicalAP242DAGDocumentExactRotation(t *testing.T) {
	raw, d := openMixedConicalDAGAppearanceDocument(t)
	base, _ := d.Snapshot()
	tx := d.Begin()
	rotation := conicalDocRotationAboutZ(5, 12, 13)
	if err := tx.State.SetMixedConicalAP242OccurrenceRotation(251, rotation); err != nil {
		t.Fatal(err)
	}
	rotation[0][0].SetInt64(91)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var saved, repeated bytes.Buffer
	if err := d.Write(&saved); err != nil {
		t.Fatal(err)
	}
	replayed, err := Read(bytes.NewReader(saved.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := replayed.Write(&repeated); err != nil || !bytes.Equal(saved.Bytes(), repeated.Bytes()) {
		t.Fatal("exact rotation edits failed to survive native save/load byte-for-byte", err)
	}
	s, _ := replayed.Snapshot()
	edit := s.Labels[mixedConicalRotationLabelKey(251)]
	if edit.Attributes[mixedConicalRotationAttribute] != conicalStrAttr("5/13,-12/13,0,12/13,5/13,0,0,0,1") {
		t.Fatal("lost the saved exact rational rotation", edit)
	}
	for _, key := range []string{appearanceSelected, appearanceSibling} {
		node := s.Nodes[key]
		if !reflect.DeepEqual(s.Labels["source-node:"+key], base.Labels["source-node:"+key]) ||
			[3]float64(node.Transform[0][:3]) != [3]float64{5. / 13, -12. / 13, 0} ||
			node.Parent != base.Nodes[key].Parent || node.Asset != base.Nodes[key].Asset {
			t.Fatal("an explicit exact edit lost its shared source identity", key)
		}
	}
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	var out bytes.Buffer
	if err := s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, &out, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}); err != nil {
		t.Fatal(err)
	}
	again, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, &out, l, geom.SurfacePatchOptions{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := again.Snapshot()
	if !reflect.DeepEqual(s.Nodes, r.Nodes) || len(r.Trimmed) != 2 {
		t.Fatal("exact STEP rotation did not reopen all shared uses")
	}
	for key, want := range map[string]string{
		appearanceSelected: "5/13,-12/13,0,82;12/13,5/13,0,48;0,0,1,9",
		appearanceSibling:  "3/13,-36/65,4/5,-598/5;4/5,-9/25,-12/25,-731/25;36/65,244/325,9/25,1892/25",
	} {
		if got := r.Labels["source-node:"+key].Attributes["step.source.world-pose"].Text; got != want {
			t.Fatal("strict reopen lost exact world composition", key, got)
		}
	}
	checkConicalDocPaint(t, r, appearanceSelected, 0, step.RGB{B: 1})
	checkConicalDocPaint(t, r, appearanceSibling, 0, step.RGB{R: 1})
}

func TestMixedConicalAP242DAGDocumentExactRotationReject(t *testing.T) {
	raw, d := openMixedConicalDAGAppearanceDocument(t)
	ctx, l := context.Background(), mixedConicalAppearanceLimits()
	s, _ := d.Snapshot()
	reflection := conicalDocRotationAboutZ(5, 12, 13)
	reflection[2][2].SetInt64(-1)
	before, _ := d.Snapshot()
	if err := s.SetMixedConicalAP242OccurrenceRotation(251, reflection); !errors.Is(err, geom.ErrUnrepresentable) ||
		!reflect.DeepEqual(s.Nodes, before.Nodes) || !reflect.DeepEqual(s.Labels, before.Labels) {
		t.Fatal("a rejected reflection changed the native state", err)
	}
	editID := mixedConicalRotationLabelKey(251)
	for _, tc := range []struct {
		name   string
		mutate func(State)
	}{
		{"affine values without exact intent", func(s State) {
			delete(s.Labels, editID)
		}},
		{"false original relationship", func(s State) {
			s.Labels[editID].Attributes["step.source.relationship"] = conicalIntAttr(251)
		}},
		{"false exact rotation", func(s State) {
			s.Labels[editID].Attributes[mixedConicalRotationAttribute] = conicalStrAttr(mixedConicalRotationText(conicalDocRotationAboutZ(3, 4, 5)))
		}},
		{"unbounded exponent", func(s State) {
			s.Labels[editID].Attributes[mixedConicalRotationAttribute] = conicalStrAttr("1e999999999,0,0,0,1,0,0,0,1")
		}},
		{"conflicting shared use", func(s State) {
			n := s.Nodes[appearanceSibling]
			n.Transform[0][3] += 2
			s.Nodes[appearanceSibling] = n
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := d.Snapshot()
			if err := s.SetMixedConicalAP242OccurrenceRotation(251, conicalDocRotationAboutZ(5, 12, 13)); err != nil {
				t.Fatal(err)
			}
			tc.mutate(s)
			out := bytes.NewBufferString("untouched")
			if err := s.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, out, bytes.NewReader(raw), l, geom.SurfacePatchOptions{}); !errors.Is(err, geom.ErrUnrepresentable) ||
				out.String() != "untouched" {
				t.Fatal("a forged, unsupported or conflicting rotation published STEP", err)
			}
		})
	}
}

func conicalDocRotationAboutZ(x, y, d int64) [3][3]*big.Rat {
	return [3][3]*big.Rat{
		{big.NewRat(x, d), big.NewRat(-y, d), new(big.Rat)},
		{big.NewRat(y, d), big.NewRat(x, d), new(big.Rat)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
	}
}
