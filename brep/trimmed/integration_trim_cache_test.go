package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type countedMassTrim struct{ points, derivatives int }

func (c *countedMassTrim) EvalUV(t float64) (geom.Vec2, error) {
	c.points++
	return geom.Vec2{X: t, Y: t * t}, nil
}
func (c *countedMassTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	c.derivatives++
	return geom.Vec2{X: 1, Y: 2 * t}, nil
}
func TestMassTrimMemoBounded(t *testing.T) {
	source := &countedMassTrim{}
	memo := massTrimMemo{values: map[massTrimKey]massTrimSample{}}
	curve := massMemoTrim{source: source, cache: &memo, id: 3}
	for i := 0; i < 10; i++ {
		p, e := curve.EvalUV(.25)
		d, f := curve.DerivativeUV(.25)
		if e != nil || f != nil || p != (geom.Vec2{X: .25, Y: .0625}) || d != (geom.Vec2{X: 1, Y: .5}) {
			t.Fatal(p, d, e, f)
		}
	}
	if source.points != 1 || source.derivatives != 1 {
		t.Fatal("same-parameter evaluations repeated", source)
	}
	for i := 0; i < massTrimCacheLimit+10; i++ {
		_, _ = curve.EvalUV(float64(i) / float64(massTrimCacheLimit+10))
	}
	if len(memo.values) != massTrimCacheLimit {
		t.Fatal("unbounded cache", len(memo.values))
	}
	x := .987654321
	p, _ := curve.EvalUV(x)
	if p != (geom.Vec2{X: x, Y: x * x}) {
		t.Fatal("full cache changed evaluation", p)
	}
}

func TestMassAffineStripProofAndExactChord(t *testing.T) {
	ctx := context.Background()
	a, b, sections := multipleOpenSplineFixture(t)
	s, _, e := CombineOpenSplineShells(ctx, a, b, parametric.Intersection, sections, SplineShellOpenOptions{})
	if e != nil {
		t.Fatal(e)
	}
	proofs := massAffineProofs{left: 4000000, values: map[massProofKey]geom.TubeAffineLineCertificate{}}
	count := 0
	for _, f := range s.faces {
		for _, u := range f.Outer {
			if _, _, ok := trimTube(u.Curve); !ok {
				continue
			}
			proven, e := proofs.zeroStrip(ctx, u)
			if e != nil || !proven {
				t.Fatal("affine strip not proved", e)
			}
			count++
			altered := u
			altered.UVEnd.X = math.Nextafter(u.UVEnd.X, math.Inf(1))
			if proven, e = proofs.zeroStrip(ctx, altered); e != nil || proven {
				t.Fatal("tolerance-close chord treated as exact", e)
			}
		}
	}
	if count != 48 || len(proofs.values) != 24 {
		t.Fatal("proof not shared across paired segment uses", count, len(proofs.values))
	}
	before := s.Faces()
	mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-.25) > 1e-12 || math.Abs(mass.Area-2.5) > 1e-12 {
		t.Fatal(mass, e)
	}
	for i, f := range s.Faces() {
		for j, u := range f.Outer {
			if (u.Curve == nil) != (before[i].Outer[j].Curve == nil) {
				t.Fatal("integration altered topology")
			}
		}
	}

	placement := geom.Affine{{0, -2, 0, 1}, {2, 0, 0, -1}, {0, 0, 2, .25}}
	placed, e := s.Transformed(placement)
	if e != nil {
		t.Fatal(e)
	}
	pm, e := placed.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(pm.Volume-2) > 1e-10 || math.Abs(pm.Area-10) > 1e-10 || pm.Centroid.Sub(placement.Apply(mass.Centroid)).Length() > 1e-10 {
		t.Fatal("placed tube mass", pm, e)
	}
	// A denied optional proof must fall back, never manufacture a zero strip.
	empty := massAffineProofs{values: map[massProofKey]geom.TubeAffineLineCertificate{}}
	for _, f := range s.faces {
		for _, u := range f.Outer {
			if _, _, ok := trimTube(u.Curve); ok {
				yes, e := empty.zeroStrip(ctx, u)
				if yes || e != nil {
					t.Fatal(yes, e)
				}
				return
			}
		}
	}
	t.Fatal("no tube fixture")
}
