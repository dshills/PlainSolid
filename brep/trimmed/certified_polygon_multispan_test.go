package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedPolygonNonbinaryKnotCrossing(t *testing.T) {
	ctx := context.Background()
	recipe := geom.ExactTensorSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 2, KnotsU: []float64{0, 0, .375, 1, 1}, KnotsV: []float64{0, 0, 1, 1}}
	for _, u := range []*big.Rat{new(big.Rat), big.NewRat(3, 8), big.NewRat(1, 1)} {
		for _, v := range []*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
			z := new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Mul(u, v), big.NewRat(3, 1)), new(big.Rat).Quo(new(big.Rat).Abs(new(big.Rat).Sub(u, big.NewRat(3, 8))), big.NewRat(5, 1)))
			recipe.Controls = append(recipe.Controls, [4]string{u.RatString(), v.RatString(), z.RatString(), "1"})
		}
	}
	data, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	var decoded geom.ExactTensorSurfaceRecipe
	if e = json.Unmarshal(data, &decoded); e != nil {
		t.Fatal(e)
	}
	p, e := geom.ImportExactTensorSurfaceRecipe(ctx, decoded, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := p.ToBSplineSurface(); e == nil {
		t.Fatal("rounded representable surrogate")
	}
	surface, e := geom.NewExactTensorSurface(ctx, p, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := concaveOriginalHoleFixture(t)
	a.faces[0].Surface = surface
	o := CertifiedPatchMeshOptions{Deflection: .04, PolygonMultiSpan: &PolygonMultiSpanMeshOptions{}}
	r, e := a.TessellatePolygonFaceCertified(ctx, 0, o)
	if e != nil {
		t.Fatal(e)
	}
	if !r.BothDirections || r.KnotCells == 0 || r.KnotPieces == 0 || len(r.Uses) != 10 {
		t.Fatal("missing full-span proof")
	}
	cross := 0
	area := 0.
	for i, uv := range r.UVTriangles {
		lo, hi := uv[0].X, uv[0].X
		for _, p := range uv {
			lo = min(lo, p.X)
			hi = max(hi, p.X)
		}
		if lo < .375 && hi > .375 {
			cross++
		}
		area += ((uv[1].X-uv[0].X)*(uv[2].Y-uv[0].Y) - (uv[1].Y-uv[0].Y)*(uv[2].X-uv[0].X)) / 2
		for _, weights := range [][3]float64{{1. / 3, 1. / 3, 1. / 3}, {.125, .375, .5}, {.5, .375, .125}} {
			u, v, z := 0., 0., 0.
			for j, w := range weights {
				u += w * uv[j].X
				v += w * uv[j].Y
				z += w * r.Mesh.Vertices[r.Mesh.Triangles[i][j]].Z
			}
			err := z - (u*v/3 + math.Abs(u-.375)/5)
			bound, _ := r.SquaredErrors[i].Float64()
			if err*err > bound+1e-27 || bound > .04*.04 {
				t.Fatal("independent two-span residual", err, bound)
			}
		}
	}
	if cross == 0 || math.Abs(area-15./32) > 1e-14 {
		t.Fatal("no true crossing or wrong area", cross, area)
	}
	// Original slanted hole use1 crosses the knot at parameter2/3, a
	// nonbinary event. Display samples need not lie on that knot; their exact
	// original parameters must bracket it while cell clipping proves coverage.
	found := false
	for _, use := range r.Uses {
		if use.Loop == 1 && use.Use == 1 {
			for i := 1; i < len(use.Parameters); i++ {
				if use.Parameters[i-1].Cmp(big.NewRat(2, 3)) < 0 && use.Parameters[i].Cmp(big.NewRat(2, 3)) > 0 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("original crossing lineage")
	}
	o.PolygonMultiSpan.MaxTotalCells = 1
	if q, e := a.TessellatePolygonFaceCertified(ctx, 0, o); e == nil || len(q.Mesh.Vertices) > 0 || len(q.Uses) > 0 {
		t.Fatal("cumulative cell quota")
	}
	bad := decoded
	bad.CountU = 4
	bad.KnotsU = []float64{0, 0, .375, .375, 1, 1}
	bad.Controls = append([][4]string(nil), decoded.Controls[:4]...)
	for _, control := range decoded.Controls[2:4] {
		z, _ := new(big.Rat).SetString(control[2])
		control[2] = z.Add(z, big.NewRat(1, 7)).RatString()
		bad.Controls = append(bad.Controls, control)
	}
	bad.Controls = append(bad.Controls, decoded.Controls[4:]...)
	if _, e := geom.ImportExactTensorSurfaceRecipe(ctx, bad, geom.SurfacePatchOptions{}); e == nil {
		t.Fatal("full-multiplicity discontinuous tensor accepted")
	}
}
