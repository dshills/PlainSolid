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
	"plainsolid/intersect"
	"testing"
)

func affinePolygonCrossingFixture(t *testing.T) (Solid, Solid) {
	world := func(x, y, z float64) geom.Vec3 { return geom.Vec3{X: x + z/4, Y: y + x/8, Z: z + y/4} }
	a := convexPolygonShellMappedFixture(t, []geom.Vec2{{}, {X: 2}, {Y: 2}}, world)
	b := convexPolygonShellMappedFixture(t, []geom.Vec2{{X: .5, Y: .25}, {X: 2.5, Y: .25}, {X: .5, Y: 2.25}}, func(x, y, z float64) geom.Vec3 { return world(x, y, z+.5) })
	return a, b
}
func TestConvexAffinePolygonCrossingMaterial(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCrossingFixture(t)
	defaultResult, e := CombineConvexAffinePolygonShells(ctx, a, b, parametric.Intersection, ConvexAffinePolygonBooleanOptions{})
	if e != nil || defaultResult.ExactVolume().Cmp(big.NewRat(9675, 8192)) != 0 {
		t.Fatal("default exact native entry", e)
	}
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		crossing, e := CertifyConvexAffinePolygonCrossing(ctx, pair[0], pair[1], ConvexAffinePolygonCrossingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if crossing.CompleteFacePairs() != 25 {
			t.Fatal("coverage")
		}
		vertices := crossing.SectionVertices()
		edges := crossing.SectionEdges()
		if len(vertices) != 5 || len(edges) != 5 {
			t.Fatal("section", len(vertices), len(edges))
		}
		for _, v := range vertices {
			source := pair[v.Owner]
			edge := source.edges[v.Edge]
			start, end := affineMaterialXYZ(source.vertices[edge.Start]), affineMaterialXYZ(source.vertices[edge.End])
			for k := range start {
				expected := new(big.Rat).Add(start[k], new(big.Rat).Mul(v.Parameter, new(big.Rat).Sub(end[k], start[k])))
				if expected.Cmp(v.Point[k]) != 0 {
					t.Fatal("original exact model edge parameter")
				}
			}
			if len(v.Occurrences) != 2 {
				t.Fatal("source edge ownership")
			}
			for _, o := range v.Occurrences {
				if pair[v.Owner].faces[o[0]].Outer[o[1]].Edge != v.Edge {
					t.Fatal("original edge id")
				}
			}
		}
		for _, tc := range []struct {
			op  intersect.AffineRingOperation
			num int64
		}{{intersect.AffineRingIntersection, 9675}, {intersect.AffineRingUnion, 56373}, {intersect.AffineRingDifference, 23349}} {

			operation := parametric.Intersection
			if tc.op == intersect.AffineRingUnion {
				operation = parametric.Union
			} else if tc.op == intersect.AffineRingDifference {
				operation = parametric.Difference
			}
			result, e := CombineConvexAffinePolygonShells(ctx, pair[0], pair[1], operation, ConvexAffinePolygonBooleanOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}})
			if e != nil {
				t.Fatal("operation", tc.op, e)
			}
			s := result.Solid()
			checkPolygonCrossingOccupancy(t, result, order, tc.op)
			if result.ExactVolume().Cmp(big.NewRat(tc.num, 8192)) != 0 {
				t.Fatal("volume", result.ExactVolume())
			}
			if len(s.vertices)-len(s.edges)+len(s.faces) != 2 {
				t.Fatal("Euler")
			}
			for fi, source := range result.FaceSources() {
				if s.faces[fi].Surface != pair[source.Owner].faces[source.Face].Surface || s.faces[fi].Reversed != (pair[source.Owner].faces[source.Face].Reversed != source.Reversed) {
					t.Fatal("original oriented support")
				}
			}
			var data bytes.Buffer
			if e = WriteJSON(&data, s); e != nil {
				t.Fatal(e)
			}
			decoded, e := ReadJSONContext(ctx, &data, JSONLimits{})
			if e != nil || len(decoded.faces) != len(s.faces) {
				t.Fatal("reload", e)
			}
			t.Log("order", order, "operation", tc.op, "faces", len(s.faces), "work", result.ExactOperations())
		}
	}
}

