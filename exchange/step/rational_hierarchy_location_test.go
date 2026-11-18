package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/geom"
	"reflect"
	"strconv"
	"testing"
)

func TestRationalHierarchyAnalyticPoint(t *testing.T) {
	t.Parallel()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			raw, _ := os.ReadFile("testdata/" + name)
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			query := func(p [3]*big.Rat, o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
				if vendor {
					return LocateOCCTRationalHierarchyAnalyticPoint(ctx, OCCTRationalSphericalHierarchy(m), p, o)
				}
				return LocateRationalHierarchyAnalyticPoint(ctx, m, p, o)
			}
			seen := map[string]bool{}
			for k, u := range m.Instances {
				p := m.Products[u.ProductID]
				var f geom.Frame
				var family string
				var points [][3]int64
				var states []AnalyticPointLocation
				switch {
				case p.Cylinder != nil:
					family = "cylinder"
					f = p.Cylinder.Frame
					points = [][3]int64{{0, 0, 1}, {2, 0, 0}, {0, 0, 3}, {3, 0, 1}, {2, 0, -1}}
					states = []AnalyticPointLocation{AnalyticPointInside, AnalyticPointBoundary, AnalyticPointBoundary, AnalyticPointOutside, AnalyticPointOutside}
				case p.Sphere != nil:
					family = "sphere"
					f = p.Sphere.Frame
					points = [][3]int64{{0, 0, 0}, {2, 0, 0}, {0, 0, -2}, {0, 3, 0}}
					states = []AnalyticPointLocation{AnalyticPointInside, AnalyticPointBoundary, AnalyticPointBoundary, AnalyticPointOutside}
				case p.Torus != nil:
					family = "torus"
					f = p.Torus.Frame
					points = [][3]int64{{3, 0, 0}, {4, 0, 0}, {2, 0, 0}, {0, 0, 0}, {3, 0, 2}}
					states = []AnalyticPointLocation{AnalyticPointInside, AnalyticPointBoundary, AnalyticPointBoundary, AnalyticPointOutside, AnalyticPointOutside}
				default:
					continue
				}
				if seen[family] {
					continue
				}
				seen[family] = true
				for n, coords := range points {
					local := mappedRatVec(f.Origin())
					axes := [3][3]*big.Rat{mappedRatVec(f.XAxis()), mappedRatVec(f.YAxis()), mappedRatVec(f.ZAxis())}
					for i := 0; i < 3; i++ {
						for j := 0; j < 3; j++ {
							local[i].Add(local[i], new(big.Rat).Mul(axes[j][i], big.NewRat(coords[j], 1)))
						}
					}
					mat := u.Matrix()
					var world [3]*big.Rat
					for i := 0; i < 3; i++ {
						world[i] = new(big.Rat).Set(mat[i][3])
						for j := 0; j < 3; j++ {
							world[i].Add(world[i], new(big.Rat).Mul(mat[i][j], local[j]))
						}
					}
					r, e := query(world, RationalHierarchyPointOptions{})
					if e != nil {
						t.Fatal(e)
					}
					if len(r.Leaves) != 7 {
						t.Fatal("paths")
					}
					leaf := r.Leaves[k]
					if leaf.Location != states[n] || !reflect.DeepEqual(leaf.Path, u.Path) || leaf.SourceBodyID != p.SourceBodyID {
						t.Fatal(family, n, leaf.Location)
					}
					for i, q := range leaf.AnalyticCoordinates() {
						if q.Cmp(big.NewRat(coords[i], 1)) != 0 {
							t.Fatal("transport", q, coords)
						}
					}
					if family == "cylinder" && n == 1 && !reflect.DeepEqual(leaf.BoundaryRoles, []string{"wall", "lower-cap"}) {
						t.Fatal("rim roles")
					}
					for _, v := range r.Leaves {
						if v.Family == "planar" && v.Location != AnalyticPointUnsupported {
							t.Fatal("planar claim")
						}
					}

					if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION"); bin != "" {
						// These selected points lie outside the unchanged planar box.
						if world[0].Cmp(big.NewRat(-10, 1)) >= 0 && world[0].Cmp(big.NewRat(-8, 1)) <= 0 {
							t.Fatal("oracle planar exclusion")
						}
						args := []string{"testdata/" + name}
						for _, q := range world {
							v, _ := q.Float64()
							args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
						}
						raw, e := exec.Command(bin, args...).Output()
						if e != nil {
							t.Fatal(e)
						}
						var got struct{ Inside, Boundary, Outside int }
						if e = json.Unmarshal(raw, &got); e != nil {
							t.Fatal(e)
						}
						inside, boundary := 0, 0
						for _, v := range r.Leaves {
							if v.Location == AnalyticPointInside {
								inside++
							}
							if v.Location == AnalyticPointBoundary {
								boundary++
							}
						}
						if got.Inside != inside || got.Boundary != boundary || got.Outside != 7-inside-boundary {
							t.Fatal("oracle", family, n, got, inside, boundary)
						}
					}
					q := leaf.AnalyticCoordinates()
					q[0].SetInt64(99)
					if leaf.AnalyticCoordinates()[0].Cmp(big.NewRat(coords[0], 1)) != 0 {
						t.Fatal("mutable")
					}
				}
			}
			point := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1)}
			m.Instances[0].Path[0]++
			if r, e := query(point, RationalHierarchyPointOptions{}); e == nil || len(r.Leaves) != 0 {
				t.Fatal("forged path")
			}
			m.Instances[0].Path[0]--
			point[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 300))
			if _, e := query(point, RationalHierarchyPointOptions{MaxCoefficientBits: 256}); e == nil {
				t.Fatal("bits")
			}
			point[0] = nil
			if _, e := query(point, RationalHierarchyPointOptions{}); e == nil {
				t.Fatal("nil scalar")
			}
		})
	}
}

func TestRationalHierarchyPointFailures(t *testing.T) {
	t.Parallel()
	p := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)}
	if r, e := LocateRationalHierarchyAnalyticPoint(nil, RationalSphericalHierarchy{}, p, RationalHierarchyPointOptions{}); e == nil || len(r.Leaves) != 0 {
		t.Fatal("nil context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := LocateRationalHierarchyAnalyticPoint(ctx, RationalSphericalHierarchy{}, p, RationalHierarchyPointOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
	for _, n := range []int{255, 8193, -1} {
		if _, e := LocateRationalHierarchyAnalyticPoint(context.Background(), RationalSphericalHierarchy{}, p, RationalHierarchyPointOptions{MaxCoefficientBits: n}); e == nil {
			t.Fatal("options")
		}
	}
	a := hierarchyPointArithmetic{bits: 256}
	q := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 200))
	_ = a.mul(q, q)
	if a.err == nil {
		t.Fatal("intermediate growth")
	}
}
