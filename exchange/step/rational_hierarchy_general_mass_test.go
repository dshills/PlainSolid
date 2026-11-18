package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"reflect"
	"testing"
)

// Independent separable Cartesian box integrals, with signed subtraction.
func generalMassBox(bounds [3][2]int64) (v *big.Rat, f [3]*big.Rat, q [3][3]*big.Rat) {
	var integrals [3][3]*big.Rat
	for i, b := range bounds {
		for power := 0; power < 3; power++ {
			a := new(big.Int).Exp(big.NewInt(b[0]), big.NewInt(int64(power+1)), nil)
			z := new(big.Int).Exp(big.NewInt(b[1]), big.NewInt(int64(power+1)), nil)
			integrals[i][power] = new(big.Rat).SetFrac(new(big.Int).Sub(z, a), big.NewInt(int64(power+1)))
		}
	}
	monomial := func(powers [3]int) *big.Rat {
		r := big.NewRat(1, 1)
		for i, p := range powers {
			r.Mul(r, integrals[i][p])
		}
		return r
	}
	v = monomial([3]int{})
	for i := 0; i < 3; i++ {
		powers := [3]int{}
		powers[i]++
		f[i] = monomial(powers)
		for j := 0; j < 3; j++ {
			p := powers
			p[j]++
			q[i][j] = monomial(p)
		}
	}
	return
}
func TestGeneralHierarchySignedMass(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		for _, family := range []string{"reentrant", "holed", "affine-pcurve-holed"} {
			t.Run(family+map[bool]string{false: "-neutral", true: "-occt"}[vendor], func(t *testing.T) {
				name := "rational-spherical-hierarchy-" + family + ".step"
				if vendor {
					name = "occt-" + name
				}
				raw, e := os.ReadFile("testdata/" + name)
				if e != nil {
					t.Fatal(e)
				}
				var m RationalSphericalHierarchy
				var query func(context.Context, GeneralRationalHierarchyMassOptions) (RationalHierarchyMass, error)
				if family == "affine-pcurve-holed" {
					a, e := readAffinePCurveHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					var native bytes.Buffer
					if e = WriteAffinePCurveHierarchyJSON(ctx, &native, a); e != nil {
						t.Fatal(e)
					}
					a, e = ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
					if e != nil {
						t.Fatal(e)
					}
					m = a.model
					query = a.Measure
				} else {
					m, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					query = func(ctx context.Context, o GeneralRationalHierarchyMassOptions) (RationalHierarchyMass, error) {
						return generalHierarchyMass(ctx, m, o, vendor)
					}
				}
				r, e := query(ctx, GeneralRationalHierarchyMassOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if r.PlanarTests == 0 || r.Triangles == 0 || len(r.Products) != 4 || len(r.Occurrences) != 7 {
					t.Fatal("work/ownership")
				}
				boxes := [][3][2]int64{{{-10, -6}, {-4, 0}, {-2, 2}}, {{-9, -7}, {-3, -1}, {-2, 2}}}
				sign := int64(-1)
				if family == "reentrant" {
					boxes = [][3][2]int64{{{-10, -6}, {-4, -3}, {-2, 2}}, {{-10, -9}, {-3, 0}, {-2, 2}}}
					sign = 1
				}
				v, f, q := generalMassBox(boxes[0])
				v2, f2, q2 := generalMassBox(boxes[1])
				v.Add(v, new(big.Rat).Mul(v2, big.NewRat(sign, 1)))
				for i := 0; i < 3; i++ {
					f[i].Add(f[i], new(big.Rat).Mul(f2[i], big.NewRat(sign, 1)))
					for j := 0; j < 3; j++ {
						q[i][j].Add(q[i][j], new(big.Rat).Mul(q2[i][j], big.NewRat(sign, 1)))
					}
				}
				eq := func(p PiPolynomial, w *big.Rat) {
					t.Helper()
					for i, c := range p.Coefficients() {
						want := new(big.Rat)
						if i == 0 {
							want = w
						}
						if c.Cmp(want) != 0 {
							t.Fatalf("exact coefficient %d: %s != %s", i, c, want)
						}
					}
				}
				var product HierarchyMaterialMass
				for _, p := range r.Products {
					if p.Family == "planar" {
						product = p
					}
				}
				eq(product.Moments.Volume(), v)
				for i := 0; i < 3; i++ {
					eq(product.Moments.FirstMoments()[i], f[i])
					for j := 0; j < 3; j++ {
						eq(product.Moments.SecondMoments()[i][j], q[i][j])
					}
				}
				// Independently transport central inertia by R I R^T, including products of inertia.
				var localI [3][3]*big.Rat
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						localI[i][j] = new(big.Rat).Neg(new(big.Rat).Sub(q[i][j], new(big.Rat).Quo(new(big.Rat).Mul(f[i], f[j]), v)))
						if i == j {
							for k := 0; k < 3; k++ {
								localI[i][j].Add(localI[i][j], new(big.Rat).Sub(q[k][k], new(big.Rat).Quo(new(big.Rat).Mul(f[k], f[k]), v)))
							}
						}
					}
				}
				for k, occ := range r.Occurrences {
					if occ.Family != "planar" {
						continue
					}
					if !reflect.DeepEqual(occ.Path, m.Instances[k].Path) || !reflect.DeepEqual(occ.SourceFaceIDs, product.SourceFaceIDs) {
						t.Fatal("ancestry")
					}
					mat := m.Instances[k].Matrix()
					for i := 0; i < 3; i++ {
						for j := 0; j < 3; j++ {
							want := new(big.Rat)
							for a := 0; a < 3; a++ {
								for b := 0; b < 3; b++ {
									want.Add(want, new(big.Rat).Mul(new(big.Rat).Mul(mat[i][a], mat[j][b]), localI[a][b]))
								}
							}
							got := occ.Moments.InertiaAboutCentroid()[i][j]
							eq(got.Numerator, new(big.Rat).Mul(want, v))
							eq(got.Denominator, v)
						}
					}
				}
				for _, o := range []GeneralRationalHierarchyMassOptions{{MaxPlanarTests: 1}, {RationalHierarchyMassOptions: RationalHierarchyMassOptions{MaxTriangles: 1}}, {MaxPlanarTests: -1}} {
					if got, e := query(ctx, o); e == nil || len(got.Products) != 0 {
						t.Fatal("quota", e)
					}
				}
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				if _, e = query(canceled, GeneralRationalHierarchyMassOptions{}); e != context.Canceled {
					t.Fatal(e)
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_MASS"); bin != "" {
					b, e := exec.Command(bin, "read", "testdata/"+name).Output()
					if e != nil {
						t.Fatal(e)
					}
					var oracle struct {
						Volume                        float64
						Solids                        int
						Center                        [3]float64
						CentralInertia, OriginInertia [3][3]float64
					}
					if e = json.Unmarshal(b, &oracle); e != nil {
						t.Fatal(e)
					}
					near := func(a, b float64) {
						t.Helper()
						if math.Abs(a-b) > 2e-8*math.Max(1, math.Abs(b)) {
							t.Fatalf("OCCT %.15g != %.15g", a, b)
						}
					}
					near(massValue(r.Sum.Volume()), oracle.Volume)
					if oracle.Solids != 7 {
						t.Fatal("solids")
					}
					for i, c := range r.Sum.Centroid() {
						near(massValue(c.Numerator)/massValue(c.Denominator), oracle.Center[i])
						for j, p := range r.Sum.InertiaAboutOrigin()[i] {
							near(massValue(p), oracle.OriginInertia[i][j])
							c := r.Sum.InertiaAboutCentroid()[i][j]
							near(massValue(c.Numerator)/massValue(c.Denominator), oracle.CentralInertia[i][j])
						}
					}
				}
			})
		}
	}
}
func TestGeneralHierarchyMassRejectsSourceMutation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy-holed.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range m.Products {
		if p.Planar == nil {
			continue
		}
		for _, en := range m.Source.Entities {
			if len(en.Records) == 1 && en.Records[0].Type == "FACE_BOUND" {
				if en.Records[0].Args[2].Text == "T" {
					en.Records[0].Args[2].Text = "F"
				} else {
					en.Records[0].Args[2].Text = "T"
				}
				m.Source.Entities[en.ID] = en
				break
			}
		}
		break
	}
	if out, e := MeasureGeneralRationalSphericalHierarchy(ctx, m, GeneralRationalHierarchyMassOptions{}); e == nil || len(out.Products) != 0 {
		t.Fatal("forged hole", e)
	}
	a := massArithmetic{bits: 256}
	var m0 SymbolicMaterialMoments
	x := [3][3]*big.Rat{}
	for i := range x {
		for j := range x[i] {
			x[i][j] = new(big.Rat)
			if i == j {
				x[i][j].SetInt(new(big.Int).Lsh(big.NewInt(1), 200))
			}
		}
	}
	integrateSignedHierarchyTetra(&m0, x, &a)
	if a.err != geom.ErrConvergence {
		t.Fatal("bit quota")
	}
}
