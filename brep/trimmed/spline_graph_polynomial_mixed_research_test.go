//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativePolynomialMixedEnvelopeReloadRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativePolynomialMixedFixture(t, 0, 1, 4)
	r, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialMixedEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	d := a.Data()
	d.Poles[(d.CountU-1)*d.CountV].Z += 1.0 / 1024
	altered, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	source, e := parametric.EncodeSurface(altered)
	if e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*polynomialEnvelopeRecipe){
		"contact type":   func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Order = 1 },
		"gap derivative": func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].DerivativeGap = "0" },
		"same vertex":    func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Vertex = r.Ledger.Contacts[0].Vertex },
		"sector cycle":   func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Sectors[0] = 5 },
		"lens edge":      func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[1] = r.Ledger.Edges[1][:1] },
		"owner":          func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[2] ^= 1 },
		"source":         func(r *polynomialEnvelopeRecipe) { r.Sources[1] = source },
		"tangent kind":   func(r *polynomialEnvelopeRecipe) { r.Kind = nativePolynomialTangencyEnvelope },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialEnvelopeRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			modified, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(modified), JSONLimits{}, PolynomialMixedEnvelopeOptions{}); out != nil || e == nil {
				t.Fatal("changed mixed contact proof accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 4}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialMixedEnvelopeOptions{}); out != nil || e == nil {
			t.Fatal("native recipe bound ignored", limits, e)
		}
	}
	lowBudget := PolynomialMixedEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("proof work bound ignored", e)
	}
	if out, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialMixedEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("ordinary mixed envelope accepted Cut", e)
	}
	base, a, b = nativePolynomialMixedFixture(t, 0, 1, 3)
	if out, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialMixedEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native mixed envelope rounded the crossing", e)
	}
}
