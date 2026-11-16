package document

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestNestedMixedConicalAP242DocumentWrite(t *testing.T) {
	raw, err := os.ReadFile("../exchange/step/testdata/occt-nested-mixed-conical-curved-ap242-exact.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	limits, geometry := STEPAssemblyOptions{MaxNodes: 9, MaxDepth: 3}, geom.SurfacePatchOptions{}
	doc, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), limits, geometry, 0)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := doc.Snapshot()
	const group1, group2 = "step-product-5/occ-58", "step-product-5/occ-422"
	const leaf1, leaf2 = group1 + "/occ-416", group2 + "/occ-416"
	n := s.Nodes[group2]
	n.Transform[1][3] -= 7.125
	s.Nodes[group2] = n
	a := s.Nodes[leaf1]
	a.Name = "common annulus"
	s.Nodes[leaf1] = a
	var denied bytes.Buffer
	if err := s.WriteNestedMixedConicalAP242STEP(ctx, &denied, bytes.NewReader(raw), limits, geometry); !errors.Is(err, geom.ErrUnrepresentable) ||
		!strings.Contains(err.Error(), "shared STEP occurrence 416") || denied.Len() != 0 {
		t.Fatal("conflicting expanded uses changed the STEP relationship", err)
	}
	b := s.Nodes[leaf2]
	b.Name = a.Name
	s.Nodes[leaf2] = b
	var out bytes.Buffer
	if err := s.WriteNestedMixedConicalAP242STEP(ctx, &out, bytes.NewReader(raw), limits, geometry); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(out.Bytes()), limits, geometry, 0)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := reloaded.Snapshot()
	if !reflect.DeepEqual(again.Nodes, s.Nodes) || len(again.Trimmed) != 2 || again.LengthUnit != s.LengthUnit {
		t.Fatal("reimport lost shared assets, names, or the selected occurrence translation")
	}
	oldLabel := s.Labels["source-node:"+group2].Attributes["step.source.local-pose"].Text
	newLabel := again.Labels["source-node:"+group2].Attributes["step.source.local-pose"].Text
	if oldLabel == newLabel || !strings.Contains(newLabel, "-217/8") {
		t.Fatal("STEP writeback lost the exact updated local translation", newLabel)
	}
}
