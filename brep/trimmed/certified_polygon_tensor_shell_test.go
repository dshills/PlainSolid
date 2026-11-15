package trimmed

import (
	"bytes"
	"context"
	"math"
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
func TestCertifiedPolygonTensorShell(t *testing.T) {
	ctx := context.Background()
	s := exactCreasedRoofShell(t)
	var data bytes.Buffer
	if e := WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	back, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	o := CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .01, PolygonMultiSpan: &PolygonMultiSpanMeshOptions{}}}
	for _, body := range []Solid{s, back} {
		r, e := body.TessellatePolygonShellCertified(ctx, o)
		if e != nil {
			t.Fatal(e)
		}
		if !r.BothDirections || r.KnotPieces == 0 || len(r.Uses) != 24 {
			t.Fatal("missing source proof")
		}
		if e := r.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		// Integral f=1/6, integral v(1-v)=1/6, so source volume37/36.
		if math.Abs(r.Mesh.SignedVolume()-37./36) > .01 {
			t.Fatal("independent mass", r.Mesh.SignedVolume())
		}
		edges := map[int][][]int{}
		for _, u := range r.Uses {
			ids := append([]int(nil), u.Vertices...)
			native := body.faces[u.Face].Outer[u.Use]
			if native.Reversed {
				for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
					ids[i], ids[j] = ids[j], ids[i]
				}
			}
			edges[u.Edge] = append(edges[u.Edge], ids)
		}
		for _, pair := range edges {
			if len(pair) != 2 || len(pair[0]) != len(pair[1]) {
				t.Fatal("shared native edge missing")
			}
			for i, id := range pair[0] {
				if id != pair[1][i] {
					t.Fatal("seam IDs differ")
				}
			}
		}
	}
	// Exhausted per-face knot budgets must return no partial shell.
	o.Face.PolygonMultiSpan = &PolygonMultiSpanMeshOptions{MaxTotalCells: 2}
	if r, e := s.TessellatePolygonShellCertified(ctx, o); e == nil || len(r.Mesh.Vertices) > 0 || len(r.Uses) > 0 {
		t.Fatal("reset per-face cell budget")
	}
	o.Face.PolygonMultiSpan = &PolygonMultiSpanMeshOptions{}
	broken := s
	broken.faces = s.Faces()
	broken.faces[5].Outer[0].Reversed = !broken.faces[5].Outer[0].Reversed
	if r, e := broken.TessellatePolygonShellCertified(ctx, o); e == nil || len(r.Mesh.Vertices) > 0 {
		t.Fatal("wrong original edge direction accepted")
	}

}
