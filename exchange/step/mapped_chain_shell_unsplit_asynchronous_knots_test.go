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

const unsplitAsynchronousPath = "testdata/standard_mapped_chain_unsplit_asynchronous_knots.step"

func unsplitAsynchronousFixture(t *testing.T) ([]byte, MappedChainShellSTEPOptions) {
	t.Helper()
	raw, err := os.ReadFile(unsplitAsynchronousPath)
	if err != nil {
		t.Fatal(err)
	}
	return raw, MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true, AllowAsynchronousSupportKnots: true}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousSupportKnots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, o := unsplitAsynchronousFixture(t)
	for _, disabled := range []MappedChainShellSTEPOptions{
		{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true},
		{AllowMultiSpanRoofs: true, AllowAsynchronousSupportKnots: true},
	} {
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled)
		if err == nil || len(bad.Crossings) != 0 || len(bad.Native.Solid.Faces()) != 0 {
			t.Fatal("the original asynchronous coedges require their own opt-in gate", err)
		}
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	solid := m.Native.Solid
	if len(m.Chain) != 3 || m.Crossing != nil || len(m.Crossings) != 2 || len(m.Native.ChainEdges) != 5 ||
		len(solid.Faces()) != 7 || len(solid.Edges()) != 21 || len(solid.Vertices()) != 16 || m.OriginalSTEP() != string(raw) {
		t.Fatal("lost the original three foreign coedges or the two new native vertices")
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	wantStations := [2]*big.Rat{big.NewRat(1, 8), big.NewRat(3, 8)}
	wantRaw := [2][2]int64{{17, 27}, {65, 21}}
	wantX := [2]float64{29.0 / 256, 99.0 / 256}
	edges, faces, vertices := solid.Edges(), solid.Faces(), solid.Vertices()
	for i, cross := range m.Crossings {
		row := m.Chain[i]
		changes := [2]bool{i == 0, i == 1}
		if cross.EdgeID != row.EdgeID || cross.ModelStation.Cmp(wantStations[i]) != 0 ||
			cross.SourceModelParameter.Cmp(wantStations[i]) != 0 || cross.OwnersChangingCells != changes ||
			cross.NativeEdges != ([2]int{2 * i, 2*i + 1}) {
			t.Fatal("source edge or owner-side crossing identity changed", i, cross)
		}
		x := wantX[i]
		a, b := edges[cross.NativeEdges[0]], edges[cross.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != (geom.Vec3{X: x, Y: .25, Z: x / 4}) {
			t.Fatal("both original owner coedges must share one exact newly introduced vertex", i)
		}
		wire, err := readMappedChainStepSpline(file, row.ModelCurveID, 3, 16, 17, true)
		if err != nil || wire.degree != 3 || len(wire.poles) != 4 ||
			wire.knots[0].Cmp(wantStations[i]) >= 0 || wire.knots[len(wire.knots)-1].Cmp(wantStations[i]) <= 0 {
			t.Fatal("fixture must retain a single smooth stored model Bézier across the original support knot", i, err)
		}
		for owner, trace := range row.Owners {
			id, span, reverse, err := mappedChainStepRawUse(file, trace.TrimmedCurveID)
			q := big.NewRat(wantRaw[i][owner], 1)
			if err != nil || id != trace.BasisID || q.Cmp(cross.RawParameters[owner]) != 0 {
				t.Fatal("original raw source pcurve ancestry or parameter changed", i, owner, err)
			}
			basis, err := readMappedChainStepSpline(file, id, 2, 4, 64, true)
			if err != nil {
				t.Fatal(err)
			}
			prior, err := basis.ownerBasis()
			if err != nil {
				t.Fatal(err)
			}
			start, end := span.Min, span.Max
			if reverse {
				start, end = end, start
			}
			want := [3]*big.Rat{mappedChainStepRat(start), q, mappedChainStepRat(end)}
			for cell := range 2 {
				p := faces[owner].Outer[2*i+cell].Curve.(trimmed.MappedSplineUV)
				original := p.Source()
				if original.Reversed != reverse || original.Curve.Degree() != prior.Degree() ||
					!slices.Equal(original.Curve.Poles(), prior.Poles()) ||
					!slices.Equal(original.Curve.Knots(), prior.Knots()) ||
					!slices.Equal(original.Curve.Weights(), prior.Weights()) {
					t.Fatal("source coedge repair lost one original owner basis", i, owner, cell)
				}
				for k := range 2 {
					raw, err := p.AuthoredParameter(big.NewRat(int64(k), 1))
					if err != nil || raw.Cmp(want[cell+k]) != 0 {
						t.Fatal("the two new coedges must partition the original raw restriction exactly", i, owner, cell, k, err)
					}
				}
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 42 {
		t.Fatal("every native coedge must retain complete source agreement", err)
	}
	var recipe, output bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var source struct{ Boundary struct{ Version int } }
	if err := json.Unmarshal(recipe.Bytes(), &source); err != nil || source.Boundary.Version != 5 {
		t.Fatal("the native source must explicitly rederive the v5 asynchronous repair", err)
	}
	reopened, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMappedChainRectangularShell(ctx, &output, reopened, m.Name, m.LengthUnit, Limits{}, o); err != nil {
		t.Fatal("source-only v5 reload must yield valid explicit standard coedges", err)
	}
	explicit, err := ReadMappedChainRectangularShell(ctx, &output, Limits{}, o)
	if err != nil || len(explicit.Chain) != 5 || explicit.Native.ChainStations[1].Cmp(wantStations[0]) != 0 ||
		explicit.Native.ChainStations[3].Cmp(wantStations[1]) != 0 {
		t.Fatal("explicit STEP transfer lost an original asynchronous model crossing", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousSupportKnotsRejectsCollapsedPartition(t *testing.T) {
	t.Parallel()
	raw, o := unsplitAsynchronousFixture(t)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	before, err := nauoClosure(f, []int64{498}, 32)
	if err != nil {
		t.Fatal(err)
	}
	// Re-encode both affected source edges as one valid cubic model spline.
	// The new companion UV basis uses the same exact raw speed as the first
	// owner with swapped chart coordinates. Thus both full source curves
	// can individually reproduce the model but both knots fall in one cell.
	model := f.Entities[81].Records[0]
	model.Args[2].Items[3].Ref = 530
	model.Args[7].Items[1].Text = "0.5"
	for id, x := range map[int64][2]string{524: {"0.125", "0.03125"}, 525: {"0.375", "0.09375"}} {
		p := f.Entities[id].Records[0].Args[1].Items
		p[0].Text, p[2].Text = x[0], x[1]
	}
	left, right := f.Entities[108].Records[0], f.Entities[134].Records[0]
	left.Args[2].Ref = right.Args[2].Ref
	for _, loop := range []int64{442, 454} {
		r := f.Entities[loop].Records[0]
		r.Args[1].Items = slices.Delete(r.Args[1].Items, 1, 2)
	}
	basisA, _ := component(f.Entities[89], "B_SPLINE_CURVE")
	basisB, _ := component(f.Entities[103], "B_SPLINE_CURVE")
	basisB.Args[1].Items = basisB.Args[1].Items[:len(basisA.Args[1].Items)]
	for i, id := range basisA.Args[1].Items {
		pointA := f.Entities[id.Ref].Records[0].Args[1].Items
		pointB := f.Entities[basisB.Args[1].Items[i].Ref].Records[0].Args[1].Items
		pointB[0], pointB[1] = pointA[1], pointA[0]
	}
	for _, componentName := range []string{"B_SPLINE_CURVE_WITH_KNOTS", "RATIONAL_B_SPLINE_CURVE"} {
		a, _ := component(f.Entities[89], componentName)
		b, _ := component(f.Entities[103], componentName)
		copy(b.Args, a.Args)
	}
	rangeOf := func(id int64, lo, hi string) {
		r := f.Entities[id].Records[0]
		r.Args[2].Items[0].Items[0].Text, r.Args[3].Items[0].Items[0].Text = lo, hi
		r.Args[4].Text = "T"
	}
	rangeOf(90, "2.", "107.")
	rangeOf(104, "2.", "107.")
	rangeOf(156, "107.", "111.")
	after, err := nauoClosure(f, []int64{498}, 32)
	if err != nil {
		t.Fatal(err)
	}
	f.Order = slices.DeleteFunc(f.Order, func(id int64) bool {
		if before[id] && !after[id] {
			delete(f.Entities, id)
			return true
		}
		return false
	})
	bad, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "one owner-side crossing per original edge") ||
		len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
		t.Fatal("both asynchronous events in one original cell must fail without a new partition", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousSupportKnotsRejectsUnrepresentableCompanion(t *testing.T) {
	t.Parallel()
	raw, o := unsplitAsynchronousFixture(t)
	for _, edit := range [][2]string{
		{"(2.,107.,111.)", "(2.,23.,27.)"},
		{"#90=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(2.)),(PARAMETER_VALUE(37.))", "#90=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(2.)),(PARAMETER_VALUE(9.))"},
		{"#127=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(37.)),(PARAMETER_VALUE(107.))", "#127=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(9.)),(PARAMETER_VALUE(23.))"},
		{"#153=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(107.)),(PARAMETER_VALUE(111.))", "#153=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(23.)),(PARAMETER_VALUE(27.))"},
	} {
		if !bytes.Contains(raw, []byte(edit[0])) {
			t.Fatal("missing known source fixture parameter", edit[0])
		}
		raw = bytes.Replace(raw, []byte(edit[0]), []byte(edit[1]), 1)
	}
	bad, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(raw), Limits{}, o)
	if !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), "edge #134: companion owner") ||
		len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
		t.Fatal("companion raw 73/5 must fail before publishing any repair", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousSupportKnotsRejectsChangedModel(t *testing.T) {
	t.Parallel()
	raw, o := unsplitAsynchronousFixture(t)
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// Change a model pole away from the start. The initial endpoint tangent
	// can still propose the same parameter law, but complete lift equality fails.
	f.Entities[525].Records[0].Args[1].Items[2].Text = "0.03912353515625"
	bad, err := ReadOCCTMappedChainRectangularShell(context.Background(), bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, o)
	if !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "edge #108 owner") ||
		len(bad.Crossings) != 0 || len(bad.Native.Solid.Edges()) != 0 {
		t.Fatal("the tangent candidate alone must not admit a changed model trace", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousOptionPreservesSynchronizedProofs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for i, path := range []string{unsplitMappedChainPath, unsplitTwoKnotPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		o := MappedChainShellSTEPOptions{
			AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true, AllowAsynchronousSupportKnots: true, AllowTwoInteriorRoofKnots: i == 1,
		}
		got, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
		if err != nil || len(got.Crossings) != i+1 {
			t.Fatal("the asynchronous option must preserve explicit synchronous model junctions", path, err)
		}
		for _, cross := range got.Crossings {
			if cross.OwnersChangingCells != [2]bool{true, true} {
				t.Fatal("both roof owners of a synchronous cut must remain recorded", path, cross)
			}
		}
	}
}

func TestOCCTMappedChainRectangularUnsplitAsynchronousSupportKnotsLive(t *testing.T) {
	t.Parallel()
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	path, err := filepath.Abs(unsplitAsynchronousPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("pinned OCCT rejected the original unsplit asynchronous material: %v\n%s", err, raw)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		t.Fatal(err)
	}
	if props.Solids != 1 || props.Faces != 7 || math.Abs(props.Volume-mappedChainAsynchronousVolume) > 1e-8 ||
		math.Abs(props.Area-5.9325913022728294) > 1e-8 {
		t.Fatal("pinned OCCT must recover the same exact material from the original two unsplit coedges", props)
	}
	t.Logf("OCCT read the unsplit source as one solid, volume %.15g, area %.15g", props.Volume, props.Area)
}
