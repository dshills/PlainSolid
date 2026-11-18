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
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeParabolaCircleGraphEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		src := parabolaCircleGraphStock(t, axis, big.NewRat(3, 4), big.NewRat(1, 64), false)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					s, h := r.Boundary(), r.History()
					if s.Topology().InnerWires != 1 || len(s.faces[h.RoofFaces[1]].Holes) != 1 {
						t.Fatal("circle did not become an interior trim", h)
					}
					for _, face := range h.WallFaces {
						wall := s.faces[face]
						base, hasBase := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
						roof, hasRoof := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
						if !hasBase || !hasRoof || base.surface != src[0] || (roof.surface != src[1] && roof.surface != src[2]) {
							t.Fatal("wall lost original roof or base edge owner", face)
						}
					}
					mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
					want := 54715159.0/27525120 - math.Pi*4317/134217728
					if op == parametric.Union {
						want = 55418249.0/27525120 + math.Pi*4317/134217728
					}
					if err != nil || !(math.Abs(mass.Volume-want) <= 1e-8) {
						t.Fatal("mixed native envelope disagrees with independent signed integrals", mass, want, err)
					}
					t.Logf("volume %.16g area %.16g", mass.Volume, mass.Area)
					checkParabolaCircleGraphReplay(t, ctx, r)
				})
			}
		}
	}
}

func checkParabolaCircleGraphReplay(t *testing.T, ctx context.Context, r *ParabolaCircleGraphEnvelope) {
	t.Helper()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil || reloaded.History() != r.History() {
		t.Fatal("source recipe replay changed mixed boundary owners", err)
	}
	for i, src := range r.sources {
		if !geom.SameSplineSurface(src, reloaded.sources[i]) {
			t.Fatal("source recipe replay changed an original spline", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("source recipe replay changed canonical bytes", err)
	}
}

func TestParabolaCircleGraphReplayRefusals(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), false)
	r, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], 0, parametric.Intersection, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("short entity budget published a shell", err)
	}
	var recipe parabolaCircleEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.CircleEdges[2] = *recipe.Ledger.ParabolaEdge
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("forged circle ledger published a shell", err)
	}
	changed := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), true)
	recipe.Ledger = parabolaCircleRecipeLedger(r.History())
	recipe.Sources, err = encodePolynomialGraphSupports(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	forged, err = json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("changed original roof published the earlier shell", err)
	}
	irrational := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 32), false)
	if got, err := CombineParabolaCircleSplineGraphs(ctx, irrational[0], irrational[1], irrational[2], 0, parametric.Intersection, 1e-9, opts); !errors.Is(err, geom.ErrUnrepresentable) || got != nil {
		t.Fatal("irrational circle produced a rounded native vertex", err)
	}
}
