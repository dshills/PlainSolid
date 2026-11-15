package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func nativeFactoredPolynomialFixture(t *testing.T, axis int, extraBranch ...bool) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	base, _, _ := polynomialGraphFixture(t, 3, axis)
	var out [2]*geom.BSplineSurface
	fg := []float64{.25, 17.0 / 32, 15.0 / 16, 39.0 / 32, 1.5}
	g0, g1 := 1.0, 3.0
	if len(extraBranch) > 0 && extraBranch[0] {
		g0, g1 = -.5, 1
		fg = []float64{-.125, -7.0 / 64, 0, 3.0 / 32, 3.0 / 16}
	}
	for owner := 0; owner < 2; owner++ {
		var poles []geom.Vec3
		for i := 0; i < 5; i++ {
			u := float64(i) / 4
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v*(g0+g1*u) - fg[i]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(4, 2, 5, 3, poles, []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if axis == 1 {
			d := s.Data()
			poles = nil
			for j := 0; j < 3; j++ {
				for i := 0; i < 5; i++ {
					p := d.Poles[i*3+j]
					p.X, p.Y = p.Y, p.X
					poles = append(poles, p)
				}
			}
			s, e = geom.NewBSplineSurface(2, 4, 3, 5, poles, d.KnotsV, d.KnotsU, nil)
			if e != nil {
				t.Fatal(e)
			}
		}
		p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		k := .5
		if owner == 1 {
			k = .3125
		}
		if (owner == 0) != (axis == 1) {
			p, e = p.InsertUKnot(k, 1, geom.SurfacePatchOptions{})
		} else {
			p, e = p.InsertVKnot(k, 1, geom.SurfacePatchOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		out[owner], e = p.ToBSplineSurface()
		if e != nil {
			t.Fatal(e)
		}
	}
	return base, out[0], out[1]
}
func factoredPolynomialVolume(order int, op parametric.BooleanOp) float64 {
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 1)}
	fg, square := new(big.Rat), new(big.Rat)
	for k, z := range g {
		for i, x := range f {
			fg.Add(fg, new(big.Rat).Quo(new(big.Rat).Mul(z, x), big.NewRat(int64(i+k+1), 1)))
			for j, y := range f {
				square.Add(square, new(big.Rat).Quo(new(big.Rat).Mul(z, new(big.Rat).Mul(x, y)), big.NewRat(int64(2*(i+j+k+1)), 1)))
			}
		}
	}
	positive := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(5, 4), fg), square)
	r := new(big.Rat)
	switch op {
	case parametric.Union:
		r.Add(big.NewRat(769, 256), positive)
	case parametric.Intersection:
		r.Sub(big.NewRat(769, 256), square)
	default:
		if order == 0 {
			r.Set(positive)
		} else {
			r.Set(square)
		}
	}
	v, _ := r.Float64()
	return v
}
func TestNativeFactoredPolynomialBooleans(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativeFactoredPolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{FactoredPolynomial: &geom.GraphPointContactOptions{}, FactoredPolynomialAxis: axis})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				if h.FactoredPolynomial == nil || !h.FactoredPolynomial.Proven || len(h.FactoredPolynomial.Factors) != 4 || len(h.ContourEdges) != 1 || h.Polynomial != nil {
					t.Fatal("factored ownership", h)
				}
				if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
					t.Fatal(s.Topology())
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Surface != pair[owner] {
							t.Fatal("source replaced")
						}
					}
				}
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-factoredPolynomialVolume(order, op)) > 3e-8 {
					t.Fatal(axis, order, op, m.Volume, e)
				}
				var data bytes.Buffer
				if e = WriteJSON(&data, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(ctx, &data, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				if e = validateExactGraphPartitionEdges(ctx, loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
}
func TestFactoredPolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_FACTORED_POLYNOMIAL")
	if exe == "" {
		t.Skip("optional independently authored original OCCT stocks")
	}
	data, e := exec.Command(exe).Output()
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Order, Operation, Solids int
		Valid                    bool
		Volume                   float64
	}
	if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
		t.Fatal(e)
	}
	seen := map[[2]int]bool{}
	for _, r := range rows {
		k := [2]int{r.Order, r.Operation}
		if r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 || seen[k] {
			t.Fatal(r)
		}
		seen[k] = true
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-factoredPolynomialVolume(r.Order, op)) > 3e-8 {
			t.Fatal(r)
		}
	}
}

func TestNativeFactoredPolynomialRejectsExtraBranch(t *testing.T) {
	for _, axis := range []int{0, 1} {
		base, a, b := nativeFactoredPolynomialFixture(t, axis, true)
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{FactoredPolynomial: &geom.GraphPointContactOptions{MaxDepth: 4}, FactoredPolynomialAxis: axis})
		if e == nil || len(s.faces) > 0 || h.FactoredPolynomial != nil {
			t.Fatal("extra vertical branch published", axis, e)
		}
	}
}
