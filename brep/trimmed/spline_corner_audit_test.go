package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCornerTerminalAudit(t *testing.T) {
	a, af := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, bf := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	var pairs [][2]int
	for i := range af {
		for j := range bf {
			pairs = append(pairs, [2]int{i, j})
		}
	}
	s, _, e := CombineOpenPlaneSplineShells(context.Background(), a, b, parametric.Intersection, pairs, intersect.SplinePlanePairDiscoveryOptions{}, SplineShellOpenOptions{})
	if e == nil || len(s.Faces()) != 0 {
		t.Fatal("expected current corner rejection")
	}
	t.Log("full assembly", e, "analytic common volume 7/32")
	// Exact isolated terminal ownership already fails independently of discovery.
	p := intersect.Patch{Min: geom.Vec2{}, Max: geom.Vec2{X: 1, Y: 1}}
	hint := [4]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 3)}
	_, e = openPlaneTerminalEndpoint(p, p, hint)
	if e == nil {
		t.Fatal("corner unexpectedly accepted")
	}
	t.Log("ownership", e)
	for i, f := range af {
		if f.axis == 2 && f.fixed != 0 {
			continue
		}
		if f.axis != 2 && f.fixed != 0 {
			continue
		}
		for j, g := range bf {
			if g.axis != 2 || g.fixed != 0 {
				continue
			}
			var patches [2]intersect.Patch
			for k, face := range []Face{a.faces[i], b.faces[j]} {
				d, e := faceUVBounds(face)
				if e != nil {
					t.Fatal(e)
				}
				patches[k], e = intersectSplineFace(face, d)
				if e != nil {
					t.Fatal(e)
				}
			}
			proof, e := intersect.CertifyOpenSplinePlanePair(context.Background(), patches[0], patches[1], intersect.OverlappingGraphTraceOptions{})
			t.Log("pair", i, j, "complete", proof.Complete, "terminals", len(proof.Terminals), "error", e)
		}
	}
}

func makeCornerShearBox(t *testing.T, lo, hi [3]float64, warp float64) (Solid, []splineBoxFace) {
	t.Helper()
	var f []splineBoxFace
	var surfaces []*geom.BSplineSurface
	var d []geom.ParameterRectangle
	var rev []bool
	for axis := range 3 {
		u, v := (axis+1)%3, (axis+2)%3
		for side, x := range [2]float64{lo[axis], hi[axis]} {
			domain := geom.ParameterRectangle{UMin: lo[u], UMax: hi[u], VMin: lo[v], VMax: hi[v]}
			p := []geom.Vec3{}
			for _, a := range [2]float64{lo[u], hi[u]} {
				for _, b := range [2]float64{lo[v], hi[v]} {
					c := [3]float64{}
					c[axis], c[u], c[v] = x, a, b
					c[2] += .25 * (c[0] + c[1])
					p = append(p, geom.Vec3{X: c[0], Y: c[1], Z: c[2]})
				}
			}
			s, e := geom.NewBSplineSurface(1, 1, 2, 2, p, []float64{lo[u], lo[u], hi[u], hi[u]}, []float64{lo[v], lo[v], hi[v], hi[v]}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces, d, rev = append(surfaces, s), append(d, domain), append(rev, side == 0)
			f = append(f, splineBoxFace{axis, u, v, x, domain, s})
		}
	}
	s, _, e := SewSplineRectangles(context.Background(), surfaces, d, rev, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return s, f
}
