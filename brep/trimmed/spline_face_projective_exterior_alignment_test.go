package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofContourDifferenceAlignsOriginalExteriorClocks(t *testing.T) {
	ctx := context.Background()
	a := projectiveContourImprintFace(t, true, true, false)
	b := projectiveContourImprintFace(t, false, false, false)
	var poles []geom.Vec3
	for u := range 2 {
		for v := range 2 {
			x, y := float64(u), float64(v)
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 1 + x*y})
		}
	}
	k := []float64{0, 0, 1, 1}
	// Preserve the original integer weights: normalizing by the maximum
	// would round 3/5 before the exact projected station could be proved.
	s, err := geom.NewBSplineSurfaceWithExactWeights(1, 1, 2, 2, poles, k, k, []float64{5, 3, 5, 3})
	if err != nil {
		t.Fatal(err)
	}
	b.Surface = s
	options := ProjectiveRoofSolidOptions{MatchExteriorPartitions: true}
	for _, axes := range [][2]int{{0, 1}, {1, 0}} {
		for order, pair := range [][2]Face{{a, b}, {b, a}} {
			q, err := BuildProjectiveSplineRoofContourDifference(ctx, pair[0], pair[1], axes, 0, options)
			if err != nil || !q.Complete || len(q.SectionEdges) != 2 || len(q.Walls) != 4 || len(q.FloorSources) != 4 {
				t.Fatal("the exact common knot must split both originally unmatched exterior walls", axes, order, err)
			}
			for i, f := range q.Solid.faces[:2] {
				if f.Surface != pair[i].Surface {
					t.Fatal("alignment replaced an original chart", axes, order, i)
				}
			}
			var matches int
			for i, wall := range q.Walls {
				sources := [2]ProjectiveSplineImprintUse{wall.Source, q.FloorSources[i]}
				if order == 1 {
					sources[0], sources[1] = sources[1], sources[0]
				}
				found := false
				for j, x := range sources[0].OriginalRange {
					if x.Cmp(big.NewRat(3, 8)) == 0 || x.Cmp(big.NewRat(5, 8)) == 0 {
						if sources[1].OriginalRange[j].Cmp(big.NewRat(1, 2)) != 0 {
							t.Fatal("3/8 physical coordinate must map to 1/2 on the original weighted owner", axes, order, i, sources)
						}
						found = true
					}
				}
				if found {
					matches++
				}
			}
			if matches != 2 {
				t.Fatal("both original wall portions must keep their own exact subrange", axes, order, matches)
			}
			p, err := q.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil || math.Abs(p.Volume-1.0/8) > 1e-10 {
				t.Fatal("exterior alignment changed the original analytic material", axes, order, p, err)
			}
		}
	}
	options.Envelope.Imprint.MaxEdgeOperations = 100
	q, err := BuildProjectiveSplineRoofContourDifference(ctx, a, b, [2]int{0, 1}, 0, options)
	if !errors.Is(err, geom.ErrConvergence) || q.Complete {
		t.Fatal("unaligned partial roof must not be published after exact work exhaustion", q, err)
	}
}
