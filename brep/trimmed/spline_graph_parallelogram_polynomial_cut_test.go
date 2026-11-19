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

func TestNativeParallelogramPolynomialCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := obliquePolynomialStock(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutParallelogramPolynomialComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, o)
				if e != nil {
					t.Fatal("native oblique Cut on independently parameterized original roofs", profile, axis, order, e)
				}
				checkNativeParallelogramCut(t, cut, profile, axis, order)
				var raw bytes.Buffer
				if e := cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadParallelogramPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(loaded.Contacts(), cut.Contacts()) {
					t.Fatal("absolute model and logical component contact ledger survives reload", profile, axis, order, e)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("original sloping base or roof coefficients changed", i)
					}
				}
				checkNativeParallelogramCut(t, loaded, profile, axis, order)
				var echo bytes.Buffer
				if e := loaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical oblique Cut recipe", e)
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

func checkNativeParallelogramCut(t *testing.T, cut *AlignedPolynomialCut, profile, axis, order int) {
	t.Helper()
	// The independent unit height antiderivatives scale by physical
	// det([[2,1],[1,3]])=5. The common sloping base cancels.
	checkNativePolynomialCutMaterialInChart(t, cut, profile, axis, order, 5, func(p geom.Vec3) float64 {
		q := obliqueFixtureUnitPoint(p)
		return ([2]float64{q.X, q.Y})[axis]
	})
	checkProjectedCutOwnerTrims(t, cut, axis, order, obliqueFixtureUnitPoint)
	for _, part := range cut.Components() {
		for _, face := range part.faces[2:] {
			wall, ok := face.Surface.(splineGraphAffineWall)
			if !ok {
				t.Fatal("wall must retain its represented physical direction")
			}
			u := face.Outer[0]
			edge := part.edges[u.Edge]
			a, b := part.vertices[edge.Start], part.vertices[edge.End]
			lo, ok := graphWallSourceTrim(u.Curve, false, true)
			if !ok || lo.surface != cut.sources[2] || lo.length != 1 || wall.origin != (geom.Vec3{X: a.X, Y: a.Y}) ||
				wall.along != (geom.Vec3{X: b.X - a.X, Y: b.Y - a.Y}) {
				t.Fatal("exact physical wall chart and original lower owner", u.Edge)
			}
		}
	}
	if order == 1 {
		v, _ := cut.ExactComponentVolumes()
		if v[1].Cmp(big.NewRat(5, 38386270208)) != 0 {
			t.Fatal("positive exact physical material of the middle lens", v[1])
		}
	}
}

func TestNativeParallelogramPolynomialCutUnrepresentableReference(t *testing.T) {
	ctx := context.Background()
	one := big.NewRat(1, 1)
	net := [][4]*big.Rat{
		{big.NewRat(1, 3), new(big.Rat), new(big.Rat), one},
		{big.NewRat(4, 3), big.NewRat(3, 1), new(big.Rat), one},
		{big.NewRat(7, 3), one, new(big.Rat), one},
		{big.NewRat(10, 3), big.NewRat(4, 1), new(big.Rat), one},
	}
	ref, e := geom.NewExactHomogeneousBezierSurface(ctx, 1, 1, net, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := newGraphEnvelopeReference(ctx, ref, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native footprint requires lossless physical corners", e)
	}
}

func TestNativeParallelogramPolynomialCutRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	base, a, b := obliquePolynomialStock(t, 0, 0)
	cut, e := CutParallelogramPolynomialComponents(ctx, base, b, a, 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := cut.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if r, e := ReadRectangularPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("axis-aligned Cut remains a separate native recipe", e)
	}
	transform := func(s *geom.BSplineSurface, x, z float64) *geom.BSplineSurface {
		d := s.Data()
		for i := range d.Poles {
			d.Poles[i].X += x
			d.Poles[i].Z += z
		}
		p, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	changed := func(sources [3]*geom.BSplineSurface) []json.RawMessage {
		r, e := encodePolynomialGraphSupports(ctx, sources)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	for _, tc := range []struct {
		name   string
		mutate func(*polynomialMixedCutRecipe)
		err    error
	}{
		{"translated original supports", func(r *polynomialMixedCutRecipe) {
			r.Sources = changed([3]*geom.BSplineSurface{transform(base, .25, 0), transform(b, .25, 0), transform(a, .25, 0)})
		}, geom.ErrConvergence},
		{"one shifted roof", func(r *polynomialMixedCutRecipe) {
			r.Sources = changed([3]*geom.BSplineSurface{base, transform(b, .25, 0), a})
		}, geom.ErrDomain},
		{"raised sloping base", func(r *polynomialMixedCutRecipe) {
			r.Sources = changed([3]*geom.BSplineSurface{transform(base, 0, 4), b, a})
		}, geom.ErrConvergence},
		{"missing middle shell", func(r *polynomialMixedCutRecipe) { r.Regions = []int{1, 3} }, geom.ErrConvergence},
		{"changed contact incidence", func(r *polynomialMixedCutRecipe) { r.Contacts[0].Components[1] = 2 }, geom.ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recipe polynomialMixedCutRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			tc.mutate(&recipe)
			bad, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if r, e := ReadParallelogramPolynomialCutJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, tc.err) {
				t.Fatal("source reproof must reject the changed physical recipe", tc.err, e)
			}
		})
	}
	base, a, b = obliquePolynomialStock(t, 0, 2)
	if r, e := CutParallelogramPolynomialComponents(ctx, base, b, a, 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonbinary physical junction cannot enter native topology", e)
	}
	if r, e := ReadParallelogramPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("aggregate topology limit", e)
	}
	o.Proof.MaxOperations = 1
	if r, e := ReadParallelogramPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("replay must freshly prove the original supports and volumes", e)
	}
}
