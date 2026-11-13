package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestOffAxisSphereFiniteUnionExposedSupports(t *testing.T) {
	s, e := NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 4}, 0, 4)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p Vec3
		l BorePointLocation
		m BoreBoundaryMask
	}{
		{Vec3{X: 2}, BoreInside, 0}, {Vec3{X: 4, Z: 3}, BoreInside, 0}, {Vec3{X: 6}, BoreBoundary, BoreLowerCap},
		{Vec3{X: 5}, BoreBoundary, BoreSphere | BoreLowerCap}, {Vec3{X: 7, Z: 2}, BoreBoundary, BoreCylinder},
		{Vec3{X: 3, Z: 4}, BoreBoundary, BoreSphere | BoreUpperCap}, {Vec3{X: 1, Z: 2}, BoreInside, 0},
	} {
		l, m, e := s.Classify(tc.p)
		if e != nil || l != tc.l || m != tc.m {
			t.Fatal(tc, l, m, e)
		}
	}
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	eq := func(r *ExactAlgebraicRoot, w int64) {
		t.Helper()
		q, _, e := NewExactRationalRoot(ctx, big.NewRat(w, 1), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, e := r.Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !v.Proven || v.Order != 0 {
			t.Fatal(w, v, e)
		}
	}
	r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 1 || len(r.Hits) != 1 {
		t.Fatal(r, e)
	}
	eq(r.Hits[0].Parameter, -5)
	v := r.Intervals[0]
	eq(v.Start, 5)
	eq(v.End, 7)
	if v.StartBoundary != BoreSphere|BoreLowerCap || v.EndBoundary != BoreCylinder|BoreLowerCap {
		t.Fatal(v)
	}
	r, e = s.IntersectLineBoundary(ctx, Vec3{X: 1}, Vec3{Z: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 0 || len(r.Hits) != 2 {
		t.Fatal("covered wall", r, e)
	}
	for _, h := range r.Hits {
		if h.Boundary != BoreSphere {
			t.Fatal(h)
		}
	}
	r, e = s.IntersectLineBoundary(ctx, Vec3{X: 7}, Vec3{Z: 1}, -8, 8, o)
	if e != nil || len(r.Intervals) != 1 || len(r.Hits) != 0 {
		t.Fatal(r, e)
	}
	eq(r.Intervals[0].Start, 0)
	eq(r.Intervals[0].End, 4)
}

func TestOffAxisSphereFiniteUnionRegularization(t *testing.T) {
	ctx := context.Background()
	o := ToroidLineOptions{ParameterTolerance: 1e-8}
	// Externally tangent components retain both genuinely exposed boundaries.
	s, e := NewOffAxisSphereFiniteCylinderUnion(5, 1, Vec2{X: 6}, -1, 1)
	if e != nil {
		t.Fatal(e)
	}
	l, m, e := s.Classify(Vec3{X: 5})
	if e != nil || l != BoreBoundary || m != BoreSphere|BoreCylinder {
		t.Fatal(l, m, e)
	}
	h, e := s.IntersectLine(ctx, Vec3{}, Vec3{X: 1}, -8, 8, o)
	if e != nil || len(h) != 3 {
		t.Fatal(h, e)
	}
	// Cylinder fully in sphere, including coincident rim at z=±4, r=3.
	s, e = NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{}, -4, 4)
	if e != nil {
		t.Fatal(e)
	}
	_, m, e = s.Classify(Vec3{X: 3, Z: 4})
	if e != nil || m != BoreSphere {
		t.Fatal(m, e)
	}
	// Sphere fully in cylinder: cap coincidence has no exposed sphere patch.
	s, e = NewOffAxisSphereFiniteCylinderUnion(5, 5, Vec2{}, -5, 5)
	if e != nil {
		t.Fatal(e)
	}
	_, m, e = s.Classify(Vec3{Z: 5})
	if e != nil || m != BoreUpperCap {
		t.Fatal(m, e)
	}
	s, e = NewOffAxisSphereFiniteCylinderUnion(5, 2, Vec2{}, 5, 6)
	if e != nil {
		t.Fatal(e)
	}
	_, m, e = s.Classify(Vec3{Z: 5})
	if e != nil || m != BoreSphere|BoreLowerCap {
		t.Fatal(m, e)
	}
	if _, e := NewOffAxisSphereFiniteCylinderUnion(5, 1, Vec2{X: 4}, -1, 1); e == nil {
		t.Fatal("unsupported internal tangency")
	}
	r, e := s.IntersectLineBoundary(ctx, Vec3{}, Vec3{Z: 1}, -8, 8, ToroidLineOptions{ParameterTolerance: 1e-8, Surface: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || len(r.Hits)+len(r.Intervals) != 0 {
		t.Fatal(r, e)
	}
}

func TestOffAxisSphereFiniteBooleanClassificationIdentities(t *testing.T) {
	u, _ := NewOffAxisSphereFiniteCylinderUnion(5, 3, Vec2{X: 4}, 0, 4)
	c, _ := NewOffAxisSphereFiniteCylinderCommon(5, 3, Vec2{X: 4}, 0, 4)
	d, _ := NewOffAxisSphereFiniteCylinderDifference(5, 3, Vec2{X: 4}, 0, 4)
	// Independent integer grid evaluates source inequalities; skip boundaries
	// only for complement identity, whose regularized closure is intentional.
	for x := -6; x <= 8; x++ {
		for y := -6; y <= 6; y++ {
			for z := -6; z <= 6; z++ {
				a := x*x + y*y + z*z - 25
				b := (x-4)*(x-4) + y*y - 9
				p := Vec3{X: float64(x), Y: float64(y), Z: float64(z)}
				ul, _, e := u.Classify(p)
				if e != nil {
					t.Fatal(e)
				}
				cl, _, e := c.Classify(p)
				if e != nil {
					t.Fatal(e)
				}
				sphere := a <= 0
				tool := b <= 0 && z >= 0 && z <= 4
				if (ul != BoreOutside) != (sphere || tool) || (cl != BoreOutside) != (sphere && tool) {
					t.Fatal("set identity", p, ul, cl)
				}
				if a == 0 || b == 0 || z == 0 || z == 4 {
					continue
				}
				dl, _, e := d.Classify(p)
				if e != nil || (dl != BoreOutside) != (sphere && !tool) {
					t.Fatal("difference identity", p, dl, e)
				}
			}
		}
	}
}
