//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"plainsolid/geom"
)

func TestNativeAcnodalCubicCutReplay(t *testing.T) {
	ctx := context.Background()
	src := acnodalCubicStockFixture(t, true)
	o := AcnodalCubicCutOptions{}
	cut, err := CutAcnodalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	var raw, replay bytes.Buffer
	if err := cut.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadAcnodalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
	if err != nil || loaded.History() != cut.History() || loaded.ExactVolume().Cmp(cut.ExactVolume()) != 0 {
		t.Fatal("readback lost original regular-cut topology or exact volume", err)
	}
	for i, surface := range [3]*geom.BSplineSurface{src[0], src[2], src[1]} {
		if !geom.SameSplineSurface(loaded.sources[i], surface) {
			t.Fatal("readback changed an original support", i)
		}
	}
	checkNativeAcnodalCut(t, loaded, [2]*geom.BSplineSurface{loaded.sources[1], loaded.sources[2]})
	if err := loaded.WriteJSON(ctx, &replay); err != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
		t.Fatal("exact source replay changed canonical bytes", err)
	}
	for _, change := range []func(*acnodalCubicCutLedger){
		func(l *acnodalCubicCutLedger) { l.Volume = "0" },
		func(l *acnodalCubicCutLedger) { l.Isolated.XYZ[0] = "0" },
		func(l *acnodalCubicCutLedger) { l.Arc[0].UVJet[0][0] = "17" },
		func(l *acnodalCubicCutLedger) { l.ArcEdges[0] = l.ArcEdges[1] },
		func(l *acnodalCubicCutLedger) { l.WallFaces[0] = l.WallFaces[1] },
	} {
		var recipe acnodalCubicCutRecipe
		if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
			t.Fatal(err)
		}
		change(recipe.Ledger)
		var edited bytes.Buffer
		if err := writePolynomialGraphRecipe(ctx, &edited, recipe); err != nil {
			t.Fatal(err)
		}
		if r, err := ReadAcnodalCubicCutJSON(ctx, &edited, JSONLimits{}, o); !errors.Is(err, geom.ErrConvergence) || r != nil {
			t.Fatal("a changed source-or-incidence ledger passed replay", err)
		}
	}
	var changedSource acnodalCubicCutRecipe
	if err := json.Unmarshal(raw.Bytes(), &changedSource); err != nil {
		t.Fatal(err)
	}
	changedSource.Sources[2] = changedSource.Sources[1]
	var changed bytes.Buffer
	if err := writePolynomialGraphRecipe(ctx, &changed, changedSource); err != nil {
		t.Fatal(err)
	}
	if r, err := ReadAcnodalCubicCutJSON(ctx, &changed, JSONLimits{}, o); err == nil || r != nil {
		t.Fatal("unchanged ledger rescued an invalid original roof law", err)
	}
	if r, err := ReadAcnodalCubicCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); err == nil || r != nil {
		t.Fatal("an entity limit returned a partial shell", err)
	}
	q := loaded.IsolatedContact()
	q[0].SetInt64(0)
	loaded.ExactVolume().SetInt64(0)
	if loaded.IsolatedContact()[0].Sign() == 0 || loaded.ExactVolume().Sign() == 0 {
		t.Fatal("caller can mutate retained material or contact proof")
	}
}
