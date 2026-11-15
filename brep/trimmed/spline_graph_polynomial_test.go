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

func polynomialGraphFixture(t *testing.T, degree, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	base, _, _ := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	controls := []float64{.25, 11.0 / 32, 3.0 / 8, 3.0 / 8, 3.0 / 8}
	if degree == 4 {
		controls = []float64{.25, .25, .25, .25, .375}
	}
	var out [2]*geom.BSplineSurface
	for owner := 0; owner < 2; owner++ {
		var poles []geom.Vec3
		for i := 0; i < 5; i++ {
			u := float64(i) / 4
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v - controls[i]
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
		knot := .5
		if (owner == 0) != (axis == 1) {
			p, e = p.InsertUKnot(knot, 1, geom.SurfacePatchOptions{})
		} else {
			p, e = p.InsertVKnot(knot, 1, geom.SurfacePatchOptions{})
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
func polynomialGraphVolume(degree, order int, op parametric.BooleanOp) float64 {
	// Independently integrate coefficients f and f² on [0,1].
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	if degree == 4 {
		f = []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	}
	integral, square := new(big.Rat), new(big.Rat)
	for i, a := range f {
		integral.Add(integral, new(big.Rat).Quo(a, big.NewRat(int64(i+1), 1)))
		for j, b := range f {
			square.Add(square, new(big.Rat).Quo(new(big.Rat).Mul(a, b), big.NewRat(int64(2*(i+j+1)), 1)))
		}
	}
	positive := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 2), integral), square)
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
func TestNativePolynomialGraphBooleans(t *testing.T) {
	ctx := context.Background()
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			base, a, b := polynomialGraphFixture(t, degree, axis)
			for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
					s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{Polynomial: &geom.GraphPointContactOptions{}, PolynomialAxis: axis})
					if e != nil {
						t.Fatal(degree, axis, order, op, e)
					}
					if h.Polynomial == nil || !h.Polynomial.Proven || h.Polynomial.Degree != degree || h.Polynomial.FreeAxis != axis || len(h.ContourEdges) != 1 {
						t.Fatal(h)
					}
					if degree == 3 && h.Polynomial.Controls[1][axis].Cmp(big.NewRat(1, 3)) != 0 {
						t.Fatal("nonbinary exact cubic control lost")
					}
					if len(s.Topology().Shells) != 1 || s.Topology().EulerCharacteristic != 2 {
						t.Fatal(s.Topology())
					}
					for owner, ids := range [][]int{h.FacesA, h.FacesB} {
						for _, id := range ids {
							if s.faces[id].Surface != pair[owner] {
								t.Fatal("original support replaced")
							}
						}
					}
					m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
					if e != nil || math.Abs(m.Volume-polynomialGraphVolume(degree, order, op)) > 3e-8 {
						t.Fatal(degree, axis, order, op, m.Volume, e)
					}
					var buf bytes.Buffer
					if e = WriteJSON(&buf, s); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadJSONContext(ctx, &buf, JSONLimits{})
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
}

func TestPolynomialGraphScopeAndQuota(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := polynomialGraphFixture(t, 3, axis)
		ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
		r, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, axis, geom.GraphPointContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, limit := range []int{r.ExactOperations - 1, r.ExactOperations} {
			q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, axis, geom.GraphPointContactOptions{MaxOperations: limit})
			if (e == nil) != (limit == r.ExactOperations) || (e != nil && (q.Proven || len(q.Controls) > 0)) {
				t.Fatal("atomic quota", e)
			}
		}
		if q, e := geom.DiscoverSplineGraphParabolaOnAxis(ctx, ap, bp, axis, geom.GraphPointContactOptions{}); e == nil || q.Proven {
			t.Fatal("parabola scope widened")
		}
		if q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, 1-axis, geom.GraphPointContactOptions{}); e == nil || q.Proven {
			t.Fatal("wrong axis")
		}
		d := a.Data()
		d.Poles[len(d.Poles)-1].Z += 1.0 / 1024
		changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, changed, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{Polynomial: &geom.GraphPointContactOptions{}, PolynomialAxis: axis})
		if e == nil || len(s.faces) > 0 || h.Polynomial != nil {
			t.Fatal("later mixed term omitted")
		}
	}
	_, a, b := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	ap, _ := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
	bp, _ := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
	if q, e := geom.DiscoverSplineGraphPolynomialContact(ctx, ap, bp, 0, geom.GraphPointContactOptions{}); e == nil || q.Proven {
		t.Fatal("not genuine cubic/quartic")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := geom.DiscoverSplineGraphPolynomialContact(canceled, ap, bp, 0, geom.GraphPointContactOptions{}); e == nil || q.Proven {
		t.Fatal("canceled proof")
	}
}
func TestPolynomialGraphOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_POLYNOMIAL_ROOFS")
	if exe == "" {
		t.Skip("optional independently authored original OCCT stocks")
	}
	for _, degree := range []int{3, 4} {
		args := []string{}
		if degree == 4 {
			args = append(args, "quartic")
		}
		data, e := exec.Command(exe, args...).Output()
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Degree, Order, Operation, Solids int
			Valid                            bool
			Volume                           float64
		}
		if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
			t.Fatal(e)
		}
		seen := map[[2]int]bool{}
		for _, r := range rows {
			k := [2]int{r.Order, r.Operation}
			if r.Degree != degree || r.Order < 0 || r.Order > 1 || r.Operation < 0 || r.Operation > 2 || seen[k] {
				t.Fatal("oracle identity", r)
			}
			seen[k] = true
			op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[r.Operation]
			if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-polynomialGraphVolume(degree, r.Order, op)) > 3e-8 {
				t.Fatal(r)
			}
		}
	}
}