func TestConvexAffinePolygonCrossingLimitsAndScope(t *testing.T) {
	ctx := context.Background()
	a, b := affinePolygonCrossingFixture(t)
	r, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, ConvexAffinePolygonCrossingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []ConvexAffinePolygonCrossingOptions{{MaxFacePairs: 24}, {MaxOperations: r.ExactOperations() - 1}} {
		if q, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, o); e == nil || q != nil {
			t.Fatal("incomplete accepted", o)
		}
	}
	if q, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, ConvexAffinePolygonCrossingOptions{MaxOperations: r.ExactOperations()}); e != nil || q == nil {
		t.Fatal("exact work", e)
	}
	for _, o := range []ConvexAffinePolygonMaterialOptions{{MaxOperations: 1}, {MaxFaces: 1, RationalUVEndpoints: true}} {
		if q, e := r.Combine(ctx, intersect.AffineRingUnion, o); e == nil || q != nil {
			t.Fatal("material limit")
		}
	}
	edges := r.SectionEdges()
	edges[0].UV[0][0][0].SetInt64(999)
	if r.SectionEdges()[0].UV[0][0][0].Cmp(big.NewRat(999, 1)) == 0 {
		t.Fatal("mutable edge")
	}
	nodes := r.SectionVertices()
	nodes[0].Parameter.SetInt64(0)
	if r.SectionVertices()[0].Parameter.Sign() == 0 {
		t.Fatal("mutable vertex")
	}
	// An omitted original pair segment cannot be repaired by a plausible auxiliary
	// triangulation and silently lose section ancestry.
	forged := *r
	source := *r.source
	source.edges = r.SectionEdges()[:len(r.SectionEdges())-1]
	forged.source = &source
	if q, e := forged.Combine(ctx, intersect.AffineRingIntersection, ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}); e == nil || q != nil {
		t.Fatal("omitted segment")
	}
	for _, pair := range [][2]Solid{{a, a}} {
		if q, e := CertifyConvexAffinePolygonCrossing(ctx, pair[0], pair[1], ConvexAffinePolygonCrossingOptions{}); e == nil || q != nil {
			t.Fatal("coplanar accepted")
		}
	}
	capA, capB := affinePolygonCapFixture(t, 0, false)
	if q, e := CertifyConvexAffinePolygonCrossing(ctx, capA, capB, ConvexAffinePolygonCrossingOptions{}); e == nil || q != nil {
		t.Fatal("cap area accepted as crossing")
	}
	distant := convexPolygonShellMappedFixture(t, []geom.Vec2{{}, {X: 2}, {Y: 2}}, func(x, y, z float64) geom.Vec3 { return geom.Vec3{X: 10 + x + z/4, Y: y + x/8, Z: z + y/4} })
	if q, e := CertifyConvexAffinePolygonCrossing(ctx, a, distant, ConvexAffinePolygonCrossingOptions{}); e == nil || q != nil {
		t.Fatal("no crossing accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := r.Combine(canceled, intersect.AffineRingUnion, ConvexAffinePolygonMaterialOptions{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
	if q, e := new(ConvexAffinePolygonCrossing).Combine(ctx, intersect.AffineRingUnion, ConvexAffinePolygonMaterialOptions{}); e == nil || q != nil {
		t.Fatal("zero certificate")
	}
	if q, e := new(ConvexAffinePolygonCapContact).Fuse(ctx, ConvexAffinePolygonCapFuseOptions{}); e == nil || q != nil {
		t.Fatal("zero cap certificate")
	}
	if new(ConvexAffinePolygonShell).ExactVolume() != nil || new(ConvexAffinePolygonCapUnion).ExactVolume() != nil || new(ConvexAffinePolygonCapContact).ExactCommonVolume() != nil {
		t.Fatal("zero volume claim")
	}
	_ = new(ConvexAffinePolygonCapContact).SupportingPlane()
	_ = new(ConvexAffinePolygonCapUnion).RemovedCapUVArea()
}

func TestConvexAffinePolygonCrossingOCCT(t *testing.T) {
	executable := os.Getenv("PLAINSOLID_OCCT_AFFINE_POLYGON_CROSSING")
	if executable == "" {
		t.Skip("set PLAINSOLID_OCCT_AFFINE_POLYGON_CROSSING to tools/occtref/affine_polygon_crossing.cxx executable")
	}
	data, e := exec.Command(executable).Output()
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
		t.Fatal("oracle", e)
	}
	a, b := affinePolygonCrossingFixture(t)
	seen := map[[2]int]bool{}
	for _, row := range rows {
		if row.Order < 0 || row.Order > 1 || row.Operation < 0 || row.Operation > 2 || seen[[2]int{row.Order, row.Operation}] {
			t.Fatal("oracle identity")
		}
		seen[[2]int{row.Order, row.Operation}] = true
		pair := [2]Solid{a, b}
		if row.Order == 1 {
			pair = [2]Solid{b, a}
		}
		op := []parametric.BooleanOp{parametric.Intersection, parametric.Union, parametric.Difference}[row.Operation]
		result, e := CombineConvexAffinePolygonShells(context.Background(), pair[0], pair[1], op, ConvexAffinePolygonBooleanOptions{Material: ConvexAffinePolygonMaterialOptions{RationalUVEndpoints: true}})
		if e != nil {
			t.Fatal(e)
		}
		volume, _ := result.ExactVolume().Float64()
		if !row.Valid || row.Solids != 1 || math.Abs(volume-row.Volume) > 1e-10 {
			t.Fatal(row, volume)
		}
	}
}

