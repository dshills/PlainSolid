package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestWriteClippedPolynomialCut(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		layout string
		solids int
	}{{"partial", 3}, {"dependent", 4}} {
		t.Run(tc.layout, func(t *testing.T) {
			input, err := os.Open("../testdata/native_clipped_polynomial_cut_" + tc.layout + ".json")
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			cut, err := trimmed.ReadClippedPolynomialCutJSON(ctx, input, trimmed.JSONLimits{}, trimmed.PolynomialNodeCutOptions{})
			if err != nil || len(cut.Components()) != tc.solids {
				t.Fatal("native components", err)
			}
			opts := ClippedPolynomialCutIGESOptions{PoleTolerance: 1e-14}
			var output, repeat bytes.Buffer
			for _, w := range []*bytes.Buffer{&output, &repeat} {
				if err := WriteClippedPolynomialCut(ctx, w, cut, .001, Limits{}, opts); err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(output.Bytes(), repeat.Bytes()) {
				t.Fatal("the exact source recipe changed its canonical IGES bytes")
			}
			raw := output.Bytes()
			d, err := readClippedPolynomialCutIGES(bytes.NewReader(raw), Limits{})
			if err != nil || d.unit != .001 {
				t.Fatal("IGES units", d.unit, err)
			}
			byType := map[int][]topologyRecord{}
			for _, record := range d.entities {
				byType[record.kind] = append(byType[record.kind], record)
			}
			parts := cut.Components()
			faces, edges := 0, 0
			for _, body := range parts {
				faces += len(body.Faces())
				edges += len(body.Edges())
			}
			for kind, count := range map[int]int{
				126: edges * 3, 128: faces, 186: tc.solids, 402: 1,
				502: tc.solids, 504: tc.solids, 508: faces, 510: faces, 514: tc.solids,
			} {
				if len(byType[kind]) != count {
					t.Fatalf("IGES type %d count %d, want %d", kind, len(byType[kind]), count)
				}
			}
			group, err := topologyInts(byType[402][0], 2+tc.solids)
			if err != nil || group[1] != tc.solids {
				t.Fatal("original material group", err)
			}
			for i, body := range parts {
				v, e := byType[502][i], byType[504][i]
				root, err := topologyInts(byType[186][i], 4)
				if err != nil || group[i+2] != byType[186][i].id || root[1] != byType[514][i].id {
					t.Fatal("original component and shell order", i, err)
				}
				vertices := body.Vertices()
				n, err := intValue(v.values[1])
				if err != nil || n != len(vertices) || len(v.values) != 2+3*n {
					t.Fatal("original vertex list count", i, err)
				}
				for j, p := range vertices {
					raw, err := igesPlanarCoordinates(v.values[2+3*j : 5+3*j])
					if err != nil {
						t.Fatal(err)
					}
					for axis, x := range [3]float64{p.X, p.Y, p.Z} {
						if raw[axis].Cmp(new(big.Rat).SetFloat64(x)) != 0 {
							t.Fatalf("component %d vertex %d axis %d changed", i, j, axis)
						}
					}
				}
				edges := body.Edges()
				use, err := topologyInts(e, 2+5*len(edges))
				if err != nil || use[1] != len(edges) {
					t.Fatal("original edge list count", i, err)
				}
				for j, edge := range edges {
					at := 2 + 5*j
					if use[at+1] != v.id || use[at+2] != edge.Start+1 || use[at+3] != v.id || use[at+4] != edge.End+1 {
						t.Fatalf("component %d edge %d changed original vertex ownership", i, j)
					}
				}
			}
			verify := func(b []byte, unit float64) error {
				return VerifyClippedPolynomialCutIGES(ctx, bytes.NewReader(b), cut, unit, Limits{}, opts)
			}
			if err := verify(bytes.Replace(raw, []byte("PlainSolid exact"), []byte("PlainSolid check"), 1), .001); err != nil {
				t.Fatal("harmless start section annotation", err)
			}
			if err := verify(raw, 1); !errors.Is(err, geom.ErrConvergence) {
				t.Fatalf("different source units accepted: %v", err)
			}
			for _, contact := range cut.Contacts() {
				for j, body := range contact.Components {
					changed := bytes.Clone(raw)
					replaceExactBezierIGESParameterDigit(t, changed, byType[502][body].id, 2+3*contact.Vertices[j])
					if _, err := readClippedPolynomialCutIGES(bytes.NewReader(changed), Limits{}); err != nil {
						t.Fatal("negative must retain a valid IGES record structure", err)
					}
					if err := verify(changed, .001); !errors.Is(err, geom.ErrConvergence) {
						t.Fatalf("changed material contact coordinate accepted: %v", err)
					}
				}
			}
			precision := bytes.Replace(raw, []byte("1.E-7"), []byte("1.E-5"), 1)
			if bytes.Equal(precision, raw) || !errors.Is(verify(precision, .001), geom.ErrConvergence) {
				t.Fatal("changed global transfer precision accepted")
			}
			status := bytes.Clone(raw)
			for _, line := range bytes.Split(status, []byte{10}) {
				if len(line) == 80 && line[72] == 'D' && bytes.Equal(line[:8], []byte("     186")) &&
					bytes.Equal(line[64:72], []byte("00020000")) {
					copy(line[64:72], "00010000")
					break
				}
			}
			if bytes.Equal(status, raw) || !errors.Is(verify(status, .001), geom.ErrConvergence) {
				t.Fatal("changed group member visibility accepted")
			}
			var refused bytes.Buffer
			for _, l := range []Limits{{MaxBytes: 256}, {MaxRecords: 4}, {MaxEntities: 4}, {MaxValues: 10}, {MaxControlPoints: 4}} {
				if err := WriteClippedPolynomialCut(ctx, &refused, cut, .001, l, opts); err == nil || refused.Len() != 0 {
					t.Fatalf("resource limit %+v published a prefix: %v", l, err)
				}
			}
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			if err := WriteClippedPolynomialCut(cancelled, &refused, cut, .001, Limits{}, opts); !errors.Is(err, context.Canceled) || refused.Len() != 0 {
				t.Fatalf("cancelled write published a prefix: %v", err)
			}
			if oracle := os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"); oracle != "" {
				verifyOCCTClippedCutIGES(t, ctx, oracle, raw, cut, faces)
			}
		})
	}
}

