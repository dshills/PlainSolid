//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestNativeParabolaCircleGraphCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	const wallCap = 335081.0 / 27525120
	disk := 4317.0 * math.Pi / 134217728
	annulus := 368009.0/27525120 + disk
	for axis := range 2 {
		src := parabolaCircleGraphStock(t, axis, big.NewRat(3, 4), big.NewRat(1, 64), false)
		for order, volumes := range [2][]float64{{wallCap, disk}, {annulus}} {
			t.Run(fmt.Sprintf("%d/%d", axis, order), func(t *testing.T) {
				r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, opts)
				if err != nil {
					t.Fatal(err)
				}
				checkNativeParabolaCircleCutComponents(t, ctx, r, [2][]int{{0, 2}, {1}}[order], volumes)
				var encoded, reencoded bytes.Buffer
				if err := r.WriteJSON(ctx, &encoded); err != nil {
					t.Fatal(err)
				}
				reload, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{}, opts)
				if err != nil || !reflect.DeepEqual(reload.History(), r.History()) {
					t.Fatal("changed Cut component ledger on source replay", err)
				}
				for i, s := range r.sources {
					if !geom.SameSplineSurface(s, reload.sources[i]) {
						t.Fatal("changed original source control net on replay", i)
					}
				}
				if err := reload.WriteJSON(ctx, &reencoded); err != nil || !bytes.Equal(encoded.Bytes(), reencoded.Bytes()) {
					t.Fatal("changed canonical Cut recipe", err)
				}
			})
		}
	}
}

func TestParabolaCircleGraphCutRejectsForgedLayer(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), false)
	r, err := CutParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := r.WriteJSON(ctx, &encoded); err != nil {
		t.Fatal(err)
	}
	if part, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{MaxEntities: 1}, opts); part != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("truncated budget published a Cut component", err)
	}
	var recipe parabolaCircleCutRecipe
	if err := json.Unmarshal(encoded.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger = recipe.Ledger[:1]
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if part, err := ReadParabolaCircleGraphCutJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); part != nil || !errors.Is(err, geom.ErrConvergence) {
		t.Fatal("omitted disk published the wall component", err)
	}
}
