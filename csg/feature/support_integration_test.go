package feature

import (
	"context"
	"errors"
	"plainsolid/brep"
	"plainsolid/brep/support"
	"plainsolid/geom"
	"testing"
)

func TestFeatureSupportDiagnostic(t *testing.T) {
	s, e := brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p := []geom.Vec3{{X: .25, Y: .25, Z: 1}, {X: .75, Y: .25, Z: 1}, {X: .5, Y: .75, Z: 1}}
	_, e = Extrude(context.Background(), s, 1, p, geom.Vec3{Z: 1}, Protrusion, Options{})
	var v *support.Violation
	if !errors.Is(e, ErrInvalid) || !errors.Is(e, support.ErrNonconvex) || !errors.As(e, &v) || v.Residual.Sign() <= 0 {
		t.Fatal(e)
	}
}
