package trimmed

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func wrappedOverlapSupport(t testing.TB) *geom.WrappedSurfacePatch {
	t.Helper()
	profile := []geom.Vec2{{X: 2}, {X: 2, Y: 2}, {Y: 2}, {X: -2, Y: 2}, {X: -2}, {X: -2, Y: -2}, {Y: -2}, {X: 2, Y: -2}, {X: 2}}
	var poles []geom.Vec3
	for _, p := range profile {
		for _, z := range []float64{-2, 3} {
			poles = append(poles, geom.Vec3{X: p.X, Y: p.Y, Z: z})
		}
	}
	s, e := geom.NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{-2, -2, 3, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}
