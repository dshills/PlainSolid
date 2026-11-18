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
	"sort"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativePolynomialNodeCutComponents(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativePolynomialNodeFixture(t, axis, 2)
		for order, sources := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			cut, e := CutSplineGraphPolynomialNodeComponents(ctx, base, sources[0], sources[1], axis, 1e-9, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(axis, order, e)
			}
			checkPolynomialNodeCut(t, cut, axis, order)
			for _, part := range cut.Components() {
				if part.faces[0].Surface != sources[0] || part.faces[1].Surface != sources[1] {
					t.Fatal("original Cut roof owners replaced")
				}
			}
			var native bytes.Buffer
			if e = cut.WriteJSON(ctx, &native); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadPolynomialNodeCutJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(e)
			}
			checkPolynomialNodeCut(t, loaded, axis, order)
			var replay bytes.Buffer
			if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(replay.Bytes(), native.Bytes()) {
				t.Fatal("noncanonical component recipe", e)
			}
			if axis == 0 && order == 0 {
				center, e := geom.RepresentedExactPoint(cut.Contact().Point)
				if e != nil {
					t.Fatal(e)
				}
				checkPolynomialNodeCutStrictCollision(t, cut.Components(), center)
			}
		}
	}
}

func checkPolynomialNodeCut(t *testing.T, cut *PolynomialNodeCut, axis, order int) {
	t.Helper()
	contact := cut.Contact()
	if contact.Parameters[axis].Cmp(big.NewRat(1, 2)) != 0 || contact.Parameters[1-axis].Cmp(big.NewRat(23, 64)) != 0 ||
		contact.Sectors[1]-contact.Sectors[0] != 2 {
		t.Fatal("wrong contact ledger", contact)
	}
	center, e := geom.RepresentedExactPoint(contact.Point)
	if e != nil {
		t.Fatal(e)
	}
	contact.Point[0].SetInt64(999)
	restored, e := geom.RepresentedExactPoint(cut.Contact().Point)
	if e != nil || restored != center {
		t.Fatal("mutable contact certificate", e)
	}
	want := polynomialNodeCutVolumes(axis, order)
	for i, part := range cut.Components() {
		if part.vertices[contact.Vertices[i]] != center || part.faces[0].Reversed || !part.faces[1].Reversed {
			t.Fatal("wrong embedded component vertex or orientation", i)
		}
		if topology := part.Topology(); len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 {
			t.Fatal("closed positive component topology", topology)
		}
		if order == 1 && len(part.faces) != 4 {
			t.Fatal("single narrow side must split into two exact native walls", len(part.faces))
		}
		mass, e := part.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want[i]) > 1e-10 {
			t.Fatal("positive component volume", axis, order, i, mass.Volume, want[i], e)
		}
	}
}

func polynomialNodeCutVolumes(axis, order int) [2]float64 {
	// Independently integrate each vertical gap on either side of u=1/2.
	// Outside the branches, the integral uses the squared nearer-root distance
	// times half the farther-root distance minus a sixth of its cube. In the
	// wedges, it is the cubed branch separation divided by six.
	want := [2][2]float64{
		{3073189611395.0 / 39307540692992, 441569933187.0 / 39307540692992},
		{14088067.0 / 39307540692992, 2590595.0 / 39307540692992},
	}[order]
	if axis == 1 {
		want[0], want[1] = want[1], want[0]
	}
	return want
}

func checkPolynomialNodeCutStrictCollision(t *testing.T, parts [2]Solid, centers ...geom.Vec3) {
	t.Helper()
	ctx := context.Background()
	compound, e := AssembleShells(ctx, []PlacedSolid{{parts[0], geom.Identity()}, {parts[1], geom.Identity()}}, 16)
	if e != nil {
		t.Fatal(e)
	}
	m, e := compound.Tessellate(ctx, parametric.TessellationOptions{Deflection: 0.02})
	if e != nil {
		t.Fatal(e)
	}
	r, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !r.Complete || len(r.Contacts) == 0 {
		t.Fatal("global collision silently accepted nonincident contact", r.Complete, e)
	}
	for _, center := range centers {
		found := false
		for _, contact := range r.Contacts {
			if contact.Kind == collision.Touching && len(contact.Points) == 1 && contact.Points[0] == center {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("global collision did not report the isolated node", center)
		}
	}
}

func TestNativePolynomialNodeCutOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_POLYNOMIAL_NODE_CUT")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
	}
	data, e := exec.Command(exe, "node-cut").Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		PartVolumes              []float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 2 {
		t.Fatal(string(data), e)
	}
	var seen [2]bool
	for _, row := range rows {
		if row.Order < 0 || row.Order > 1 || seen[row.Order] || row.Operation != 2 || !row.Valid || row.Solids != 2 || len(row.PartVolumes) != 2 {
			t.Fatal(row)
		}
		seen[row.Order] = true
		want := polynomialNodeCutVolumes(0, row.Order)
		sort.Float64s(want[:])
		sort.Float64s(row.PartVolumes)
		for i, v := range row.PartVolumes {
			if v <= 0 || math.Abs(v-want[i]) > 1e-9 {
				t.Fatal(row, want)
			}
		}
	}
}

func TestNativePolynomialNodeCutRejectsChangedRecipe(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialNodeFixture(t, 0, 2)
	cut, e := CutSplineGraphPolynomialNodeComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = cut.WriteJSON(ctx, &data); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*polynomialNodeCutRecipe){
		"coordinate":     func(r *polynomialNodeCutRecipe) { r.Point[0] = "3/8" },
		"parameter":      func(r *polynomialNodeCutRecipe) { r.Parameters[0] = "3/8" },
		"local vertex":   func(r *polynomialNodeCutRecipe) { r.Vertices[1]++ },
		"sector":         func(r *polynomialNodeCutRecipe) { r.Sectors[0]++ },
		"removed axis":   func(r *polynomialNodeCutRecipe) { r.Axis = nil },
		"version":        func(r *polynomialNodeCutRecipe) { r.Version++ },
		"exchanged tops": func(r *polynomialNodeCutRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"identical tops": func(r *polynomialNodeCutRecipe) { r.Sources[1] = r.Sources[2] },
	} {
		t.Run(name, func(t *testing.T) {
			var r polynomialNodeCutRecipe
			if e := json.Unmarshal(data.Bytes(), &r); e != nil {
				t.Fatal(e)
			}
			change(&r)
			raw, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadPolynomialNodeCutJSON(ctx, bytes.NewReader(raw), JSONLimits{}, PolynomialNodeCutOptions{}); e == nil || out != nil {
				t.Fatal("altered contact recipe accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 16}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadPolynomialNodeCutJSON(ctx, bytes.NewReader(data.Bytes()), limits, PolynomialNodeCutOptions{}); e == nil || out != nil {
			t.Fatal("recipe limit ignored", limits, e)
		}
	}
	o := PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadPolynomialNodeCutJSON(ctx, bytes.NewReader(data.Bytes()), JSONLimits{}, o); !errors.Is(e, geom.ErrConvergence) || out != nil {
		t.Fatal("proof work limit ignored", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := ReadPolynomialNodeCutJSON(canceled, bytes.NewReader(data.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{}); !errors.Is(e, context.Canceled) || out != nil {
		t.Fatal("cancellation ignored", e)
	}
	base, a, b = nativePolynomialNodeFixture(t, 0, 3)
	if out, e := CutSplineGraphPolynomialNodeComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); !errors.Is(e, geom.ErrUnrepresentable) || out != nil {
		t.Fatal("unrepresentable component contact accepted", e)
	}
}
