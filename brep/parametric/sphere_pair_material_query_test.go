package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestBoundSpherePairMaterial(t *testing.T) {
	ctx := context.Background()
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []float64{-1, 1} {
			vector := func(v float64) geom.Vec3 {
				var x [3]float64
				x[axis] = sign * v
				return geom.Vec3{X: x[0], Y: x[1], Z: x[2]}
			}
			for _, op := range []BooleanOp{Union, Intersection, Difference} {
				a, b := Ball{Radius: 5}, Ball{Center: vector(4), Radius: 3}
				s, e := NewSpherePairSet(ctx, a, b, op, 1e-8)
				if e != nil {
					t.Fatal(axis, sign, op, e)
				}
				owners := s.SourceFaces()
				if len(owners[0]) != 1 || len(owners[1]) != 1 || owners[0][0] == owners[1][0] {
					t.Fatal("source ownership", owners)
				}
				m, e := s.Solid().MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-8})
				want := 180 * math.Pi
				if op == Intersection {
					want = 68 * math.Pi / 3
				} else if op == Difference {
					want = 144 * math.Pi
				}
				if e != nil || math.Abs(m.Volume-want) > 1e-5 {
					t.Fatal("independent spherical cap volume", m, e, want)
				}
				expected := geom.SpherePairFirst
				if op == Intersection {
					expected = geom.SpherePairSecond
				}
				hit, e := s.FirstBoundaryHit(ctx, vector(-6), vector(1), 15, o)
				if e != nil || !hit.HasHit || hit.Boundary != expected {
					t.Fatal("bound query", hit, e)
				}
				if axis == 0 && sign == 1 && op == Union {
					mesh, e := s.Solid().Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
					if e != nil || mesh.ValidateSolid() != nil {
						t.Fatal("native mesh", e)
					}
					hits, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
					if e != nil || !hits.Complete || len(hits.Contacts) != 0 {
						t.Fatal("native embedding", hits, e)
					}
				}
				owners[0][0] = 999
				b.Center = vector(100)
				if s.SourceFaces()[0][0] == 999 {
					t.Fatal("mutable ancestry")
				}
				loc, _, e := s.Classify(vector(2))
				wantLoc := geom.BoreInside
				if op == Difference {
					wantLoc = geom.BoreOutside
				}
				if e != nil || loc != wantLoc {
					t.Fatal("immutable model", loc, e)
				}
			}
		}
	}
}
func TestBoundSpherePairModesAndGuards(t *testing.T) {
	ctx := context.Background()
	a := Ball{Radius: 5}
	for _, op := range []BooleanOp{Union, Intersection, Difference} {
		s, e := NewSpherePairSet(ctx, a, a, op, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		owners := s.SourceFaces()
		if op == Difference {
			if len(s.Solid().Faces()) != 0 {
				t.Fatal("identical empty difference")
			}
		} else if len(s.Solid().Faces()) != 1 || len(owners[0]) != 1 || len(owners[1]) != 1 || owners[0][0] != owners[1][0] {
			t.Fatal("coincident owners", owners)
		}
	}
	inner := Ball{Center: geom.Vec3{X: 1}, Radius: 1}
	for _, swap := range []bool{false, true} {
		first, second := a, inner
		if swap {
			first, second = second, first
		}
		s, e := NewSpherePairSet(ctx, first, second, Union, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		owner := 0
		if swap {
			owner = 1
		}
		if len(s.SourceFaces()[owner]) != 1 || len(s.SourceFaces()[1-owner]) != 0 {
			t.Fatal("contained ownership", s.SourceFaces())
		}
	}
	tangent := Ball{Center: geom.Vec3{X: 8}, Radius: 3}
	if s, e := NewSpherePairSet(ctx, a, tangent, Union, 1e-8); e == nil || len(s.Solid().Faces()) != 0 {
		t.Fatal("nonmanifold native union accepted", e)
	}
	cavity := Ball{Center: geom.Vec3{X: 2}, Radius: 3}
	if s, e := NewSpherePairSet(ctx, a, cavity, Difference, 1e-8); e == nil || len(s.Solid().Faces()) != 0 {
		t.Fatal("nonmanifold native cavity accepted", e)
	}
	s, e := NewSpherePairSet(ctx, a, tangent, Intersection, 1e-8)
	if e != nil || len(s.Solid().Faces()) != 0 {
		t.Fatal("regularized empty native common", e)
	}
	if s, e := NewSpherePairSet(ctx, a, Ball{Center: geom.Vec3{X: 3, Y: 4}, Radius: 3}, Union, 1e-8); e == nil || len(s.Solid().Faces()) != 0 {
		t.Fatal("unproved frame accepted", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, e := NewSpherePairSet(canceled, a, inner, Union, 1e-8); e != context.Canceled || len(s.Solid().Faces()) != 0 {
		t.Fatal("atomic cancel", e)
	}
	if _, _, e := (SpherePairSet{}).Classify(geom.Vec3{}); e == nil {
		t.Fatal("unbound zero query")
	}
}

func TestBoundSpherePairReversedSources(t *testing.T) {
	s, e := NewSpherePairSet(context.Background(), Ball{Center: geom.Vec3{X: 4}, Radius: 3}, Ball{Radius: 5}, Difference, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	m, e := s.Solid().MassProperties(context.Background(), IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(m.Volume-40*math.Pi/3) > 1e-5 {
		t.Fatal("reversed difference cap", m, e)
	}
	r, e := s.ClipLine(context.Background(), geom.Vec3{}, geom.Vec3{X: 1}, 0, 10, geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Intervals) != 1 || r.Intervals[0].StartBoundary != geom.SpherePairSecond || r.Intervals[0].EndBoundary != geom.SpherePairFirst {
		t.Fatal("original input masks", r, e)
	}
}

func TestBoundSpherePairHorizontalMaterial(t *testing.T) {
	s, e := NewSpherePairSet(context.Background(), Ball{Radius: 5}, Ball{Center: geom.Vec3{X: 4}, Radius: 3}, Intersection, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	section, e := s.HorizontalMaterial(3)
	if e != nil {
		t.Fatal(e)
	}
	r, e := section.FirstBoundaryHit(context.Background(), geom.Vec2{}, geom.Vec2{X: 1}, 8, geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || !r.HasHit || !r.Isolated || r.Boundary != geom.SpherePairFirst|geom.SpherePairSecond {
		t.Fatal(r, e)
	}
	owners := s.SourceFaces()
	if len(owners[0]) != 1 || len(owners[1]) != 1 {
		t.Fatal("native support ownership", owners)
	}
	if _, e := (SpherePairSet{}).HorizontalMaterial(0); e == nil {
		t.Fatal("unbound native view")
	}
}
