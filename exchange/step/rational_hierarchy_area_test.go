package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestRationalHierarchyBoundaryArea(t *testing.T) {
	ctx := context.Background()
	for _, vendor := range []bool{false, true} {
		for _, family := range []string{"", "reentrant", "holed", "affine-pcurve-holed"} {
			name := "rational-spherical-hierarchy"
			if vendor {
				name = "occt-" + name
			}
			if family != "" {
				name += "-" + family
			}
			name += ".step"
			t.Run(name, func(t *testing.T) {
				raw, e := os.ReadFile("testdata/" + name)
				if e != nil {
					t.Fatal(e)
				}
				var m RationalSphericalHierarchy
				var query func(context.Context, RationalHierarchyAreaOptions) (RationalHierarchyArea, error)
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
					query = a.BoundaryArea
				} else {
					m, e = readRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9, vendor)
					if e != nil {
						t.Fatal(e)
					}
					query = func(ctx context.Context, o RationalHierarchyAreaOptions) (RationalHierarchyArea, error) {
						if vendor {
							return MeasureOCCTRationalHierarchyBoundaryArea(ctx, OCCTRationalSphericalHierarchy(m), o)
						}
						return MeasureRationalHierarchyBoundaryArea(ctx, m, o)
					}
				}
				r, e := query(ctx, RationalHierarchyAreaOptions{})
				if e != nil {
					t.Fatal(e)
				}
				planar := int64(52)
				if family == "reentrant" {
					planar = 78
				}
				if family == "holed" || family == "affine-pcurve-holed" {
					planar = 120
				}
				if len(r.Products) != 4 || len(r.Occurrences) != 7 || r.PlanarTests == 0 || r.LoopEdges == 0 {
					t.Fatal("work")
				}
				eq := func(p PiPolynomial, want [3]int64) {
					t.Helper()
					for i, c := range p.Coefficients() {
						w := new(big.Rat)
						if i < 3 {
							w.SetInt64(want[i])
						}
						if c.Cmp(w) != 0 {
							t.Fatal("area coefficient", i, c, w)
						}
					}
				}
				eq(r.Sum, [3]int64{planar, 72, 24})
				products := map[int64]HierarchyMaterialArea{}
				for _, p := range r.Products {
					products[p.ProductID] = p
					var sum PiPolynomial
					a := massArithmetic{bits: 4096}
					holes := 0
					for _, face := range p.Faces {
						if face.SourceFaceID == 0 || face.SourceSurfaceID == 0 || len(face.SourceBoundIDs) == 0 {
							t.Fatal("source")
						}
						sum = a.add(sum, face.Area)
						if p.Family == "planar" {
							if len(face.SignedLoopAreas) != len(face.SourceBoundIDs) {
								t.Fatal("loop mapping")
							}
							var total big.Rat
							for i, s := range face.SignedLoopAreas {
								q, ok := new(big.Rat).SetString(s)
								if !ok || i == 0 && q.Sign() <= 0 || i > 0 && q.Sign() >= 0 {
									t.Fatal("winding")
								}
								total.Add(&total, q)
								if i > 0 {
									holes++
								}
								bound := m.Source.Entities[face.SourceBoundIDs[i]]
								if len(bound.Records) != 1 {
									t.Fatal("bound")
								}
							}
							if total.Cmp(face.Area.Coefficients()[0]) != 0 {
								t.Fatal("subtraction")
							}
						}
					}
					if !reflect.DeepEqual(sum.Coefficients(), p.Area.Coefficients()) {
						t.Fatal("face sum")
					}
					switch p.Family {
					case "planar":
						eq(p.Area, [3]int64{planar, 0, 0})
						if (family == "holed" || family == "affine-pcurve-holed") && holes != 2 {
							t.Fatal("hole evidence", holes)
						}
					case "cylinder":
						eq(p.Area, [3]int64{0, 20, 0})
					case "sphere":
						eq(p.Area, [3]int64{0, 16, 0})
					case "torus":
						eq(p.Area, [3]int64{0, 0, 12})
					default:
						t.Fatal(p.Family)
					}
				}
				for i, p := range r.Occurrences {
					if !reflect.DeepEqual(p.Path, m.Instances[i].Path) || !reflect.DeepEqual(p.Faces, products[p.ProductID].Faces) {
						t.Fatal("world identity")
					}
				}
				// Every result owns its mutable evidence, including repeated shared products.
				old := r.Occurrences[0].Faces[0].SourceBoundIDs[0]
				r.Products[0].Faces[0].SourceBoundIDs[0] = -1
				if r.Occurrences[0].Faces[0].SourceBoundIDs[0] != old {
					t.Fatal("alias")
				}
				r.Occurrences[0].Faces[0].SourceBoundIDs[0] = -2
				fresh, e := query(ctx, RationalHierarchyAreaOptions{})
				if e != nil || fresh.Occurrences[0].Faces[0].SourceBoundIDs[0] != old {
					t.Fatal("mutation", e)
				}
				for _, o := range []RationalHierarchyAreaOptions{{MaxPlanarTests: 1}, {MaxLoopEdges: 1}, {MaxCoefficientBits: 255}, {Source: Limits{MaxEntities: 1}}} {
					if got, e := query(ctx, o); e == nil || len(got.Products) != 0 {
						t.Fatal("budget", e)
					}
				}
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				if _, e = query(canceled, RationalHierarchyAreaOptions{}); e != context.Canceled {
					t.Fatal(e)
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); bin != "" {
					b, e := exec.Command(bin, "read", "testdata/"+name).Output()
					if e != nil {
						t.Fatal(e)
					}
					var oracle struct {
						Area   float64
						Solids int
					}
					if e = json.Unmarshal(b, &oracle); e != nil {
						t.Fatal(e)
					}
					if oracle.Solids != 7 || math.Abs(oracle.Area-massValue(r.Sum)) > 2e-8*math.Max(1, oracle.Area) {
						t.Fatal("OCCT", oracle, massValue(r.Sum))
					}
				}
			})
		}
	}
}
func TestHierarchyAreaRejectsForgedSource(t *testing.T) {
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy-holed.step")
	ctx := context.Background()
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m.Instances[0].Path[0]++
	if out, e := MeasureRationalHierarchyBoundaryArea(ctx, m, RationalHierarchyAreaOptions{}); e == nil || len(out.Products) != 0 {
		t.Fatal("forged path")
	}
	if _, e := MeasureRationalHierarchyBoundaryArea(nil, m, RationalHierarchyAreaOptions{}); e == nil {
		t.Fatal("nil context")
	}
}
