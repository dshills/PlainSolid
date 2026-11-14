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

func TestHierarchyCylinderDistance(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		for _, thin := range []bool{false, true} {
			name := "rational-spherical-hierarchy"
			if vendor {
				name = "occt-" + name
			}
			if thin {
				name += "-thin-cylinder"
			}
			name += ".step"
			t.Run(name, func(t *testing.T) {
				raw, e := os.ReadFile("testdata/" + name)
				if e != nil {
					t.Fatal(e)
				}
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
					if m.Products[u.ProductID].Cylinder != nil {
						index = i
						break
					}
				}
				u := m.Instances[index]
				cyl := m.Products[u.ProductID].Cylinder
				type sample struct {
					name          string
					p             geom.Vec3
					distance      string
					numeric       float64
					sets, circles int
					inside        bool
				}
				samples := []sample{{"side", geom.Vec3{X: 3, Z: 1}, "1", 1, 1, 0, false}, {"cap", geom.Vec3{X: 1, Z: -1}, "1", 1, 1, 0, false}, {"rim-irrational", geom.Vec3{X: 3, Y: 1, Z: -1}, "", 15 - 4*math.Sqrt(10), 1, 0, false}, {"side-cap-tie", geom.Vec3{X: 1, Z: 1}, "1", 1, 2, 0, true}, {"three-point-tie", geom.Vec3{X: .5, Z: 1.5}, "9/4", 2.25, 3, 0, true}, {"cap-cap-axis", geom.Vec3{Z: 1.5}, "9/4", 2.25, 2, 0, true}, {"boundary-rim", geom.Vec3{X: 2}, "0", 0, 1, 0, false}, {"side-irrational", geom.Vec3{X: 1, Y: 1, Z: 1.5}, "", 6 - 4*math.Sqrt(2), 1, 0, true}}
				if thin {
					samples = []sample{{"axis-circle", geom.Vec3{Z: 1.5}, "1", 1, 1, 1, true}, {"circle-cap-tie", geom.Vec3{Z: 1}, "1", 1, 2, 1, true}, {"cap-center", geom.Vec3{}, "0", 0, 1, 0, false}, {"outside-rim", geom.Vec3{X: 2, Z: 4}, "2", 2, 1, 0, false}}
				}
				for _, c := range samples {
					t.Run(c.name, func(t *testing.T) {
						local := cyl.Frame.Origin().Add(cyl.Frame.XAxis().Scale(c.p.X)).Add(cyl.Frame.YAxis().Scale(c.p.Y)).Add(cyl.Frame.ZAxis().Scale(c.p.Z))
						point, e := u.ApplyExact(local)
						if e != nil {
							t.Fatal(e)
						}
						result, e := distanceHierarchyCylinders(ctx, m, point, RationalHierarchyCylinderDistanceOptions{}, vendor)
						if e != nil {
							t.Fatal(e)
						}
						leaf := result.Leaves[index]
						if !leaf.Supported || len(leaf.NearestBoundary) != c.sets {
							t.Fatal("sets", len(leaf.NearestBoundary), c.sets)
						}
						for i, l := range result.Leaves {
							if m.Products[m.Instances[i].ProductID].Cylinder == nil && (l.Supported || l.BoundarySquaredDistance() != nil || l.UnsupportedReason == "") {
								t.Fatal("unsupported")
							}
						}
						if c.distance != "" {
							q, _ := new(big.Rat).SetString(c.distance)
							want, _, _ := geom.NewExactRationalRoot(ctx, q, geom.ExactCurvePlaneEventOptions{})
							cmp, e := leaf.BoundarySquaredDistance().Compare(ctx, want, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !cmp.Proven || cmp.Order != 0 {
								t.Fatal("exact distance", e)
							}
						}
						if c.distance == "" {
							coeff := []*big.Rat{big.NewRat(4, 1), big.NewRat(-12, 1), big.NewRat(1, 1)}
							if c.name == "rim-irrational" {
								coeff[0] = big.NewRat(65, 1)
								coeff[1] = big.NewRat(-30, 1)
							}
							sign, e := leaf.BoundarySquaredDistance().SignPolynomial(ctx, coeff, geom.ExactCurvePlaneEventOptions{})
							if e != nil || sign.Sign != 0 {
								t.Fatal("exact irrational polynomial", e)
							}
						}
						bound, e := leaf.BoundarySquaredDistance().Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
						if e != nil || !bound.Converged {
							t.Fatal(e)
						}
						lo, _ := bound.Interval.Lower.Float64()
						hi, _ := bound.Interval.Upper.Float64()
						if c.numeric < lo-1e-12 || c.numeric > hi+1e-12 {
							t.Fatal("bounds", lo, hi, c.numeric)
						}
						if c.inside {
							zero, _, _ := geom.NewExactRationalRoot(ctx, new(big.Rat), geom.ExactCurvePlaneEventOptions{})
							cmp, e := leaf.MaterialSquaredDistance().Compare(ctx, zero, geom.ExactCurvePlaneEventOptions{})
							if e != nil || !cmp.Proven || cmp.Order != 0 || leaf.Location != AnalyticPointInside {
								t.Fatal("interior")
							}
						}
						circles := 0
						var enclosures []HierarchySphereNearestEnclosure
						for _, s := range leaf.NearestBoundary {
							if len(s.SourceFaceIDs) == 0 {
								t.Fatal("source owners")
							}
							for _, fid := range s.SourceFaceIDs {
								if _, e := simple(m.Source, fid, "ADVANCED_FACE", 4); e != nil {
									t.Fatal(e)
								}
							}
							if s.Kind == "circle" {
								circles++
								if s.Radius().Cmp(big.NewRat(1, 1)) != 0 || len(s.SourceFaceIDs) != cyl.Cells || s.RadialScale() != nil {
									t.Fatal("circle")
								}
								axis := s.Axis()
								norm := new(big.Rat)
								for _, q := range axis {
									norm.Add(norm, new(big.Rat).Mul(q, q))
								}
								if norm.Cmp(big.NewRat(1, 1)) != 0 {
									t.Fatal("circle axis")
								}
								if _, e = s.Enclose(ctx, big.NewRat(1, 100), geom.ExactCurvePlaneEventOptions{}); e != geom.ErrDomain {
									t.Fatal("arbitrary circle point")
								}
							} else {
								b, e := s.Enclose(ctx, big.NewRat(1, 1000000000000), geom.ExactCurvePlaneEventOptions{})
								if e != nil {
									t.Fatal(e)
								}
								enclosures = append(enclosures, b)
								if c.name == "boundary-rim" || c.name == "outside-rim" {
									if len(s.SourceFaceIDs) != 3 {
										t.Fatal("rim ownership", s.SourceFaceIDs)
									}
								}
							}
							r := s.Radius()
							r.SetInt64(999)
							if s.Radius().Cmp(big.NewRat(999, 1)) == 0 {
								t.Fatal("mutable")
							}
						}
						if circles != c.circles {
							t.Fatal("circle count")
						}
						if out, e := distanceHierarchyCylinders(ctx, m, point, RationalHierarchyCylinderDistanceOptions{MaxOperations: 1}, vendor); e == nil || len(out.Leaves) != 0 {
							t.Fatal("budget")
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
							volume := math.Pi * cyl.Radius * cyl.Radius * cyl.Height
							matched := false
							for _, row := range rows {
								if math.Abs(row.Volume-volume) > 1e-6 || math.Abs(row.BoundaryDistance*row.BoundaryDistance-c.numeric) > 1e-6 {
									continue
								}
								md := c.numeric
								if c.inside {
									md = 0
								}
								if math.Abs(row.MaterialDistance*row.MaterialDistance-md) > 1e-6 {
									continue
								}
								ok := true
								for _, b := range enclosures {
									found := false
									for _, p := range row.Points {
										yes := true
										for i, v := range p {
											l, _ := b.Lower[i].Float64()
											h, _ := b.Upper[i].Float64()
											if v < l-1e-6 || v > h+1e-6 {
												yes = false
											}
										}
										if yes {
											found = true
										}
									}
									if !found {
										ok = false
									}
								}
								if ok && circles == 0 && len(row.Points) != len(enclosures) {
									ok = false
								}
								if ok {
									matched = true
								}
							}
							if !matched {
								t.Fatal("OCCT", c.name, rows)
							}
						}
					})
				}
			})
		}
	}
}
func TestHierarchyCylinderDistanceGuards(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for _, o := range []RationalHierarchyCylinderDistanceOptions{{MaxCoefficientBits: 255}, {MaxSubdivisions: -1}, {Source: Limits{MaxEntities: 1}}} {
		if out, e := DistanceRationalHierarchyCylinders(ctx, m, p, o); e == nil || len(out.Leaves) != 0 {
			t.Fatal("limits")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DistanceRationalHierarchyCylinders(c, m, p, RationalHierarchyCylinderDistanceOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if out, e := DistanceRationalHierarchyCylinders(ctx, m, p, RationalHierarchyCylinderDistanceOptions{}); e == nil || len(out.Leaves) != 0 {
		t.Fatal("ancestry")
	}
}
