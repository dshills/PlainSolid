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

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const unsplitThreeKnotPath = "testdata/standard_mapped_chain_unsplit_three_knots.step"

func unsplitThreeKnotFixture(t *testing.T) ([]byte, MappedChainShellSTEPOptions) {
	t.Helper()
	raw, err := os.ReadFile(unsplitThreeKnotPath)
	if err != nil {
		t.Fatal(err)
	}
	return raw, MappedChainShellSTEPOptions{
		AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowUnsplitSupportKnot: true, AllowThreeSupportKnots: true,
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnots(t *testing.T) {
	ctx := context.Background()
	raw, o := unsplitThreeKnotFixture(t)
	disabled := o
	disabled.AllowThreeSupportKnots = false
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled); err == nil ||
		len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
		t.Fatal("one shared and two separate crossings require their own explicit gate", err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chain) != 3 || m.Crossing != nil || len(m.Crossings) != 3 || len(m.Native.ChainEdges) != 6 || m.OriginalSTEP() != string(raw) {
		t.Fatal("one original shared edge must be repaired into four exact paired restrictions")
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	source := m.Chain[0]
	wire, err := readMappedChainStepSpline(f, source.ModelCurveID, 3, 16, 33, true)
	if err != nil || wire.degree != 3 || len(wire.poles) != 7 {
		t.Fatal("the foreign source must store exactly two cubic model spans", err)
	}
	stations := [3]*big.Rat{big.NewRat(1, 16), big.NewRat(1, 8), big.NewRat(3, 16)}
	for _, k := range wire.knots[4:7] {
		if k.Cmp(stations[1]) != 0 {
			t.Fatal("the shared crease must be the only stored model knot")
		}
	}
	solid := m.Native.Solid
	edges, vertices := solid.Edges(), solid.Vertices()
	wantRaw := [3][2]int64{{93, 443}, {197, 387}, {317, 283}}
	wantOwners := [3][2]bool{{true, false}, {true, true}, {false, true}}
	wantPoints := [3]geom.Vec3{
		{X: 107.0 / 2048, Y: .25, Z: 107.0 / 8192},
		{X: 29.0 / 256, Y: .25, Z: 29.0 / 1024},
		{X: 369.0 / 2048, Y: .25, Z: 875.0 / 16384},
	}
	for i, cross := range m.Crossings {
		if cross.EdgeID != source.EdgeID || cross.ModelStation.Cmp(stations[i]) != 0 || cross.SourceModelParameter.Cmp(stations[i]) != 0 ||
			cross.OwnersChangingCells != wantOwners[i] || cross.NativeEdges != ([2]int{i, i + 1}) {
			t.Fatal("stored and inferred support crossings must retain their exact source ancestry", i, cross)
		}
		a, b := edges[cross.NativeEdges[0]], edges[cross.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != wantPoints[i] {
			t.Fatal("the owner restrictions must share the same exact native vertex", i)
		}
		for owner := range 2 {
			if cross.RawParameters[owner].Cmp(big.NewRat(wantRaw[i][owner], 1)) != 0 {
				t.Fatal("the original raw crossing parameter changed", i, owner)
			}
		}
	}
	for owner, face := range solid.Faces()[:2] {
		trace := source.Owners[owner]
		id, uv, reversed, err := mappedChainStepRawUse(f, trace.TrimmedCurveID)
		if err != nil || id != trace.BasisID {
			t.Fatal("the original unsplit raw owner interval must remain available", owner, err)
		}
		basis, err := readMappedChainStepSpline(f, id, 2, 4, 64, true)
		if err != nil {
			t.Fatal(err)
		}
		original, err := basis.ownerBasis()
		if err != nil {
			t.Fatal(err)
		}
		lo, hi := uv.Min, uv.Max
		if reversed {
			lo, hi = hi, lo
		}
		stops := [5]*big.Rat{
			mappedChainStepRat(lo), big.NewRat(wantRaw[0][owner], 1), big.NewRat(wantRaw[1][owner], 1),
			big.NewRat(wantRaw[2][owner], 1), mappedChainStepRat(hi),
		}
		for cell := range 4 {
			q := face.Outer[cell].Curve.(trimmed.MappedSplineUV)
			base := q.Source()
			if base.Reversed != reversed || base.Curve.Degree() != original.Degree() ||
				!slices.Equal(base.Curve.Poles(), original.Poles()) || !slices.Equal(base.Curve.Weights(), original.Weights()) ||
				!slices.Equal(base.Curve.Knots(), original.Knots()) {
				t.Fatal("the four native restrictions must retain the original whole owner basis", owner, cell)
			}
			for k := range 2 {
				p, err := q.AuthoredParameter(big.NewRat(int64(k), 1))
				if err != nil || p.Cmp(stops[cell+k]) != 0 {
					t.Fatal("lost an exact original raw interval after repair", owner, cell, k, err)
				}
			}
		}
	}
	verifyThreeKnotMaterial(t, ctx, m)
	var recipe, explicit bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var saved struct{ Boundary struct{ Version int } }
	if err := json.Unmarshal(recipe.Bytes(), &saved); err != nil || saved.Boundary.Version != 7 {
		t.Fatal("the unsplit proof must persist through source-only v7", err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMappedChainRectangularShell(ctx, &explicit, shell, m.Name, m.LengthUnit, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadMappedChainRectangularShell(ctx, &explicit, Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	verifyThreeKnotMaterial(t, ctx, reopened)
	for i, station := range stations {
		if reopened.Native.ChainStations[i+1].Cmp(station) != 0 {
			t.Fatal("canonical STEP lost an inferred support station", i)
		}
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsRejectsInvalidPartition(t *testing.T) {
	ctx := context.Background()
	raw, o := unsplitThreeKnotFixture(t)
	decimal := func(x float64) string {
		value, err := mappedChainStepDecimal(mappedChainStepRat(x))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	tests := []struct {
		name, message string
		want          error
		edit          func(*File)
	}{
		{"unrepresentable companion raw interval", "common three-cut partition", geom.ErrUnrepresentable, func(f *File) {
			k, _ := component(f.Entities[113], "B_SPLINE_CURVE_WITH_KNOTS")
			k.Args[1].Items[3].Text = "58."
			f.Entities[114].Records[0].Args[3].Items[0].Items[0].Text = "58."
		}},
		{"shared stored knot moved", "stored model knot", geom.ErrConvergence, func(f *File) {
			f.Entities[91].Records[0].Args[7].Items[1].Text = "0.1171875"
		}},
		{"coincident inferred support stations", "separate inferred support stations must be distinct", geom.ErrConvergence, func(f *File) {
			// Move only the second owner's off-seam crease. Its original model
			// trace remains exact, so the owner-level proof succeeds before the
			// pair proof rejects both exclusive stations at the same point.
			roof := f.Entities[22].Records[0]
			roof.Args[11].Items[1].Text, roof.Args[11].Items[2].Text = "0.05224609375", "0.11328125"
			for u, row := range roof.Args[3].Items {
				for i, ref := range row.Items {
					x := [4]float64{0, 107.0 / 2048, 29.0 / 256, 1}[i]
					y := float64(u)
					z := x*y + .125*max(0, x-29.0/256) + .125*(y-.25)*max(0, x-107.0/2048)
					p := f.Entities[ref.Ref].Records[0].Args[1].Items
					p[0].Text, p[2].Text = decimal(x), decimal(z)
				}
			}
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
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.message) || len(bad.Crossings) != 0 ||
				len(bad.Native.Solid.Edges()) != 0 {
				t.Fatal("the complete paired partition must be proved before any repair is published", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsReversedEdge(t *testing.T) {
	raw, o := unsplitThreeKnotFixture(t)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(f.Entities[91].Records[0].Args[2].Items)
	edge := f.Entities[118].Records[0]
	edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
	f.Entities[100].Records[0].Args[4].Text = "F"
	f.Entities[114].Records[0].Args[4].Text = "T"
	for _, id := range []int64{490, 504} {
		f.Entities[id].Records[0].Args[4].Text = "F"
	}
	m, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if err != nil || len(m.Crossings) != 3 || m.Crossings[0].SourceModelParameter.Cmp(big.NewRat(3, 16)) != 0 ||
		m.Crossings[2].SourceModelParameter.Cmp(big.NewRat(1, 16)) != 0 ||
		m.Crossings[0].ModelStation.Cmp(big.NewRat(1, 16)) != 0 ||
		m.Crossings[2].ModelStation.Cmp(big.NewRat(3, 16)) != 0 ||
		m.Crossings[0].OwnersChangingCells != [2]bool{true, false} || m.Crossings[2].OwnersChangingCells != [2]bool{false, true} {
		t.Fatal("native stations and original reversed source parameters must be kept together", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsPreservesEarlierModes(t *testing.T) {
	ctx := context.Background()
	_, o := unsplitThreeKnotFixture(t)
	o.AllowAsynchronousSupportKnots = true
	o.AllowMixedSupportKnots = true
	o.AllowIndependentOwnerBases = true
	o.AllowMixedOwnerBases = true
	for i, path := range []string{unsplitTwoKnotPath, unsplitAsynchronousPath, unsplitMixedKnotPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
		if err != nil || len(m.Crossings) != 2 {
			t.Fatal("the new gate must preserve earlier original support arrangements", path, err)
		}
		var recipe bytes.Buffer
		if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
			t.Fatal(err)
		}
		var record struct{ Boundary struct{ Version int } }
		if err := json.Unmarshal(recipe.Bytes(), &record); err != nil || record.Boundary.Version != 4+i {
			t.Fatal("the enabled options must preserve the earlier native reproof identity", path, err)
		}
	}
}

func TestOCCTMappedChainRectangularUnsplitThreeSupportKnotsLive(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, o := unsplitThreeKnotFixture(t)
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
		raw  []byte
	}{{"unsplit-source", raw}, {"canonical-repaired", explicit.Bytes()}} {
		path := filepath.Join(t.TempDir(), input.name+".step")
		if err := os.WriteFile(path, input.raw, 0600); err != nil {
			t.Fatal(err)
		}
		data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
		if err != nil {
			t.Fatalf("pinned OCCT rejected %s: %v\n%s", input.name, err, data)
		}
		var p struct {
			Volume, Area  float64
			Faces, Solids int
		}
		if err := json.Unmarshal(data, &p); err != nil {
			t.Fatal(err)
		}
		if p.Solids != 1 || p.Faces != 7 || math.Abs(p.Volume-mappedChainThreeVolume) > 1e-8 ||
			math.Abs(p.Area-mappedChainThreeArea) > 1e-8 {
			t.Fatal("original and canonical STEP must retain one valid identical material", input.name, p)
		}
		t.Logf("OCCT %s volume %.15g, area %.15g", input.name, p.Volume, p.Area)
	}
}
