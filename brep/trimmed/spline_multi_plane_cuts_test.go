package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSplineMultiPlaneCutsTripleCap(t *testing.T) {
	ctx := context.Background()
	stock, cuts := splineThreeCapFixture(t)
	for trial := 0; trial < 2; trial++ {
		r, e := CombineSplineMultiPlaneCuts(ctx, stock, cuts, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.CapIntersections) != 3 || r.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("three active caps must each meet the other two", r.CapIntersections, r.Solid.Topology())
		}
		mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		// Integrate positive .375−(x+y)/4 on .75≤x≤1 and 0≤y≤1.5−x.
		if e != nil || math.Abs(mass.Volume-19./1536) > 1e-12 {
			t.Fatal("independent retained wedge volume", mass, e)
		}
		assertSplineMultiEdgeOwners(t, r)
		var triple int
		for _, h := range r.CapIntersections {
			edge := r.Solid.edges[h.Edge]
			for i, p := range h.Points {
				stored := r.Solid.vertices[[2]int{edge.Start, edge.End}[i]]
				if p[0].Cmp(ratInset(stored.X)) != 0 || p[1].Cmp(ratInset(stored.Y)) != 0 || p[2].Cmp(ratInset(stored.Z)) != 0 {
					t.Fatal("cap witness does not identify its canonical edge vertex", h.Edge, i)
				}
				if p[0].Cmp(big.NewRat(3, 4)) == 0 && p[1].Cmp(big.NewRat(3, 4)) == 0 && p[2].Cmp(big.NewRat(3, 4)) == 0 {
					triple++
				}
			}
			for i, cut := range h.Cuts {
				id := r.ToolFaces[cut][cuts[cut].ActiveFace][0]
				if r.Solid.faces[id].Surface != cuts[cut].Tool.faces[h.Faces[i]].Surface {
					t.Fatal("lost original cap chart", h.Cuts, h.Faces)
				}
			}
		}
		if triple != 3 {
			t.Fatal("all three cap-edge proofs must meet at exactly one native vertex", triple)
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, r.Solid); e != nil {
			t.Fatal(e)
		}
		back, e := ReadJSONContext(ctx, &data, JSONLimits{})
		if e != nil || back.Topology().EulerCharacteristic != 2 || len(back.Edges()) != len(r.Solid.Edges()) {
			t.Fatal("native intersection reload", e)
		}
		cuts[0], cuts[2] = cuts[2], cuts[0]
	}
	bad := append([]SplineHalfspaceCut(nil), cuts...)
	bad[2] = bad[1]
	if r, e := CombineSplineMultiPlaneCuts(ctx, stock, bad, SplineRectangleEmbeddingOptions{}); !errors.Is(e, geom.ErrConvergence) || len(r.Solid.faces)+len(r.ToolFaces) != 0 {
		t.Fatal("coincident original cap ownership must reject atomically", r, e)
	}
}

func splineThreeCapFixture(t *testing.T) (Solid, []SplineHalfspaceCut) {
	t.Helper()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	lateral, _ := makeOpenSplineBox(t, [3]float64{.75, -1, -1}, [3]float64{2, 2, 2}, 0)
	return stock, []SplineHalfspaceCut{{slabTool(t, true, .375, .25), 4}, {slabTool(t, false, .75, 0), 5}, {lateral, 0}}
}

func TestSplineMultiPlaneCutsOCCT(t *testing.T) {
	path := os.Getenv("PLAINSOLID_OCCT_MULTI_PLANE_CUTS")
	if path == "" {
		t.Skip("set PLAINSOLID_OCCT_MULTI_PLANE_CUTS to the independent OCCT oracle")
	}
	ctx := context.Background()
	data, e := exec.CommandContext(ctx, path).Output()
	if e != nil {
		t.Fatal(e)
	}
	var reference []float64
	if e := json.Unmarshal(data, &reference); e != nil || len(reference) != 8 {
		t.Fatalf("invalid OCCT result %s: %v", data, e)
	}
	stock, cuts := splineThreeCapFixture(t)
	r, e := CombineSplineMultiPlaneCuts(ctx, stock, cuts, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := r.Solid.Topology()
	for i, got := range []float64{m.Volume, m.Area, m.Centroid.X, m.Centroid.Y, m.Centroid.Z, float64(p.Vertices), float64(p.Edges), float64(p.Faces)} {
		if math.Abs(got-reference[i]) > 1e-12 || math.IsNaN(got) {
			t.Fatalf("measure %d: PlainSolid %.16g, OCCT %.16g", i, got, reference[i])
		}
	}
}

func TestSplineMultiPlaneCutsRetainsEntireCurvedFace(t *testing.T) {
	ctx := context.Background()
	stock, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, .125)
	base, _ := makeOpenSplineBox(t, [3]float64{-1, -1, 0}, [3]float64{2, 2, 2}, 0)
	cuts := []SplineHalfspaceCut{{slabTool(t, true, .25, 0), 4}}
	for _, s := range [][2]float64{{.25, 0}, {0, .25}} {
		tool, e := base.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {s[0], s[1], 1, .125}})
		if e != nil {
			t.Fatal(e)
		}
		cuts = append(cuts, SplineHalfspaceCut{tool, 4})
	}
	r, e := CombineSplineMultiPlaneCuts(ctx, stock, cuts, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.StockFaces[4]) != 0 || len(r.StockFaces[5]) != 1 || r.Solid.faces[r.StockFaces[5][0]].Surface != stock.faces[5].Surface || len(r.CapIntersections) != 3 {
		t.Fatal("full original upper spline and three lower caps required", r.StockFaces, len(r.CapIntersections))
	}
	assertSplineMultiEdgeOwners(t, r)
	for i, p := range stock.vertices {
		id := r.OriginalVertices[i]
		if p.Z < .25 && id != -1 || p.Z >= 1 && (id < 0 || p != r.Solid.vertices[id]) {
			t.Fatal("incorrect original vertex ancestry", i, p, id)
		}
	}
	// The integral of the upper face is 33/32. With m=max(X,Y), the
	// lower face is 1/4 for m≤1/2 and 1/8+m/4 elsewhere; its integral is 29/96.
	mass, e := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-35./48) > 1e-12 || r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("independent curved-body volume", mass, e)
	}
	lateral, _ := makeOpenSplineBox(t, [3]float64{.75, -1, -1}, [3]float64{2, 2, 2}, 0)
	cuts[2] = SplineHalfspaceCut{lateral, 0}
	if out, e := CombineSplineMultiPlaneCuts(ctx, stock, cuts, SplineRectangleEmbeddingOptions{}); !errors.Is(e, geom.ErrConvergence) || len(out.StockFaces)+len(out.Solid.faces) != 0 {
		t.Fatal("crossing an original curved support must reject atomically", e, out.StockFaces)
	}
}

func assertSplineMultiEdgeOwners(t *testing.T, r SplineMultiPlaneCuts) {
	t.Helper()
	owners := make([]int, len(r.Solid.edges))
	for _, groups := range [][][]int{r.StockEdges, r.SectionEdges} {
		for _, group := range groups {
			for _, id := range group {
				owners[id]++
			}
		}
	}
	for _, seam := range r.CapIntersections {
		owners[seam.Edge]++
	}
	for i, count := range owners {
		if count != 1 {
			t.Fatal("every result edge needs one ownership class", i, count)
		}
	}
}
