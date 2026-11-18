package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"plainsolid/intersect"
	"testing"
)

func affinePolygonPairedFixture(t *testing.T, full bool) (Solid, Solid) {
	a := convexPolygonShellMappedFixture(t, []geom.Vec2{{}, {X: 4}, {X: 4, Y: 2}, {X: .25, Y: 2}, {Y: 1.75}}, func(x, y, z float64) geom.Vec3 { return geom.Vec3{X: x - 2, Y: y - 1, Z: 2*z - 4} })
	var b Solid
	if full {
		_, b = pairedAffineShellFixture(t)
	} else {
		_, b = affineMaterialOnePairedFixture(t)
	}
	return a, b
}
func TestConvexAffinePolygonPairedMaterial(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, false)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		if q, e := CertifyConvexAffinePolygonCrossing(ctx, pair[0], pair[1], ConvexAffinePolygonCrossingOptions{}); e == nil || q != nil {
			t.Fatal("strict scope widened")
		}
		crossing, e := CertifyConvexAffinePolygonCrossing(ctx, pair[0], pair[1], ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true})
		if e != nil {
			t.Fatal(e)
		}
		if crossing.CompleteFacePairs() != 42 {
			t.Fatal("coverage")
		}
		paired := 0
		for vi, node := range crossing.SectionVertices() {
			if len(node.Owners) != 2 {
				continue
			}
			paired++
			if node.Owner != -1 || node.Edge != -1 || node.Parameter != nil || len(node.ContactPairs) != 4 {
				t.Fatal("paired owner semantics")
			}
			degree := 0
			for _, edge := range crossing.SectionEdges() {
				for _, v := range edge.Vertices {
					if v == vi {
						degree++
					}
				}
			}
			if degree != 2 && degree != 4 {
				t.Fatal("germ incidence")
			}
			for _, owner := range node.Owners {
				if owner.Owner == order && owner.Parameter.Cmp(big.NewRat(8, 15)) != 0 {
					t.Fatal("nonrectangular original edge parameter", owner.Parameter)
				}
				if len(owner.Occurrences) != 2 {
					t.Fatal("original incidents")
				}
				edge := pair[owner.Owner].edges[owner.Edge]
				start, end := affineMaterialXYZ(pair[owner.Owner].vertices[edge.Start]), affineMaterialXYZ(pair[owner.Owner].vertices[edge.End])
				for k := range start {
					value := new(big.Rat).Add(start[k], new(big.Rat).Mul(owner.Parameter, new(big.Rat).Sub(end[k], start[k])))
					if value.Cmp(node.Point[k]) != 0 {
						t.Fatal("independent original edge witness")
					}
				}
			}
		}
		if paired != 1 {
			t.Fatal("paired nodes", paired)
		}
		for _, tc := range []struct {
			op  intersect.AffineRingOperation
			num int64
		}{{intersect.AffineRingIntersection, 139}, {intersect.AffineRingUnion, 2093}, {intersect.AffineRingDifference, 1901}} {
			if tc.op == intersect.AffineRingDifference && order == 1 {
				tc.num = 53
			}
			result, e := crossing.Combine(ctx, tc.op, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true})
			if e != nil {
				t.Fatal("material", order, tc.op, e)
			}
			solid := result.Solid()
			if result.ExactVolume().Cmp(big.NewRat(tc.num, 64)) != 0 || len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
				t.Fatal("native measure/topology", result.ExactVolume())
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, solid); e != nil {
				t.Fatal(e)
			}
			decoded, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil || len(decoded.faces) != len(solid.faces) {
				t.Fatal("native reload", e)
			}
			for fi, source := range result.FaceSources() {
				if solid.faces[fi].Surface != pair[source.Owner].faces[source.Face].Surface || solid.faces[fi].Reversed != (pair[source.Owner].faces[source.Face].Reversed != source.Reversed) {
					t.Fatal("original selected support")
				}
			}
			t.Log("paired", order, tc.op, "faces", len(solid.faces), "volume", result.ExactVolume())
		}
	}
}

func TestConvexAffinePolygonFourGermMaterial(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, true)
	for _, pair := range [][2]Solid{{a, b}, {b, a}} {
		crossing, e := CertifyConvexAffinePolygonCrossing(ctx, pair[0], pair[1], ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true})
		if e != nil {
			t.Fatal(e)
		}
		paired, four := 0, 0
		for vi, node := range crossing.SectionVertices() {
			if len(node.Owners) != 2 {
				continue
			}
			paired++
			degree := 0
			for _, edge := range crossing.SectionEdges() {
				for _, id := range edge.Vertices {
					if id == vi {
						degree++
					}
				}
			}
			if degree == 4 {
				four++
			}
		}
		if paired != 2 || four != 1 {
			t.Fatal("independent paired graph", paired, four)
		}
		for _, tc := range []struct {
			op  intersect.AffineRingOperation
			num int64
		}{{intersect.AffineRingIntersection, 293}, {intersect.AffineRingUnion, 2131}} {
			result, e := crossing.Combine(ctx, tc.op, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true})
			if e != nil || result.ExactVolume().Cmp(big.NewRat(tc.num, 64)) != 0 {
				t.Fatal(tc.op, e)
			}
			t.Log("four-germ", tc.op, "faces", len(result.Solid().faces))
		}
		if result, e := crossing.Combine(ctx, intersect.AffineRingDifference, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}); e == nil || result != nil {
			t.Fatal("unproved pinched/disconnected cut")
		}
	}
}

