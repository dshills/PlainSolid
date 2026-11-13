package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// A seam across both caps ensures each section loop crosses original edges;
// no entire closed contour is hidden in one face. The side faces share the
// same subdivision so the input shell has no topological T junctions.
func multipleOpenSplineFixture(t *testing.T) (Solid, Solid, []SplineShellOpenSection) {
	t.Helper()
	ctx := context.Background()
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	var capIDs []int
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		for side := 0; side < 2; side++ {
			spans := [][2]float64{{0, 1}}
			if axis != 0 {
				spans = [][2]float64{{0, .5}, {.5, 1}}
			}
			for _, span := range spans {
				lo, hi := [3]float64{}, [3]float64{1, 1, 1}
				lo[0], hi[0] = span[0], span[1]
				var poles []geom.Vec3
				for _, a := range []float64{lo[u], hi[u]} {
					for _, b := range []float64{lo[v], hi[v]} {
						p := [3]float64{}
						p[axis] = float64(side)
						p[u] = a
						p[v] = b
						poles = append(poles, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
					}
				}
				s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
				if e != nil {
					t.Fatal(e)
				}
				if axis == 2 {
					capIDs = append(capIDs, len(surfaces))
				}
				surfaces = append(surfaces, s)
				domains = append(domains, geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]})
				reversed = append(reversed, side == 0)
			}
		}
	}
	a, _, e := SewSplineRectangles(ctx, surfaces, domains, reversed, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := makeOpenSplineBox(t, [3]float64{.25, .25, -.5}, [3]float64{.75, .75, 1.5}, 0)
	var ap, bp []splineEmbeddingFace
	if _, e = a.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &ap); e != nil {
		t.Fatal("A embedding", e)
	}
	if _, e = b.validateSplineRectangleEmbedding(ctx, SplineRectangleEmbeddingOptions{}, &bp); e != nil {
		t.Fatal("B embedding", e)
	}
	var pairs [][2]int
	for _, i := range capIDs {
		d := domains[i]
		// Each cap half meets both y sides and just its nearest x side.
		xside := 0
		if d.UMin == .5 {
			xside = 1
		}
		for _, j := range []int{xside, 2, 3} {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	sections, e := discoverAffineSplineOpenSections(ctx, a, b, ap, bp, pairs, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal("independent affine section discovery", e)
	}
	if len(sections) != 12 {
		t.Fatal("section pair count", len(sections))
	}
	return a, b, sections
}

func TestCombineMultipleOpenSplineWires(t *testing.T) {
	a, b, sections := multipleOpenSplineFixture(t)
	for _, tc := range []struct {
		name         string
		op           parametric.BooleanOp
		volume, area float64
		euler        int
	}{
		{"union", parametric.Union, 1.25, 8, 2},
		{"common", parametric.Intersection, .25, 2.5, 2},
		{"through-hole", parametric.Difference, .75, 7.5, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, route := range []string{"explicit", "reordered", "automatic"} {
				t.Run(route, func(t *testing.T) {
					var out Solid
					var h SplineShellOpenHistory
					var e error
					if route == "automatic" {
						var history SplineShellCombineHistory
						out, history, e = CombineSplineShells(context.Background(), a, b, tc.op, SplineShellCombineOptions{})
						h = SplineShellOpenHistory{FacesA: history.FacesA, FacesB: history.FacesB, EdgesA: history.EdgesA, EdgesB: history.EdgesB, SectionEdges: history.SectionEdges}
						if e == nil && history.Method != "affine-open-wire" {
							t.Fatal(history.Method)
						}
					} else {
						input := append([]SplineShellOpenSection(nil), sections...)
						if route == "reordered" {
							for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
								input[i], input[j] = input[j], input[i]
							}
						}
						out, h, e = CombineOpenSplineShells(context.Background(), a, b, tc.op, input, SplineShellOpenOptions{})
					}
					if e != nil {
						t.Fatal(e)
					}

					p, e := out.MassProperties(context.Background(), parametric.IntegrationOptions{})
					if e != nil || math.Abs(p.Volume-tc.volume) > 1e-10 || math.Abs(p.Area-tc.area) > 1e-10 || p.Centroid.Sub(geom.Vec3{X: .5, Y: .5, Z: .5}).Length() > 1e-10 {
						t.Fatal(p, e)
					}
					if len(out.Topology().Shells) != 1 || out.Topology().EulerCharacteristic != tc.euler {
						t.Fatal(out.Topology())
					}
					// Every original bar side is cut twice. Union retains both end
					// fragments, while common/cut retain its single middle fragment.
					for face := 0; face < 4; face++ {
						want := 1
						if tc.op == parametric.Union {
							want = 2
						}
						if len(h.FacesB[face]) != want {
							t.Fatalf("bar side %d ancestry %v, want %d retained fragments", face, h.FacesB[face], want)
						}
					}
					if len(h.SectionEdges) != 24 || len(h.FacesA) != len(a.faces) || len(h.FacesB) != len(b.faces) {
						t.Fatal("missing ancestry", h)
					}
					seenFaces := map[int]bool{}
					for _, operand := range [][][]int{h.FacesA, h.FacesB} {
						for _, ids := range operand {
							for _, id := range ids {
								if id < 0 || id >= len(out.faces) || seenFaces[id] {
									t.Fatal("invalid or duplicate face ancestry", id)
								}
								seenFaces[id] = true
							}
						}
					}
					if len(seenFaces) != len(out.faces) {
						t.Fatal("unattributed output faces")
					}
					seenEdges := map[int]bool{}
					for _, id := range h.SectionEdges {
						if id < 0 || id >= len(out.edges) || seenEdges[id] {
							t.Fatal("invalid or duplicate section edge", id)
						}
						seenEdges[id] = true
					}
					if len(h.EdgesA) != len(a.edges) || len(h.EdgesB) != len(b.edges) {
						t.Fatal("missing edge ancestry")
					}
					two := false
					for operand, rows := range [][][]int{h.EdgesA, h.EdgesB} {
						source := []Solid{a, b}[operand]
						for original, ids := range rows {
							previous := -math.MaxFloat64
							for _, id := range ids {
								if id < 0 || id >= len(out.edges) || seenEdges[id] {
									t.Fatal("duplicate or invalid inherited edge", id)
								}
								seenEdges[id] = true
								edge := out.edges[id]
								center := out.vertices[edge.Start].Add(out.vertices[edge.End]).Scale(.5)
								base := source.edges[original]
								origin := source.vertices[base.Start]
								direction := source.vertices[base.End].Sub(origin)
								parameter := center.Sub(origin).Dot(direction) / direction.Dot(direction)
								if parameter <= previous {
									t.Fatal("edge ancestry not ordered in original parameter", ids)
								}
								previous = parameter
							}
							if operand == 1 && len(ids) == 2 {
								two = true
							}
						}
					}
					if len(seenEdges) != len(out.edges) {
						t.Fatal("unattributed output edge")
					}
					if tc.op == parametric.Union && !two {
						t.Fatal("expected retained bar edge fragments")
					}

				})
			}

		})
	}
}
