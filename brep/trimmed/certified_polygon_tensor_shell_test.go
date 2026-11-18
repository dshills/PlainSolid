package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func exactCreasedRoofShell(t *testing.T) Solid {
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	// z=1+f(u)*v*(1-v), f triangular with peak1/3 at u=3/8.
	// Every original roof edge remains the same exact straight model edge.
	r := geom.ExactTensorSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 2, CountU: 3, CountV: 3, KnotsU: []float64{0, 0, .375, 1, 1}, KnotsV: []float64{0, 0, 0, 1, 1, 1}}
	for i, u := range []*big.Rat{new(big.Rat), big.NewRat(3, 8), big.NewRat(1, 1)} {
		for j, v := range []*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)} {
			z := big.NewRat(1, 1)
			if i == 1 && j == 1 {
				z = big.NewRat(7, 6)
			}
			r.Controls = append(r.Controls, [4]string{u.RatString(), v.RatString(), z.RatString(), "1"})
		}
	}
	p, e := geom.ImportExactTensorSurfaceRecipe(context.Background(), r, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	surface, e := geom.NewExactTensorSurface(context.Background(), p, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	parts := localSewFaces(s)
	parts[5].Face.Surface = surface
	result, _, e := Sew(context.Background(), parts, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal("independent native sewing", e)
	}
	return result
}
