package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const mixedBasisMappedChainPath = "testdata/occt_mapped_chain_three_crossings_mixed_bases.step"

func verifyMixedBasisMappedChain(t *testing.T, ctx context.Context, raw []byte) []byte {
	t.Helper()
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowIndependentOwnerBases: true}
	if _, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o); err == nil {
		t.Fatal("the previous independent gate does not authorize a mixed foreign basis graph")
	}
	o.AllowIndependentOwnerBases = false
	o.AllowMixedOwnerBases = true
	original, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	verifyThreeKnotMaterial(t, ctx, original)
	owners := original.Shell.IndependentOwnerBases()
	if owners[0] == owners[1] || len(original.Chain) != 6 || len(original.Crossings) != 0 || len(original.Planes) != 5 {
		t.Fatal("both original owner forms and the six explicit shared coedges are required", owners)
	}
	var recipe, nativeReplay bytes.Buffer
	if err := original.Shell.WriteJSON(ctx, &recipe, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	var record struct {
		Boundary struct {
			Version int
			Owners  []struct {
				Source  json.RawMessage
				Sources []json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(recipe.Bytes(), &record); err != nil || record.Boundary.Version != 2 || len(record.Boundary.Owners) != 2 {
		t.Fatal("native source version and owner inventory", err)
	}
	for i, r := range record.Boundary.Owners {
		if owners[i] && (len(r.Source) != 0 || len(r.Sources) != 6) || !owners[i] && (len(r.Source) == 0 || len(r.Sources) != 0) {
			t.Fatal("native source must retain the original basis organization", i)
		}
	}
	shell, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(recipe.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil || shell.IndependentOwnerBases() != owners {
		t.Fatal("native reload changed source ownership", err)
	}
	if err := shell.WriteJSON(ctx, &nativeReplay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), nativeReplay.Bytes()) {
		t.Fatal("native source must replay byte-identically", err)
	}
	var encoded, stepReplay bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &encoded, shell, original.Name, original.LengthUnit, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	canonical, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(encoded.Bytes()), Limits{}, o)
	if err != nil || canonical.Shell.IndependentOwnerBases() != owners {
		t.Fatal("strict STEP reopen changed source ownership", err)
	}
	verifyThreeKnotMaterial(t, ctx, canonical)
	nativeReplay.Reset()
	if err := canonical.Shell.WriteJSON(ctx, &nativeReplay, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(recipe.Bytes(), nativeReplay.Bytes()) {
		t.Fatal("strict STEP must retain the original source-only native record", err)
	}
	if err := WriteMappedChainRectangularShell(ctx, &stepReplay, canonical.Shell, canonical.Name, canonical.LengthUnit, Limits{}, o); err != nil ||
		!bytes.Equal(encoded.Bytes(), stepReplay.Bytes()) {
		t.Fatal("canonical STEP must replay byte-identically", err)
	}
	o.AllowMixedOwnerBases, o.AllowIndependentOwnerBases = false, true
	if bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(encoded.Bytes()), Limits{}, o); err == nil || len(bad.Chain) != 0 {
		t.Fatal("the independent-only canonical gate must reject the mixed graph", err)
	}
	stepReplay.Reset()
	if err := WriteMappedChainRectangularShell(ctx, &stepReplay, shell, original.Name, original.LengthUnit, Limits{}, o); err == nil || stepReplay.Len() != 0 {
		t.Fatal("mixed export must be atomic without its opt-in", err)
	}
	verifyMixedBasisMappedChainSources(t, ctx, original, canonical, owners)
	return encoded.Bytes()
}

func verifyMixedBasisMappedChainSources(t *testing.T, ctx context.Context, original, canonical MappedChainSTEPModel, independent [2]bool) {
	t.Helper()
	reference, err := mappedChainThreeSource(t, ctx).BuildRectangularSolid(ctx, 1e-9, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantFaces, origFaces, canonFaces := reference.Solid.Faces(), original.Native.Solid.Faces(), canonical.Native.Solid.Faces()
	wantStations := []*big.Rat{new(big.Rat), big.NewRat(1, 16), big.NewRat(1, 8), big.NewRat(3, 16), big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 1)}
	sharedRaw := []float64{478, 443, 387, 283, 23, 17, 10}
	for i, station := range wantStations {
		if original.Native.ChainStations[i].Cmp(station) != 0 || canonical.Native.ChainStations[i].Cmp(station) != 0 {
			t.Fatal("lost the exact model parameter station", i, station)
		}
	}
	file, err := Parse(bytes.NewReader([]byte(original.OriginalSTEP())), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for owner, separate := range independent {
		want := wantFaces[1]
		if separate {
			want = wantFaces[0]
		}
		if !geom.SameSplineSurface(want.Surface.(*geom.BSplineSurface), origFaces[owner].Surface.(*geom.BSplineSurface)) ||
			!geom.SameSplineSurface(want.Surface.(*geom.BSplineSurface), canonFaces[owner].Surface.(*geom.BSplineSurface)) {
			t.Fatal("one original multi-span chart changed", owner)
		}
		for i, cell := range original.Chain {
			foreign, strict := cell.Owners[owner], canonical.Chain[i].Owners[owner]
			if (foreign.TrimmedCurveID == 0) != separate || strict.TrimmedCurveID == 0 ||
				(foreign.BasisID == original.Chain[0].Owners[owner].BasisID) != (i == 0 || !separate) ||
				(strict.BasisID == canonical.Chain[0].Owners[owner].BasisID) != (i == 0 || !separate) {
				t.Fatal("both exact coedge forms must retain their original basis grouping", owner, i)
			}
			a, b := origFaces[owner].Outer[i].Curve.(trimmed.MappedSplineUV), canonFaces[owner].Outer[i].Curve.(trimmed.MappedSplineUV)
			source, exported := a.Source(), b.Source()
			if source.Range != exported.Range || source.Reversed != exported.Reversed || a.ParameterWeight().Cmp(b.ParameterWeight()) != 0 ||
				!sameMixedMappedChainBasis(source.Curve, exported.Curve) {
				t.Fatal("strict STEP changed an original basis, raw span, traversal or positive map", owner, i)
			}
			basis, err := readMappedChainStepSpline(file, foreign.BasisID, 2, 4, 64, true)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := basis.ownerBasis()
			if err != nil || !sameMixedMappedChainBasis(raw, source.Curve) {
				t.Fatal("original STEP coefficients must survive exactly", owner, i, err)
			}
			if separate {
				lo := float64(10 + 8*i)
				if source.Range != (trimmed.UVInterval{Min: lo, Max: lo + 7}) || a.ParameterWeight().Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("original independent raw domain or map changed", i)
				}
				continue
			}
			q := want.Outer[i].Curve.(trimmed.MappedSplineUV)
			if source.Range != (trimmed.UVInterval{Min: sharedRaw[i+1], Max: sharedRaw[i]}) || !source.Reversed ||
				!sameMixedMappedChainBasis(source.Curve, q.Source().Curve) || a.ParameterWeight().Cmp(q.ParameterWeight()) != 0 {
				t.Fatal("the original descending raw restrictions and exact positive maps must survive", i, a.ParameterWeight(), q.ParameterWeight())
			}
		}
	}
}

func sameMixedMappedChainBasis(a, b *geom.BSpline) bool {
	return a.Degree() == b.Degree() && slices.Equal(a.Poles(), b.Poles()) &&
		slices.Equal(a.Weights(), b.Weights()) && slices.Equal(a.Knots(), b.Knots())
}

func TestOCCTMappedChainRectangularThreeSupportKnotsMixedBases(t *testing.T) {
	raw, err := os.ReadFile(mixedBasisMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	verifyMixedBasisMappedChain(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularThreeSupportKnotsMixedBasesSwappedRoofs(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mixedBasisMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowMixedOwnerBases: true}
	m, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range f.Entities {
		shell, found := component(e, "CLOSED_SHELL")
		if !found {
			continue
		}
		faces := shell.Args[1].Items
		a := slices.IndexFunc(faces, func(v Value) bool { return v.Ref == m.FaceIDs[0] })
		b := slices.IndexFunc(faces, func(v Value) bool { return v.Ref == m.FaceIDs[1] })
		faces[a], faces[b] = faces[b], faces[a]
	}
	exported := verifyMixedBasisMappedChain(t, ctx, serializeWrappedProofFile(f))
	canonical, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(exported), Limits{}, o)
	bases := m.Shell.IndependentOwnerBases()
	if err != nil || canonical.Shell.IndependentOwnerBases() != [2]bool{bases[1], bases[0]} {
		t.Fatal("mixed basis modes must follow the actual owners after reordering", err)
	}
}

func TestOCCTMappedChainRectangularThreeSupportKnotsMixedBasesRejectsInconsistentGraph(t *testing.T) {
	ctx := context.Background()
	raw, err := os.ReadFile(mixedBasisMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	o := MappedChainShellSTEPOptions{AllowMultiSpanRoofs: true, AllowTwoInteriorRoofKnots: true, AllowMixedOwnerBases: true}
	foreign, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	var export bytes.Buffer
	if err := WriteMappedChainRectangularShell(ctx, &export, foreign.Shell, foreign.Name, foreign.LengthUnit, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	canonical, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(export.Bytes()), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	shared, separate := 0, 1
	if foreign.Shell.IndependentOwnerBases()[0] {
		shared, separate = 1, 0
	}
	t.Run("identical clone breaks common original basis identity", func(t *testing.T) {
		file, err := Parse(bytes.NewReader(export.Bytes()), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		use := canonical.Chain[1].Owners[shared]
		clone := file.Entities[use.BasisID]
		clone.ID = -1
		file.Entities[clone.ID] = clone
		file.Entities[use.TrimmedCurveID].Records[0].Args[1].Ref = clone.ID
		bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(file)), Limits{}, o)
		if err == nil || len(bad.Chain) != 0 || len(bad.Native.Solid.Edges()) != 0 {
			t.Fatal("exactly coincident spline copies still need a consistent common basis identity", err)
		}
	})
	t.Run("one independent owner use changes to parameter trim", func(t *testing.T) {
		file, err := Parse(bytes.NewReader(raw), Limits{})
		if err != nil {
			t.Fatal(err)
		}
		use := foreign.Chain[1].Owners[separate]
		pc := file.Entities[use.PCurveID].Records[0]
		definition := file.Entities[pc.Args[2].Ref].Records[0]
		parameter := func(text string) Value {
			return Value{Kind: Aggregate, Items: []Value{{Kind: Typed, Text: "PARAMETER_VALUE", Items: []Value{{Kind: Number, Text: text}}}}}
		}
		file.Entities[-1] = Entity{ID: -1, Records: []Record{{Type: "TRIMMED_CURVE", Args: []Value{
			{Kind: String}, {Kind: Reference, Ref: use.BasisID}, parameter("18."), parameter("25."),
			{Kind: Enumeration, Text: "T"}, {Kind: Enumeration, Text: "PARAMETER"},
		}}}}
		definition.Args[1].Items[0].Ref = -1
		bad, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(serializeWrappedProofFile(file)), Limits{}, o)
		if err == nil || len(bad.Chain) != 0 || len(bad.Native.Solid.Edges()) != 0 {
			t.Fatal("the foreign owner form must be consistent along the original explicit chain", err)
		}
	})
}

func TestOCCTMappedChainRectangularThreeSupportKnotsMixedBasesLive(t *testing.T) {
	author, reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_MIXED_BASIS_AUTHOR"), os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP")
	if author == "" || reader == "" {
		t.Skip("optional pinned OCCT mixed-basis author and STEP consumer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "occt-authored-mixed-bases.step")
	if out, err := exec.CommandContext(ctx, author, "write-three-knots-mixed-basis", path).CombinedOutput(); err != nil {
		t.Fatalf("pinned OCCT author failed: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	exported := verifyMixedBasisMappedChain(t, ctx, raw)
	verifyIndependentThreeKnotOCCTTransfer(t, ctx, reader, raw, exported)
}
