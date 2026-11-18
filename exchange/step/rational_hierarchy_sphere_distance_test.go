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

func TestHierarchySphereDistance(t *testing.T) {
	t.Parallel()
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
			var persisted bytes.Buffer
			if vendor {
				if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &persisted, OCCTRationalSphericalHierarchy(m)); e != nil {
					t.Fatal(e)
				}
				a, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(persisted.Bytes()), IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				m = RationalSphericalHierarchy(a)
			} else {
				if e = WriteRationalSphericalHierarchyJSON(ctx, &persisted, m); e != nil {
					t.Fatal(e)
				}
				m, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(persisted.Bytes()), IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
			}
			index := -1
			for i, u := range m.Instances {
				if m.Products[u.ProductID].Sphere != nil {
					index = i
					break
				}
			}
			u := m.Instances[index]
			sphere := m.Products[u.ProductID].Sphere
			type sample struct {
				name      string
				p         [3]int64
				s         int64
				bd        int64
				location  AnalyticPointLocation
				nonunique bool
			}
			cases := []sample{{"outside-rational", [3]int64{3, 0, 0}, 9, 1, AnalyticPointOutside, false}, {"inside-rational", [3]int64{1, 0, 0}, 1, 1, AnalyticPointInside, false}, {"boundary", [3]int64{2, 0, 0}, 4, 0, AnalyticPointBoundary, false}, {"center", [3]int64{}, 0, 4, AnalyticPointInside, true}, {"outside-irrational", [3]int64{3, 1, 0}, 10, -1, AnalyticPointOutside, false}, {"inside-irrational", [3]int64{1, 1, 0}, 2, -1, AnalyticPointInside, false}}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					point, _ := hierarchyTestWorldLine(u, sphere.Frame, c.p, [3]int64{1, 0, 0})
					query := func(o RationalHierarchySphereDistanceOptions) (RationalHierarchySphereDistance, error) {
						return distanceHierarchySpheres(ctx, m, point, o, vendor)
					}
					result, e := query(RationalHierarchySphereDistanceOptions{})
					if e != nil {
						t.Fatal(e)
					}
					leaf := result.Leaves[index]
					if !leaf.Supported || leaf.Location != c.location || leaf.SourceFaceID != sphere.SourceFaceID || leaf.NearestBoundary.Nonunique != c.nonunique {
						t.Fatal("classification/set")
					}
					if result.Operations <= 0 {
						t.Fatal("work")
					}
					for i, l := range result.Leaves {
						if m.Products[m.Instances[i].ProductID].Sphere == nil && (l.Supported || l.BoundarySquaredDistance() != nil || l.MaterialSquaredDistance() != nil || l.UnsupportedReason == "") {
							t.Fatal("unsupported")
						}
					}
					root := leaf.BoundarySquaredDistance()
					if c.bd >= 0 {
						expected, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(c.bd, 1), geom.ExactCurvePlaneEventOptions{})
						cmp, e := root.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !cmp.Proven || cmp.Order != 0 {
							t.Fatal("exact distance", e)
						}
					} else {
						coeff := []*big.Rat{big.NewRat((c.s-4)*(c.s-4), 1), big.NewRat(-2*(c.s+4), 1), big.NewRat(1, 1)}
						sign, e := root.SignPolynomial(ctx, coeff, geom.ExactCurvePlaneEventOptions{})
						if e != nil || sign.Sign != 0 {
							t.Fatal("distance equation", e)
						}
					}
					if c.location == AnalyticPointInside {
						zero, _, _ := geom.NewExactRationalRoot(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
						cmp, e := leaf.MaterialSquaredDistance().Compare(ctx, zero, geom.ExactCurvePlaneEventOptions{})
						if e != nil || !cmp.Proven || cmp.Order != 0 {
							t.Fatal("material0", e)
						}
					}
					enc, e := root.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
					if e != nil || !enc.Converged {
						t.Fatal(e)
					}
					lo, _ := enc.Interval.Lower.Float64()
					hi, _ := enc.Interval.Upper.Float64()
					want := math.Pow(math.Sqrt(float64(c.s))-2, 2)
					if want < lo-1e-12 || want > hi+1e-12 {
						t.Fatal("distance bound")
					}
					var nearest HierarchySphereNearestEnclosure
					if c.nonunique {
						if leaf.NearestBoundary.RadialScale() != nil || leaf.NearestBoundary.Radius().Cmp(big.NewRat(2, 1)) != 0 {
							t.Fatal("entire sphere")
						}
						if _, e = leaf.NearestBoundary.Enclose(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{}); e != geom.ErrDomain {
							t.Fatal("arbitrary center witness", e)
						}
					} else {
						scale := leaf.NearestBoundary.RadialScale()
						sign, e := scale.SignPolynomial(ctx, []*big.Rat{big.NewRat(-4, 1), new(big.Rat), big.NewRat(c.s, 1)}, geom.ExactCurvePlaneEventOptions{})
						if e != nil || sign.Sign != 0 {
							t.Fatal("exact witness equation", e)
						}
						nearest, e = leaf.NearestBoundary.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
						if e != nil {
							t.Fatal(e)
						}
						for i := range nearest.Lower {
							if new(big.Rat).Sub(nearest.Upper[i], nearest.Lower[i]).Cmp(big.NewRat(1, 1000000000000)) > 0 {
								t.Fatal("width")
							}
						}
						if _, e = leaf.NearestBoundary.Enclose(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); e == nil {
							t.Fatal("enclosure budget")
						}
					}
					center := leaf.NearestBoundary.Center()
					center[0].SetInt64(999)
					radius := leaf.NearestBoundary.Radius()
					radius.SetInt64(999)
					if leaf.NearestBoundary.Radius().Cmp(big.NewRat(2, 1)) != 0 || leaf.NearestBoundary.Center()[0].Cmp(big.NewRat(999, 1)) == 0 {
						t.Fatal("mutable witness")
					}
					if _, e = query(RationalHierarchySphereDistanceOptions{MaxOperations: 1}); e == nil {
						t.Fatal("cumulative budget")
					}
					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE"); bin != "" && !c.nonunique {
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
						matched := false
						for _, row := range rows {
							if math.Abs(row.Volume-32*math.Pi/3) > 1e-6 || math.Abs(row.BoundaryDistance*row.BoundaryDistance-want) > 1e-6 {
								continue
							}
							material := want
							if c.location == AnalyticPointInside {
								material = 0
							}
							if math.Abs(row.MaterialDistance*row.MaterialDistance-material) > 1e-6 {
								continue
							}
							for _, p := range row.Points {
								ok := true
								for i, v := range p {
									l, _ := nearest.Lower[i].Float64()
									h, _ := nearest.Upper[i].Float64()
									if v < l-1e-6 || v > h+1e-6 {
										ok = false
									}
								}
								if ok {
									matched = true
								}
							}
						}
						if !matched {
							t.Fatal("OCCT nearest/distance", rows)
						}
					}
				})
			}
		})
	}
}
func TestHierarchySphereDistanceGuards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for _, o := range []RationalHierarchySphereDistanceOptions{{MaxCoefficientBits: 255}, {MaxSubdivisions: -1}, {Source: Limits{MaxEntities: 1}}} {
		if out, e := DistanceRationalHierarchySpheres(ctx, m, p, o); e == nil || len(out.Leaves) != 0 {
			t.Fatal("limit")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DistanceRationalHierarchySpheres(c, m, p, RationalHierarchySphereDistanceOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if out, e := DistanceRationalHierarchySpheres(ctx, m, p, RationalHierarchySphereDistanceOptions{}); e == nil || len(out.Leaves) != 0 {
		t.Fatal("forged source")
	}
}
