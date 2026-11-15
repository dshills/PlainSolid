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

const unsplitMappedChainPath = "testdata/standard_mapped_chain_unsplit_knot.step"

func readUnsplitMappedChainFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(unsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOCCTMappedChainRectangularUnsplitSupportKnot(t *testing.T) {
	ctx := context.Background()
	raw := readUnsplitMappedChainFixture(t)
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true}
	for _, disabled := range []MappedChainShellSTEPOptions{{AllowMultiSpanRoofs: true}, {AllowUnsplitSupportKnot: true}} {
		rejected, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled)
		if err == nil || rejected.Crossing != nil || len(rejected.Native.Solid.Faces()) != 0 {
			t.Fatal("foreign unsplit source requires both explicit capabilities", err)
		}
	}
	opened, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	crossing, solid := opened.Crossing, opened.Native.Solid
	if crossing == nil || len(opened.Chain) != 3 || len(opened.Native.ChainEdges) != 4 ||
		len(solid.Vertices()) != 15 || len(solid.Edges()) != 20 || len(solid.Faces()) != 7 ||
		opened.OriginalSTEP() != string(raw) {
		t.Fatal("original source edge inventory and repaired native topology")
	}
	row := opened.Chain[0]
	station := big.NewRat(1, 8)
	wantRaw := [2]*big.Rat{big.NewRat(5, 1), big.NewRat(27, 1)}
	if crossing.EdgeID != row.EdgeID || crossing.SourceModelParameter.Cmp(station) != 0 ||
		crossing.ModelStation.Cmp(station) != 0 || crossing.NativeEdges != ([2]int{0, 1}) {
		t.Fatal("unsplit source station and paired native edge IDs changed", crossing)
	}
	edges := solid.Edges()
	left, right := edges[crossing.NativeEdges[0]], edges[crossing.NativeEdges[1]]
	point := geom.Vec3{X: 29.0 / 256, Y: 23.0 / 128, Z: 667.0 / 32768}
	if left.End != right.Start || solid.Vertices()[left.End] != point {
		t.Fatal("the repaired owners must share one exact native vertex")
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var sourceEdges, sourceVertices int
	for _, entity := range file.Entities {
		if _, ok := component(entity, "EDGE_CURVE"); ok {
			sourceEdges++
		}
		if _, ok := component(entity, "VERTEX_POINT"); ok {
			sourceVertices++
		}
	}
	if sourceEdges != 19 || sourceVertices != 14 {
		t.Fatal("the imported source must have no pre-split topological vertex", sourceEdges, sourceVertices)
	}
	for owner, face := range solid.Faces()[:2] {
		trace := row.Owners[owner]
		basisID, sourceRange, reversed, err := mappedChainStepRawUse(file, trace.TrimmedCurveID)
		if err != nil || basisID != trace.BasisID || wantRaw[owner].Cmp(mappedChainStepRat(sourceRange.Min)) <= 0 ||
			wantRaw[owner].Cmp(mappedChainStepRat(sourceRange.Max)) >= 0 ||
			crossing.RawParameters[owner].Cmp(wantRaw[owner]) != 0 {
			t.Fatal("lost original raw restriction entity or its exact crossing value", owner, err)
		}
		wire, err := readMappedChainStepSpline(file, basisID, 2, 4, 64, true)
		if err != nil {
			t.Fatal(err)
		}
		basis, err := wire.ownerBasis()
		if err != nil {
			t.Fatal(err)
		}
		start, end := sourceRange.Min, sourceRange.Max
		if reversed {
			start, end = end, start
		}
		wantEnds := [3]*big.Rat{mappedChainStepRat(start), wantRaw[owner], mappedChainStepRat(end)}
		for cell := 0; cell < 2; cell++ {
			p := face.Outer[cell].Curve.(trimmed.MappedSplineUV)
			original := p.Source()
			if original.Reversed != reversed || original.Curve.Degree() != basis.Degree() ||
				!slices.Equal(original.Curve.Knots(), basis.Knots()) ||
				!slices.Equal(original.Curve.Poles(), basis.Poles()) ||
				!slices.Equal(original.Curve.Weights(), basis.Weights()) {
				t.Fatal("native subedge did not retain the original whole spline", owner, cell)
			}
			for i := range 2 {
				raw, err := p.AuthoredParameter(big.NewRat(int64(i), 1))
				if err != nil || raw.Cmp(wantEnds[cell+i]) != 0 {
					t.Fatal("native subedge endpoints changed the original raw interval", owner, cell, i, err)
				}
			}
		}
	}
	proof, err := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 40 {
		t.Fatal("native shell must still prove every complete coedge", err)
	}
	var recipe bytes.Buffer
	if err := opened.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var canonical bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &canonical, shell, opened.Name, opened.LengthUnit, Limits{}, o); err != nil {
		t.Fatal("native source recipe must rebuild a standalone STEP with explicit split coedges", err)
	}
	replayed, err := ReadMappedChainRectangularShell(ctx, &canonical, Limits{}, o)
	if err != nil || len(replayed.Chain) != 4 || replayed.Native.ChainStations[1].Cmp(station) != 0 {
		t.Fatal("explicit STEP wire lost the certified crossing", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitSupportKnotRejectsMalformedSource(t *testing.T) {
	ctx := context.Background()
	raw := readUnsplitMappedChainFixture(t)
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true}
	rejected := func(input []byte, limits Limits) error {
		t.Helper()
		got, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(input), limits, o)
		if err == nil || got.Crossing != nil || len(got.Chain) != 0 || len(got.Native.Solid.Vertices()) != 0 {
			t.Fatal("foreign STEP proof failures must discard the entire partial shell", err)
		}
		return err
	}
	replace := func(input []byte, old, new string) []byte {
		t.Helper()
		if !bytes.Contains(input, []byte(old)) {
			t.Fatal("missing exact fixture value", old)
		}
		return bytes.Replace(input, []byte(old), []byte(new), 1)
	}
	rejected(replace(raw, "(0.,0.125,0.25)", "(0.,0.12500000000000003,0.25)"), Limits{})
	rejected(replace(raw, "0.21484375,0.00908203125", "0.21484375,0.0090820312500001"), Limits{})
	rejected(replace(raw, "(6,5,6),(0.,0.125,0.25)", "(6,4,1,6),(0.,0.125,0.1875,0.25)"), Limits{})
	rejected(raw, Limits{MaxVertices: 14})
	rejected(raw, Limits{MaxEdgeUses: 38})
	irrational := replace(raw, "(0.,0.11328125,1.),(0.,1.)", "(0.,0.125,1.),(0.,1.)")
	if err := rejected(irrational, Limits{}); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("an unresolved algebraic crossing cannot become a rounded knot", err)
	}
	coincident := raw
	for _, change := range [][2]string{
		{"#82=CARTESIAN_POINT('',(0.,0.25))", "#82=CARTESIAN_POINT('',(0.11328125,0.25))"},
		{"#83=CARTESIAN_POINT('',(0.125,0.125))", "#83=CARTESIAN_POINT('',(0.11328125,0.125))"},
		{"#84=CARTESIAN_POINT('',(0.375,0.125))", "#84=CARTESIAN_POINT('',(0.11328125,0.125))"},
		{"#85=CARTESIAN_POINT('',(0.5,0.25))", "#85=CARTESIAN_POINT('',(0.11328125,0.25))"},
	} {
		coincident = replace(coincident, change[0], change[1])
	}
	if err := rejected(coincident, Limits{}); !errors.Is(err, geom.ErrConvergence) || !strings.Contains(err.Error(), "must be isolated") {
		t.Fatal("a coedge following the entire support knot has no isolated crossing", err)
	}

	// An affine change of the first raw basis domain preserves the original
	// model edge and raw span endpoints. The true crossing now occurs at
	// raw 17/7 on the first owner and cannot become a native float64 limit.
	changed := replace(raw, "(2.,23.,27.)", "(2.,5.,9.)")
	changed = replace(changed, "#90=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(2.)),(PARAMETER_VALUE(9.))",
		"#90=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(2.)),(PARAMETER_VALUE(3.))")
	changed = replace(changed, "#131=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(9.)),(PARAMETER_VALUE(23.))",
		"#131=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(3.)),(PARAMETER_VALUE(5.))")
	changed = replace(changed, "#146=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(23.)),(PARAMETER_VALUE(27.))",
		"#146=TRIMMED_CURVE('',#89,(PARAMETER_VALUE(5.)),(PARAMETER_VALUE(9.))")
	if err := rejected(changed, Limits{}); !errors.Is(err, geom.ErrUnrepresentable) || !strings.Contains(err.Error(), "edge #108 owner") {
		t.Fatal("an unrepresentable exact raw restriction must fail at the unsplit source edge", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitSupportKnotReversedEdge(t *testing.T) {
	ctx := context.Background()
	file, err := Parse(bytes.NewReader(readUnsplitMappedChainFixture(t)), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	model := file.Entities[81].Records[0]
	slices.Reverse(model.Args[2].Items)
	edge := file.Entities[108].Records[0]
	edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
	file.Entities[90].Records[0].Args[4].Text = "F"
	file.Entities[104].Records[0].Args[4].Text = "T"
	file.Entities[426].Records[0].Args[4].Text = "F"
	file.Entities[437].Records[0].Args[4].Text = "F"
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowUnsplitSupportKnot: true}
	opened, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(file)), Limits{}, o)
	if err != nil || opened.Crossing == nil || opened.Crossing.ModelStation.Cmp(big.NewRat(1, 8)) != 0 ||
		opened.Crossing.RawParameters[0].Cmp(big.NewRat(5, 1)) != 0 || opened.Crossing.RawParameters[1].Cmp(big.NewRat(27, 1)) != 0 {
		t.Fatal("both original owner parameter laws must survive reversed source orientation", err)
	}
}

func TestOCCTMappedChainRectangularUnsplitSupportKnotLive(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if exe == "" {
		t.Skip("optional pinned OCCT STEP consumer")
	}
	path, err := filepath.Abs(unsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, exe, "read", path).CombinedOutput()
	if err != nil {
		t.Fatalf("independent unsplit STEP validity check failed: %v\n%s", err, data)
	}
	var props struct {
		Volume, Area  float64
		Faces, Solids int
	}
	if err := json.Unmarshal(data, &props); err != nil {
		t.Fatal(err)
	}
	if props.Solids != 1 || props.Faces != 7 || math.Abs(props.Volume-mappedChainCrossKnotVolume) > 1e-8 ||
		math.Abs(props.Area-6.251604174083588) > 1e-8 {
		t.Fatal("independent OCCT did not recover the same unsplit material", props)
	}
	t.Logf("OCCT exact-equivalent unsplit fixture volume %.15g, area %.15g", props.Volume, props.Area)
}
