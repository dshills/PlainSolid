package iges

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialThreeTransverseCut(t *testing.T) {
	ctx := context.Background()
	opts := ClippedPolynomialCutIGESOptions{PoleTolerance: 1e-14}
	for _, tc := range []struct {
		layout string
		solids int
	}{{"partial", 4}, {"dependent", 5}} {
		t.Run(tc.layout, func(t *testing.T) {
			input, err := os.Open("../testdata/native_clipped_polynomial_three_cut_" + tc.layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			cut, err := trimmed.ReadClippedPolynomialThreeTransverseCutJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialNodeCutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parts, contacts := cut.Components(), cut.Contacts()
			if len(parts) != tc.solids || len(contacts) != 3 {
				t.Fatalf("source has %d solids and %d contact pairs", len(parts), len(contacts))
			}
			var output bytes.Buffer
			if err := WriteClippedPolynomialThreeTransverseCut(ctx, &output, cut, .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			raw := output.Bytes()
			verify := func(data []byte, unit float64) error {
				return VerifyClippedPolynomialThreeTransverseCutIGES(ctx, bytes.NewReader(data), cut, unit, Limits{}, opts)
			}
			if err := verify(raw, .001); err != nil {
				t.Fatal(err)
			}
			if err := verify(raw, 1); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("changed source units accepted: %v", err)
			}
			graph, err := readClippedPolynomialCutIGES(bytes.NewReader(raw), Limits{})
			if err != nil || graph.unit != .001 {
				t.Fatalf("IGES graph units %g: %v", graph.unit, err)
			}
			byType := make(map[int][]topologyRecord)
			for _, record := range graph.entities {
				byType[record.kind] = append(byType[record.kind], record)
			}
			faces, edges := 0, 0
			for _, part := range parts {
				faces += len(part.Faces())
				edges += len(part.Edges())
			}
			for kind, expected := range map[int]int{
				126: 3 * edges, 128: faces, 186: tc.solids, 402: 1,
				502: tc.solids, 504: tc.solids, 508: faces, 510: faces, 514: tc.solids,
			} {
				if len(byType[kind]) != expected {
					t.Fatalf("IGES type %d count %d, want %d", kind, len(byType[kind]), expected)
				}
			}
			group, err := topologyInts(byType[402][0], 2+tc.solids)
			if err != nil || group[1] != tc.solids {
				t.Fatalf("original material group: %v", err)
			}
			for i, solid := range byType[186] {
				fields, err := topologyInts(solid, 4)
				if err != nil || group[i+2] != solid.id || fields[1] != byType[514][i].id {
					t.Fatalf("component %d changed original group order or shell: %v", i, err)
				}
			}
			vertexLists, edgeLists := byType[502], byType[504]
			for i, contact := range contacts {
				if contact.Order != 1 || vertexLists[contact.Components[0]].id == vertexLists[contact.Components[1]].id {
					t.Fatalf("contact %d lost original independent vertex IDs", i)
				}
				for j, body := range contact.Components {
					v, index := vertexLists[body], contact.Vertices[j]
					p, err := igesPlanarCoordinates(v.values[2+3*index : 5+3*index])
					if err != nil {
						t.Fatal(err)
					}
					for axis := range p {
						if p[axis].Cmp(contact.Point[axis]) != 0 {
							t.Fatalf("contact %d occurrence %d changed original coordinate", i, j)
						}
					}
					changed := bytes.Clone(raw)
					replaceExactBezierIGESParameterDigit(t, changed, v.id, 2+3*index)
					if err := verify(changed, .001); !errors.Is(err, geom.ErrConvergence) {
						t.Fatalf("modified contact %d occurrence %d accepted: %v", i, j, err)
					}
				}
			}
			// Rewire a third-contact edge endpoint to the earlier component's
			// exact same point, changing only its topology identity.
			aliased := bytes.Clone(raw)
			aliasThirdCutIGESContact(t, aliased, parts, vertexLists, edgeLists, contacts[2])
			if _, err := readClippedPolynomialCutIGES(bytes.NewReader(aliased), Limits{}); err != nil {
				t.Fatal("aliased endpoint must retain parseable IGES structure", err)
			}
			if err := verify(aliased, .001); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("third contact welded by a shared vertex ID: %v", err)
			}
			var refused bytes.Buffer
			if err := writeClippedCutIGESBodies(ctx, &refused, parts, contacts[:2], .001, Limits{}, opts); !errors.Is(err, geom.ErrConvergence) || refused.Len() != 0 {
				t.Fatalf("missing third source contact published bytes: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"); oracle != "" {
				verifyOCCTClippedCutIGES(t, ctx, oracle, raw, cut, faces)
			}
		})
	}
}

func aliasThirdCutIGESContact(t *testing.T, raw []byte, parts []trimmed.Solid, vertices, edges []topologyRecord, contact trimmed.PolynomialMixedCutContact) {
	t.Helper()
	a, b := contact.Components[0], contact.Components[1]
	controlledEdges := parts[b].Edges()
	e := edges[b]
	refs, err := topologyInts(e, 2+5*len(controlledEdges))
	if err != nil {
		t.Fatal(err)
	}
	for i, edge := range controlledEdges {
		for j, vertex := range [...]int{edge.Start, edge.End} {
			if vertex != contact.Vertices[1] {
				continue
			}
			slot := 3 + 5*i + 2*j
			if refs[slot] != vertices[b].id || refs[slot+1] != vertex+1 {
				t.Fatal("the original third-contact edge does not own its local vertex")
			}
			replaceCutIGESInteger(t, raw, e.id, slot, vertices[a].id)
			replaceCutIGESInteger(t, raw, e.id, slot+1, contact.Vertices[0]+1)
			return
		}
	}
	t.Fatal("no incident edge at the third contact")
}

func replaceCutIGESInteger(t *testing.T, raw []byte, entity, parameter, value int) {
	t.Helper()
	var digits []*byte
	current := 0
	owner := strconv.Itoa(entity)
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(line) != 80 || line[72] != 'P' || strings.TrimSpace(string(line[64:72])) != owner {
			continue
		}
		for i, c := range line[:64] {
			switch {
			case c == ',' || c == ';':
				current++
			case current == parameter && c >= '0' && c <= '9':
				digits = append(digits, &line[i])
			}
		}
	}
	changed := fmt.Sprintf("%0*d", len(digits), value)
	if len(digits) == 0 || len(changed) != len(digits) {
		t.Fatalf("cannot replace IGES entity %d parameter %d in its original field width", entity, parameter)
	}
	for i, pos := range digits {
		*pos = changed[i]
	}
}