func TestConvexAffinePolygonPairedBindingRejects(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonPairedFixture(t, false)
	options := ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}
	r, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, options)
	if e != nil {
		t.Fatal(e)
	}
	paired := -1
	for i, node := range r.SectionVertices() {
		if len(node.Owners) == 2 {
			paired = i
		}
	}
	if paired < 0 {
		t.Fatal("fixture")
	}
	reject := func(bad ConvexAffinePolygonCrossing) {
		t.Helper()
		if output, e := bad.Combine(ctx, intersect.AffineRingIntersection, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}); e == nil || output != nil {
			t.Fatal("incomplete paired binding")
		}
	}
	bad := *r
	bad.nodes = r.SectionVertices()
	bad.nodes[paired].Owners[1].Parameter.Add(bad.nodes[paired].Owners[1].Parameter, big.NewRat(1, 16))
	reject(bad)
	bad = *r
	bad.nodes = r.SectionVertices()
	bad.nodes[paired].ContactPairs = bad.nodes[paired].ContactPairs[:3]
	reject(bad)
	point, empty := -1, -1
	for i, contact := range r.contacts {
		kind, _ := contact.Contact().Kind()
		if kind == intersect.AffinePatchContactPoint {
			point = i
		}
		if kind == intersect.AffinePatchContactEmpty {
			empty = i
		}
	}
	if point < 0 || empty < 0 {
		t.Fatal("point-pair coverage fixture")
	}
	bad = *r
	bad.contacts = append([]*intersect.AffinePolygonContact(nil), r.contacts...)
	bad.contacts[point] = r.contacts[empty]
	reject(bad)
	bad = *r
	source := *r.source
	source.edges = r.SectionEdges()
	source.edges = append(source.edges, source.edges[0])
	bad.source = &source
	reject(bad)
	options.MaxOperations = r.ExactOperations()
	if output, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, options); e != nil || output == nil {
		t.Fatal("exact proof budget", e)
	}
	options.MaxOperations--
	if output, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, options); e == nil || output != nil {
		t.Fatal("one-short proof budget")
	}
}
func TestConvexAffinePolygonPairedOCCT(t *testing.T) {
	executable := os.Getenv("PLAINSOLID_OCCT_AFFINE_POLYGON_PAIRED")
	if executable == "" {
		t.Skip("set PLAINSOLID_OCCT_AFFINE_POLYGON_PAIRED to tools/occtref/affine_polygon_paired.cxx executable")
	}
	for _, full := range []bool{false, true} {
		args := []string{}
		if full {
			args = []string{"full"}
		}
		data, e := exec.Command(executable, args...).Output()
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Order, Operation int
			Valid            bool
			Solids           int
			Volume           float64
		}
		if e = json.Unmarshal(data, &rows); e != nil || len(rows) != 6 {
			t.Fatal(e)
		}
		a, b := affinePolygonPairedFixture(t, full)
		seen := map[[2]int]bool{}
		for _, row := range rows {
			if row.Order < 0 || row.Order > 1 || row.Operation < 0 || row.Operation > 2 || seen[[2]int{row.Order, row.Operation}] {
				t.Fatal("oracle row")
			}
			seen[[2]int{row.Order, row.Operation}] = true
			pair := [2]Solid{a, b}
			if row.Order == 1 {
				pair = [2]Solid{b, a}
			}
			op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[row.Operation]
			output, e := CombineConvexAffinePolygonShells(context.Background(), pair[0], pair[1], op, ConvexAffinePolygonBooleanOptions{Crossing: ConvexAffinePolygonCrossingOptions{AllowPairedEdgeJunctions: true}, Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}})
			if full && op == parametric.Difference {
				if e == nil || output != nil {
					t.Fatal("strict four-germ Cut")
				}
				wantSolids, wantVolume := 1, 1747.0/64
				if row.Order == 1 {
					wantSolids, wantVolume = 2, 91.0/64
				}
				if !row.Valid || row.Solids != wantSolids || math.Abs(row.Volume-wantVolume) > 1e-10 {
					t.Fatal("independent unsupported Cut reference", row)
				}
				continue
			}
			if e != nil {
				t.Fatal(e)
			}
			volume, _ := output.ExactVolume().Float64()
			if !row.Valid || row.Solids != 1 || math.Abs(volume-row.Volume) > 1e-10 {
				t.Fatal(row, volume)
			}
		}
	}
}
