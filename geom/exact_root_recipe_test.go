package geom

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestExactRootRecipeCanonicalRoundtripAndBinding(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell := cells.Cells[0]
	recipe, e := cell.EndpointRootRecipe(context.Background(), 1, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	data, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	var decoded ExactRootRecipe
	if e = json.Unmarshal(data, &decoded); e != nil {
		t.Fatal(e)
	}
	root, e := ImportExactRoot(context.Background(), decoded, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	canonical, e := root.Export(context.Background(), ExactRootCodecOptions{})
	if e != nil || !reflect.DeepEqual(canonical, recipe) || !reflect.DeepEqual(canonical.Coefficients, []string{"-1", "0", "2"}) {
		t.Fatal(canonical, e)
	}
	decoded.Coefficients[0] = "99"
	classification, e := root.Classify(context.Background(), ExactCurvePlaneEventOptions{})
	if e != nil || classification.Kind != ExactRootKindIrrational {
		t.Fatal(classification, e)
	}
	binding, e := cell.BindEndpointRoot(context.Background(), 1, root, ExactCurvePlaneEventOptions{})
	if e != nil || !binding.Proven || binding.Order != 0 {
		t.Fatal(binding, e)
	}
	plane := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}
	event, e := cell.BindPlaneRoot(context.Background(), plane, root, ExactCurvePlaneEventOptions{})
	if e != nil || event.EndpointMask() != 2 {
		t.Fatal(event, e)
	}
	eventRecipe, e := event.RootRecipe(context.Background(), ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	eventRoot, e := ImportExactRoot(context.Background(), eventRecipe, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	comparison, e := root.Compare(context.Background(), eventRoot, ExactCurvePlaneEventOptions{})
	if e != nil || !comparison.Proven || comparison.Order != 0 {
		t.Fatal(comparison, e)
	}
	near := recipe
	near.Coefficients = append([]string(nil), recipe.Coefficients...)
	near.Coefficients[0] = new(big.Rat).Sub(big.NewRat(-1, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))).RatString()
	different, e := ImportExactRoot(context.Background(), near, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = cell.BindPlaneRoot(context.Background(), plane, different, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal("near recipe falsely bound", e)
	}
	if _, e = cell.BindEndpointRoot(context.Background(), 1, different, ExactCurvePlaneEventOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
func TestExactRootRecipeRepeatedAndRationalKinds(t *testing.T) {
	r, e := ImportExactRoot(context.Background(), ExactRootRecipe{Version: 1, Coefficients: []string{"1", "0", "-4", "0", "4"}, Lower: "2/3", Upper: "3/4"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	recipe, e := r.Export(context.Background(), ExactRootCodecOptions{})
	if e != nil || !reflect.DeepEqual(recipe.Coefficients, []string{"-1", "0", "2"}) {
		t.Fatal(recipe, e)
	}
	rational, e := ImportExactRoot(context.Background(), ExactRootRecipe{Version: 1, Coefficients: []string{"1/9", "-2/3", "1"}, Lower: "0", Upper: "1"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, e := rational.Classify(context.Background(), ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != ExactRootKindRational || kind.Rational.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(kind, e)
	}
	recipe, e = rational.Export(context.Background(), ExactRootCodecOptions{})
	if e != nil || recipe.Lower != "1/3" || recipe.Upper != "1/3" || !reflect.DeepEqual(recipe.Coefficients, []string{"-1", "3"}) {
		t.Fatal(recipe, e)
	}
	// An additional irrelevant factor need not disappear from canonical form.
	other, e := ImportExactRoot(context.Background(), ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "-1", "2", "2"}, Lower: "2/3", Upper: "3/4"}, ExactRootCodecOptions{})
	if e != nil {
		t.Fatal(e)
	}
	comparison, e := r.Compare(context.Background(), other, ExactCurvePlaneEventOptions{})
	if e != nil || !comparison.Proven || comparison.Order != 0 {
		t.Fatal(comparison, e)
	}
}
func TestExactRootRecipeRejectsMalformedNonuniqueAndLimits(t *testing.T) {
	base := ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "0", "2"}, Lower: "0", Upper: "1"}
	for _, recipe := range []ExactRootRecipe{{Version: 2, Coefficients: base.Coefficients, Lower: "0", Upper: "1"}, {Version: 1, Coefficients: []string{"1e999999999", "1"}, Lower: "0", Upper: "1"}, {Version: 1, Coefficients: base.Coefficients, Lower: "-1", Upper: "1"}, {Version: 1, Coefficients: []string{"1/0", "1"}, Lower: "0", Upper: "1"}} {
		if _, e := ImportExactRoot(context.Background(), recipe, ExactRootCodecOptions{}); e == nil {
			t.Fatal("accepted invalid recipe", recipe)
		}
	}
	for _, o := range []ExactRootCodecOptions{{MaxTextBytes: 2}, {MaxDegree: 1}, {ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}}} {
		if _, e := ImportExactRoot(context.Background(), base, o); e == nil {
			t.Fatal("budget")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ImportExactRoot(ctx, base, ExactRootCodecOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
