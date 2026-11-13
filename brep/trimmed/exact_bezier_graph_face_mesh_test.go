package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactBezierAffineGraphFace(t *testing.T) {
	poles := []geom.Vec3{{}, {X: 1, Y: 3, Z: 2}, {X: 2, Z: 1}, {X: 3, Y: 3, Z: 3}}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, degree := range []int{3, 4} {
		h := make([][3]*big.Rat, degree+1)
		for i := range h {
			y := big.NewRat(1, 2)
			y.Add(y, big.NewRat(int64(i), int64(4*degree)))
			y.Add(y, big.NewRat(int64(i*(i-1)), int64(8*degree*(degree-1))))
			h[i] = [3]*big.Rat{big.NewRat(int64(i), int64(degree)), y, big.NewRat(1, 1)}
		}
		q, e := NewExactBezierUV(h)
		if e != nil {
			t.Fatal(e)
		}
		o := ExactBezierGraphFaceOptions{Display: geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: .001}}}
		r, e := TessellateExactBezierGraphFaceCertified(context.Background(), p, q, new(big.Rat), o)
		if e != nil {
			t.Fatal(e)
		}
		// Independently x(t)=t,y(t)=1/2+t/4+t²/8, integral=2/3.
		area, _ := r.PolygonUVArea.Float64()
		if area < 2./3 || area-2./3 > .001 {
			t.Fatal("analytic polygon area", area)
		}
		if math.Abs(r.Mesh.Area()-area*math.Sqrt(54)) > 1e-12 {
			t.Fatal("affine area Jacobian")
		}
		if r.FillSquaredDistance.Sign() <= 0 {
			t.Fatal("missing nonzero fill bound")
		}
		for i, id := range r.CurveVertices {
			if r.Mesh.Vertices[id] != r.Curve.Vertices[i].Point {
				t.Fatal("source seam display changed")
			}
		}
		if len(r.Mesh.Triangles) != len(r.Boundary)-2 {
			t.Fatal("triangle coverage")
		}
		weighted := q.HomogeneousControls()
		for axis := range weighted[1] {
			weighted[1][axis].Mul(weighted[1][axis], big.NewRat(3, 2))
		}
		rational, err := NewExactBezierUV(weighted)
		if err != nil {
			t.Fatal(err)
		}
		rationalMesh, err := TessellateExactBezierGraphFaceCertified(context.Background(), p, rational, new(big.Rat), o)
		if err != nil {
			t.Fatal("positive rational graph", err)
		}
		integral := 0.
		for j := 0; j <= 1000; j++ {
			u := float64(j) / 1000
			uv, err := rational.EvalUV(u)
			if err != nil {
				t.Fatal(err)
			}
			derivative, err := rational.DerivativeUV(u)
			if err != nil {
				t.Fatal(err)
			}
			factor := 2.
			if j == 0 || j == 1000 {
				factor = 1
			} else if j%2 == 1 {
				factor = 4
			}
			integral += factor * uv.Y * derivative.X / 3000
		}
		polygon, _ := rationalMesh.PolygonUVArea.Float64()
		if math.Abs(polygon-integral) > .001 {
			t.Fatal("independent rational integral", polygon, integral)
		}
		denied, e := TessellateExactBezierGraphFaceCertified(context.Background(), p, q, big.NewRat(1, 2), o)
		if e == nil || len(denied.Mesh.Vertices) != 0 {
			t.Fatal("accepted touching baseline")
		}
		bad := q.HomogeneousControls()
		bad[1][0] = big.NewRat(-1, 8)
		outside, _ := NewExactBezierUV(bad)
		if _, e = TessellateExactBezierGraphFaceCertified(context.Background(), p, outside, new(big.Rat), o); e == nil {
			t.Fatal("accepted nonmonotone/outside hull")
		}
		limited := o
		limited.MaxTriangles = 1
		denied, e = TessellateExactBezierGraphFaceCertified(context.Background(), p, q, new(big.Rat), limited)
		if e == nil || len(denied.Mesh.Vertices) != 0 {
			t.Fatal("partial limited mesh")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e = TessellateExactBezierGraphFaceCertified(ctx, p, q, new(big.Rat), o); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
		curved, _, _, _ := bezierSeamFixture(t, degree)
		if _, e = TessellateExactBezierGraphFaceCertified(context.Background(), curved, q, new(big.Rat), o); e == nil {
			t.Fatal("accepted nonaffine support")
		}
	}
}
