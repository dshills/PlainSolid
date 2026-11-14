package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep"
	"plainsolid/geom"
	"strconv"
	"testing"
)

func TestGeneralPlanarHierarchyReentrant(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		name := "rational-spherical-hierarchy-reentrant.step"
		if vendor {
			name = "occt-rational-spherical-hierarchy-reentrant.step"
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
				reloaded, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				m = RationalSphericalHierarchy(reloaded)
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
			for k, u := range m.Instances {
				if m.Products[u.ProductID].Planar != nil {
					index = k
				}
			}
			for _, c := range []struct {
				x, y, z                string
				want                   AnalyticPointLocation
				faces, edges, vertices int
			}{{"-19/2", "-2", "0", AnalyticPointInside, 0, 0, 0}, {"-8", "-7/2", "0", AnalyticPointInside, 0, 0, 0}, {"-8", "-2", "0", AnalyticPointOutside, 0, 0, 0}, {"-9", "-3", "0", AnalyticPointBoundary, 2, 1, 0}, {"-9", "-3", "-2", AnalyticPointBoundary, 3, 3, 1}, {"-8", "-3", "0", AnalyticPointBoundary, 1, 0, 0}, {"-10", "-4", "-2", AnalyticPointBoundary, 3, 3, 1}, {"-11", "-4", "-2", AnalyticPointOutside, 0, 0, 0}} {
				var point [3]*big.Rat
				for i, s := range []string{c.x, c.y, c.z} {
					point[i], _ = new(big.Rat).SetString(s)
				}
				r, e := locateHierarchyAnalyticPoint(ctx, m, point, RationalHierarchyPointOptions{}, vendor, true, true)
				if e != nil {
					t.Fatal(c, e)
				}
				p := r.Leaves[index]
				if p.Location != c.want || len(p.ContactFaceIDs) != c.faces || len(p.ContactEdgeIDs) != c.edges || len(p.ContactVertexIDs) != c.vertices {
					t.Fatal(c, p)
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_LOCATION"); bin != "" {
					args := []string{"testdata/" + name}
					for _, q := range point {
						v, _ := q.Float64()
						args = append(args, strconv.FormatFloat(v, 'g', 17, 64))
					}
					raw, e := exec.Command(bin, args...).Output()
					if e != nil {
						t.Fatal(e)
					}
					var ext struct{ Inside, Boundary, Outside int }
					if e = json.Unmarshal(raw, &ext); e != nil {
						t.Fatal(e)
					}
					inside, boundary, outside := 0, 0, 0
					for _, v := range r.Leaves {
						switch v.Location {
						case AnalyticPointInside:
							inside++
						case AnalyticPointBoundary:
							boundary++
						case AnalyticPointOutside:
							outside++
						}
					}
					if ext.Inside != inside || ext.Boundary != boundary || ext.Outside != outside {
						t.Fatal("oracle", ext, inside, boundary, outside)
					}
				}
				old, e := locateHierarchyAnalyticPoint(ctx, m, point, RationalHierarchyPointOptions{}, vendor, true)
				if e != nil || old.Leaves[index].Location != AnalyticPointUnsupported {
					t.Fatal("convex mode widened", e)
				}
			}
			degenerate := [3]*big.Rat{big.NewRat(-19, 2), big.NewRat(-7, 2), big.NewRat(-3, 2)}
			r, e := locateHierarchyAnalyticPoint(ctx, m, degenerate, RationalHierarchyPointOptions{}, vendor, true, true)
			if e != nil || r.Leaves[index].Location != AnalyticPointInside || r.RayDirections < 2 {
				t.Fatal("ray retry", r.RayDirections, e)
			}
			if out, e := locateHierarchyAnalyticPoint(ctx, m, degenerate, RationalHierarchyPointOptions{MaxRayDirections: 1}, vendor, true, true); e == nil || len(out.Leaves) != 0 {
				t.Fatal("degenerate ray accepted")
			}
			if out, e := locateHierarchyAnalyticPoint(ctx, m, degenerate, RationalHierarchyPointOptions{MaxPlanarTests: r.PlanarTests - 1}, vendor, true, true); e == nil || len(out.Leaves) != 0 {
				t.Fatal("work limit")
			}

		})
	}
}

