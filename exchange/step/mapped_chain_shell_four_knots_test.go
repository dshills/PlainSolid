package step

import (
	"bytes"
	"context"
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
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const fourKnotMappedChainPath = "testdata/occt_mapped_chain_four_unsplit_mixed_bases.step"

// The fourth original crease adds (1-407/16384)^2/64 to the v7 volume.
const mappedChainFourVolume = 16287280937.0 / 17179869184
const mappedChainFourArea = 6.2301472227365142

func fourKnotMappedChainOptions() MappedChainShellSTEPOptions {
	return MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true,
		AllowUnsplitSupportKnot: true, AllowFourSupportKnots: true, AllowMixedOwnerBases: true}
}

func verifyFourKnotMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := fourKnotMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	bases := m.Shell.IndependentOwnerBases()
	if bases[0] == bases[1] || len(m.Chain) != 3 || len(m.Crossings) != 4 || m.Crossing != nil {
		t.Fatal("the original mixed basis graph requires three original edges and four recovered cuts")
	}
	north := 0
	if !bases[0] {
		north = 1
	}
	solid := m.Native.Solid
	edges, vertices, faces := solid.Edges(), solid.Vertices(), solid.Faces()
	requireFourKnotMaterial(t, ctx, m.Native)
	points := [4]geom.Vec3{{X: 407.0 / 16384, Y: .25, Z: 407.0 / 65536}, {X: 107.0 / 2048, Y: .25, Z: 107.0 / 8192},
		{X: 29.0 / 256, Y: .25, Z: 29.0 / 1024}, {X: 369.0 / 2048, Y: .25, Z: 875.0 / 16384}}
	northRaw, southRaw := []int64{2, 9, 17, 37, 65, 107}, []int64{48, 44, 40, 32, 24, 16}
	stations := []int64{1, 2, 4, 6}
	for i, cut := range m.Crossings {
		if cut.EdgeID != m.Chain[0].EdgeID || cut.StoredModelKnot != (i == 2) || cut.SourceModelParameter.Cmp(big.NewRat(stations[i], 1)) != 0 ||
			cut.ModelStation.Cmp(big.NewRat(stations[i], 32)) != 0 || cut.RawParameters[north].Cmp(big.NewRat(northRaw[i+1], 1)) != 0 ||
			cut.RawParameters[1-north].Cmp(big.NewRat(southRaw[i+1], 1)) != 0 || cut.OwnersChangingCells[north] != (i <= 2) ||
			cut.OwnersChangingCells[1-north] != (i >= 2) || cut.NativeEdges != ([2]int{m.Native.ChainEdges[i], m.Native.ChainEdges[i+1]}) {
			t.Fatal("stored and inferred crossings must keep their original stations, owners, and native edge ancestry", i, cut)
		}
		a, b := edges[cut.NativeEdges[0]], edges[cut.NativeEdges[1]]
		if a.End != b.Start || vertices[a.End] != points[i] {
			t.Fatal("both owners must use one exact common vertex", i)
		}
	}
	file, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	parent := m.Chain[0]
	model, err := readMappedChainStepSpline(file, parent.ModelCurveID, 3, 16, 33, true)
	if err != nil || model.degree != 3 || len(model.poles) != 7 || model.knots[0].Sign() != 0 || model.knots[10].Cmp(big.NewRat(8, 1)) != 0 {
		t.Fatal("authored unsplit model must have two original cubic branches on [0,8]", err)
	}
	for _, knot := range model.knots[4:7] {
		if knot.Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("the shared material kink is the only stored model knot")
		}
	}
	for owner, u := range parent.Owners {
		original, err := originalMappedChainStepOwner(file, u, true)
		if err != nil {
			t.Fatal(err)
		}
		stops := southRaw
		if owner == north {
			stops = northRaw
		}
		for i := range 5 {
			c := faces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			if !sameMixedMappedChainBasis(c.Source().Curve, original.Curve) {
				t.Fatal("repaired children must restrict the complete original owner basis", owner, i)
			}
			for j := range 2 {
				raw, err := c.AuthoredParameter(big.NewRat(int64(j), 1))
				if err != nil || raw.Cmp(big.NewRat(stops[i+j], 1)) != 0 {
					t.Fatal("repaired child changed an exact raw limit", owner, i, j, err)
				}
			}
		}
	}
	verifyMappedRoofSlopeJump(t, ctx, faces[north].Surface.(*geom.BSplineSurface), 0, 407.0/16384, big.NewRat(1, 2), big.NewRat(1, 16))
	var recipe, canonical, replay bytes.Buffer
	if err := m.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(recipe.Bytes(), []byte(`"Boundary":{"Version":8`)) {
		t.Fatal("four cuts require their own native proof identifier")
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &canonical, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(canonical.Bytes()), Limits{}, o)
	if err != nil || len(strict.Chain) != 3 || len(strict.Crossings) != 4 || strict.Shell.IndependentOwnerBases() != bases {
		t.Fatal("strict canonical reopen must re-prove the four original-source coedge stations", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	if err := strict.Shell.WriteJSON(ctx, &replay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), replay.Bytes()) {
		t.Fatal("strict STEP re-export must recover the identical native v8 recipe", err)
	}
	loaded, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	native, err := loaded.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	requireFourKnotMaterial(t, ctx, native)
	replay.Reset()
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(canonical.Bytes(), replay.Bytes()) {
		t.Fatal("four-cut canonical replay must preserve the captured graph byte-for-byte", err)
	}
	label := []byte(`"Boundary":{"Version":8`)
	downgraded := bytes.Replace(recipe.Bytes(), label, []byte(`"Boundary":{"Version":7`), 1)
	if _, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(downgraded), trimmed.MappedChainShellNativeOptions{}); err == nil {
		t.Fatal("four-cut source cannot be reinterpreted as v7")
	}
	return canonical.Bytes()
}

