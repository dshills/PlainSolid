package iges

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialThreeTransverseCommon(t *testing.T) {
	ctx := context.Background()
	opts := ClippedPolynomialCommonIGESOptions{PoleTolerance: 1e-14}
	for _, tc := range []struct {
		layout       string
		volume, area float64
	}{
		{"partial", 26955626929285653.0 / 2251799813685248.0, 32.985201663206595},
		{"dependent", 6580961624997.0 / 549755813888.0, 31.655074821630230},
	} {
		t.Run(tc.layout, func(t *testing.T) {
			source, err := os.ReadFile("../testdata/native_clipped_polynomial_three_common_" + tc.layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			common, err := trimmed.ReadClippedPolynomialThreeTransverseCommonJSON(ctx, bytes.NewReader(source), trimmed.JSONLimits{}, trimmed.PolynomialCubicCrossingTangencyEnvelopeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := WriteClippedPolynomialThreeTransverseCommon(ctx, &output, common, .001, Limits{}, opts); err != nil {
				t.Fatal(err)
			}
			verify := func(data []byte) error {
				return VerifyClippedPolynomialThreeTransverseCommonIGES(ctx, bytes.NewReader(data), common, .001, Limits{}, opts)
			}
			raw := output.Bytes()
			if err := verify(raw); err != nil {
				t.Fatal(err)
			}
			graph, err := readTopologySource(bytes.NewReader(raw), Limits{}, true)
			if err != nil || graph.unit != .001 {
				t.Fatalf("IGES graph units %g: %v", graph.unit, err)
			}
			body, contacts := common.Boundary(), common.Contacts()
			checkClippedCommonIGESGraph(t, graph, body)
			vertices := checkThreeTransverseCommonIGESContacts(t, graph, body, contacts)
			modified := bytes.Clone(raw)
			replaceExactBezierIGESParameterDigit(t, modified, vertices, 2+3*contacts[2].Vertex)
			if _, err := readTopologySource(bytes.NewReader(modified), Limits{}, true); err != nil {
				t.Fatal("modified third contact must retain parseable IGES structure", err)
			}
			if err := verify(modified); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("modified third contact accepted: %v", err)
			}
			var refused bytes.Buffer
			if err := WriteClippedPolynomialThreeTransverseCommon(ctx, &refused, &trimmed.ClippedPolynomialThreeTransverseCommon{}, .001, Limits{}, opts); !errors.Is(err, geom.ErrInvalid) || refused.Len() != 0 {
				t.Fatalf("unproved native input published bytes: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_COMMON_IGES"); oracle != "" {
				opened := checkOCCTClippedCommonIGES(t, ctx, oracle, raw, body)
				if math.Abs(opened.Volume-tc.volume) > 1e-7 || math.Abs(opened.Area-tc.area) > 1e-7 {
					t.Fatalf("independent Common volume %.16g area %.16g; expected %.16g and %.16g", opened.Volume, opened.Area, tc.volume, tc.area)
				}
				checkOCCTThreeTransverseCommonIGESRays(t, body, opened)
			}
		})
	}
}

func checkThreeTransverseCommonIGESContacts(t *testing.T, graph topologySource, body trimmed.Solid, source []trimmed.PolynomialMixedEnvelopeContact) int {
	t.Helper()
	var vertices, edges topologyRecord
	for _, entity := range graph.entities {
		if entity.kind == 502 {
			vertices = entity
		}
		if entity.kind == 504 {
			edges = entity
		}
	}
	model := body.Edges()
	fields, err := topologyInts(edges, 2+5*len(model))
	if err != nil || fields[1] != len(model) {
		t.Fatalf("original model edge list: %v", err)
	}
	for i, edge := range model {
		at := 3 + 5*i
		if fields[at] != vertices.id || fields[at+1] != edge.Start+1 ||
			fields[at+2] != vertices.id || fields[at+3] != edge.End+1 {
			t.Fatalf("edge %d changed its original endpoints", i)
		}
	}
	if len(source) != 3 {
		t.Fatal("the original contact ledger must contain three crossings")
	}
	for i, contact := range source {
		vertex := contact.Vertex
		coords, err := igesPlanarCoordinates(vertices.values[2+3*vertex : 5+3*vertex])
		if err != nil {
			t.Fatal(err)
		}
		for axis := range coords {
			if coords[axis].Cmp(contact.Point[axis]) != 0 {
				t.Fatalf("contact %d no longer has the original exact coordinate", i)
			}
		}
		rays := 0
		for edge := range model {
			at := 3 + 5*edge
			if fields[at+1] == vertex+1 || fields[at+3] == vertex+1 {
				if !slices.Contains(contact.Rays[:], edge) {
					t.Fatalf("contact %d has an unrelated incident edge %d", i, edge)
				}
				rays++
			}
		}
		if rays != len(contact.Rays) {
			t.Fatalf("contact %d has %d original rays, want four", i, rays)
		}
	}
	return vertices.id
}

func checkOCCTThreeTransverseCommonIGESRays(t *testing.T, body trimmed.Solid, opened occtExactBezierIGESComponent) {
	t.Helper()
	points := body.Vertices()
	sourceID := make(map[[3]float64]int, len(points))
	for i, p := range points {
		sourceID[[3]float64{p.X, p.Y, p.Z}] = i
	}
	readbackID := make([]int, len(opened.Vertices))
	for i, p := range opened.Vertices {
		readbackID[i] = sourceID[p] // Coordinates were checked by the common qualification.
	}
	if len(opened.EdgeVertices) != opened.Edges {
		t.Fatalf("the independent oracle must report every edge's endpoint indices: %+v", opened)
	}
	byEndpoints := make(map[[2]int]int, len(opened.EdgeVertices))
	for _, edge := range opened.EdgeVertices {
		if edge[0] < 0 || edge[0] >= len(readbackID) || edge[1] < 0 || edge[1] >= len(readbackID) {
			t.Fatalf("independent IGES edge endpoint not indexed: %v", edge)
		}
		a, b := readbackID[edge[0]], readbackID[edge[1]]
		byEndpoints[[2]int{min(a, b), max(a, b)}]++
	}
	for i, edge := range body.Edges() {
		a, b := edge.Start, edge.End
		key := [2]int{min(a, b), max(a, b)}
		if byEndpoints[key] == 0 {
			t.Fatalf("independent IGES edge %d changed its original indexed endpoints %v", i, key)
		}
		byEndpoints[key]--
	}
	t.Log("OCCT preserved every original edge endpoint, including the four rays at each of the three contacts")
}
