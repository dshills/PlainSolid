package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func twoPolynomialNodeCutVolumes(order int, wide bool) []float64 {
	// Independently integrate the positive height gap in each exact slab.
	// Outside both roots, with distances l and h to the two graphs at the
	// chart boundary (l <= h), its primitive is l²h/2 - l³/6. Inside the
	// strip it is |g-f|³/6. Scale both by 105/128 before free-axis integration.
	if wide {
		if order == 0 {
			return []float64{2043652979.0 / 167503724544, 12543367027.0 / 167503724544}
		}
		return []float64{688490045.0 / 117922622078976, 3536369.0 / 1842540969984, 4818269699.0 / 117922622078976}
	}
	if order == 0 {
		return []float64{187084323688387.0 / 15094095626108928, 378006913287489.0 / 5031365208702976}
	}
	return []float64{370377155.0 / 15094095626108928, 2625475.0 / 15094095626108928, 230417735.0 / 29480655519744}
}

func TestNativeTwoPolynomialNodeCutComponents(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, wide := range []bool{false, true} {
		for _, axis := range []int{0, 1} {
			base, a, b := nativeTwoPolynomialNodeFixture(t, axis, 4)
			if wide {
				base, a, b = nativeWideTwoPolynomialNodeFixture(t, axis)
			}
			for order, owners := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutSplineGraphTwoPolynomialNodeComponents(ctx, base, owners[0], owners[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(wide, axis, order, e)
				}
				checkTwoPolynomialNodeCutGraph(t, cut, axis, order, wide)
				for i, part := range cut.Components() {
					if part.faces[0].Surface != owners[0] || part.faces[1].Surface != owners[1] {
						t.Fatal("missing original Cut roof provenance", i)
					}
					mass, e := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					want := twoPolynomialNodeCutVolumes(order, wide)[i]
					if e != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want) > 1e-10 {
						t.Fatal("positive component volume", wide, axis, order, i, mass.Volume, want, e)
					}
				}
				var native bytes.Buffer
				if e = cut.WriteJSON(ctx, &native); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadTwoPolynomialNodeCutJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(e)
				}
				checkTwoPolynomialNodeCutGraph(t, loaded, axis, order, wide)
				var rewritten bytes.Buffer
				if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(rewritten.Bytes(), native.Bytes()) {
					t.Fatal("native canonical two-contact recipe", e)
				}
				if wide && axis == 0 && order == 0 {
					parts := cut.Components()
					var points [2]geom.Vec3
					for i, contact := range cut.Contacts() {
						points[i], e = geom.RepresentedExactPoint(contact.Point)
						if e != nil {
							t.Fatal(e)
						}
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(parts), points[:]...)
				}
			}
		}
	}
}

func TestNativeTwoPolynomialNodeCutOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_TWO_POLYNOMIAL_NODE_CUT")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node-pair-cut").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Axis, Order, Operation, Solids int
		Valid                          bool
		Volume                         float64
		PartVolumes                    []float64
		FreeSlab                       *int
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 10 {
		t.Fatal(string(data), e)
	}
	var whole [2][2]bool
	var slabs [2][3]bool
	for _, r := range rows {
		if r.Axis < 0 || r.Axis > 1 || r.Order < 0 || r.Order > 1 || r.Operation != 2 || !r.Valid || r.Solids < 1 || len(r.PartVolumes) != r.Solids {
			t.Fatal(r)
		}
		want := twoPolynomialNodeCutVolumes(r.Order, true)
		if r.FreeSlab != nil {
			i := *r.FreeSlab
			if r.Order != 1 || i < 0 || i > 2 || slabs[r.Axis][i] || r.Solids != 1 || math.Abs(r.Volume-want[i]) > 1e-9 || math.Abs(r.PartVolumes[0]-want[i]) > 1e-9 {
				t.Fatal("independent original free-slab Cut", r, want)
			}
			slabs[r.Axis][i] = true
			continue
		}
		if whole[r.Axis][r.Order] {
			t.Fatal("duplicate full-stock oracle row", r)
		}
		whole[r.Axis][r.Order] = true
		total := 0.0
		for _, v := range want {
			total += v
		}
		if math.Abs(r.Volume-total) > 1e-9 {
			t.Fatal("full-stock Cut volume", r, total)
		}
		if r.Order == 0 {
			if r.Solids != 2 {
				t.Fatal("independent exterior components", r)
			}
			slices.Sort(r.PartVolumes)
			for i, v := range r.PartVolumes {
				if math.Abs(v-want[i]) > 1e-9 {
					t.Fatal("exterior component mass", r, want)
				}
			}
		}
	}
}

