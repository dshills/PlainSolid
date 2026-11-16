package document

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRoundedMixedConicalAP242DAGDocument(t *testing.T) {
	source, err := os.ReadFile("../exchange/step/testdata/occt-dag-mixed-conical-curved-ap242-rounded.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	l := MixedConicalAP242DAGDocumentLimits{Assembly: step.MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}, MaxNodes: 12}
	geometry := geom.SurfacePatchOptions{}
	if d, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(source), l, geometry, 0); err == nil || d != nil {
		t.Fatal("strict reader repaired the rounded document source", err)
	}
	o := step.MixedConicalAP242DAGRepairOptions{MaxDistance: 3e-12}
	d, err := ReadRoundedMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(source), l, o, 1)
	if err != nil || d.Repair.OriginalSTEP() != string(source) {
		t.Fatal("rounded DAG document lost its original repair source", err)
	}
	s, _ := d.Document.Snapshot()
	proof := s.Labels["source:step-repair"]
	if len(s.Trimmed) != 2 || len(s.Nodes) != 12 || len(proof.Attributes["step.source.original-sha256"].Text) != 64 ||
		proof.Attributes["step.source.original-sha256"] == proof.Attributes["step.source.corrected-sha256"] {
		t.Fatal("editable DAG lost its two shared materials or source provenance")
	}
	var receipt bytes.Buffer
	if err := step.WriteRepairedMixedConicalAP242DAGJSON(ctx, &receipt, d.Repair); err != nil {
		t.Fatal(err)
	}
	fromReceipt, err := ReadRepairedMixedConicalAP242DAGReceipt(ctx, bytes.NewReader(receipt.Bytes()), l, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if err := d.Document.Write(&a); err != nil {
		t.Fatal(err)
	}
	if err := fromReceipt.Document.Write(&b); err != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("repaired DAG receipt did not reconstruct the original document bytes", err)
	}
	short := l
	short.Assembly.MaxLeafInstances--
	if r, err := ReadRepairedMixedConicalAP242DAGReceipt(ctx, bytes.NewReader(receipt.Bytes()), short, 0, 1); !errors.Is(err, geom.ErrConvergence) || r.Document != nil {
		t.Fatal("source receipt bypassed the document's caller-supplied expansion bound", err)
	}
	repairDAG := d.Repair.Assembly()
	path := repairDAG.Instances[0].Path
	if len(path) != 3 {
		t.Fatal("the original first occurrence no longer has three relationships", path)
	}
	parent := mixedConicalOccurrenceKey(mixedConicalProductKey(repairDAG.Assembly.Roots[0]), path[0])
	n := s.Nodes[parent]
	n.Transform[1][3] += 1.25
	s.Nodes[parent] = n
	var out bytes.Buffer
	if err := s.WriteMixedConicalAP242DAGSTEP(ctx, &out, strings.NewReader(d.Repair.CorrectedModelSTEP()), l, geometry); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMixedConicalAP242DAGSTEP(ctx, bytes.NewReader(out.Bytes()), l, geometry, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := strict.Snapshot()
	if !reflect.DeepEqual(got.Nodes, s.Nodes) || d.Repair.OriginalSTEP() != string(source) {
		t.Fatal("DAG writeback lost a current occurrence change or changed the original source")
	}
}
