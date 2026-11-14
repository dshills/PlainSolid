package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func mixedWallFixture(t *testing.T) (Solid, Solid, []RationalIsoShellPairProposal, MixedAlgebraicShellSectionOptions) {
	a, _ := crossingFootprintRoofShell(t, false)
	b, _ := crossingFootprintRoofShell(t, true)
	o := AlgebraicShellSectionOptions{}
	o.Trace.UniformOverlapGraphFallback = intersect.UniformGraphDiscoveryOptions{GridDivisions: 16, AxisDivisions: [2]int{8, 16}}
	o.Trace.UniformOverlapGraphFallback.Enabled = true
	o.Trace.UniformOverlapGraphFallback.Plane.Arcs.ResolveSubdivisionCorners = true
	o.Trace.UniformOverlapGraphFallback.MaxArcs = 1024
	o.Trace.UniformOverlapGraphFallback.MaxTubes = 1024
	o.Trace.Coverage.Isolation.ParameterTolerance = .125
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	o.Trace.Branches.PruneSeparatedEndpoints = true
	o.Trace.Branches.MaxComparisons = 4096
	o.Trace.MaxTopologyPairs = 1000000
	o.Trace.Coverage.MaxCandidateTests = 4096
	o.Trace.Coverage.IsoPlaneFallback = true
	o.Trace.Coverage.IsoPlane.AllowTrapezoidCharts = true
	o.Trace.PlaneFallback.Enabled = true
	o.Trace.PlaneFallback.TrapezoidCharts = true
	o.Trace.Coverage.IsoPlane.MaxOperations = 64000000
	o.Trace.PlaneFallback.Plane.Arcs.ResolveSubdivisionCorners = true

	pieces := []geom.RationalIsoTubePiece{
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: 0, VMax: .5}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .375, VMax: .9375}}, FreeAxis: 1},
		{Box: geom.SurfacePairParameterBox{A: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .25, VMax: .875}, B: geom.ParameterRectangle{UMin: .9375, UMax: 1.0625, VMin: .75, VMax: 1}}, FreeAxis: 3},
	}
	cuts := []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(11, 15)}
	return a, b, []RationalIsoShellPairProposal{{Faces: [2]int{5, 3}, SourceOperand: 1, FixedAxis: 0, Fixed: 1, Cuts: cuts, Pieces: pieces}}, MixedAlgebraicShellSectionOptions{Section: o}
}
func TestMixedOriginalShellWires(t *testing.T) {
	a, b, p, o := mixedWallFixture(t)
	r, err := SectionSplineShellsMixedAlgebraic(context.Background(), a, b, p, o)
	if err != nil {
		t.Fatal(err)
	}
	branches, wires := r.Branches(), r.Wires()
	t.Log("pairs", len(r.Pairs()), "branches", len(branches), "wires", len(wires))
	if len(branches) != 12 || len(wires) != 2 {
		t.Fatal("incomplete original section")
	}
	seen := make([]bool, len(branches))
	restricted := 0
	for _, br := range branches {
		if br.Restricted != nil {
			restricted++
			if br.Faces != [2]int{5, 3} || br.RestrictedSource != 1 || len(br.Segments) != 0 {
				t.Fatal("lost restricted owner")
			}
			d, _ := br.Restricted.Piece(1)
			if d.Free.Lower.Cmp(big.NewRat(109, 130)) != 0 {
				t.Fatal("rounded domain")
			}
		}
	}
	if restricted != 1 {
		t.Fatal("missing mixed branch")
	}
	for _, w := range wires {
		for i, u := range w {
			if seen[u.Branch] {
				t.Fatal("duplicate branch")
			}
			seen[u.Branch] = true
			v := w[(i+1)%len(w)]
			x, y := branches[u.Branch], branches[v.Branch]
			end, start := 1, 0
			if u.Reversed {
				end = 0
			}
			if v.Reversed {
				start = 1
			}
			if x.Ends[end].Operand != y.Ends[start].Operand || x.EndEdges[end] != y.EndEdges[start] {
				t.Fatal("lost original edge")
			}
			eq, _, e := x.Ends[end].Certificate.ModelEvent.CompareModelPoint(context.Background(), y.Ends[start].Certificate.ModelEvent, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !eq {
				t.Fatal("nonincident", e)
			}
		}
	}
	for _, s := range seen {
		if !s {
			t.Fatal("omitted branch")
		}
	}

	if _, err := json.Marshal(r); !errors.Is(err, geom.ErrUnrepresentable) {
		t.Fatal("erased restrictions", err)
	}
	var encoded bytes.Buffer
	if err := WriteMixedAlgebraicShellSectionJSON(&encoded, r); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadMixedAlgebraicShellSectionJSON(context.Background(), bytes.NewReader(encoded.Bytes()), JSONLimits{}, o)
	if err != nil || len(restored.Wires()) != 2 {
		t.Fatal("reload", err)
	}
	var recipe nativeMixedShellSection
	if err := json.Unmarshal(encoded.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Owners[0].Edges[0]++
	mutated, _ := json.Marshal(recipe)
	if got, err := ReadMixedAlgebraicShellSectionJSON(context.Background(), bytes.NewReader(mutated), JSONLimits{}, o); err == nil || got != nil {
		t.Fatal("forged owner accepted", err)
	}
	if err := json.Unmarshal(encoded.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Rational[0].Cuts[len(recipe.Rational[0].Cuts)-1] = "7/10"
	truncated, _ := json.Marshal(recipe)
	if got, err := ReadMixedAlgebraicShellSectionJSON(context.Background(), bytes.NewReader(truncated), JSONLimits{}, o); err == nil || got != nil {
		t.Fatal("incomplete retained interval accepted", err)
	}
	if got, err := ReadMixedAlgebraicShellSectionJSON(context.Background(), bytes.NewReader(encoded.Bytes()), JSONLimits{MaxEntities: 1}, o); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("entity budget", err)
	}
	// Exhaustion at the exact-chain stage must not publish the already found
	// regular branches as a complete or usable result.
	small := o
	small.MaxExactOperations = 1
	if got, err := SectionSplineShellsMixedAlgebraic(context.Background(), a, b, p, small); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("partial publication", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := SectionSplineShellsMixedAlgebraic(canceled, a, b, p, o); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("canceled publication", err)
	}
	p[0].Cuts[1].SetInt64(0)
	branches[0].Faces = [2]int{-1, -1}
	wires[0][0].Branch = -1
	if r.Branches()[0].Faces[0] < 0 || r.Wires()[0][0].Branch < 0 {
		t.Fatal("mutable certificate")
	}
}

func TestMixedSectionRecipeStructuralReject(t *testing.T) {
	for _, value := range []string{"1e999999999", "1/0", "01", "-0", "1/1"} {
		q := nativeMixedShellSection{Version: 1, Kind: "trimmed-mixed-algebraic-shell-section", Rational: []nativeMixedIsoPair{{Cuts: []string{"0", value}, Pieces: []geom.RationalIsoTubePiece{{}}}}}
		data, _ := json.Marshal(q)
		r, err := ReadMixedAlgebraicShellSectionJSON(context.Background(), bytes.NewReader(data), JSONLimits{}, MixedAlgebraicShellSectionOptions{})
		if r != nil || !errors.Is(err, geom.ErrInvalid) {
			t.Fatal(value, err)
		}
	}
	var out bytes.Buffer
	if err := WriteMixedAlgebraicShellSectionJSON(&out, &MixedAlgebraicShellSection{}); !errors.Is(err, geom.ErrInvalid) || out.Len() != 0 {
		t.Fatal("forged output", err)
	}
}
