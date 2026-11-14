package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"
)

func massValue(p PiPolynomial) float64 {
	v := 0.
	for i, c := range p.Coefficients() {
		f, _ := c.Float64()
		v += f * math.Pow(math.Pi, float64(i))
	}
	return v
}
func TestRationalHierarchyMass(t *testing.T) {
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			raw, e := os.ReadFile("testdata/" + name)
			if e != nil {
				t.Fatal(e)
			}
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			query := func(ctx context.Context, o RationalHierarchyMassOptions) (RationalHierarchyMass, error) {
				if vendor {
					return MeasureOCCTRationalSphericalHierarchy(ctx, OCCTRationalSphericalHierarchy(m), o)
				}
				return MeasureRationalSphericalHierarchy(ctx, m, o)
			}
			r, e := query(ctx, RationalHierarchyMassOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Products) != 4 || len(r.Occurrences) != 7 || r.Triangles != 12 {
				t.Fatal("work", r.Triangles)
			}
			want := [3]*big.Rat{big.NewRat(24, 1), big.NewRat(136, 3), big.NewRat(12, 1)}
			for i, c := range r.Sum.Volume().Coefficients() {
				w := new(big.Rat)
				if i < 3 {
					w = want[i]
				}
				if c.Cmp(w) != 0 {
					t.Fatal("volume", i, c, w)
				}
			}
			if !vendor {
				expected := [3][3]string{{"-216", "674/3", "210"}, {"-60", "272", "138"}, {"0", "422/3", "36"}}
				for i, p := range r.Sum.FirstMoments() {
					cs := p.Coefficients()
					for j, s := range expected[i] {
						w, _ := new(big.Rat).SetString(s)
						if cs[j].Cmp(w) != 0 {
							t.Fatal("first", i, j, cs[j], w)
						}
					}
				}
			}
			for _, p := range r.Products {
				I := p.Moments.InertiaAboutCentroid()
				if p.Family == "cylinder" {
					for i, v := range []float64{21 * math.Pi, 21 * math.Pi, 24 * math.Pi} {
						got := massValue(I[i][i].Numerator) / massValue(I[i][i].Denominator)
						if math.Abs(got-v) > 1e-10 {
							t.Fatal("cylinder inertia", got)
						}
					}
				}

				if p.Family == "sphere" || p.Family == "torus" {
					want := [3]float64{256 * math.Pi / 15, 256 * math.Pi / 15, 256 * math.Pi / 15}
					if p.Family == "torus" {
						want = [3]float64{123 * math.Pi * math.Pi / 4, 123 * math.Pi * math.Pi / 4, 117 * math.Pi * math.Pi / 2}
					}
					for i := 0; i < 3; i++ {
						for j := 0; j < 3; j++ {
							w := 0.
							if i == j {
								w = want[i]
							}
							got := massValue(I[i][j].Numerator) / massValue(I[i][j].Denominator)
							if math.Abs(got-w) > 1e-9 {
								t.Fatal("analytic local inertia", p.Family, i, j, got, w)
							}
						}
					}
				}
				if len(p.Path) != 0 || len(p.SourceFaceIDs) == 0 {
					t.Fatal("provenance")
				}
			}

			for k, leaf := range r.Occurrences {
				if leaf.Family != "cylinder" {
					continue
				}
				// Independently rotate the analytic central inertia 21*pi I +
				// 3*pi zz^T, including nonzero off-diagonal entries.
				matrix := m.Instances[k].Matrix()
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						want := new(big.Rat).Mul(matrix[i][2], matrix[j][2])
						want.Mul(want, big.NewRat(3, 1))
						if i == j {
							want.Add(want, big.NewRat(21, 1))
						}
						q := leaf.Moments.InertiaAboutCentroid()[i][j]
						n, d := q.Numerator.Coefficients(), q.Denominator.Coefficients()
						got := new(big.Rat).Quo(n[2], d[1])
						if got.Cmp(want) != 0 {
							t.Fatal("exact rotated inertia", i, j, got, want)
						}
					}
				}
			}
			c := r.Sum.Volume().Coefficients()
			c[0].SetInt64(999)
			if r.Sum.Volume().Coefficients()[0].Cmp(want[0]) != 0 {
				t.Fatal("mutable")
			}
			if out, e := query(ctx, RationalHierarchyMassOptions{MaxTriangles: 1}); e == nil || len(out.Products) != 0 {
				t.Fatal("triangle limit")
			}
			cancel, cancelFn := context.WithCancel(ctx)
			cancelFn()
			if _, e = query(cancel, RationalHierarchyMassOptions{}); e == nil {
				t.Fatal("cancel")
			}
			m.Instances[0].Path[0]++
			if _, e = query(ctx, RationalHierarchyMassOptions{}); e == nil {
				t.Fatal("forged ancestry")
			}
			m.Instances[0].Path[0]--
			if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_MASS"); bin != "" {
				out, e := exec.Command(bin, "read", "testdata/"+name).Output()
				if e != nil {
					t.Fatal(e)
				}
				var oracle struct {
					Volume                        float64
					Solids                        int
					Center                        [3]float64
					CentralInertia, OriginInertia [3][3]float64
				}
				if e = json.Unmarshal(out, &oracle); e != nil {
					t.Fatal(e)
				}
				near := func(a, b float64) {
					t.Helper()
					if math.Abs(a-b) > 2e-8*math.Max(1, math.Abs(b)) {
						t.Fatalf("oracle %.15g != %.15g", a, b)
					}
				}
				near(massValue(r.Sum.Volume()), oracle.Volume)
				if oracle.Solids != 7 {
					t.Fatal("solids")
				}
				for i, q := range r.Sum.Centroid() {
					near(massValue(q.Numerator)/massValue(q.Denominator), oracle.Center[i])
					for j, p := range r.Sum.InertiaAboutOrigin()[i] {
						near(massValue(p), oracle.OriginInertia[i][j])
						q := r.Sum.InertiaAboutCentroid()[i][j]
						near(massValue(q.Numerator)/massValue(q.Denominator), oracle.CentralInertia[i][j])
					}
				}
			}
		})
	}
}

func TestSymbolicMassArithmeticLimits(t *testing.T) {
	a := massArithmetic{bits: 256}
	p := a.scalar(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 200)), 0)
	_ = a.mul(p, p)
	if a.err == nil {
		t.Fatal("coefficient growth")
	}
	a = massArithmetic{bits: 256}
	p = a.scalar(big.NewRat(1, 1), 4)
	_ = a.mul(p, p)
	if a.err == nil {
		t.Fatal("degree overflow")
	}
	if r, e := MeasureRationalSphericalHierarchy(nil, RationalSphericalHierarchy{}, RationalHierarchyMassOptions{}); e == nil || len(r.Products) != 0 {
		t.Fatal("nil context")
	}
	for _, o := range []RationalHierarchyMassOptions{{MaxTriangles: -1}, {MaxCoefficientBits: 255}, {MaxCoefficientBits: 8193}} {
		if _, e := MeasureRationalSphericalHierarchy(context.Background(), RationalSphericalHierarchy{}, o); e == nil {
			t.Fatal("options")
		}
	}
}
