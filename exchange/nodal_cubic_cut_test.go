package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestNodalCubicCutExchange(t *testing.T) {
	ctx := context.Background()
	receipt, err := os.ReadFile("testdata/native_nodal_cubic_cut_interior.json")
	if err != nil {
		t.Fatal(err)
	}
	cut, err := trimmed.ReadNodalCubicCutJSON(ctx, bytes.NewReader(receipt), trimmed.JSONLimits{}, trimmed.NodalCubicCutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const name, lengthUnit = "Nodal Cut", .001
	so := step.NodalCubicSTEPOptions{PoleTolerance: 1e-14}
	igo := iges.NodalCubicIGESOptions{PoleTolerance: 1e-14}
	var st, ig bytes.Buffer
	if err := step.WriteNodalCubicCut(ctx, &st, cut, name, lengthUnit, step.Limits{}, so); err != nil {
		t.Fatal(err)
	}
	if err := iges.WriteNodalCubicCut(ctx, &ig, cut, lengthUnit, iges.Limits{}, igo); err != nil {
		t.Fatal(err)
	}
	if err := step.VerifyNodalCubicCutSTEP(ctx, bytes.NewReader(st.Bytes()), cut, name, lengthUnit, step.Limits{}, so); err != nil {
		t.Fatal(err)
	}
	if err := iges.VerifyNodalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, lengthUnit, iges.Limits{}, igo); err != nil {
		t.Fatal(err)
	}
	node := cut.Contact()
	checkVariableCutSTEPContact(t, st.Bytes(), trimmed.PolynomialMixedCutContact{TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{Point: node.Point}})
	tampered := bytes.Replace(st.Bytes(), []byte(".CURVE_3D."), []byte(".PCURVE_S2."), 1)
	if bytes.Equal(st.Bytes(), tampered) || !errors.Is(step.VerifyNodalCubicCutSTEP(ctx, bytes.NewReader(tampered), cut, name, lengthUnit, step.Limits{}, so), geom.ErrConvergence) {
		t.Fatal("changed original curve master accepted")
	}
	if err := iges.VerifyNodalCubicCutIGES(ctx, bytes.NewReader(ig.Bytes()), cut, 1, iges.Limits{}, igo); !errors.Is(err, geom.ErrConvergence) {
		t.Fatalf("changed IGES physical units accepted: %v", err)
	}
	check := func(format, reader string, data []byte) {
		t.Helper()
		if reader == "" {
			return
		}
		out := readPinnedExactComponents(t, ctx, format, data, reader)
		if out.Solids != 2 || len(out.Components) != 2 || out.IndexedVertices != 9 {
			t.Fatalf("independent %s changed the two components or original vertex occurrences", format)
		}
		var pos [3]float64
		for i, p := range node.Point {
			pos[i], _ = p.Float64()
		}
		expected := [2]float64{1.0 / 100925440, 4619.0 / 100925440}
		for i, component := range out.Components {
			if component.Shells != 1 || component.Faces != [2]int{2, 4}[i] || component.Edges != [2]int{4, 7}[i] ||
				component.Volume <= 0 || math.Abs(component.Volume-expected[i]) > 5e-8*expected[i] {
				t.Fatalf("independent %s solid %d has signed volume %.16g, want %.16g", format, i, component.Volume, expected[i])
			}
			vertex := -1
			for j, p := range component.Vertices {
				if p == pos {
					if vertex >= 0 {
						t.Fatalf("independent %s duplicated the contact within solid %d", format, i)
					}
					vertex = j
				}
			}
			rays := 0
			for _, e := range component.EdgeVertices {
				if e[0] == vertex || e[1] == vertex {
					rays++
				}
			}
			if vertex < 0 || rays != 2 {
				t.Fatalf("independent %s lost the original contact or its two rays of solid %d", format, i)
			}
		}
	}
	check("step", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_STEP_COMPONENTS"), st.Bytes())
	check("igs", os.Getenv("PLAINSOLID_OCCT_CLIPPED_CUT_IGES"), ig.Bytes())
}