func replaceExactBezierIGESParameterDigit(t *testing.T, raw []byte, entity, parameter int) {
	t.Helper()
	current := 0
	for _, line := range bytes.Split(raw, []byte{10}) {
		if len(line) != 80 || line[72] != 'P' || strings.TrimSpace(string(line[64:72])) != strconv.Itoa(entity) {
			continue
		}
		for i, c := range line[:64] {
			if c == ',' {
				current++
			} else if current == parameter && c >= '0' && c <= '9' {
				if c == '0' {
					line[i] = '1'
				} else {
					line[i] = '0'
				}
				return
			}
		}
	}
	t.Fatalf("no parameter %d in IGES entity %d", parameter, entity)
}

type occtExactBezierIGESReport struct {
	Faces, Solids, TransferredRoots, IndexedVertices int
	Components                                       []struct {
		Faces, Edges, Shells int
		Area, Volume         float64
		Vertices             [][3]float64
	}
}

func readOCCTExactBezierIGES(t *testing.T, ctx context.Context, oracle string, data []byte) occtExactBezierIGESReport {
	t.Helper()
	path := filepath.Join(t.TempDir(), "exact-bezier.igs")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.CommandContext(ctx, oracle, path).CombinedOutput()
	var result occtExactBezierIGESReport
	start := bytes.Index(raw, []byte("{\"TransferredRoots\":"))
	if err != nil || start < 0 || json.Unmarshal(raw[start:], &result) != nil {
		t.Fatalf("independent OCCT IGES reader: %v\n%s", err, raw)
	}
	return result
}

func verifyOCCTClippedCutIGES(t *testing.T, ctx context.Context, oracle string, data []byte, cut *trimmed.ClippedPolynomialCut, faces int) {
	t.Helper()
	result := readOCCTExactBezierIGES(t, ctx, oracle, data)
	parts := cut.Components()
	vertices := 0
	for _, part := range parts {
		vertices += len(part.Vertices())
	}
	if result.IndexedVertices != vertices {
		t.Fatalf("independent OCCT shared vertex IDs: %d distinct across %d component-local points", result.IndexedVertices, vertices)
	}
	if result.Solids != len(parts) || len(result.Components) != len(parts) || result.Faces != faces || result.TransferredRoots != 1 {
		t.Fatalf("independent OCCT material counts: %+v", result)
	}
	matched := make([]bool, len(parts))
	expectedVolumes, total := cut.ExactComponentVolumes()
	var volume float64
	for _, body := range result.Components {
		match := -1
		sources := map[[3]float64]bool{}
		for _, point := range body.Vertices {
			sources[point] = true
		}
		for i, part := range parts {
			if matched[i] || body.Faces != len(part.Faces()) || body.Edges != len(part.Edges()) || len(body.Vertices) != len(part.Vertices()) {
				continue
			}
			equal := true
			for _, p := range part.Vertices() {
				equal = equal && sources[[3]float64{p.X, p.Y, p.Z}]
			}
			if equal {
				match = i
				break
			}
		}
		if match < 0 || body.Shells != 1 || body.Volume <= 0 {
			t.Fatalf("independent OCCT original component incidence or orientation: %+v", body)
		}
		want, _ := expectedVolumes[match].Float64()
		if math.Abs(body.Volume-want) > math.Max(1e-12, want*1e-10) {
			t.Fatalf("independent OCCT component %d volume %.16g, exact source %.16g", match, body.Volume, want)
		}
		matched[match] = true
		volume += body.Volume
	}
	want, _ := total.Float64()
	if math.Abs(volume-want) > 1e-7 {
		t.Fatalf("independent OCCT volume %.16g, exact source %.16g", volume, want)
	}
	t.Logf("OCCT transferred %d solids with %d faces, %d separately indexed exact source vertices and volume %.16g",
		result.Solids, result.Faces, result.IndexedVertices, volume)
}
