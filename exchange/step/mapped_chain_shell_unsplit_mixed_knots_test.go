package step

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
	"slices"
	"strings"
	"testing"
	"time"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const unsplitMixedKnotPath = "testdata/standard_mapped_chain_unsplit_mixed_knots.step"

func unsplitMixedKnotFixture(t *testing.T) ([]byte, MappedChainShellSTEPOptions) {
	t.Helper()
	raw, err := os.ReadFile(unsplitMixedKnotPath)
	if err != nil {
		t.Fatal(err)
	}
	o := MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowUnsplitSupportKnot: true, AllowMixedSupportKnots: true,
	}
	return raw, o
}

func TestOCCTMappedChainRectangularUnsplitMixedSupportKnots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, o := unsplitMixedKnotFixture(t)
	for _, disabled := range []MappedChainShellSTEPOptions{
		{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowUnsplitSupportKnot: true},
		{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true, AllowMixedSupportKnots: true},
		{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowMixedSupportKnots: true},
	} {
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled)
		if err == nil || len(bad.Crossings) != 0 || len(bad.Native.Solid.Faces()) != 0 {
			t.Fatal("all explicit foreign mixed capabilities are required", err)
		}
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := m.Native.Solid
	if len(m.Chain) != 3 || m.Crossing != nil || len(m.Crossings) != 2 || len(m.Native.ChainEdges) != 5 ||
		len(solid.Faces()) != 7 || len(solid.Edges()) != 22 || len(solid.Vertices()) != 17 || m.OriginalSTEP() != string(raw) {
		t.Fatal("one original foreign edge must yield the three certified native restrictions")
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	source := m.Chain[0]
	wire, err := readMappedChainStepSpline(f, source.ModelCurveID, 3, 16, 33, true)
	if err != nil || wire.degree != 3 || len(wire.poles) != 7 {
		t.Fatal("the original edge must store precisely two cubic model spans", err)
	}
	stations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	for i, k := range wire.knots[4:7] {
		if k.Cmp(stations[0]) != 0 {
			t.Fatal("the only stored model knot must be the synchronous material kink", i)
		}
	}
	wantPoint := [2]geom.Vec3{
		{X: 29.0 / 256, Y: .25, Z: 29.0 / 1024},
		{X: 369.0 / 2048, Y: .25, Z: 875.0 / 16384},
	}
	wantRaw := [2][2]int64{{41, 51}, {65, 43}}
	edges, vertices := solid.Edges(), solid.Vertices()
	for i, p := range m.Crossings {
		if p.EdgeID != source.EdgeID || p.SourceModelParameter.Cmp(stations[i]) != 0 ||
			p.ModelStation.Cmp(stations[i]) != 0 || p.OwnersChangingCells != ([2]bool{true, i == 0}) ||
			p.NativeEdges != ([2]int{i, i + 1}) {
			t.Fatal("stored and inferred mixed crossings must preserve distinct owner provenance", i, p)
		}
		left, right := edges[p.NativeEdges[0]], edges[p.NativeEdges[1]]
		if left.End != right.Start || vertices[left.End] != wantPoint[i] {
			t.Fatal("both rebuilt model sides must share the exact crossing vertex", i)
		}
		for owner, tau := range p.RawParameters {
			if tau.Cmp(big.NewRat(wantRaw[i][owner], 1)) != 0 {
				t.Fatal("lost original raw restriction parameter", i, owner, tau)
			}
		}
	}
	for owner, face := range solid.Faces()[:2] {
		trace := source.Owners[owner]
		basisID, span, reversed, err := mappedChainStepRawUse(f, trace.TrimmedCurveID)
		if err != nil || basisID != trace.BasisID {
			t.Fatal("original whole support pcurve ancestry was lost", owner, err)
		}
		input, err := readMappedChainStepSpline(f, basisID, 2, 4, 64, true)
		if err != nil {
			t.Fatal(err)
		}
		basis, err := input.ownerBasis()
		if err != nil {
			t.Fatal(err)
		}
		first, last := span.Min, span.Max
		if reversed {
			first, last = last, first
		}
		stops := [4]*big.Rat{
			mappedChainStepRat(first), big.NewRat(wantRaw[0][owner], 1),
			big.NewRat(wantRaw[1][owner], 1), mappedChainStepRat(last),
		}
		for i := range 3 {
			q := face.Outer[i].Curve.(trimmed.MappedSplineUV)
			original := q.Source()
			if original.Reversed != reversed || original.Curve.Degree() != basis.Degree() ||
				!slices.Equal(original.Curve.Poles(), basis.Poles()) || !slices.Equal(original.Curve.Weights(), basis.Weights()) ||
				!slices.Equal(original.Curve.Knots(), basis.Knots()) {
				t.Fatal("native repair changed the original raw UV spline", owner, i)
			}
			for side := range 2 {
				v, err := q.AuthoredParameter(big.NewRat(int64(side), 1))
				if err != nil || v.Cmp(stops[i+side]) != 0 {
					t.Fatal("native restrictions failed to partition the original pcurve map", owner, i, side, err)
				}
			}
		}
	}
	cert, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !cert.Complete || len(cert.Uses) != 44 {
		t.Fatal("all rebuilt original source coedges must retain complete model equality", err)
	}
	var recipe, output bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var sourceRecord struct{ Boundary struct{ Version int } }
	if err := json.Unmarshal(recipe.Bytes(), &sourceRecord); err != nil || sourceRecord.Boundary.Version != 6 {
		t.Fatal("native material must retain the mixed source-only reproof mode", err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMappedChainRectangularShell(ctx, &output, shell, m.Name, m.LengthUnit, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadMappedChainRectangularShell(ctx, &output, Limits{}, o)
	if err != nil || len(reopened.Chain) != 5 || reopened.Native.ChainStations[1].Cmp(stations[0]) != 0 ||
		reopened.Native.ChainStations[2].Cmp(stations[1]) != 0 {
		t.Fatal("canonical explicit transfer lost one original crossing", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitMixedSupportKnotsRejectsInvalidSourceEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, o := unsplitMixedKnotFixture(t)
	tests := []struct {
		name, context string
		want          error
		edit          func(*File)
	}{
		{"moved stored model knot", "edge #112 owner", geom.ErrConvergence, func(f *File) {
			f.Entities[85].Records[0].Args[7].Items[1].Text = "0.1171875"
		}},
		{"missing second interior support crossing", "original support crossing missing", geom.ErrConvergence, func(f *File) {
			f.Entities[13].Records[0].Args[10].Items[2].Text = "0.25"
		}},
		{"unrepresentable companion raw limit", "edge #112: companion owner", geom.ErrUnrepresentable, func(f *File) {
			k, _ := component(f.Entities[107], "B_SPLINE_CURVE_WITH_KNOTS")
			k.Args[1].Items[3].Text = "28."
			f.Entities[108].Records[0].Args[3].Items[0].Items[0].Text = "28."
		}},
		{"changed model inside the smooth branch", "edge #112 owner", geom.ErrConvergence, func(f *File) {
			f.Entities[547].Records[0].Args[1].Items[2].Text = "0.06640625"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			test.edit(f)
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.context) || len(bad.Crossings) != 0 ||
				len(bad.Native.Solid.Edges()) != 0 {
				t.Fatal("invalid source event must fail before publishing a repaired shell", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularUnsplitMixedSupportKnotsReversedEdge(t *testing.T) {
	t.Parallel()
	raw, o := unsplitMixedKnotFixture(t)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(f.Entities[85].Records[0].Args[2].Items)
	edge := f.Entities[112].Records[0]
	edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
	f.Entities[94].Records[0].Args[4].Text = "F"
	f.Entities[108].Records[0].Args[4].Text = "T"
	for _, id := range []int64{454, 467} {
		f.Entities[id].Records[0].Args[4].Text = "F"
	}
	m, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if err != nil || len(m.Crossings) != 2 || m.Crossings[0].ModelStation.Cmp(big.NewRat(1, 8)) != 0 ||
		m.Crossings[1].ModelStation.Cmp(big.NewRat(3, 16)) != 0 || m.Crossings[1].SourceModelParameter.Cmp(big.NewRat(1, 16)) != 0 ||
		m.Crossings[1].RawParameters[0].Cmp(big.NewRat(65, 1)) != 0 || m.Crossings[1].RawParameters[1].Cmp(big.NewRat(43, 1)) != 0 ||
		m.Crossings[0].OwnersChangingCells != [2]bool{true, true} || m.Crossings[1].OwnersChangingCells != [2]bool{true, false} {
		t.Fatal("mixed proof must infer the same owner and accurately reflect reversed source intervals", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitMixedSupportKnotsPreservesExistingModes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, o := unsplitMixedKnotFixture(t)
	o.AllowAsynchronousSupportKnots = true
	for i, path := range []string{unsplitTwoKnotPath, unsplitAsynchronousPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
		if err != nil || len(m.Crossings) != 2 {
			t.Fatal("the mixed gate must leave synchronized and distinct-edge imports available", path, err)
		}
		var recipe bytes.Buffer
		if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
			t.Fatal(err)
		}
		var record struct{ Boundary struct{ Version int } }
		if err := json.Unmarshal(recipe.Bytes(), &record); err != nil || record.Boundary.Version != 4+i {
			t.Fatal("the enabled options must preserve the existing v4 and v5 repair identities", path, err)
		}
	}
}

func TestOCCTMappedChainRectangularUnsplitMixedSupportKnotsLive(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, o := unsplitMixedKnotFixture(t)
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	var explicit bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &explicit, m.Shell, m.Name, m.LengthUnit, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name string
		file []byte
	}{{"unsplit-original", raw}, {"canonical-repaired", explicit.Bytes()}} {
		path := filepath.Join(t.TempDir(), input.name+".step")
		if err := os.WriteFile(path, input.file, 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT rejected %s: %v\n%s", input.name, err, data)
		}
		var props struct {
			Volume, Area  float64
			Faces, Solids int
		}
		if err := json.Unmarshal(data, &props); err != nil {
			t.Fatal(err)
		}
		if props.Solids != 1 || props.Faces != 7 || math.Abs(props.Volume-mappedChainMixedVolume) > 1e-8 ||
			math.Abs(props.Area-6.1387980719367325) > 1e-8 {
			t.Fatal("pinned OCCT must retain the same mixed-crease material", input.name, props)
		}
		t.Logf("OCCT %s volume %.15g, area %.15g", input.name, props.Volume, props.Area)
	}
}
