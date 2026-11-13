package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactBezierGraphChainC0(t *testing.T) {
	s, _ := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	// Left y=1/2+t²/4 on x=t/2. Right y=3/4-t/4+t²/8,
	// x=(1+t)/2. C0 junction has opposite tangent slopes.
	// Exact total area = (1/2)(7/12+2/3)=5/8.
	pieces := []ExactBezierUV{}
	for side, n := range []int{3, 4} {
		h := make([][3]*big.Rat, n+1)
		for i := range h {
			x := big.NewRat(int64(i+side*n), int64(2*n))
			y := big.NewRat(1, 2)
			if side == 0 {
				y.Add(y, big.NewRat(int64(i*(i-1)), int64(4*n*(n-1))))
			} else {
				y = big.NewRat(3, 4)
				y.Sub(y, big.NewRat(int64(i), int64(4*n)))
				y.Add(y, big.NewRat(int64(i*(i-1)), int64(8*n*(n-1))))
			}
			h[i] = [3]*big.Rat{x, y, big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		pieces = append(pieces, q)
	}
	o := ExactBezierGraphFaceOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .001}}}
	r, e := TessellateExactBezierGraphChainCertified(context.Background(), p, pieces, new(big.Rat), o)
	if e != nil {
		t.Fatal(e)
	}
	area, _ := r.PolygonUVArea.Float64()
	if math.Abs(area-5./8) > .001 {
		t.Fatal("independent integral", area)
	}
	if r.PieceVertices[0][len(r.PieceVertices[0])-1] != r.PieceVertices[1][0] || r.JunctionVertices[1] != r.PieceVertices[1][0] {
		t.Fatal("junction duplicated")
	}
	for i, proof := range r.Pieces {
		for j, k := range r.PieceVertices[i] {
			if r.Mesh.Vertices[k] != proof.Curve.Vertices[j].Point {
				t.Fatal("changed seam point")
			}
		}
	}
	if len(r.Mesh.Triangles) != len(r.Boundary)-2 || r.FillSquaredDistance.Sign() <= 0 {
		t.Fatal("coverage/bound")
	}
	squared, _ := r.FillSquaredDistance.Float64()
	bound := math.Sqrt(squared)
	height := func(x float64) float64 {
		if x <= .5 {
			u := 2 * x
			return .5 + u*u/4
		}
		u := 2*x - 1
		return .75 - u/4 + u*u/8
	}
	for _, v := range r.Mesh.Vertices {
		if v.Y-height(v.X) > bound {
			t.Fatal("display-to-source vertical witness exceeds bound")
		}
	}
	// Independent planar point/triangle distance checks source-to-display.
	for ix := 0; ix <= 40; ix++ {
		x := float64(ix) / 40
		for iy := 0; iy <= 10; iy++ {
			point := geom.Vec3{X: x, Y: height(x) * float64(iy) / 10}
			best := math.Inf(1)
			for _, tri := range r.Mesh.Triangles {
				inside := true
				for j, a := range tri {
					b := tri[(j+1)%3]
					pa, pb := r.Mesh.Vertices[a], r.Mesh.Vertices[b]
					edge := pb.Sub(pa)
					if edge.Cross(point.Sub(pa)).Z < -1e-15 {
						inside = false
					}
					fraction := max(0., min(1., point.Sub(pa).Dot(edge)/edge.Dot(edge)))
					best = min(best, point.Sub(pa.Add(edge.Scale(fraction))).Length())
				}
				if inside {
					best = 0
					break
				}
			}
			if best > bound+1e-14 {
				t.Fatal("source-to-display independent witness", point, best, bound)
			}
		}
	}
	for _, delta := range []*big.Rat{big.NewRat(1, 64), big.NewRat(-1, 64)} {
		h := pieces[1].HomogeneousControls()
		h[0][0].Add(h[0][0], delta)
		bad, _ := NewExactBezierUV(h)
		v, e := TessellateExactBezierGraphChainCertified(context.Background(), p, []ExactBezierUV{pieces[0], bad}, new(big.Rat), o)
		if e == nil || len(v.Mesh.Vertices) != 0 {
			t.Fatal("gap/overlap accepted")
		}
	}
	h := pieces[1].HomogeneousControls()
	h[0][1].Add(h[0][1], big.NewRat(1, 64))
	bad, _ := NewExactBezierUV(h)
	if _, e = TessellateExactBezierGraphChainCertified(context.Background(), p, []ExactBezierUV{pieces[0], bad}, new(big.Rat), o); e == nil {
		t.Fatal("discontinuous join accepted")
	}
	o.Display.MaxSegments = 1
	if v, e := TessellateExactBezierGraphChainCertified(context.Background(), p, pieces, new(big.Rat), o); e == nil || len(v.Mesh.Vertices) != 0 {
		t.Fatal("global budget ignored")
	}
}
