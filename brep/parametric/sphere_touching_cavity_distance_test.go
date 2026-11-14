package parametric

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestNativeSphereTouchingCavityStillRejected(t *testing.T) {
	if _, e := NewSpherePairSet(context.Background(), Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 4}, Radius: 1}, Difference, 1e-9); e == nil {
		t.Fatal("analytic nearest must not imply native manifold assembly")
	}
}
