package step

import (
	"bytes"
	"context"
	"encoding/json"
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

const unsplitTwoKnotPath = "testdata/standard_mapped_chain_unsplit_two_knots.step"

func unsplitTwoKnotFixture(t *testing.T) ([]byte, MappedChainShellSTEPOptions) {
	t.Helper()
	data, err := os.ReadFile(unsplitTwoKnotPath)
	if err != nil {
		t.Fatal(err)
	}
	return data, MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true, AllowTwoInteriorRoofKnots: true}
}

func TestOCCTMappedChainRectangularUnsplitTwoSupportKnots(t *testing.T) {
	ctx := context.Background()
	raw, o := unsplitTwoKnotFixture(t)
	for _, disabled := range []MappedChainShellSTEPOptions{
		{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true},
		{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true},
	} {
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled)
		if err == nil || len(bad.Crossings) != 0 || len(bad.Native.Solid.Faces()) != 0 {
			t.Fatal("two unsplit crossings require both STEP capabilities", err)
		}
	}
	for _, limits := range []Limits{{MaxVertices: 17}, {MaxEdgeUses: 44}} {
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), limits, o)
		if err == nil || len(bad.Crossings) != 0 || len(bad.Native.Solid.Vertices()) != 0 {
			t.Fatal("output limits must include both repaired vertices and their new edge uses", err)
		}
	}
	model, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := model.Native.Solid
	if len(model.Chain) != 3 || model.Crossing != nil || len(model.Crossings) != 2 || len(model.Native.ChainEdges) != 5 ||
		len(solid.Vertices()) != 18 || len(solid.Edges()) != 23 || len(solid.Faces()) != 7 || model.OriginalSTEP() != string(raw) {
		t.Fatal("must retain three original shared STEP edges and prove two new native splits")
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	original := model.Chain[0]
	wire, err := readMappedChainStepSpline(file, original.ModelCurveID, 3, 16, 49, true)
	if err != nil || wire.degree != 5 || len(wire.poles) != 16 {
		t.Fatal("fixture must carry the three original C0 model branches", err)
	}
	wantStation := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	wantRaw := [2][2]int64{{41, 51}, {65, 43}}
	wantPoint := [2]geom.Vec3{
		{X: 29.0 / 256, Y: 23.0 / 128, Z: 667.0 / 32768},
		{X: 369.0 / 2048, Y: 83.0 / 512, Z: 39395.0 / 1048576},
	}
	edges, vertices := solid.Edges(), solid.Vertices()
	for i, crossing := range model.Crossings {
		if crossing.EdgeID != original.EdgeID || crossing.ModelStation.Cmp(wantStation[i]) != 0 ||
			crossing.SourceModelParameter.Cmp(wantStation[i]) != 0 || crossing.NativeEdges != ([2]int{i, i + 1}) {
			t.Fatal("wrong original source junction ancestry", i, crossing)
		}
		left, right := edges[crossing.NativeEdges[0]], edges[crossing.NativeEdges[1]]
		if left.End != right.Start || vertices[left.End] != wantPoint[i] {
			t.Fatal("both repaired branches must use the same exact native vertex", i)
		}
		for owner, raw := range crossing.RawParameters {
			if raw.Cmp(big.NewRat(wantRaw[i][owner], 1)) != 0 {
				t.Fatal("lost exact owner raw parameter", i, owner, raw)
			}
		}
	}
	for owner, face := range solid.Faces()[:2] {
		trace := original.Owners[owner]
		id, span, reversed, err := mappedChainStepRawUse(file, trace.TrimmedCurveID)
		if err != nil || id != trace.BasisID {
			t.Fatal("lost the unsplit original owner restriction", owner, err)
		}
		basis, err := readMappedChainStepSpline(file, id, 2, 4, 64, true)
		if err != nil {
			t.Fatal(err)
		}
		originalBasis, err := basis.ownerBasis()
		if err != nil {
			t.Fatal(err)
		}
		start, end := span.Min, span.Max
		if reversed {
			start, end = end, start
		}
		params := [4]*big.Rat{mappedChainStepRat(start), big.NewRat(wantRaw[0][owner], 1), big.NewRat(wantRaw[1][owner], 1), mappedChainStepRat(end)}
		for i := range 3 {
			curve := face.Outer[i].Curve.(trimmed.MappedSplineUV)
			c := curve.Source().Curve
			if curve.Source().Reversed != reversed || c.Degree() != originalBasis.Degree() ||
				!slices.Equal(c.Knots(), originalBasis.Knots()) || !slices.Equal(c.Poles(), originalBasis.Poles()) ||
				!slices.Equal(c.Weights(), originalBasis.Weights()) {
				t.Fatal("repaired coedge must retain its unsplit original whole basis", owner, i)
			}
			for side := range 2 {
				p, err := curve.AuthoredParameter(big.NewRat(int64(side), 1))
				if err != nil || p.Cmp(params[i+side]) != 0 {
					t.Fatal("repaired coedge raw interval changed", owner, i, side, err)
				}
			}
		}
	}
	edgesProof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !edgesProof.Complete || len(edgesProof.Uses) != 46 {
		t.Fatal("all three branches and planar seams require complete coedge proof", err)
	}
	var native, canonical bytes.Buffer
	if err := model.Shell.WriteJSON(ctx, &native, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &native, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMappedChainRectangularShell(ctx, &canonical, shell, model.Name, model.LengthUnit, Limits{}, o); err != nil {
		t.Fatal("reopened native source must export explicit coedges", err)
	}
	reopened, err := ReadMappedChainRectangularShell(ctx, &canonical, Limits{}, o)
	if err != nil || len(reopened.Chain) != 5 || reopened.Native.ChainStations[1].Cmp(wantStation[0]) != 0 ||
		reopened.Native.ChainStations[2].Cmp(wantStation[1]) != 0 {
		t.Fatal("canonical STEP exchange lost one of the repaired junctions", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitTwoSupportKnotsRejectsJunctions(t *testing.T) {
	data, o := unsplitTwoKnotFixture(t)
	cases := []struct {
		name, wantError string
		change          func(*File, Record)
	}{
		{"swapped model junction poles", "edge #118 owner", func(_ *File, model Record) {
			p := model.Args[2].Items
			p[5], p[10] = p[10], p[5]
		}},
		{"missing second junction", "stored model junctions must match", func(file *File, model Record) {
			p := model.Args[2].Items
			for _, v := range p[6:11] {
				delete(file.Entities, v.Ref)
				file.Order = slices.DeleteFunc(file.Order, func(id int64) bool { return id == v.Ref })
			}
			model.Args[2].Items = append(p[:6:6], p[11:]...)
			model.Args[6].Items = append(model.Args[6].Items[:2:2], model.Args[6].Items[3])
			model.Args[7].Items = append(model.Args[7].Items[:2:2], model.Args[7].Items[3])
		}},
		{"collapsed knot interval", "edge #118", func(_ *File, model Record) {
			model.Args[7].Items[2].Text = model.Args[7].Items[1].Text
		}},
		{"collapsed geometric middle branch", "edge #118 owner", func(file *File, model Record) {
			p := model.Args[2].Items
			first := file.Entities[p[5].Ref].Records[0]
			for _, pole := range p[6:11] {
				file.Entities[pole.Ref].Records[0].Args[1].Items = slices.Clone(first.Args[1].Items)
			}
		}},
		{"second junction needs a different parameter map", "one owner parameter map", func(_ *File, model Record) {
			model.Args[7].Items[2].Text = "0.18"
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(data), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			test.change(f, f.Entities[91].Records[0])
			out, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
			if err == nil || !strings.Contains(err.Error(), test.wantError) || len(out.Crossings) != 0 || len(out.Native.Solid.Edges()) != 0 {
				t.Fatal("invalid stored junction must fail atomically", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularUnsplitTwoSupportKnotsReversedEdge(t *testing.T) {
	raw, o := unsplitTwoKnotFixture(t)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	model := f.Entities[91].Records[0]
	slices.Reverse(model.Args[2].Items)
	model.Args[7].Items[1].Text, model.Args[7].Items[2].Text = "0.0625", "0.125"
	edge := f.Entities[118].Records[0]
	edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
	f.Entities[100].Records[0].Args[4].Text = "F"
	f.Entities[114].Records[0].Args[4].Text = "T"
	for _, id := range []int64{485, 498} {
		f.Entities[id].Records[0].Args[4].Text = "F"
	}
	out, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if err != nil || len(out.Crossings) != 2 {
		t.Fatal("reversing the whole unsplit source edge must preserve all three original branches", err)
	}
	modelStations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 16)}
	sourceKnots := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(1, 16)}
	wantRaw := [2][2]int64{{41, 51}, {65, 43}}
	for i, p := range out.Crossings {
		if p.ModelStation.Cmp(modelStations[i]) != 0 || p.SourceModelParameter.Cmp(sourceKnots[i]) != 0 ||
			p.RawParameters[0].Cmp(big.NewRat(wantRaw[i][0], 1)) != 0 || p.RawParameters[1].Cmp(big.NewRat(wantRaw[i][1], 1)) != 0 {
			t.Fatal("source spline knots and native station order must survive reversed traversal", i, p)
		}
	}
}

func TestOCCTMappedChainRectangularUnsplitTwoSupportKnotsLive(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	path, err := filepath.Abs(unsplitTwoKnotPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("independent unsplit two-knot STEP validity check failed: %v\n%s", err, data)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Solids != 1 || props.Faces != 7 || math.Abs(props.Volume-mappedChainTwoKnotVolume) > 1e-8 ||
		math.Abs(props.Area-6.228134630379123) > 1e-8 {
		t.Fatal("pinned OCCT must recover the same material from the unsplit source", props)
	}
	t.Logf("OCCT exact-equivalent two-knot fixture volume %.15g, area %.15g", props.Volume, props.Area)
}
