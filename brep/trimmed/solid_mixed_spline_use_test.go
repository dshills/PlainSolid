package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestStoredMixedQuarticSharedCoedge(t *testing.T) {
	ctx := context.Background()
	s := exactCreasedRoofShell(t)
	faces := s.Faces()
	u := faces[5].Outer[0]
	var poles []geom.Vec3
	for i := 0; i < 5; i++ {
		v := float64(i) / 4
		poles = append(poles, geom.Vec3{X: (1-v)*u.UVStart.X + v*u.UVEnd.X, Y: (1-v)*u.UVStart.Y + v*u.UVEnd.Y})
	}
	curve, e := geom.NewBSpline(4, poles, []float64{2, 2, 2, 2, 2, 6, 6, 6, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	faces[5].Outer[0].Curve = SplineUV{Curve: curve, Range: UVInterval{Min: 2, Max: 6}}
	s, e = NewSolid(s.Vertices(), s.Edges(), faces, s.tolerance)
	if e != nil {
		t.Fatal(e)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	o := StoredMixedClosedMeshOptions{Faces: make([]SolidMixedFaceMeshOptions, len(s.faces))}
	for i := range o.Faces {
		o.Faces[i] = SolidMixedFaceMeshOptions{Face: MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{Interior: CurvedQuadraticTrimMeshOptions{Deflection: .04}}}}
	}
	if _, e := s.TessellateMixedFaceCertified(ctx, 5, o.Faces[5]); e != nil {
		t.Fatal("face zero", e)
	}
	r, e := s.TessellateMixedClosedCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) > 0 {
		t.Fatal("display contacts")
	}
	v, _ := r.DisplayVolume.Float64()
	if math.Abs(v-37./36) > .03 {
		t.Fatal("independent volume", v)
	}
	found := false
	for _, use := range r.Network.Faces[5].Coedges {
		if use.Use == 0 && use.Loop == 0 {
			found = true
			if use.TrimParameters[0].Sign() != 0 || use.TrimParameters[len(use.TrimParameters)-1].Cmp(big.NewRat(1, 1)) != 0 {
				t.Fatal("original normalized range")
			}
		}
	}
	if !found {
		t.Fatal("missing original quartic use")
	}
	// Equal endpoints with a different interior cannot borrow the straight
	// shared model edge certificate.
	badPoles := append([]geom.Vec3(nil), poles...)
	badPoles[2].Y += .125
	badCurve, e := geom.NewBSpline(4, badPoles, []float64{2, 2, 2, 2, 2, 6, 6, 6, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	bad := s
	bad.faces = s.Faces()
	bad.faces[5].Outer[0].Curve = SplineUV{Curve: badCurve, Range: UVInterval{Min: 2, Max: 6}}
	if q, e := bad.TessellateMixedClosedCertified(ctx, o); e == nil || len(q.Mesh.Vertices) > 0 {
		t.Fatal("same-endpoint unequal interior accepted")
	}
}
func TestStoredMixedRestrictedRationalCubic(t *testing.T) {
	ctx := context.Background()
	c, e := geom.NewBSpline(3, []geom.Vec3{{X: 0, Y: 0}, {X: .25, Y: .5}, {X: .75, Y: .5}, {X: 1, Y: 0}}, []float64{2, 2, 2, 2, 6, 6, 6, 6}, []float64{1, 2, 2, 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, reverse := range []bool{false, true} {
		q := SplineUV{Curve: c, Range: UVInterval{Min: 2, Max: 6}, Reversed: reverse}
		converted, e := mixedStoredUseCurveContext(ctx, Use{Curve: q}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		original, e := geom.NewExactSplineCurve(c, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, t0 := range []*big.Rat{new(big.Rat), big.NewRat(1, 7), big.NewRat(1, 2), big.NewRat(1, 1)} {
			t1 := new(big.Rat).Set(t0)
			if reverse {
				t1.Sub(big.NewRat(1, 1), t1)
			}
			t1.Add(big.NewRat(2, 1), t1.Mul(t1, big.NewRat(4, 1)))
			net, _, e := original.BezierControlNet(ctx, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			local := new(big.Rat).Quo(new(big.Rat).Sub(t1, big.NewRat(2, 1)), big.NewRat(4, 1))
			h := net.Control
			for n := len(h) - 1; n > 0; n-- {
				for i := 0; i < n; i++ {
					for k := 0; k < 4; k++ {
						h[i][k] = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), local), h[i][k]), new(big.Rat).Mul(local, h[i+1][k]))
					}
				}
			}
			point := [2]*big.Rat{new(big.Rat).Quo(h[0][0], h[0][3]), new(big.Rat).Quo(h[0][1], h[0][3])}
			uv, e := mixedExactEvaluate(converted, t0)
			if e != nil {
				t.Fatal(e)
			}
			if uv[0].Cmp(point[0]) != 0 || uv[1].Cmp(point[1]) != 0 {
				t.Fatal("source rational identity")
			}
		}
	}
}

func TestStoredHigherSplineGuards(t *testing.T) {
	c, e := geom.NewBSpline(3, []geom.Vec3{{}, {X: .25, Y: .5}, {X: .75, Y: .5}, {X: 1}}, []float64{2, 2, 2, 2, 6, 6, 6, 6}, []float64{1, 2, 2, 1})
	if e != nil {
		t.Fatal(e)
	}
	q := SplineUV{Curve: c, Range: UVInterval{Min: 2, Max: 6}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if x, e := mixedStoredUseCurveContext(ctx, Use{Curve: q}, geom.SurfacePatchOptions{}); e == nil || x != nil {
		t.Fatal("cancelled conversion")
	}
	if x, e := mixedStoredUseCurveContext(context.Background(), Use{Curve: q}, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || x != nil {
		t.Fatal("conversion budget")
	}
	// Restriction has exact nonbinary endpoints; legacy ExactBezierUV cannot
	// own them, and must fail rather than silently replacing source endpoints.
	q.Range = UVInterval{Min: 3, Max: 5}
	if x, e := mixedStoredUseCurveContext(context.Background(), Use{Curve: q}, geom.SurfacePatchOptions{}); e == nil || x != nil {
		t.Fatal("unrepresentable endpoint accepted")
	}
}
