package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestSplineCornerFan(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	hint := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	var anchors [2]*SplineCornerAnchor
	for i, pair := range [][2]int{{0, 5}, {2, 5}} {
		box := geom.ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: .25}
		if i == 1 {
			box = geom.ParameterRectangle{UMin: 0, UMax: .25, VMin: -.125, VMax: .125}
		}
		r, e := CertifySplineFaceCornerAnchor(ctx, a, b, pair, [2]geom.ParameterRectangle{{UMax: .125, VMax: .125}, box}, 0, geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: geom.SurfacePairTubeOptions{FreeAxis: i}, Anchor: hint}, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(i, e)
		}
		anchors[i] = r
	}
	point, e := CertifySplineFacePointContact(ctx, a, b, [2]int{4, 5}, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	result, e := CertifySplineCornerFan(ctx, anchors, point, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !result.PendingAssembly() || result.Vertex() != anchors[0].Vertex() {
		t.Fatal("binding")
	}
	signs := result.PlaneSigns()
	changes := 0
	for i, v := range signs {
		if v == 0 {
			t.Fatal("zero sign")
		}
		if v != signs[(i+1)%3] {
			changes++
		}
	}
	if changes != 2 {
		t.Fatal(signs)
	}
	reversed, e := CertifySplineCornerFan(ctx, [2]*SplineCornerAnchor{anchors[1], anchors[0]}, point, SplineRectangleEmbeddingOptions{})
	if e != nil || reversed.OrientedEdges() != result.OrientedEdges() || reversed.OrientedFaces() != result.OrientedFaces() {
		t.Fatal("nondeterministic", e)
	}
	for _, bad := range [][2]*SplineCornerAnchor{{anchors[0], anchors[0]}, {anchors[0], nil}} {
		if r, e := CertifySplineCornerFan(ctx, bad, point, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
			t.Fatal("missing/repeated sector")
		}
	}
	if r, e := CertifySplineCornerFan(ctx, anchors, nil, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
		t.Fatal("missing point proof")
	}
	if r, e := CertifySplineCornerFan(ctx, anchors, point, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || r != nil {
		t.Fatal("budget")
	}
	// Corrupt only the retained point sector's orientation in a private fixture;
	// the immutable geometry evidence remains true, but its oriented link fails.
	badStock := a
	badStock.faces = append([]Face(nil), a.faces...)
	badStock.faces[4].Reversed = !badStock.faces[4].Reversed
	copied := [2]SplineCornerAnchor{*anchors[0], *anchors[1]}
	for i := range copied {
		copied[i].solids[0] = badStock
	}
	badPoint := *point
	badPoint.solids[0] = badStock
	if r, e := CertifySplineCornerFan(ctx, [2]*SplineCornerAnchor{&copied[0], &copied[1]}, &badPoint, SplineRectangleEmbeddingOptions{}); e == nil || r != nil {
		t.Fatal("broken oriented link")
	}
	if r, e := CertifySplineCornerFan(ctx, anchors, point, SplineRectangleEmbeddingOptions{MaxFaces: 1}); e == nil || r != nil {
		t.Fatal("face budget")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := CertifySplineCornerFan(canceled, anchors, point, SplineRectangleEmbeddingOptions{}); e != context.Canceled || r != nil {
		t.Fatal("cancel", e)
	}
}
