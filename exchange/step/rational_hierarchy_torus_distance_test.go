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
	"strconv"
	"testing"
)

func TestHierarchyTorusDistance(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-" + name
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := os.ReadFile("testdata/" + name)
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			var native bytes.Buffer
			if vendor {
				if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &native, OCCTRationalSphericalHierarchy(m)); e != nil {
					t.Fatal(e)
				}
				q, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				m = RationalSphericalHierarchy(q)
			} else {
				if e = WriteRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
			}
			index := -1
			for i, u := range m.Instances {
				if m.Products[u.ProductID].Torus != nil {
					index = i
					break
				}
			}
			u := m.Instances[index]
			torus := m.Products[u.ProductID].Torus
			type sample struct {
				name        string
				p           geom.Vec3
				kind, exact string
			}
			cases := []sample{{"outer", geom.Vec3{X: 5}, "point", "1"}, {"inner", geom.Vec3{X: 1}, "point", "1"}, {"top", geom.Vec3{X: 3, Z: 2}, "point", "1"}, {"inside", geom.Vec3{X: 3.5}, "point", "1/4"}, {"boundary", geom.Vec3{X: 4}, "point", "0"}, {"axis", geom.Vec3{}, "axis-circle", "4"}, {"axis-raised", geom.Vec3{Z: 1}, "axis-circle", ""}, {"meridian", geom.Vec3{X: 3}, "meridian-circle", "1"}, {"nested-irrational", geom.Vec3{X: 1, Y: 1, Z: 2}, "point", ""}, {"nested-inside", geom.Vec3{X: 2, Y: 2, Z: .5}, "point", ""}}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					p := torus.Frame.Origin().Add(torus.Frame.XAxis().Scale(c.p.X)).Add(torus.Frame.YAxis().Scale(c.p.Y)).Add(torus.Frame.ZAxis().Scale(c.p.Z))
					point, e := u.ApplyExact(p)
					if e != nil {
						t.Fatal(e)
					}
					result, e := distanceHierarchyTori(ctx, m, point, RationalHierarchyTorusDistanceOptions{}, vendor)
					if e != nil {
						t.Fatal(e)
					}
					leaf := result.Leaves[index]
					if !leaf.Supported || leaf.SourceFaceID != torus.SourceFaceID || leaf.NearestBoundary.Kind() != c.kind {
						t.Fatal("classification/set")
					}
					for i, l := range result.Leaves {
						if m.Products[m.Instances[i].ProductID].Torus == nil && (l.Supported || l.BoundarySquaredDistance() != nil || l.UnsupportedReason == "") {
							t.Fatal("unsupported")
						}
					}
					distance := math.Sqrt(math.Pow(math.Hypot(c.p.X, c.p.Y)-3, 2) + c.p.Z*c.p.Z)
					want := math.Pow(distance-1, 2)
					if c.exact != "" {
						q, _ := new(big.Rat).SetString(c.exact)
						root, _, _ := geom.NewExactRationalRoot(ctx, q, geom.ExactCurvePlaneEventOptions{})
						cmp, e := leaf.BoundarySquaredDistance().Compare(ctx, root, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !cmp.Proven || cmp.Order != 0 {
							t.Fatal("exact value", e)
						}
					}

					if c.name == "nested-irrational" {
						sign, e := leaf.BoundarySquaredDistance().SignPolynomial(ctx, []*big.Rat{big.NewRat(15376, 1), big.NewRat(-9088, 1), big.NewRat(1272, 1), big.NewRat(-64, 1), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
						if e != nil || sign.Sign != 0 {
							t.Fatal("distance quartic", e)
						}
					}
					b, e := leaf.BoundarySquaredDistance().Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
					if e != nil || !b.Converged {
						t.Fatal(e)
					}
					lo, _ := b.Interval.Lower.Float64()
					hi, _ := b.Interval.Upper.Float64()
					if want < lo-1e-11 || want > hi+1e-11 {
						t.Fatal("distance interval", lo, hi, want)
					}
					if distance < 1 {
						zero, _, _ := geom.NewExactRationalRoot(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
						cmp, e := leaf.MaterialSquaredDistance().Compare(ctx, zero, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !cmp.Proven || cmp.Order != 0 || leaf.Location != AnalyticPointInside {
							t.Fatal("material")
						}
					}
					var nearest HierarchySphereNearestEnclosure
					if c.kind == "point" {
						nearest, e = leaf.NearestBoundary.EnclosePoint(ctx, big.NewRat(1, 10000000000), geom.ExactCurvePlaneEventOptions{})
						if e != nil {
							t.Fatal(e)
						}
						if c.name != "boundary" {
							if _, e = leaf.NearestBoundary.EnclosePoint(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); e == nil {
								t.Fatal("unresolved point enclosure quota")
							}
						}
						if _, e = leaf.NearestBoundary.EncloseCircle(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{}); e != geom.ErrDomain {
							t.Fatal("not circle")
						}
					} else {
						circle, e := leaf.NearestBoundary.EncloseCircle(ctx, big.NewRat(1, 10000000000), geom.ExactCurvePlaneEventOptions{})
						if e != nil {
							t.Fatal(e)
						}
						norm := new(big.Rat)
						for _, q := range circle.Axis {
							norm.Add(norm, new(big.Rat).Mul(q, q))
						}
						if norm.Cmp(big.NewRat(1, 1)) != 0 || circle.RadiusLower.Sign() <= 0 {
							t.Fatal("circle proof")
						}
						if _, e = leaf.NearestBoundary.EnclosePoint(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{}); e != geom.ErrDomain {
							t.Fatal("arbitrary circle witness")
						}
						radius := 1.
						centerLocal := c.p
						if c.kind == "axis-circle" {
							radius = 3 - 3/distance
							centerLocal = geom.Vec3{Z: c.p.Z / distance}
						}
						l, _ := circle.RadiusLower.Float64()
						h, _ := circle.RadiusUpper.Float64()
						if radius < l-1e-10 || radius > h+1e-10 {
							t.Fatal("circle radius", l, h, radius)
						}
						world := torus.Frame.Origin().Add(torus.Frame.XAxis().Scale(centerLocal.X)).Add(torus.Frame.YAxis().Scale(centerLocal.Y)).Add(torus.Frame.ZAxis().Scale(centerLocal.Z))
						mat := u.Matrix()
						coords := []float64{world.X, world.Y, world.Z}
						for i := 0; i < 3; i++ {
							v, _ := mat[i][3].Float64()
							for j := 0; j < 3; j++ {
								a, _ := mat[i][j].Float64()
								v += a * coords[j]
							}
							lo, _ := circle.CenterLower[i].Float64()
							hi, _ := circle.CenterUpper[i].Float64()
							if v < lo-1e-10 || v > hi+1e-10 {
								t.Fatal("circle center")
							}
						}
					}
					// Exact retained scale equations, independent from the numeric oracle.
					local := leaf.NearestBoundary.LocalQuery()
					s := new(big.Rat).Add(new(big.Rat).Mul(local[0], local[0]), new(big.Rat).Mul(local[1], local[1]))
					A := new(big.Rat).Add(new(big.Rat).Add(s, new(big.Rat).Mul(local[2], local[2])), big.NewRat(9, 1))
					if root := leaf.NearestBoundary.InverseRadialRoot(); root != nil {
						sign, e := root.SignPolynomial(ctx, []*big.Rat{big.NewRat(-1, 1), new(big.Rat), s}, geom.ExactCurvePlaneEventOptions{})
						if e != nil || sign.Sign != 0 {
							t.Fatal("radial equation", e)
						}
					}
					if root := leaf.NearestBoundary.TubeScaleRoot(); root != nil {
						coeff := []*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat).Mul(big.NewRat(-2, 1), A), new(big.Rat), new(big.Rat).Sub(new(big.Rat).Mul(A, A), new(big.Rat).Mul(big.NewRat(36, 1), s))}
						sign, e := root.SignPolynomial(ctx, coeff, geom.ExactCurvePlaneEventOptions{})
						if e != nil || sign.Sign != 0 {
							t.Fatal("tube equation", e)
						}
					}
					radius := leaf.NearestBoundary.MinorRadius()
					radius.SetInt64(999)
					if leaf.NearestBoundary.MinorRadius().Cmp(big.NewRat(1, 1)) != 0 {
						t.Fatal("mutable")
					}
					if out, e := distanceHierarchyTori(ctx, m, point, RationalHierarchyTorusDistanceOptions{MaxOperations: 1}, vendor); e == nil || len(out.Leaves) != 0 {
						t.Fatal("quota")
					}
					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE"); bin != "" {
						args := []string{"testdata/" + name}
						for _, q := range point {
							v, _ := q.Float64()
							args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
						}
						raw, e := exec.Command(bin, args...).Output()
						if e != nil {
							t.Fatal(e)
						}
						var rows []struct {
							Volume, BoundaryDistance, MaterialDistance float64
							Points                                     [][3]float64
						}
						if e = json.Unmarshal(raw, &rows); e != nil {
							t.Fatal(e)
						}
						found := false
						for _, row := range rows {
							if math.Abs(row.Volume-6*math.Pi*math.Pi) > 1e-6 || math.Abs(row.BoundaryDistance*row.BoundaryDistance-want) > 1e-6 {
								continue
							}
							md := want
							if distance < 1 {
								md = 0
							}
							if math.Abs(row.MaterialDistance*row.MaterialDistance-md) > 1e-6 {
								continue
							}
							if c.kind != "point" {
								found = true
								continue
							}
							for _, p := range row.Points {
								ok := true
								for i, v := range p {
									lo, _ := nearest.Lower[i].Float64()
									hi, _ := nearest.Upper[i].Float64()
									if v < lo-1e-6 || v > hi+1e-6 {
										ok = false
									}
								}
								if ok {
									found = true
								}
							}
						}
						if !found {
							t.Fatal("OCCT", rows)
						}
					}
				})
			}
		})
	}
}
func TestHierarchyDistanceFourFamilyIntegration(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1)}
	planar, e := DistanceRationalHierarchyPlanarMaterials(ctx, m, p, RationalHierarchyPlanarDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sphere, e := DistanceRationalHierarchySpheres(ctx, m, p, RationalHierarchySphereDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cylinder, e := DistanceRationalHierarchyCylinders(ctx, m, p, RationalHierarchyCylinderDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	torus, e := DistanceRationalHierarchyTori(ctx, m, p, RationalHierarchyTorusDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, u := range m.Instances {
		count := 0
		for _, ok := range []bool{planar.Leaves[i].Supported, sphere.Leaves[i].Supported, cylinder.Leaves[i].Supported, torus.Leaves[i].Supported} {
			if ok {
				count++
			}
		}
		if count != 1 || planar.Leaves[i].ProductID != u.ProductID || sphere.Leaves[i].ProductID != u.ProductID || cylinder.Leaves[i].ProductID != u.ProductID || torus.Leaves[i].ProductID != u.ProductID {
			t.Fatal("family routing/provenance")
		}
	}
}

func TestHierarchyTorusDistanceGuardsAndRationalMeridian(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	index := -1
	for i, u := range m.Instances {
		if m.Products[u.ProductID].Torus != nil {
			index = i
			break
		}
	}
	u := m.Instances[index]
	torus := m.Products[u.ProductID].Torus
	axes := [3][3]*big.Rat{mappedRatVec(torus.Frame.XAxis()), mappedRatVec(torus.Frame.YAxis()), mappedRatVec(torus.Frame.ZAxis())}
	local := [3]*big.Rat{big.NewRat(9, 5), big.NewRat(12, 5), new(big.Rat)}
	p := mappedRatVec(torus.Frame.Origin())
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			p[i].Add(p[i], new(big.Rat).Mul(axes[j][i], local[j]))
		}
	}
	matrix := u.Matrix()
	var world [3]*big.Rat
	for i := range world {
		world[i] = new(big.Rat).Set(matrix[i][3])
		for j := 0; j < 3; j++ {
			world[i].Add(world[i], new(big.Rat).Mul(matrix[i][j], p[j]))
		}
	}
	r, e := DistanceRationalHierarchyTori(ctx, m, world, RationalHierarchyTorusDistanceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	set := r.Leaves[index].NearestBoundary
	if set.Kind() != "meridian-circle" {
		t.Fatal("rational centerline")
	}
	circle, e := set.EncloseCircle(ctx, big.NewRat(1, 1000), geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range circle.CenterLower {
		if q.Cmp(world[i]) != 0 || q.Cmp(circle.CenterUpper[i]) != 0 {
			t.Fatal("exact circle center")
		}
	}
	norm := new(big.Rat)
	for _, q := range circle.Axis {
		norm.Add(norm, new(big.Rat).Mul(q, q))
	}
	if norm.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("proper circle")
	}
	for _, o := range []RationalHierarchyTorusDistanceOptions{{MaxOperations: 1}, {MaxCoefficientBits: 255}, {Source: Limits{MaxEntities: 1}}} {
		if out, e := DistanceRationalHierarchyTori(ctx, m, world, o); e == nil || len(out.Leaves) != 0 {
			t.Fatal("limits")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DistanceRationalHierarchyTori(c, m, world, RationalHierarchyTorusDistanceOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e = set.EncloseCircle(c, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if out, e := DistanceRationalHierarchyTori(ctx, m, world, RationalHierarchyTorusDistanceOptions{}); e == nil || len(out.Leaves) != 0 {
		t.Fatal("forged ancestry")
	}
}