// Independent inverse of the authored physical map, followed by raw prism
// inequalities; this does not use the production supporting-plane classifier.
func checkPolygonCrossingOccupancy(t *testing.T, r *ConvexAffinePolygonMaterial, order int, op intersect.AffineRingOperation) {
	t.Helper()
	for _, record := range r.Fragments() {
		part := r.SourceFaceFragments(record.Owner, record.Face)
		cycle := part.Cycles()[record.Fragment]
		vertices := part.Vertices()
		uv := [2]*big.Rat{new(big.Rat), new(big.Rat)}
		for _, u := range cycle.Uses {
			for k := range uv {
				uv[k].Add(uv[k], vertices[u.Start][k])
			}
		}
		for k := range uv {
			uv[k].Quo(uv[k], big.NewRat(int64(len(cycle.Uses)), 1))
		}
		point, e := r.crossing.shells[record.Owner].queries[record.Face].chart.EvalExact(uv[0], uv[1])
		if e != nil {
			t.Fatal(e)
		}
		z := new(big.Rat).Mul(big.NewRat(128, 1), point[2])
		z.Sub(z, new(big.Rat).Mul(big.NewRat(32, 1), point[1]))
		z.Add(z, new(big.Rat).Mul(big.NewRat(4, 1), point[0]))
		z.Quo(z, big.NewRat(129, 1))
		x := new(big.Rat).Sub(point[0], new(big.Rat).Quo(z, big.NewRat(4, 1)))
		y := new(big.Rat).Sub(point[1], new(big.Rat).Quo(x, big.NewRat(8, 1)))
		dx, dy, dz := new(big.Rat), new(big.Rat), new(big.Rat)
		if 1-record.Owner == 1-order {
			dx, dy, dz = big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(1, 2)
		}
		sum := new(big.Rat).Add(x, y)
		limit := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Add(dx, dy))
		inside := x.Cmp(dx) > 0 && y.Cmp(dy) > 0 && sum.Cmp(limit) < 0 && z.Cmp(dz) > 0 && z.Cmp(new(big.Rat).Add(dz, big.NewRat(2, 1))) < 0
		keep := inside
		if op == intersect.AffineRingUnion || (op == intersect.AffineRingDifference && record.Owner == 0) {
			keep = !inside
		}
		if record.InsideOther != inside || record.Retained != keep || record.Reversed != (keep && op == intersect.AffineRingDifference && record.Owner == 1) {
			t.Fatal("independent raw occupancy", record, inside)
		}
	}
}
