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

func TestHierarchyBoundaryNormals(t *testing.T) {
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
				var query func([3]*big.Rat, RationalHierarchyNormalOptions) (RationalHierarchyNormals, error)
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
					query = func(p [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
						return a.BoundaryNormals(ctx, p, o)
					}
				} else {
					var e error
					m, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					query = func(p [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
						return boundaryNormalsHierarchy(ctx, m, p, o, vendor)
					}
				}
				visited := map[string]bool{}
				for k, u := range m.Instances {
					p := m.Products[u.ProductID]
					kind := ""
					var frame geom.Frame
					type sample struct {
						point   [3]int64
						normals [][3]int64
						oracle  bool
					}
					var samples []sample
					switch {
					case p.Planar != nil:
						kind = "planar"
						frame, _ = geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
						samples = []sample{{[3]int64{-10, -4, -2}, [][3]int64{{-1, 0, 0}, {0, -1, 0}, {0, 0, -1}}, true}, {[3]int64{-10, -2, 0}, [][3]int64{{-1, 0, 0}}, true}, {[3]int64{-9, -3, 0}, [][3]int64{{1, 0, 0}, {0, 1, 0}}, true}}
						if family == "affine-pcurve-holed" {
							samples = append(samples, sample{[3]int64{-9, -2, 0}, [][3]int64{{1, 0, 0}}, true}, sample{[3]int64{-8, -2, 0}, nil, true})
						}
					case p.Cylinder != nil:
						kind = "cylinder"
						frame = p.Cylinder.Frame
						samples = []sample{{[3]int64{2, 0, 0}, [][3]int64{{1, 0, 0}, {1, 0, 0}, {0, 0, -1}}, true}, {[3]int64{0, 2, 1}, [][3]int64{{0, 1, 0}}, true}, {[3]int64{0, 0, 3}, [][3]int64{{0, 0, 1}}, true}}
						if p.Cylinder.Cells == 4 {
							samples[1].normals = append(samples[1].normals, [3]int64{0, 1, 0})
						}
					case p.Sphere != nil:
						kind = "sphere"
						frame = p.Sphere.Frame
						samples = []sample{{[3]int64{2, 0, 0}, [][3]int64{{1, 0, 0}}, true}, {[3]int64{0, 0, 2}, [][3]int64{{0, 0, 1}}, true}}
					case p.Torus != nil:
						kind = "torus"
						frame = p.Torus.Frame
						samples = []sample{{[3]int64{4, 0, 0}, [][3]int64{{1, 0, 0}}, true}, {[3]int64{2, 0, 0}, [][3]int64{{-1, 0, 0}}, true}, {[3]int64{3, 0, 1}, [][3]int64{{0, 0, 1}}, true}}
					}
					if visited[kind] {
						continue
					}
					visited[kind] = true
					for si, s := range samples {
						point, _ := hierarchyTestWorldLine(u, frame, s.point, [3]int64{1, 0, 0})
						r, e := query(point, RationalHierarchyNormalOptions{})
						if e != nil {
							t.Fatal(kind, si, e)
						}
						leaf := r.Leaves[k]
						if len(leaf.Normals) != len(s.normals) {
							t.Fatal(kind, si, "count", len(leaf.Normals), len(s.normals))
						}
						used := map[int]bool{}
						for _, n := range leaf.Normals {
							v := n.Vector()
							sq := new(big.Rat)
							for _, c := range v {
								sq.Add(sq, new(big.Rat).Mul(c, c))
							}
							if sq.Cmp(n.SquaredNorm()) != 0 || sq.Sign() <= 0 || n.SourceFaceID == 0 || n.SourceSurfaceID == 0 {
								t.Fatal("normal")
							}
							matched := false
							for j, w := range s.normals {
								if used[j] {
									continue
								}
								_, want := hierarchyTestWorldLine(u, frame, s.point, w)
								cross := [3]*big.Rat{}
								dot := new(big.Rat)
								for i := range cross {
									cross[i] = new(big.Rat).Sub(new(big.Rat).Mul(v[(i+1)%3], want[(i+2)%3]), new(big.Rat).Mul(v[(i+2)%3], want[(i+1)%3]))
									dot.Add(dot, new(big.Rat).Mul(v[i], want[i]))
								}
								if cross[0].Sign() == 0 && cross[1].Sign() == 0 && cross[2].Sign() == 0 && dot.Sign() > 0 {
									used[j] = true
									matched = true
									break
								}
							}
							if !matched {
								t.Fatal(kind, si, "direction", v)
							}
							v[0].SetInt64(999)
							sq.SetInt64(999)
							if n.Vector()[0].Cmp(big.NewRat(999, 1)) == 0 {
								t.Fatal("mutable")
							}
						}
						if len(s.normals) > 1 {
							if bad, e := query(point, RationalHierarchyNormalOptions{MaxNormals: 1}); e == nil || len(bad.Leaves) != 0 {
								t.Fatal("normal budget")
							}
						}
						if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_NORMALS"); bin != "" && s.oracle {
							args := []string{"testdata/" + name}
							for _, q := range point {
								f, _ := q.Float64()
								args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
							}
							raw, e := exec.Command(bin, args...).Output()
							if e != nil {
								t.Fatal(kind, si, e)
							}
							var external [][3]float64
							if e = json.Unmarshal(raw, &external); e != nil {
								t.Fatal(e, string(raw))
							}
							var native [][3]float64
							for _, l := range r.Leaves {
								for _, n := range l.Normals {
									var v [3]float64
									sum := 0.
									for i, q := range n.Vector() {
										v[i], _ = q.Float64()
										sum += v[i] * v[i]
									}
									for i := range v {
										v[i] /= math.Sqrt(sum)
									}
									native = append(native, v)
								}
							}
							if len(native) != len(external) {
								t.Fatal(kind, si, "OCCT count", native, external)
							}
							used := map[int]bool{}
							for _, v := range native {
								match := -1
								for j, w := range external {
									if used[j] {
										continue
									}
									dist := 0.
									for i := range v {
										dist += math.Abs(v[i] - w[i])
									}
									if dist < 1e-6 {
										match = j
										break
									}
								}
								if match < 0 {
									t.Fatal(kind, si, "OCCT direction", v, external)
								}
								used[match] = true
							}
						}
					}
				}
			})
		}
	}
}
func TestHierarchyNormalsReject(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	for _, o := range []RationalHierarchyNormalOptions{{MaxNormals: -1}, {Point: RationalHierarchyPointOptions{MaxCoefficientBits: 255}}, {Point: RationalHierarchyPointOptions{MaxPlanarTests: 1}}} {
		if got, e := BoundaryNormalsRationalHierarchy(ctx, m, p, o); e == nil || len(got.Leaves) != 0 {
			t.Fatal("limits")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = BoundaryNormalsRationalHierarchy(c, m, p, RationalHierarchyNormalOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if got, e := BoundaryNormalsRationalHierarchy(ctx, m, p, RationalHierarchyNormalOptions{}); e == nil || len(got.Leaves) != 0 {
		t.Fatal("forged source")
	}
}

func TestHierarchyRationalNonseamNormal(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for k, u := range m.Instances {
		p := m.Products[u.ProductID]
		if p.Cylinder == nil {
			continue
		}
		frame := p.Cylinder.Frame
		axes := [3][3]*big.Rat{mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis()), mappedRatVec(frame.ZAxis())}
		local := [3]*big.Rat{big.NewRat(6, 5), big.NewRat(8, 5), big.NewRat(1, 1)}
		v := mappedRatVec(frame.Origin())
		d := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				v[i].Add(v[i], new(big.Rat).Mul(axes[j][i], local[j]))
				if j < 2 {
					d[i].Add(d[i], new(big.Rat).Mul(axes[j][i], local[j]))
				}
			}
		}
		matrix := u.Matrix()
		var point, want [3]*big.Rat
		for i := 0; i < 3; i++ {
			point[i] = new(big.Rat).Set(matrix[i][3])
			want[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				point[i].Add(point[i], new(big.Rat).Mul(matrix[i][j], v[j]))
				want[i].Add(want[i], new(big.Rat).Mul(matrix[i][j], d[j]))
			}
		}
		result, e := BoundaryNormalsRationalHierarchy(ctx, m, point, RationalHierarchyNormalOptions{})
		if e != nil {
			t.Fatal(e)
		}
		leaf := result.Leaves[k]
		if len(leaf.Normals) != 1 {
			t.Fatal("nonseam ownership")
		}
		n := leaf.Normals[0]
		for i, c := range n.Vector() {
			if c.Cmp(want[i]) != 0 {
				t.Fatal("exact rational normal", c, want[i])
			}
		}
		if n.SourceFaceID != p.Cylinder.NativeFaceSourceIDs[2] || n.SquaredNorm().Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("source role")
		}
		break
	}
}
