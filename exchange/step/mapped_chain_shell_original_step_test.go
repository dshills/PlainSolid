package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
)

func canonicalUnsplitMixedBasisSTEP(t *testing.T, ctx context.Context, input []byte) []byte {
	t.Helper()
	o := unsplitMappedChainMixedBasisOptions()
	source, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(input), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := WriteMappedChainRectangularOriginalShell(ctx, &b, source, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(b.Bytes()), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	verifyThreeKnotMaterial(t, ctx, strict)
	if strict.Name != source.Name || strict.LengthUnit != source.LengthUnit || len(strict.Planes) != 0 || len(strict.Chain) != 3 ||
		len(strict.Crossings) != 3 || strict.Shell.IndependentOwnerBases() != source.Shell.IndependentOwnerBases() {
		t.Fatal("strict reopen must retain the original unsplit ownership, stations and material")
	}
	verifyOriginalMappedChainSTEPGraph(t, source, strict)
	var nativeSource, nativeStrict, replay bytes.Buffer
	if err := source.Shell.WriteJSON(ctx, &nativeSource, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := strict.Shell.WriteJSON(ctx, &nativeStrict, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(nativeSource.Bytes(), nativeStrict.Bytes()) {
		t.Fatal("canonical export must reconstruct the identical v7 source-only native recipe", err)
	}
	loaded, err := trimmed.ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(nativeStrict.Bytes()), trimmed.MappedChainShellNativeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	strict.Native, err = loaded.BuildRectangularSolid(ctx, 1e-9, o.Geometry)
	if err != nil {
		t.Fatal(err)
	}
	verifyThreeKnotMaterial(t, ctx, strict)
	if err := WriteMappedChainRectangularOriginalShell(ctx, &replay, strict, Limits{}, o); err != nil || !bytes.Equal(b.Bytes(), replay.Bytes()) {
		t.Fatal("re-export of the captured canonical original graph must be byte-identical", err)
	}
	return b.Bytes()
}

func verifyOriginalMappedChainSTEPGraph(t *testing.T, source, strict MappedChainSTEPModel) {
	t.Helper()
	a, err := Parse(bytes.NewBufferString(source.OriginalSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse(bytes.NewBufferString(strict.OriginalSTEP()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range source.Chain {
		transfer := strict.Chain[i]
		stored, err := readMappedChainStepSpline(a, row.ModelCurveID, 3, 16, 33, source.foreign)
		if err != nil {
			t.Fatal(err)
		}
		model, err := readMappedChainStepSpline(b, transfer.ModelCurveID, 3, 16, 33, false)
		if err != nil || !sameOriginalMappedStepSpline(stored, model) {
			t.Fatal("the original model domain, knots, poles and weights must survive", i, err)
		}
		for owner, use := range row.Owners {
			to := transfer.Owners[owner]
			p, err := originalMappedChainStepOwner(a, use, source.foreign)
			if err != nil {
				t.Fatal(err)
			}
			q, err := originalMappedChainStepOwner(b, to, false)
			if err != nil || to.TrimmedCurveID == 0 || !sameMixedMappedChainBasis(p.Curve, q.Curve) || p.Range != q.Range || p.Reversed != q.Reversed {
				t.Fatal("the original owner coefficients, raw restrictions and senses must survive", i, owner, err)
			}
		}
	}
	for i, cut := range source.Crossings {
		transfer := strict.Crossings[i]
		if transfer.EdgeID != strict.Chain[0].EdgeID || cut.SourceModelParameter.Cmp(transfer.SourceModelParameter) != 0 || cut.ModelStation.Cmp(transfer.ModelStation) != 0 ||
			cut.StoredModelKnot != transfer.StoredModelKnot || cut.OwnersChangingCells != transfer.OwnersChangingCells || cut.NativeEdges != transfer.NativeEdges ||
			cut.RawParameters[0].Cmp(transfer.RawParameters[0]) != 0 || cut.RawParameters[1].Cmp(transfer.RawParameters[1]) != 0 {
			t.Fatal("the canonical graph must preserve the three-cut source and native ancestry", i)
		}
	}
}

func sameOriginalMappedStepSpline(a, b mappedChainStepSpline) bool {
	same := func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }
	return a.degree == b.degree && slices.EqualFunc(a.knots, b.knots, same) && slices.EqualFunc(a.weights, b.weights, same) &&
		slices.EqualFunc(a.poles, b.poles, func(a, b []*big.Rat) bool { return slices.EqualFunc(a, b, same) })
}

func TestOCCTMappedChainRectangularUnsplitMixedBasesOriginalSTEP(t *testing.T) {
	raw, err := os.ReadFile(mixedBasisUnsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	canonicalUnsplitMixedBasisSTEP(t, context.Background(), raw)
}

func TestOCCTMappedChainRectangularUnsplitMixedBasesOriginalSTEPScaledReverse(t *testing.T) {
	ctx := context.Background()
	o := unsplitMappedChainMixedBasisOptions()
	input, err := os.ReadFile(mixedBasisUnsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	raw := canonicalUnsplitMixedBasisSTEP(t, ctx, input)
	baseline, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Parse(bytes.NewReader(raw), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, edge := range baseline.Chain {
		_, knots, _, err := mappedChainStepSplineParts(f, edge.ModelCurveID)
		if err != nil {
			t.Fatal(err)
		}
		for i, knot := range knots[1].Items {
			q, err := affineSplineSourceRat(knot)
			if err != nil {
				t.Fatal(err)
			}
			knots[1].Items[i].Text, err = mappedChainStepDecimal(q.Mul(q, big.NewRat(3, 1)))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	first := baseline.Chain[0]
	model := f.Entities[first.ModelCurveID].Records[0]
	slices.Reverse(model.Args[2].Items)
	edge := f.Entities[first.EdgeID].Records[0]
	edge.Args[1], edge.Args[2] = edge.Args[2], edge.Args[1]
	for _, use := range first.Owners {
		trim := f.Entities[use.TrimmedCurveID].Records[0]
		if trim.Args[4].Text == "T" {
			trim.Args[4].Text = "F"
		} else {
			trim.Args[4].Text = "T"
		}
	}
	for _, entity := range f.Entities {
		if use, ok := component(entity, "ORIENTED_EDGE"); ok && use.Args[3].Ref == first.EdgeID {
			use.Args[4].Text = "F"
		}
	}
	modified := serializeWrappedProofFile(f)
	m, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(modified), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Crossings) != 3 || m.Crossings[0].SourceModelParameter.Cmp(big.NewRat(9, 1)) != 0 ||
		m.Crossings[1].SourceModelParameter.Cmp(big.NewRat(6, 1)) != 0 || m.Crossings[2].SourceModelParameter.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("scaled reversed input must retain its three actual source stations")
	}
	var exported bytes.Buffer
	if err := WriteMappedChainRectangularOriginalShell(ctx, &exported, m, Limits{}, o); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(exported.Bytes()), Limits{}, o)
	if err != nil || len(strict.Crossings) != 3 {
		t.Fatal("the captured nonminimal model scale and file direction must survive", err)
	}
	verifyOriginalMappedChainSTEPGraph(t, m, strict)
	var before, after bytes.Buffer
	if err := baseline.Shell.WriteJSON(ctx, &before, trimmed.MappedChainShellNativeOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := strict.Shell.WriteJSON(ctx, &after, trimmed.MappedChainShellNativeOptions{}); err != nil || !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("changing only the captured raw model scale and storage direction leaves the native source recipe identical", err)
	}
	if reader := os.Getenv("PLAINSOLID_OCCT_MAPPED_CHAIN_STEP"); reader != "" {
		verifyIndependentThreeKnotOCCTTransfer(t, ctx, reader, modified, exported.Bytes())
	}
}

func TestOCCTMappedChainRectangularUnsplitMixedBasesOriginalSTEPRejectsOutsideScope(t *testing.T) {
	ctx := context.Background()
	o := unsplitMappedChainMixedBasisOptions()
	input, err := os.ReadFile(mixedBasisUnsplitMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	raw := canonicalUnsplitMixedBasisSTEP(t, ctx, input)
	m, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	disabled := o
	disabled.AllowThreeSupportKnots = false
	if bad, err := ReadMappedChainRectangularShell(ctx, bytes.NewReader(raw), Limits{}, disabled); err == nil || len(bad.Crossings) != 0 {
		t.Fatal("strict canonical unsplit repair requires its original three-cut option", err)
	}
	var output bytes.Buffer
	if err := WriteMappedChainRectangularOriginalShell(ctx, &output, m, Limits{}, disabled); err == nil || output.Len() != 0 {
		t.Fatal("canonical unsplit export is atomic without its original three-cut option", err)
	}
	// The public native model alone cannot supply the captured authored model
	// scale. Only a proof tied to the original STEP graph supplies that evidence.
	if err := WriteMappedChainRectangularOriginalShell(ctx, &output, MappedChainSTEPModel{Shell: m.Shell, Native: m.Native}, Limits{}, o); err == nil || output.Len() != 0 {
		t.Fatal("the original model graph is required for source-preserving output", err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &output, m, Limits{MaxEntities: 1}, o); err == nil || output.Len() != 0 {
		t.Fatal("source export must verify the retained graph within the caller's limits", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := WriteMappedChainRectangularOriginalShell(canceled, &output, m, Limits{}, o); err != context.Canceled || output.Len() != 0 {
		t.Fatal("canceled original graph export must remain atomic", err)
	}
	explicit, err := os.ReadFile(mixedBasisMappedChainPath)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ReadOCCTMappedChainRectangularShell(ctx, bytes.NewReader(explicit), Limits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMappedChainRectangularOriginalShell(ctx, &output, other, Limits{}, o); err == nil || output.Len() != 0 {
		t.Fatal("the original-source tier applies to one mixed three-cut repair", err)
	}
}