func checkTwoPolynomialNodeCutGraph(t *testing.T, cut *TwoPolynomialNodeCut, axis, order int, wide bool) {
	t.Helper()
	regions := []int{0, 4}
	if order == 1 {
		regions = []int{1, 2, 3}
	}
	parts := cut.Components()
	if !slices.Equal(cut.Regions(), regions) || len(parts) != len(regions) {
		t.Fatal("wrong positive sector material", cut.Regions(), len(parts))
	}
	for i, p := range parts {
		topology := p.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || p.faces[0].Reversed || !p.faces[1].Reversed {
			t.Fatal("nonembedded or inward layer", i, topology)
		}
		if regions[i] == 2 && (len(p.faces) != 2 || len(p.faces[0].Outer) != 4 || len(p.edges) != 4) {
			t.Fatal("middle lens has four exact sides and two roof caps", topology)
		}
	}
	wantStations := [2]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2)}
	if wide {
		wantStations[1] = big.NewRat(3, 4)
	}
	contacts := cut.Contacts()
	for i, c := range contacts {
		wantComponents := [2]int{0, 1}
		if order == 1 {
			wantComponents = [2]int{i, i + 1}
		}
		if c.Components != wantComponents || c.Parameters[axis].Cmp(wantStations[i]) != 0 {
			t.Fatal("contact graph lost original incidence", i, c)
		}
		p, e := geom.RepresentedExactPoint(c.Point)
		if e != nil {
			t.Fatal(e)
		}
		for j, component := range c.Components {
			part, v := parts[component], c.Vertices[j]
			degree := 0
			for _, e := range part.edges {
				if e.Start == v || e.End == v {
					degree++
				}
			}
			if part.vertices[v] != p || degree != 2 {
				t.Fatal("expected two original branch incidences", i, component, v)
			}
		}
	}
	if order == 0 && (contacts[0].Vertices[0] == contacts[1].Vertices[0] || contacts[0].Vertices[1] == contacts[1].Vertices[1]) {
		t.Fatal("both shared positions must keep distinct local vertex identities")
	}
	if order == 1 && contacts[0].Vertices[1] == contacts[1].Vertices[0] {
		t.Fatal("middle lens must preserve both original contacts separately")
	}
	contacts[0].Point[0].SetInt64(999)
	if cut.Contacts()[0].Point[0].Cmp(contacts[0].Point[0]) == 0 {
		t.Fatal("caller changed the stored proof")
	}
}

func TestNativeTwoPolynomialNodeCutRecipeRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeWideTwoPolynomialNodeFixture(t, 0)
	cut, e := CutSplineGraphTwoPolynomialNodeComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = cut.WriteJSON(ctx, &native); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*polynomialNodeComponentsRecipe){
		"second point":    func(r *polynomialNodeComponentsRecipe) { r.Contacts[1].Point[0] = "1/2" },
		"removed contact": func(r *polynomialNodeComponentsRecipe) { r.Contacts = r.Contacts[:1] },
		"wrong pair":      func(r *polynomialNodeComponentsRecipe) { r.Contacts[1].Components[0] = 0 },
		"wrong vertex":    func(r *polynomialNodeComponentsRecipe) { r.Contacts[0].Vertices[1]++ },
		"wrong sector":    func(r *polynomialNodeComponentsRecipe) { r.Regions[1] = 4 },
		"exchanged roofs": func(r *polynomialNodeComponentsRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"version":         func(r *polynomialNodeComponentsRecipe) { r.Version++ },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialNodeComponentsRecipe
			if e := json.Unmarshal(native.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			mutate(&recipe)
			bytesOut, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if r, e := ReadTwoPolynomialNodeCutJSON(ctx, bytes.NewReader(bytesOut), JSONLimits{}, PolynomialNodeCutOptions{}); r != nil || e == nil {
				t.Fatal("changed two-contact provenance was accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadTwoPolynomialNodeCutJSON(ctx, bytes.NewReader(native.Bytes()), limits, PolynomialNodeCutOptions{}); out != nil || e == nil {
			t.Fatal("reader ignored bounds", limits, e)
		}
	}
	if r, e := ReadTwoPolynomialNodeCutJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reader ignored proof work limit", e)
	}
	base, a, b = nativeTwoPolynomialNodeFixture(t, 0, 3)
	if r, e := CutSplineGraphTwoPolynomialNodeComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonbinary contact accepted", e)
	}
}
