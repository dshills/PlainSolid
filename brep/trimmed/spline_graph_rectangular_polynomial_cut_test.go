//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestNativeRectangularPolynomialCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := rectangularPolynomialStock(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutRectangularPolynomialComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, o)
				if e != nil {
					t.Fatal("nonunit native Cut on both original roofs", profile, axis, order, e)
				}
				checkNativeRectangularCut(t, cut, profile, axis, order)
				var raw bytes.Buffer
				if e := cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadRectangularPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(loaded.Contacts(), cut.Contacts()) {
					t.Fatal("source replay changes exact physical incidence", profile, axis, order, e)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("reload changed original base or roof coefficients", i)
					}
				}
				checkNativeRectangularCut(t, loaded, profile, axis, order)
				var replay bytes.Buffer
				if e := loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical physical rectangle Cut recipe", e)
				}
				if profile == 0 && axis == 0 && order == 0 {
					var points [2]geom.Vec3
					for i, c := range loaded.Contacts() {
						points[i], e = geom.RepresentedExactPoint(c.Point)
						if e != nil {
							t.Fatal(e)
						}
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(loaded.Components()), points[:]...)
				}
			}
		}
	}
}

func checkNativeRectangularCut(t *testing.T, cut *AlignedPolynomialCut, profile, axis, order int) {
	t.Helper()
	origins, lengths := [2]float64{2, -1}, [2]float64{2, 3}
	// Independent antiderivatives use the unit roof equations and the
	// physical XY Jacobian six. The shared sloping base cancels.
	checkNativePolynomialCutMaterialOnFootprint(t, cut, profile, axis, order, origins[axis], lengths[axis], 6)
	checkProjectedCutOwnerTrims(t, cut, axis, order, func(p geom.Vec3) geom.Vec3 {
		return geom.Vec3{X: (p.X - 2) / 2, Y: (p.Y + 1) / 3, Z: p.Z}
	})
	if order == 1 {
		volumes, _ := cut.ExactComponentVolumes()
		if volumes[1].Cmp(big.NewRat(3, 19193135104)) != 0 {
			t.Fatal("exact physical material of the thin middle lens", volumes[1])
		}
	}
}

func TestNativeRectangularPolynomialCutRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	base, a, b := rectangularPolynomialStock(t, 0, 0)
	cut, e := CutRectangularPolynomialComponents(ctx, base, b, a, 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := cut.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if c, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); c != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("physical Cut uses its own source recipe kind", e)
	}
	for _, tc := range []struct {
		name   string
		change func(*polynomialMixedCutRecipe)
	}{
		{"unit contact station", func(r *polynomialMixedCutRecipe) { r.Contacts[0].Point[0] = "1/4" }},
		{"lost lens", func(r *polynomialMixedCutRecipe) { r.Regions = []int{1, 3} }},
		{"wrong component incidence", func(r *polynomialMixedCutRecipe) { r.Contacts[0].Components[1] = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recipe polynomialMixedCutRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			tc.change(&recipe)
			bad, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if r, e := ReadRectangularPolynomialCutJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
				t.Fatal("source reconstruction must rebind the physical component ledger", e)
			}
		})
	}
	rebuild := func(s *geom.BSplineSurface, change func(*geom.Vec3)) *geom.BSplineSurface {
		d := s.Data()
		for i := range d.Poles {
			change(&d.Poles[i])
		}
		out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	skew := rebuild(base, func(p *geom.Vec3) { p.X += p.Y / 8 })
	if r, e := CutRectangularPolynomialComponents(ctx, skew, b, a, 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("native Cut requires the proved axis-aligned footprint", e)
	}
	raised := rebuild(base, func(p *geom.Vec3) { p.Z += 4 })
	if r, e := CutRectangularPolynomialComponents(ctx, raised, b, a, 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("original sloping base must be strictly below both roofs", e)
	}
	shifted := rebuild(b, func(p *geom.Vec3) { p.X += .25 })
	if r, e := CutRectangularPolynomialComponents(ctx, base, shifted, a, 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("both original footprints must match", e)
	}
	base, a, b = rectangularPolynomialStock(t, 0, 2)
	if r, e := CutRectangularPolynomialComponents(ctx, base, b, a, 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native components must not round a nonbinary physical junction", e)
	}
	if r, e := ReadRectangularPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("aggregate topology limit", e)
	}
	o.Proof.MaxOperations = 1
	if r, e := ReadRectangularPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reload must prove the original owners and exact volumes", e)
	}
}
