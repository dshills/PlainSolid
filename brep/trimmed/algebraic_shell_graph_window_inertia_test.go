package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphInertiaDegreeLimitPreservesFirstMoments(t *testing.T) {
	var controls [][4]*big.Rat
	for i := 0; i < 5; i++ {
		for j := 0; j < 2; j++ {
			z := new(big.Rat)
			if i == 4 {
				z.SetInt64(1)
			}
			controls = append(controls, [4]*big.Rat{big.NewRat(int64(i), 4), big.NewRat(int64(j), 1), z, big.NewRat(1, 1)})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 4, 1, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	g, e := geom.NewAffineProjectedGraph(context.Background(), p, [2]int{0, 1}, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := AlgebraicGraphMaterialQuery{patches: [2]*geom.SurfacePatch{p, p}, graphs: [2]*geom.AffineProjectedGraph{g, g}, axes: [2]int{0, 1}, residual: 2, regions: []graphMaterialRegion{{faces: [2]int{0, 1}, upper: 1}}}
	leaf := AlgebraicGraphWindowLeaf{Faces: [2]int{0, 1}, Window: graphWindow(big.NewRat(3, 8), big.NewRat(3, 8), big.NewRat(1, 8))}
	if _, e := q.integrateGraphPolynomialCellMode(context.Background(), leaf, 262144, 16384, 1); e != nil {
		t.Fatal("legacy first-moment degree narrowed", e)
	}
	result, e := q.integrateGraphPolynomialCellMode(context.Background(), leaf, 1048576, 16384, 2)
	if !errors.Is(e, geom.ErrConvergence) || result[0] != nil {
		t.Fatal("second degree gate", e)
	}
}
