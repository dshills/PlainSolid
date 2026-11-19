//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBuildProjectiveSplineRoofBandDifferenceKeepsSeparatePositiveBands(t *testing.T) {
	ctx := context.Background()
	f := projectiveNativeBandFaces(t)
	for order, pair := range [][2]Face{f, {f[1], f[0]}} {
		for _, axes := range [][2]int{{0, 1}, {1, 0}} {
			r, err := BuildProjectiveSplineRoofBandDifference(ctx, pair[0], pair[1], axes, 0, ProjectiveRoofSolidOptions{})
			if err != nil || !r.Complete || len(r.Components) != order+1 {
				t.Fatal("the signed bands must yield the exact number of positive components", order, axes, err)
			}
			for i, component := range r.Components {
				v, x, faces, edges, vertices, seams, sourceBand := 1.0/6, 1.5, 8, 14, 8, 2, 1
				if order == 1 {
					v, x, faces, edges, vertices, seams, sourceBand = 5.0/6, []float64{3.0 / 10, 27.0 / 10}[i], 5, 9, 6, 1, 2*i
				}
				s := component.Solid
				if !component.Complete || component.BaseFace != -1 || len(component.SectionEdges) != seams ||
					len(s.faces) != faces || len(s.edges) != edges || len(s.vertices) != vertices || r.Bands[i] != sourceBand ||
					len(component.FloorSources) != len(component.Walls) {
					t.Fatal("original source band topology or exterior wall ownership changed", order, axes, i, len(s.faces), len(s.edges), len(s.vertices))
				}
				for owner, indices := range component.RoofFragments {
					for _, id := range indices {
						q := s.faces[id]
						if q.Surface != pair[owner].Surface || q.Reversed != (owner == 1) {
							t.Fatal("both original roof owners and exact floor reversal must be retained", order, axes, i, owner)
						}
					}
				}
				p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
				if err != nil || math.Abs(p.Volume-v) > 1e-10 || math.Abs(p.Centroid.X-x) > 1e-10 ||
					math.Abs(p.Centroid.Y-.5) > 1e-10 {
					t.Fatal("closed result differs from the independently integrated original solid", order, axes, i, p, err)
				}
				var data, replay bytes.Buffer
				if err := WriteJSON(&data, s); err != nil {
					t.Fatal("write exact positive band", err)
				}
				q, err := ReadJSONContext(ctx, bytes.NewReader(data.Bytes()), JSONLimits{})
				if err != nil {
					t.Fatal("read exact positive band", err)
				}
				if err := WriteJSON(&replay, q); err != nil || !bytes.Equal(data.Bytes(), replay.Bytes()) {
					t.Fatal("native component must replay byte-identically", err)
				}
			}
		}
	}
	o := ProjectiveRoofSolidOptions{Sew: ExactSewOptions{MaxComparisons: 1}}
	q, err := BuildProjectiveSplineRoofBandDifference(ctx, f[1], f[0], [2]int{0, 1}, 0, o)
	if !errors.Is(err, geom.ErrConvergence) || q.Complete || len(q.Components) != 0 {
		t.Fatal("short native assembly cannot publish any positive-band prefix", q, err)
	}
}
