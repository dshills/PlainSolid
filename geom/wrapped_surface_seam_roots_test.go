package geom

import (
	"context"
	"testing"
)

func TestWrappedAlgebraicSeamRoots(t *testing.T) {
	ctx := context.Background()
	c, _ := NewCircle(Vec3{}, Vec3{Z: 1}, 2)
	curve, _ := c.RationalBSpline()
	var poles []Vec3
	var weights []float64
	for i, p := range curve.poles {
		for _, z := range []float64{4, -6, 9} {
			poles = append(poles, Vec3{p.X, p.Y, z})
			weights = append(weights, curve.weights[i])
		}
	}
	s, e := NewBSplineSurface(2, 2, len(curve.poles), 3, poles, curve.knots, []float64{-2, -2, -2, 3, 3, 3}, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	plane, _ := NewPlane(Vec3{Z: .5}, Vec3{Z: 1})
	r, e := TraceWrappedSurfacePlaneContext(ctx, w, plane, SurfaceTraceOptions{})
	if e != nil || !r.Complete || len(r.Contours) != 2 || len(r.SeamJoins) != 2 {
		t.Fatal(r, e)
	}
	for _, c := range r.Contours {
		if !c.Closed {
			t.Fatal("algebraic circle not closed")
		}
	}
	for _, j := range r.SeamJoins {
		p := j.RootCertificate
		if !p.Matched || !p.TransverseContinuation || p.Lower.Cmp(p.Upper) >= 0 {
			t.Fatal(p)
		}
		// No rational point was recovered, and the root remains bracketed.
		contour := r.Fundamental.Contours[j.Contours[0]]
		a := contour.Arcs[0]
		end := 0
		if j.Ends[0] == 1 {
			a = contour.Arcs[len(contour.Arcs)-1]
			end = 1
		}
		if a.Reversed {
			end = 1 - end
		}
		if _, e := r.Fundamental.Isolation.Arcs[a.Index].ExactEndpoint(end, ArcExactEndpointOptions{}); e == nil {
			t.Fatal("expected irrational root")
		}
	}
}
