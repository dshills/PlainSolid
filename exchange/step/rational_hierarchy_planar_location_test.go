package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
)

func TestRationalHierarchyPlanarPoint(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy.step"
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := os.ReadFile("testdata/" + name)
			m, e := readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
			if e != nil {
				t.Fatal(e)
			}
			before, _ := mappedPlanarSourceBytes(m.Source)
			index := -1
			for i, u := range m.Instances {
				if m.Products[u.ProductID].Planar != nil {
					index = i
					break
				}
			}
			u := m.Instances[index]
			cases := []struct {
				point                  [3]int64
				location               AnalyticPointLocation
				faces, edges, vertices int
			}{
				{[3]int64{-9, -2, 0}, AnalyticPointInside, 0, 0, 0}, {[3]int64{-10, -2, 0}, AnalyticPointBoundary, 1, 0, 0}, {[3]int64{-10, -4, 0}, AnalyticPointBoundary, 2, 1, 0}, {[3]int64{-10, -4, -2}, AnalyticPointBoundary, 3, 3, 1}, {[3]int64{-11, -4, -2}, AnalyticPointOutside, 0, 0, 0}}
			for _, c := range cases {
				matrix := u.Matrix()
				var world [3]*big.Rat
				for i := range world {
					world[i] = new(big.Rat).Set(matrix[i][3])
					for j := 0; j < 3; j++ {
						world[i].Add(world[i], new(big.Rat).Mul(matrix[i][j], big.NewRat(c.point[j], 1)))
					}
				}
				query := func(o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
					if vendor {
						return LocateOCCTRationalHierarchyPoint(ctx, OCCTRationalSphericalHierarchy(m), world, o)
					}
					return LocateRationalHierarchyPoint(ctx, m, world, o)
				}
				r, e := query(RationalHierarchyPointOptions{})
				if e != nil {
					t.Fatal(e)
				}
				p := r.Leaves[index]
				if p.Location != c.location || len(p.ContactFaceIDs) != c.faces || len(p.ContactEdgeIDs) != c.edges || len(p.ContactVertexIDs) != c.vertices {
					t.Fatal("classification", p)
				}
				if !reflect.DeepEqual(p.Path, u.Path) {
					t.Fatal("path")
				}
				coords := p.ProductCoordinates()
				for i, q := range coords {
					if q.Cmp(big.NewRat(c.point[i], 1)) != 0 {
						t.Fatal("local coordinate")
					}
				}
				coords[0].SetInt64(123)
				if p.ProductCoordinates()[0].Cmp(big.NewRat(c.point[0], 1)) != 0 {
					t.Fatal("mutable coordinates")
				}
				if len(p.ContactFaceIDs) > 0 {
					p.ContactFaceIDs[0] = 999
				}
				again, e := query(RationalHierarchyPointOptions{})
				if e != nil || again.Leaves[index].Location != c.location {
					t.Fatal(e)
				}
				for _, id := range again.Leaves[index].ContactFaceIDs {
					if id == 999 {
						t.Fatal("mutable proof")
					}
				}
				if out, e := query(RationalHierarchyPointOptions{MaxPlanarTests: r.PlanarTests - 1}); e == nil || len(out.Leaves) != 0 {
					t.Fatal("budget")
				}
				legacy, e := locateHierarchyAnalyticPoint(ctx, m, world, RationalHierarchyPointOptions{}, vendor)
				if e != nil || legacy.Leaves[index].Location != AnalyticPointUnsupported {
					t.Fatal("old API widened")
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION"); bin != "" {
					args := []string{"testdata/" + name}
					for _, q := range world {
						f, _ := q.Float64()
						args = append(args, strconv.FormatFloat(f, 'g', 17, 64))
					}
					raw, e := exec.Command(bin, args...).Output()
					if e != nil {
						t.Fatal(e)
					}
					var external struct{ Inside, Boundary, Outside int }
					if e = json.Unmarshal(raw, &external); e != nil {
						t.Fatal(e)
					}
					inside, boundary, outside := 0, 0, 0
					for _, p := range r.Leaves {
						switch p.Location {
						case AnalyticPointInside:
							inside++
						case AnalyticPointBoundary:
							boundary++
						case AnalyticPointOutside:
							outside++
						default:
							t.Fatal("unexpected unsupported fixture")
						}
					}
					if external.Inside != inside || external.Boundary != boundary || external.Outside != outside {
						t.Fatal("oracle", external, inside, boundary, outside)
					}
				}
			}
			after, _ := mappedPlanarSourceBytes(m.Source)
			if !bytes.Equal(before, after) {
				t.Fatal("source mutation")
			}
			// A nonbinary represented rational displacement is not snapped onto a face.
			q := [3]*big.Rat{new(big.Rat).Add(big.NewRat(-10, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 200))), big.NewRat(-2, 1), new(big.Rat)}
			r, e := locateHierarchyAnalyticPoint(ctx, m, q, RationalHierarchyPointOptions{}, vendor, true)
			if e != nil || r.Leaves[index].Location != AnalyticPointInside {
				t.Fatal("near face", e)
			}
		})
	}
}
