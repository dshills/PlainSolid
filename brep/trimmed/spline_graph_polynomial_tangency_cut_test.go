//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func polynomialTangencyCutVolumes(order int) [2]float64 {
	// Outside the two roots, the original dependent height integral is
	// 105/128 times l²h/2-l³/6, where l and h are the nearer and farther
	// distances to the rectangle side. Inside the roots it is 105(g-f)^3/768.
	// Integrate the original factors in the two free slabs divided at 1/2.
	if order == 0 {
		return [2]float64{1634063.0 / 138412032, 4414762109.0 / 57579405312}
	}
	return [2]float64{186959.0 / 4913442586624, 1230001.0 / 4913442586624}
}

func TestNativePolynomialTangencyCutComponents(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativePolynomialTangencyFixture(t, axis, 2, 1)
		for order, owners := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			cut, e := CutSplineGraphPolynomialTangencyComponents(ctx, base, owners[0], owners[1], axis, 1e-9, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(axis, order, e)
			}
			checkNativePolynomialTangencyCut(t, cut, axis, order)
			want := polynomialTangencyCutVolumes(order)
			for i, part := range cut.Components() {
				mass, e := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want[i]) > 1e-10 {
					t.Fatal("analytical positive component mass", axis, order, i, mass.Volume, want[i], e)
				}
			}
			var raw bytes.Buffer
			if e = cut.WriteJSON(ctx, &raw); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadPolynomialTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(e)
			}
			checkNativePolynomialTangencyCut(t, loaded, axis, order)
			if loaded.pointCut.contact.Vertices != cut.pointCut.contact.Vertices || !sameRationalVertex(loaded.pointCut.contact.Point, cut.pointCut.contact.Point) {
				t.Fatal("source ledger changed across reload", loaded.Contact(), cut.Contact())
			}
			for i, source := range cut.pointCut.sources {
				if !geom.SameSplineSurface(source, loaded.pointCut.sources[i]) {
					t.Fatal("original roof or base changed on reload", i)
				}
			}
			var replay bytes.Buffer
			if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(replay.Bytes(), raw.Bytes()) {
				t.Fatal("tangent Cut proof is not canonical", e)
			}
			if axis == 0 && order == 0 {
				p, e := geom.RepresentedExactPoint(cut.Contact().Point)
				if e != nil {
					t.Fatal(e)
				}
				checkPolynomialNodeCutStrictCollision(t, cut.Components(), p)
			}
		}
	}
}

func checkNativePolynomialTangencyCut(t *testing.T, cut *PolynomialTangencyCut, axis, order int) {
	t.Helper()
	contact := cut.Contact()
	wantSectors := [2]int{0, 3}
	if order == 1 {
		wantSectors = [2]int{1, 2}
	}
	if contact.Sectors != wantSectors || contact.Parameters[axis].Cmp(big.NewRat(1, 2)) != 0 || contact.Parameters[1-axis].Cmp(big.NewRat(23, 64)) != 0 {
		t.Fatal("original tangent material selection or point", contact)
	}
	center, e := geom.RepresentedExactPoint(contact.Point)
	if e != nil {
		t.Fatal(e)
	}
	for i, part := range cut.Components() {
		topology := part.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
			part.faces[0].Reversed || !part.faces[1].Reversed || part.faces[0].Surface != cut.pointCut.sources[1] || part.faces[1].Surface != cut.pointCut.sources[2] {
			t.Fatal("embedded outward layer with original support ownership", topology)
		}
		vertex := contact.Vertices[i]
		if part.vertices[vertex] != center {
			t.Fatal("local tangent vertex missing", vertex, center)
		}
		count := 0
		for id, edge := range part.edges {
			if edge.Start != vertex && edge.End != vertex {
				continue
			}
			m, ok := edge.Curve.(surfaceTrimCurve)
			if !ok || m.surface != cut.pointCut.sources[1] {
				t.Fatal("unowned tangent cut model edge", id)
			}
			count++
			for _, face := range part.faces[:2] {
				found := false
				for _, use := range face.Outer {
					if use.Edge == id && matchesTangencyTrim(use.Curve, m.curve) {
						found = true
					}
				}
				if !found {
					t.Fatal("missing exact original tangent coedge", id)
				}
			}
		}
		if count != 2 || order == 1 && len(part.faces) != 4 {
			t.Fatal("tangent link degree or exact midpoint wall split", count, topology)
		}
	}
	contact.Point[0].SetInt64(999)
	if contact.Point[0].Cmp(cut.Contact().Point[0]) == 0 {
		t.Fatal("published contact can mutate the source proof")
	}
}

func TestNativePolynomialTangencyCutRejectsChangedRecipe(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativePolynomialTangencyFixture(t, 0, 2, 1)
	cut, e := CutSplineGraphPolynomialTangencyComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = cut.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for name, modify := range map[string]func(*polynomialNodeCutRecipe){
		"wrong contact kind": func(r *polynomialNodeCutRecipe) { r.Kind = nativePolynomialNodeCut },
		"wrong vertex":       func(r *polynomialNodeCutRecipe) { r.Vertices[0]++ },
		"wrong point":        func(r *polynomialNodeCutRecipe) { r.Point[2] = "1/1" },
		"wrong parameter":    func(r *polynomialNodeCutRecipe) { r.Parameters[0] = "1/4" },
		"wrong sector":       func(r *polynomialNodeCutRecipe) { r.Sectors[1] = 2 },
		"swapped roofs":      func(r *polynomialNodeCutRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"coincident roofs":   func(r *polynomialNodeCutRecipe) { r.Sources[1] = r.Sources[2] },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialNodeCutRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			modify(&recipe)
			changed, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadPolynomialTangencyCutJSON(ctx, bytes.NewReader(changed), JSONLimits{}, PolynomialNodeCutOptions{}); q != nil || e == nil {
				t.Fatal("unbound native contact", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 4}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if q, e := ReadPolynomialTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialNodeCutOptions{}); q != nil || e == nil {
			t.Fatal("native guard ignored", limits, e)
		}
	}
	if q, e := ReadPolynomialTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}); q != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native work guard ignored", e)
	}
	base, a, b = nativePolynomialTangencyFixture(t, 0, 3, 1)
	if q, e := CutSplineGraphPolynomialTangencyComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); q != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("unrepresentable tangent vertex was accepted", e)
	}
}
