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

func twoPolynomialFixture(t *testing.T, axis int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	base, _, _ := polynomialGraphFixture(t, 3, axis)
	// A-B=(105/128)(v-f)(v-g), independently authored exact degree-eight tensor.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(5, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	sum, prod := make([]*big.Rat, 9), make([]*big.Rat, 9)
	for i := range sum {
		sum[i] = new(big.Rat)
		prod[i] = new(big.Rat)
	}
	for i, x := range f {
		sum[i].Add(sum[i], x)
		for j, y := range g {
			prod[i+j].Add(prod[i+j], new(big.Rat).Mul(x, y))
		}
	}
	for i, x := range g {
		sum[i].Add(sum[i], x)
	}
	bern := func(p []*big.Rat, i int) *big.Rat {
		v := new(big.Rat)
		for j := 0; j <= i; j++ {
			v.Add(v, new(big.Rat).Mul(p[j], new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(8, int64(j)))))
		}
		return v
	}
	var out [2]*geom.BSplineSurface
	for owner := 0; owner < 2; owner++ {
		var h [][4]*big.Rat
		for i := 0; i < 9; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), 8), big.NewRat(int64(j), 2)
				z := new(big.Rat).Add(big.NewRat(3, 1), new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(64, 1)))
				if owner == 0 {
					gap := new(big.Rat).Sub(bern(prod, i), new(big.Rat).Mul(v, bern(sum, i)))
					if j == 2 {
						gap.Add(gap, big.NewRat(1, 1))
					}
					z.Add(z, new(big.Rat).Mul(big.NewRat(105, 128), gap))
				}
				h = append(h, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
			}
		}
		du, dv := 8, 2
		if axis == 1 {
			var tr [][4]*big.Rat
			for j := 0; j < 3; j++ {
				for i := 0; i < 9; i++ {
					p := h[3*i+j]
					p[0], p[1] = p[1], p[0]
					tr = append(tr, p)
				}
			}
			h = tr
			du, dv = 2, 8
		}
		p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), du, dv, h, geom.SurfacePatchOptions{})
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
			t.Fatal("lossless original", e)
		}
	}
	return base, out[0], out[1]
}
func twoPolynomialVolume(order int, op parametric.BooleanOp) float64 {
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{big.NewRat(5, 8), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	// Negative strip magnitude is (105/128)*(g-f)^3/6; integrate the full quadratic independently.
	d := make([]*big.Rat, 5)
	for i := range d {
		d[i] = new(big.Rat).Set(g[i])
		if i < len(f) {
			d[i].Sub(d[i], f[i])
		}
	}
	neg, total := new(big.Rat), big.NewRat(1, 3)
	for i, x := range f {
		total.Sub(total, new(big.Rat).Quo(x, big.NewRat(int64(2*(i+1)), 1)))
		for j, y := range g {
			total.Add(total, new(big.Rat).Quo(new(big.Rat).Mul(x, y), big.NewRat(int64(i+j+1), 1)))
		}
	}
	for i, x := range g {
		total.Sub(total, new(big.Rat).Quo(x, big.NewRat(int64(2*(i+1)), 1)))
	}
	for i, x := range d {
		for j, y := range d {
			for k, z := range d {
				term := new(big.Rat).Mul(x, y)
				term.Mul(term, z)
				neg.Add(neg, new(big.Rat).Quo(term, big.NewRat(int64(6*(i+j+k+1)), 1)))
			}
		}
	}
	neg.Mul(neg, big.NewRat(105, 128))
	total.Mul(total, big.NewRat(105, 128))
	positive := new(big.Rat).Add(total, neg)
	v := new(big.Rat)
	switch op {
	case parametric.Intersection:
		v.Sub(big.NewRat(769, 256), neg)
	case parametric.Union:
		v.Add(big.NewRat(769, 256), positive)
	default:
		if order == 0 {
			v.Set(positive)
		} else {
			v.Set(neg)
		}
	}
	out, _ := v.Float64()
	return out
}
func TestNativeTwoPolynomialGraphs(t *testing.T) {
	for _, axis := range []int{0, 1} {
		base, a, b := twoPolynomialFixture(t, axis)
		for order, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference} {
				s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, pair[0], pair[1], op, 1e-9, SplineGraphBooleanOptions{TwoPolynomials: &geom.GraphPointContactOptions{}, TwoPolynomialsAxis: axis})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				if h.TwoPolynomials == nil || !h.TwoPolynomials.Proven || len(h.ContourEdges) != 2 {
					t.Fatal("both original branches", h)
				}
				if op == parametric.Difference && order == 1 {
					if len(h.WallFaces) != 4 || len(h.DomainSides) != 4 {
						t.Fatal("original two-ended wall subdivision", h.WallFaces, h.DomainSides)
					}
					counts := map[int]int{}
					for _, side := range h.DomainSides {
						counts[side]++
					}
					wantSides := []int{1, 3}
					if axis == 1 {
						wantSides = []int{0, 2}
					}
					for _, side := range wantSides {
						if counts[side] != 2 {
							t.Fatal("original side ownership", counts)
						}
					}
				}
				shells := 1
				if op == parametric.Difference && order == 0 {
					shells = 2
				}
				if len(s.Topology().Shells) != shells || s.Topology().EulerCharacteristic != 2*shells {
					t.Fatal(s.Topology())
				}
				for owner, ids := range [][]int{h.FacesA, h.FacesB} {
					for _, id := range ids {
						if s.faces[id].Reversed != (owner == 1 && op == parametric.Difference) {
							t.Fatal("original signed roof ownership")
						}
						if s.faces[id].Surface != pair[owner] {
							t.Fatal("source replaced")
						}
					}
				}
				m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if e != nil || math.Abs(m.Volume-twoPolynomialVolume(order, op)) > 2e-7 {
					t.Fatal(axis, order, op, m.Volume, twoPolynomialVolume(order, op), e)
				}
				var buf bytes.Buffer
				if e = WriteJSON(&buf, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSONContext(context.Background(), &buf, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				if e = validateExactGraphPartitionEdges(context.Background(), loaded, geom.GraphPointContactOptions{}); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
}

func TestTwoPolynomialOCCT(t *testing.T) {
	exe := os.Getenv("PLAINSOLID_OCCT_TWO_POLYNOMIALS")
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
		solids := 1
		if r.Order == 0 && op == parametric.Difference {
			solids = 2
		}
		if !r.Valid || r.Solids != solids || math.Abs(r.Volume-twoPolynomialVolume(r.Order, op)) > 2e-8 {
			t.Fatal(r, twoPolynomialVolume(r.Order, op))
		}
	}
}
func TestNativeTwoPolynomialAtomicLimits(t *testing.T) {
	base, a, b := twoPolynomialFixture(t, 0)
	for _, o := range []SplineGraphBooleanOptions{
		{TwoPolynomials: &geom.GraphPointContactOptions{MaxOperations: 1}},
		{TwoPolynomials: &geom.GraphPointContactOptions{MaxCells: 1}},
		{TwoPolynomials: &geom.GraphPointContactOptions{}, TwoPolynomialsAxis: 2},
		{TwoPolynomials: &geom.GraphPointContactOptions{}, Polynomial: &geom.GraphPointContactOptions{}},
	} {
		s, h, e := CombineSplineGraphSolidsWithOptions(context.Background(), base, a, b, parametric.Union, 1e-9, o)
		if e == nil || len(s.faces) != 0 || h.TwoPolynomials != nil {
			t.Fatal("partial publication", e)
		}
	}
}