func requireFourKnotMaterial(t *testing.T, ctx context.Context, native trimmed.MappedChainRectangularSolid) {
	t.Helper()
	s := native.Solid
	if len(native.ChainEdges) != 7 || len(s.Faces()) != 7 || len(s.Edges()) != 26 || len(s.Vertices()) != 21 {
		t.Fatal("four-cut roof with its additional original side corner requires 7 faces, 26 edges and 21 vertices")
	}
	proof, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if err != nil || !proof.Complete || len(proof.Uses) != 52 {
		t.Fatal("all native uses require complete continuous equality", err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if err != nil || math.Abs(mass.Volume-mappedChainFourVolume) > 1e-8 || math.Abs(mass.Area-mappedChainFourArea) > 1e-8 {
		t.Fatal("native material must match the independently authored four-cut roof", mass, err)
	}
}

func verifyMappedRoofSlopeJump(t *testing.T, ctx context.Context, surface *geom.BSplineSurface, axis int, knot float64, cross, wantJump *big.Rat) {
	t.Helper()
	patch, err := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, v := patch.KnotBreaks()
	breaks := [2][]float64{u, v}[axis]
	i := slices.Index(breaks, knot)
	if i <= 0 || i >= len(breaks)-1 {
		t.Fatal("the authored crease must be an interior knot", knot)
	}
	var z [2]*big.Rat
	uv := [2]*big.Rat{cross, cross}
	uv[axis] = mappedChainStepRat(knot)
	for side, bounds := range [][2]float64{{breaks[i-1], knot}, {knot, breaks[i+1]}} {
		box := patch.Domain()
		if axis == 0 {
			box.UMin, box.UMax = bounds[0], bounds[1]
		} else {
			box.VMin, box.VMax = bounds[0], bounds[1]
		}
		p, err := patch.Trim(box)
		if err != nil {
			t.Fatal(err)
		}
		jet, err := p.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		z[side] = [2]*big.Rat{jet.Du[2], jet.Dv[2]}[axis]
	}
	if new(big.Rat).Sub(z[1], z[0]).Cmp(wantJump) != 0 {
		t.Fatal("the original support knot must change the exact off-seam derivative", knot, z)
	}
}

func TestOCCTMappedChainRectangularFourSupportKnots(t *testing.T) {
	raw, err := os.ReadFile(fourKnotMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	verifyFourKnotMappedChain(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularFourSupportKnotsClosedDisplay(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(fourKnotMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, fourKnotMappedChainOptions())
	if err != nil {
		t.Fatal(err)
	}
	verifyMappedChainCutDisplay(t, ctx, m)
}

func verifyMappedChainCutDisplay(t *testing.T, ctx context.Context, m MappedChainSTEPModel) {
	t.Helper()
	fill := trimmed.MixedExactMultiSpanMeshOptions{Face: trimmed.MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005, Interior: trimmed.CurvedQuadraticTrimMeshOptions{Deflection: .04},
	}}
	verifyMappedChainCutDisplayWithOptions(t, ctx, m, fill, .005)
}

func verifyMappedChainCutDisplayWithOptions(t *testing.T, ctx context.Context, m MappedChainSTEPModel, fill trimmed.MixedExactMultiSpanMeshOptions, boundaryDeflection float64) trimmed.CertifiedMappedChainClosedMesh {
	t.Helper()
	return verifyMappedChainCutDisplayWithClosedOptions(t, ctx, m, trimmed.MappedChainClosedMeshOptions{
		Boundary: trimmed.MappedChainPolylineOptions{Deflection: boundaryDeflection}, Curved: [2]trimmed.MixedExactMultiSpanMeshOptions{fill, fill},
	})
}

func verifyMappedChainCutDisplayWithClosedOptions(t *testing.T, ctx context.Context, m MappedChainSTEPModel, o trimmed.MappedChainClosedMeshOptions) trimmed.CertifiedMappedChainClosedMesh {
	t.Helper()
	display, err := m.Shell.TessellateClosedCertified(ctx, nil, o)
	if err != nil || !display.DisplayCollision.Complete || len(display.DisplayCollision.Contacts) != 0 {
		t.Fatal("all exact cuts must support one closed display with no unintended contact", err)
	}
	edges, points := m.Native.Solid.Edges(), m.Native.Solid.Vertices()
	for _, cut := range m.Crossings {
		part := slices.IndexFunc(m.Chain, func(cell MappedChainSTEPCell) bool { return cell.EdgeID == cut.EdgeID })
		point := points[edges[cut.NativeEdges[0]].End]
		found := false
		for _, sample := range display.ChainSamples {
			if sample.ModelParameter.Cmp(cut.ModelStation) == 0 && display.Mesh.Vertices[sample.Vertex] == point &&
				sample.AuthoredParts == ([2]int{part, part}) && sample.AuthoredParameters[0].Cmp(cut.RawParameters[0]) == 0 &&
				sample.AuthoredParameters[1].Cmp(cut.RawParameters[1]) == 0 {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("the shared display ledger lost an original support station", cut.ModelStation)
		}
	}
	t.Logf("closed display retains %d exact shared stations across %d triangles", len(m.Crossings), len(display.Mesh.Triangles))
	return display
}

func TestOCCTMappedChainRectangularFourSupportKnotsRejectsInvalidSource(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(fourKnotMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := fourKnotMappedChainOptions()
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	older := o
	older.AllowFourSupportKnots, older.AllowThreeSupportKnots = false, true
	if bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, older); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("the older original-support bounds must reject the fourth event", err)
	}
	north := 0
	if !m.Shell.IndependentOwnerBases()[0] {
		north = 1
	}
	for _, test := range []struct {
		name, message string
		want          error
		edit          func(*File)
	}{
		{"moved material knot", "stored model knot", geom.ErrConvergence, func(f *File) {
			_, knots, _, err := mappedChainStepSplineParts(f, m.Chain[0].ModelCurveID)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[1].Text = "3.875"
		}},
		{"exact trace with nonbinary first owner stop", "not representable", geom.ErrUnrepresentable, func(f *File) {
			_, knots, _, err := mappedChainStepSplineParts(f, m.Chain[0].Owners[north].BasisID)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[1].Text = "108."
		}},
		{"overlapping inferred owner events on the same seam", "separate inferred support stations must be distinct", geom.ErrConvergence, func(f *File) {
			face := f.Entities[m.FaceIDs[1-north]].Records[0]
			roof := f.Entities[face.Args[2].Ref].Records[0]
			roof.Args[11].Items[1].Text, roof.Args[11].Items[2].Text = "0.02484130859375", "0.11328125"
			for i, row := range roof.Args[3].Items {
				for j, pole := range row.Items {
					x := [4]float64{0, 407.0 / 16384, 29.0 / 256, 1}[j]
					y := float64(i)
					z := x*y + .125*max(0, x-29.0/256) + .125*(y-.25)*max(0, x-407.0/16384)
					p := f.Entities[pole.Ref].Records[0].Args[1].Items
					for axis, value := range []float64{x, y, z} {
						text, err := mappedChainStepDecimal(mappedChainStepRat(value))
						if err != nil {
							t.Fatal(err)
						}
						p[axis].Text = text
					}
				}
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			file, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			test.edit(file)
			bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(file)), Limits{}, o)
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), test.message) || len(bad.Crossings) != 0 || len(bad.Native.ChainEdges) != 0 {
				t.Fatal("unsupported four-cut input must reject atomically", err)
			}
		})
	}
}

func TestOCCTMappedChainRectangularFourSupportKnotsLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_FOUR_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT four-cut author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-four-support-crossings.step")
	if out, err := exec.CommandContext(ctx, author, "write-four-knots-mixed-unsplit", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical := verifyFourKnotMappedChain(t, ctx, raw)
	verifyPinnedMappedChainTransfer(t, ctx, reader, raw, canonical, mappedChainFourVolume, mappedChainFourArea)
}
