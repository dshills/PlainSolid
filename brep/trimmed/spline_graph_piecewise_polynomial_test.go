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
	"plainsolid/internal/slowtest"
	"testing"
)

func piecewisePolynomialFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	base, _, _ := openParabolicGraphFixture(t, [3]float64{.25, .5, .25}, false)
	var out [2]*geom.BSplineSurface
	for owner := 0; owner < 2; owner++ {
		n, degree := 9, 4
		knots := []float64{0, 0, 0, 0, 0, .5, .5, .5, .5, 1, 1, 1, 1, 1}
		f := []float64{.25, 11.0 / 32, .375, .375, .375, 13.0 / 32, 7.0 / 16, 15.0 / 32, .625}
		if owner == 1 {
			n, degree = 3, 2
			knots = []float64{0, 0, 0, 1, 1, 1}
		}
		var poles []geom.Vec3
		for i := 0; i < n; i++ {
			u := float64(i) / float64(n-1)
			for j := 0; j < 3; j++ {
				v := float64(j) / 2
				z := 3 + u*v/64
				if owner == 0 {
					z += v - f[i]
				}
				poles = append(poles, geom.Vec3{X: u, Y: v, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(degree, 2, n, 3, poles, knots, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		if axis == 1 {
			d := s.Data()
			poles = nil
			for j := 0; j < 3; j++ {
				for i := 0; i < n; i++ {
					p := d.Poles[3*i+j]
					p.X, p.Y = p.Y, p.X
					poles = append(poles, p)
				}
			}
			s, e = geom.NewBSplineSurface(2, degree, 3, n, poles, d.KnotsV, d.KnotsU, nil)
			if e != nil {
				t.Fatal(e)
			}
		}
		if owner == 1 {
			p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if axis == 0 {
				p, e = p.InsertUKnot(.75, 1, geom.SurfacePatchOptions{})
				if e == nil {
					p, e = p.InsertVKnot(.3125, 1, geom.SurfacePatchOptions{})
				}
			} else {
				p, e = p.InsertVKnot(.75, 1, geom.SurfacePatchOptions{})
				if e == nil {
					p, e = p.InsertUKnot(.3125, 1, geom.SurfacePatchOptions{})
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			s, e = p.ToBSplineSurface()
			if e != nil {
				t.Fatal(e)
			}
		}
		out[owner] = s
	}
	return base, out[0], out[1]
}
func piecewisePolynomialVolume(order int, op parametric.BooleanOp) float64 {
	fs := [][]*big.Rat{{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}, {big.NewRat(3, 8), big.NewRat(1, 8), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}}
	I, Q := new(big.Rat), new(big.Rat)
	for _, f := range fs {
		for i, x := range f {
			I.Add(I, new(big.Rat).Quo(x, big.NewRat(int64(2*(i+1)), 1)))
			for j, y := range f {
				Q.Add(Q, new(big.Rat).Quo(new(big.Rat).Mul(x, y), big.NewRat(int64(4*(i+j+1)), 1)))
			}
		}
	}
	P := new(big.Rat).Add(new(big.Rat).Sub(big.NewRat(1, 2), I), Q)
	r := new(big.Rat)
	switch op {
	case parametric.Union:
		r.Add(big.NewRat(769, 256), P)
	case parametric.Intersection:
		r.Sub(big.NewRat(769, 256), Q)
	default:
		if order == 0 {
			r.Set(P)
		} else {
			r.Set(Q)
		}
	}
	v, _ := r.Float64()
	return v
}
func TestNativePiecewisePolynomialBooleans(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := piecewisePolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{PiecewisePolynomial: &geom.PiecewisePolynomialOptions{PiecewiseParabolaOptions: geom.PiecewiseParabolaOptions{FreeAxis: axis}}})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				r := h.PiecewisePolynomial
				if r == nil || !r.Proven || len(r.Pieces) != 3 || len(r.Joins) != 2 || len(h.ContourEdges) != 3 {
					t.Fatal("complete original chain", r)
				}
				for i, want := range []int{3, 4, 4} {
					if r.Pieces[i].Constant == nil || r.Pieces[i].Constant.Degree != want {
						t.Fatal("actual polynomial change")
					}
				}
				j := r.Joins[0]
				if j.Point[2].Cmp(big.NewRat(3075, 1024)) != 0 || j.Tangents[0][1-axis].Sign() != 0 || j.Tangents[1][1-axis].Cmp(big.NewRat(1, 4)) != 0 || j.Tangents[0][2].Cmp(big.NewRat(3, 512)) != 0 || j.Tangents[1][2].Cmp(big.NewRat(1, 128)) != 0 {
					t.Fatal("original one-sided kink", j)
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
				incidence := map[int]int{}
				for _, id := range h.ContourEdges {
					edge := s.edges[id]
					incidence[edge.Start]++
					incidence[edge.End]++
				}
				ends, joins := 0, 0
				for _, n := range incidence {
					if n == 1 {
						ends++
					} else if n == 2 {
						joins++
					} else {
						t.Fatal(incidence)
					}
				}
				if ends != 2 || joins != 2 {
					t.Fatal(incidence)
				}
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-piecewisePolynomialVolume(order, op)) > 3e-8 {
					t.Fatal(axis, order, op, m.Volume, e)
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
func TestPiecewisePolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_PIECEWISE_POLYNOMIAL")
	if exe == "" {
		t.Skip("optional independent original OCCT stocks")
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
		if !r.Valid || r.Solids != 1 || math.Abs(r.Volume-piecewisePolynomialVolume(r.Order, op)) > 3e-8 {
			t.Fatal(r)
		}
	}
}

func TestNativePiecewisePolynomialAtomicCoverage(t *testing.T) {
	base, a, b := piecewisePolynomialFixture(t, 0)
	for _, o := range []geom.PiecewisePolynomialOptions{{PiecewiseParabolaOptions: geom.PiecewiseParabolaOptions{MaxPieces: 2}}, {PiecewiseParabolaOptions: geom.PiecewiseParabolaOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}}, {Mode: geom.PolynomialGraphMode(3)}} {
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{PiecewisePolynomial: &o})
		if e == nil || len(s.faces) > 0 || h.PiecewisePolynomial != nil {
			t.Fatal("partial assembly", e)
		}
	}
	d := a.Data()
	d.Poles[len(d.Poles)-1].Z += 1.0 / 1024
	changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, changed, b, parametric.Union, 1e-9, SplineGraphBooleanOptions{PiecewisePolynomial: &geom.PiecewisePolynomialOptions{}})
	if e == nil || len(s.faces) > 0 || h.PiecewisePolynomial != nil {
		t.Fatal("later original cell omitted", e)
	}
}

func TestNativePiecewisePolynomialProofModes(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, mode := range []geom.PolynomialGraphMode{geom.FreeFactoredPolynomialGraph, geom.BivariatePolynomialGraph} {
		base, a, b := nativeFactoredPolynomialFixture(t, 0)
		volume := factoredPolynomialVolume
		if mode == geom.BivariatePolynomialGraph {
			base, a, b = nativeBivariatePolynomialFixture(t, 0)
			volume = bivariatePolynomialVolume
		}
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(ctx, base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{PiecewisePolynomial: &geom.PiecewisePolynomialOptions{Mode: mode}})
				if e != nil {
					t.Fatal(mode, order, op, e)
				}
				if h.PiecewisePolynomial == nil || h.PiecewisePolynomial.Mode != mode || len(h.ContourEdges) != 2 {
					t.Fatal("mode ownership")
				}
				m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-volume(order, op)) > 3e-8 {
					t.Fatal(mode, order, op, m.Volume, e)
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