func TestGeneralPlanarFiniteHoles(t *testing.T) {
	ctx := context.Background()
	outer := []geom.Vec3{{X: -10, Y: -4, Z: -2}, {X: -6, Y: -4, Z: -2}, {X: -6, Y: 0, Z: -2}, {X: -10, Y: 0, Z: -2}}
	hole := []geom.Vec3{{X: -9, Y: -3, Z: -2}, {X: -9, Y: -1, Z: -2}, {X: -7, Y: -1, Z: -2}, {X: -7, Y: -3, Z: -2}}
	solid, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = WriteAdvanced(&raw, Model{Bodies: []Body{{Name: "hole", Solid: solid}}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var id int64
	for _, n := range f.Order {
		for _, r := range f.Entities[n].Records {
			if r.Type == "MANIFOLD_SOLID_BREP" {
				id = n
			}
		}
	}
	body, e := readBody(f, id, 100, 100, 1000, true)
	if e != nil {
		t.Fatal(e)
	}
	br, _ := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	shell, _ := simple(f, br.Args[1].Ref, "CLOSED_SHELL", 2)
	faces, _ := listRefs(shell.Args[1], 1)
	a := hierarchyPointArithmetic{bits: 4096}
	tick := func() error { return ctx.Err() }
	proof, e := prepareHierarchyGeneralPlanar(ctx, IndependentExactMaterial{Source: f, Planar: &body}, faces, &a, tick)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range []struct {
		x, y, z  string
		want     AnalyticPointLocation
		contacts int
	}{{"-8", "-2", "-2", AnalyticPointOutside, 0}, {"-8", "-2", "0", AnalyticPointOutside, 0}, {"-19/2", "-2", "-2", AnalyticPointBoundary, 1}, {"-9", "-2", "0", AnalyticPointBoundary, 1}, {"-19/2", "-2", "0", AnalyticPointInside, 0}} {
		var point [3]*big.Rat
		for i, s := range []string{c.x, c.y, c.z} {
			point[i], _ = new(big.Rat).SetString(s)
		}
		where, fs, _, _, _, e := proof.classify(point, &a, tick, 64)
		if e != nil || where != c.want || len(fs) != c.contacts {
			t.Fatal(c, where, fs, e)
		}
	}

	// The transverse line crosses both cap-hole walls without filling the hole.
	zero := func() *big.Rat { return new(big.Rat) }
	u := RationalHierarchyInstance{pose: mappedRationalBasis{Origin: [3]*big.Rat{zero(), zero(), zero()}, X: [3]*big.Rat{big.NewRat(1, 1), zero(), zero()}, Y: [3]*big.Rat{zero(), big.NewRat(1, 1), zero()}, Z: [3]*big.Rat{zero(), zero(), big.NewRat(1, 1)}}}
	ledger := RationalHierarchyLine{}
	opts := func() (geom.ExactCurvePlaneEventOptions, error) {
		return geom.ExactCurvePlaneEventOptions{MaxOperations: 2000000 - ledger.Operations, MaxCoefficientBits: 4096, MaxSubdivisions: 4096}, nil
	}
	add := func(root *geom.ExactAlgebraicRoot, owner HierarchyLineOwner) error {
		ledger.Hits = append(ledger.Hits, HierarchyLineHit{Parameter: root, Owners: []HierarchyLineOwner{owner}})
		return nil
	}
	leaf, e := clipHierarchyGeneralPlanar(ctx, HierarchyLineLeaf{Owner: HierarchyLineOwner{SourceFaceIDs: faces}}, proof, u, [3]*big.Rat{big.NewRat(-11, 1), big.NewRat(-2, 1), zero()}, [3]*big.Rat{big.NewRat(1, 1), zero(), zero()}, zero(), big.NewRat(6, 1), &a, &ledger, 65536, 256, opts, add)
	if e != nil || !leaf.Supported || len(leaf.Spans) != 2 || len(ledger.Hits) != 4 {
		t.Fatal("holed line", leaf, e)
	}
	for i, span := range leaf.Spans {
		for j, root := range []*geom.ExactAlgebraicRoot{span.Lower, span.Upper} {
			want := [][2]int64{{1, 2}, {4, 5}}[i][j]
			expected, _, _ := geom.NewExactRationalRoot(ctx, big.NewRat(want, 1), geom.ExactCurvePlaneEventOptions{})
			order, e := root.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !order.Proven || order.Order != 0 {
				t.Fatal("holed interval", want, e)
			}
		}
	}
	// The signed source inner-loop use cannot be reversed independently.
	face, _ := simple(f, faces[0], "ADVANCED_FACE", 4)
	bounds, _ := listRefs(face.Args[1], 1)
	if len(bounds) < 2 {
		t.Fatal("holed cap fixture")
	}
	en := f.Entities[bounds[1]]
	v := &en.Records[0].Args[2]
	if v.Text == "T" {
		v.Text = "F"
	} else {
		v.Text = "T"
	}
	f.Entities[bounds[1]] = en
	if _, e = prepareHierarchyGeneralPlanar(ctx, IndependentExactMaterial{Source: f, Planar: &body}, faces, &a, tick); e == nil {
		t.Fatal("reversed hole source accepted")
	}
}