func TestExactBezierReversePreservesProofType(t *testing.T) {
	for _, degree := range []int{3, 4} {
		h := make([][3]*big.Rat, degree+1)
		for i := range h {
			h[i] = [3]*big.Rat{big.NewRat(int64(i), int64(degree)), big.NewRat(int64(i*i), int64(degree*degree)), big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		r, ok := reversePCurve(q).(ExactBezierUV)
		if !ok {
			t.Fatal("lost exact pcurve evidence")
		}
		back, ok := reversePCurve(r).(ExactBezierUV)
		if !ok {
			t.Fatal("reverse involution")
		}
		reversed, restored, original := r.HomogeneousControls(), back.HomogeneousControls(), q.HomogeneousControls()
		for i := range h {
			for j := 0; j < 3; j++ {
				if reversed[degree-i][j].Cmp(h[i][j]) != 0 || restored[i][j].Cmp(h[i][j]) != 0 || original[i][j].Cmp(h[i][j]) != 0 {
					t.Fatal("exact immutable reverse")
				}
			}
		}
	}
}

func TestPolynomialGraphRejectsFifthDegree(t *testing.T) {
	var patches [2]*geom.SurfacePatch
	for owner := 0; owner < 2; owner++ {
		var poles []geom.Vec3
		for i := 0; i <= 8; i++ {
			extra := 0.0
			if i >= 5 {
				n := new(big.Int).Binomial(int64(i), 5)
				extra = float64(n.Int64()) / 4096
			}
			for j := 0; j < 2; j++ {
				u, v := float64(i)/8, float64(j)
				z := 3.0
				if owner == 0 {
					z += v - .25 - extra
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		knots := make([]float64, 18)
		for i := 9; i < 18; i++ {
			knots[i] = 1
		}
		s, e := geom.NewBSplineSurface(8, 1, 9, 2, poles, knots, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		patches[owner], e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := geom.DiscoverSplineGraphPolynomialContact(context.Background(), patches[0], patches[1], 0, geom.GraphPointContactOptions{})
	if e == nil || r.Proven || len(r.Controls) > 0 {
		t.Fatal("genuine fifth-degree numerator accepted", e)
	}
}
