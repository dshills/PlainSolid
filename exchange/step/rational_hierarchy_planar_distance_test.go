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
	"plainsolid/internal/slowtest"
	"strconv"
	"testing"
)

func TestHierarchyPlanarDistance(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		for _, family := range []string{"reentrant", "affine-pcurve-holed"} {
			name := "rational-spherical-hierarchy-" + family + ".step"
			if vendor {
				name = "occt-" + name
			}
			t.Run(name, func(t *testing.T) {
				raw, _ := os.ReadFile("testdata/" + name)
				var m RationalSphericalHierarchy
				var query func([3]*big.Rat, RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error)
				if family == "affine-pcurve-holed" {
					a, e := readAffinePCurveHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					var b bytes.Buffer
					if e = WriteAffinePCurveHierarchyJSON(ctx, &b, a); e != nil {
						t.Fatal(e)
					}
					a, e = ReadAffinePCurveHierarchyJSON(ctx, bytes.NewReader(b.Bytes()), IndependentMaterialJSONLimits{})
					if e != nil {
						t.Fatal(e)
					}
					m = a.model
					query = func(p [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error) {
						return a.PlanarDistance(ctx, p, o)
					}
				} else {
					var e error
					m, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					query = func(p [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error) {
						return distanceHierarchyPlanar(ctx, m, p, o, vendor)
					}
				}
				index := -1
				for i, u := range m.Instances {
					if m.Products[u.ProductID].Planar != nil {
						index = i
					}
				}
				if index < 0 {
					t.Fatal("planar")
				}
				u := m.Instances[index]
				type sample struct {
					name                   string
					p                      geom.Vec3
					boundary, material     string
					nearest                []geom.Vec3
					faces, edges, vertices int
				}
				samples := []sample{{"face", geom.Vec3{X: -11, Y: -2}, "1", "1", []geom.Vec3{{X: -10, Y: -2}}, 1, 0, 0}, {"vertex", geom.Vec3{X: -11, Y: -5, Z: -3}, "3", "3", []geom.Vec3{{X: -10, Y: -4, Z: -2}}, 3, 3, 1}, {"boundary-edge", geom.Vec3{X: -9, Y: -3}, "0", "0", []geom.Vec3{{X: -9, Y: -3}}, 2, 1, 0}, {"inside", geom.Vec3{X: -9.5, Y: -3.5}, "1/4", "0", []geom.Vec3{{X: -10, Y: -3.5}, {X: -9.5, Y: -4}}, 1, 0, 0}}
				if family == "reentrant" {
					samples = append(samples, sample{"notch", geom.Vec3{X: -8, Y: -2}, "1", "1", []geom.Vec3{{X: -9, Y: -2}, {X: -8, Y: -3}}, 1, 0, 0}, sample{"above-notch", geom.Vec3{X: -8, Y: -2, Z: 3}, "2", "2", []geom.Vec3{{X: -9, Y: -2, Z: 2}, {X: -8, Y: -3, Z: 2}}, 2, 1, 0})
				} else {
					samples = append(samples, sample{"hole", geom.Vec3{X: -8, Y: -2}, "1", "1", []geom.Vec3{{X: -9, Y: -2}, {X: -7, Y: -2}, {X: -8, Y: -3}, {X: -8, Y: -1}}, 1, 0, 0}, sample{"above-hole", geom.Vec3{X: -8, Y: -2, Z: 3}, "2", "2", []geom.Vec3{{X: -9, Y: -2, Z: 2}, {X: -7, Y: -2, Z: 2}, {X: -8, Y: -3, Z: 2}, {X: -8, Y: -1, Z: 2}}, 2, 1, 0})
				}
				for _, s := range samples {
					t.Run(s.name, func(t *testing.T) {
						point, e := u.ApplyExact(s.p)
						if e != nil {
							t.Fatal(e)
						}
						r, e := query(point, RationalHierarchyPlanarDistanceOptions{})
						if e != nil {
							t.Fatal(e)
						}
						leaf := r.Leaves[index]
						bd, _ := new(big.Rat).SetString(s.boundary)
						md, _ := new(big.Rat).SetString(s.material)
						if !leaf.Supported || leaf.BoundarySquaredDistance().Cmp(bd) != 0 || leaf.MaterialSquaredDistance().Cmp(md) != 0 || len(leaf.NearestBoundary) != len(s.nearest) {
							t.Fatal("distance", leaf.BoundarySquaredDistance(), leaf.MaterialSquaredDistance(), len(leaf.NearestBoundary))
						}
						for i, l := range r.Leaves {
							if i != index && (l.Supported || l.BoundarySquaredDistance() != nil || l.MaterialSquaredDistance() != nil || l.UnsupportedReason == "") {
								t.Fatal("unsupported analytic")
							}
						}
						expected := map[string]bool{}
						key := func(p [3]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() + "," + p[2].RatString() }
						for _, p := range s.nearest {
							q, e := u.ApplyExact(p)
							if e != nil {
								t.Fatal(e)
							}
							expected[key(q)] = true
						}
						for _, n := range leaf.NearestBoundary {
							q := n.Point()
							if !expected[key(q)] {
								t.Fatal("point", q)
							}
							delete(expected, key(q))
							if len(n.SourceFaceIDs) != s.faces || len(n.SourceEdgeIDs) != s.edges || len(n.SourceVertexIDs) != s.vertices {
								t.Fatal("owners", n.SourceFaceIDs, n.SourceEdgeIDs, n.SourceVertexIDs)
							}
							for _, fid := range n.SourceFaceIDs {
								if _, e := simple(m.Source, fid, "ADVANCED_FACE", 4); e != nil {
									t.Fatal(e)
								}
							}
							q[0].SetInt64(999)
							if n.Point()[0].Cmp(big.NewRat(999, 1)) == 0 {
								t.Fatal("mutable point")
							}
						}
						if len(expected) != 0 {
							t.Fatal("missing ties")
						}
						q := leaf.BoundarySquaredDistance()
						q.SetInt64(999)
						if leaf.BoundarySquaredDistance().Cmp(bd) != 0 {
							t.Fatal("mutable distance")
						}
						if s.name == "above-hole" || s.name == "above-notch" {
							if out, e := query(point, RationalHierarchyPlanarDistanceOptions{MaxNearestPoints: 1}); e == nil || len(out.Leaves) != 0 {
								t.Fatal("tie limit")
							}
						}
						if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_DISTANCE"); bin != "" {
							args := []string{"testdata/" + name}
							for _, q := range point {
								f, _ := q.Float64()
								args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
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
								t.Fatal(e, string(raw))
							}
							volume := 48.
							if family == "reentrant" {
								volume = 28
							}
							found := false
							for _, row := range rows {
								if math.Abs(row.Volume-volume) > 1e-6 {
									continue
								}
								found = true
								b, _ := bd.Float64()
								d, _ := md.Float64()
								if math.Abs(row.BoundaryDistance*row.BoundaryDistance-b) > 1e-6 || math.Abs(row.MaterialDistance*row.MaterialDistance-d) > 1e-6 || len(row.Points) != len(leaf.NearestBoundary) {
									t.Fatal("OCCT distances/ties", row)
								}
								used := map[int]bool{}
								for _, n := range leaf.NearestBoundary {
									match := -1
									for i, p := range row.Points {
										if used[i] {
											continue
										}
										dist := 0.
										for j, q := range n.Point() {
											v, _ := q.Float64()
											dist += math.Abs(v - p[j])
										}
										if dist < 1e-6 {
											match = i
											break
										}
									}
									if match < 0 {
										t.Fatal("OCCT point", n.Point(), row.Points)
									}
									used[match] = true
								}
							}
							if !found {
								t.Fatal("missing OCCT product")
							}
						}
					})
				}
				p, _ := u.ApplyExact(geom.Vec3{X: -8, Y: -2})
				for _, o := range []RationalHierarchyPlanarDistanceOptions{{MaxCandidates: 1}, {MaxPlanarTests: 1}, {MaxCoefficientBits: 255}, {Source: Limits{MaxEntities: 1}}} {
					if got, e := query(p, o); e == nil || len(got.Leaves) != 0 {
						t.Fatal("budget")
					}
				}
			})
		}
	}
}
func TestHierarchyPlanarDistanceReject(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DistanceRationalHierarchyPlanarMaterials(c, m, p, RationalHierarchyPlanarDistanceOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if out, e := DistanceRationalHierarchyPlanarMaterials(ctx, m, p, RationalHierarchyPlanarDistanceOptions{}); e == nil || len(out.Leaves) != 0 {
		t.Fatal("ancestry")
	}
	if _, e = DistanceRationalHierarchyPlanarMaterials(ctx, m, [3]*big.Rat{}, RationalHierarchyPlanarDistanceOptions{}); e == nil {
		t.Fatal("nil coordinates")
	}
}
