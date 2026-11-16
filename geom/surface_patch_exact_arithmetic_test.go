package geom

import (
	"errors"
	"math/big"
	"testing"
)

func TestSurfacePatchBoundsCheckReducedQuotients(t *testing.T) {
	denominator := new(big.Int).Lsh(big.NewInt(1), 100)
	weight := new(big.Rat).SetFrac(big.NewInt(1), denominator)
	poles := [2][3]*big.Rat{
		{big.NewRat(-3, 1), big.NewRat(11, 19), big.NewRat(-8, 1)},
		{big.NewRat(3, 1), big.NewRat(17, 19), big.NewRat(-4, 1)},
	}
	var p SurfacePatch
	for _, pole := range poles {
		h := exactHomogeneous{nil, nil, nil, weight}
		for axis, q := range pole {
			h[axis] = new(big.Rat).Mul(q, weight)
		}
		p.net = append(p.net, h)
	}
	b := &polynomialBudget{maxBits: 64, maxOperations: 6}
	if err := p.computeBounds(b); err != nil || b.operations != 6 {
		t.Fatalf("the reduced quotients fit the bit budget: %v, %d", err, b.operations)
	}
	for axis := range 3 {
		if coordinate(p.bounds.Min, axis) != rationalFloatBound(poles[0][axis], true) ||
			coordinate(p.bounds.Max, axis) != rationalFloatBound(poles[1][axis], false) {
			t.Fatal("wrong outward-rounded hull", p.bounds)
		}
	}
	b = &polynomialBudget{maxBits: 64, maxOperations: 4}
	if err := p.computeBounds(b); !errors.Is(err, ErrConvergence) || b.operations != 5 {
		t.Fatalf("same work must be charged even for reduced quotients: %v, %d", err, b.operations)
	}
	p.net[1][2] = new(big.Rat).SetInt64(1)
	b = &polynomialBudget{maxBits: 64, maxOperations: 6}
	if err := p.computeBounds(b); !errors.Is(err, ErrConvergence) || b.operations != 6 {
		t.Fatalf("the sixth exact quotient still exceeds the bit budget: %v, %d", err, b.operations)
	}
}

func TestSurfaceArcDataKeepsExactCoefficientsAndWork(t *testing.T) {
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := []Vec3{{-1, 0, -.75}, {-2, .5, .25}, {0, 1, .125}, {-.5, -.5, 2},
		{.5, .5, 1}, {.5, 1, .25}, {1, 0, 0}, {1, .5, .25}, {1, 1, -1}}
	s, err := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, []float64{1, 1.5, 1, 1.125, 1.25, 1, 1, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err = p.Trim(ParameterRectangle{.17, .73, .29, .92})
	if err != nil {
		t.Fatal(err)
	}
	z, err := NewPlane(Vec3{}, Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	oblique, err := NewRawSupportPlane(Vec3{X: -1, Y: .125, Z: .5}, Vec3{X: .25}, Vec3{Z: 2}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const operations = 4*9 + 5*(6+6)
	for _, plane := range []Plane{z, oblique.plane} {
		b := &polynomialBudget{maxOperations: operations, maxBits: 16384}
		d, err := p.arcData(plane, b)
		if err != nil || b.operations != operations {
			t.Fatalf("arc-data work changed: %v, %d", err, b.operations)
		}
		eq := exactPlaneCoefficients(plane)
		offset := new(big.Rat).Set(eq[3])
		for axis := range 3 {
			c := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, axis))
			offset.Add(offset, new(big.Rat).Mul(c, eq[axis]))
		}
		for i, h := range p.net {
			f := new(big.Rat).Mul(offset, h[3])
			for axis := range 3 {
				c := new(big.Rat).SetFloat64(coordinate(p.bounds.Min, axis))
				q := new(big.Rat).Sub(h[axis], c.Mul(c, h[3]))
				if q.Cmp(d.fields[axis][i]) != 0 {
					t.Fatal("centered field changed", axis, i)
				}
				f.Add(f, q.Mul(q, eq[axis]))
			}
			if d.fields[4][i].Cmp(f) != 0 {
				t.Fatal("plane coefficient changed", i)
			}
		}
		domain := p.Domain()
		for field, c := range d.fields {
			for axis, bounds := range [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}} {
				width := new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[1]), new(big.Rat).SetFloat64(bounds[0]))
				scale := new(big.Rat).Quo(big.NewRat(2, 1), width)
				var derivatives []*big.Rat
				for i := range c {
					if axis == 0 && i < 6 {
						derivatives = append(derivatives, new(big.Rat).Mul(scale, new(big.Rat).Sub(c[i+3], c[i])))
					}
					if axis == 1 && i%3 != 2 {
						derivatives = append(derivatives, new(big.Rat).Mul(scale, new(big.Rat).Sub(c[i+1], c[i])))
					}
				}
				want := coefficientRange(derivatives)
				got := [2]exactInterval{d.du[field], d.dv[field]}[axis]
				if got.lo.Cmp(want.lo) != 0 || got.hi.Cmp(want.hi) != 0 {
					t.Fatal("derivative hull changed", field, axis)
				}
			}
		}
		b = &polynomialBudget{maxOperations: operations - 1, maxBits: 16384}
		if _, err := p.arcData(plane, b); !errors.Is(err, ErrConvergence) || b.operations != operations {
			t.Fatalf("arc-data work limit changed: %v, %d", err, b.operations)
		}
	}
}
