package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedPartialRevolvedPolygon(t *testing.T) {
	frame, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	profile := []geom.Vec2{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	for _, tc := range []struct {
		angle  float64
		offset bool
	}{{math.Pi / 2, false}, {-math.Pi / 3, false}, {3 * math.Pi / 2, true}} {
		p := append([]geom.Vec2(nil), profile...)
		moment := 2.5
		if tc.offset {
			moment = 5.5
			for i := range p {
				p[i].X++
			}
		}
		o := CertifiedRevolvePolygonOptions{Deflection: .1}
		m, e := RevolvePolygonCertified(context.Background(), frame, p, tc.angle, o)
		if e != nil {
			t.Fatal(tc, e)
		}
		if !m.IdealSweepEmbedded || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("embedding")
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if len(m.OriginalFace) != len(m.Mesh.Triangles) || m.SourceImageSquaredDistance.Cmp(ratInset(.01)) > 0 {
			t.Fatal("certificate")
		}
		got, _ := m.SignedDisplayVolume.Float64()
		want := math.Abs(tc.angle) * moment
		if math.Abs(got-want) > .002*want {
			t.Fatal(got, want)
		}
		edges := map[[2]int]bool{}
		for _, tri := range m.Mesh.Triangles {
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				edges[[2]int{a, b}] = true
			}
		}
		if len(m.Mesh.Vertices)-len(edges)+len(m.Mesh.Triangles) != 2 {
			t.Fatal("Euler")
		}
		t.Log(tc.angle, tc.offset, m.Segments, len(m.Mesh.Triangles), m.TrigTerms, got, want)
	}
}
func TestCertifiedRevolutionTrigIndependent(t *testing.T) {
	o := CertifiedRevolvePolygonOptions{MaxTrigTerms: 10000, MaxBits: 16384}
	for _, x := range []float64{-6, -1, 0, .25, 4.5, 6} {
		work := 0
		s, c, r, e := revolutionTrigTaylor(context.Background(), ratInset(x), o, &work)
		if e != nil {
			t.Fatal(e)
		}
		// Independent longer exact Taylor expansion; its own remaining term is
		// included, rather than treating binary math.Sin/Cos as an exact oracle.
		term := big.NewRat(1, 1)
		ss, cc := new(big.Rat), big.NewRat(1, 1)
		for k := 1; k <= 97; k++ {
			term.Mul(term, ratInset(x))
			term.Quo(term, big.NewRat(int64(k), 1))
			if k == 97 {
				break
			}
			target := cc
			sign := (k / 2) % 2
			if k%2 == 1 {
				target = ss
				sign = ((k - 1) / 2) % 2
			}
			if sign == 0 {
				target.Add(target, term)
			} else {
				target.Sub(target, term)
			}
		}
		for i, p := range []*big.Rat{s, c} {
			ref := ss
			if i == 1 {
				ref = cc
			}
			distance := new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(p, ref)), new(big.Rat).Abs(term))
			if distance.Cmp(r) > 0 {
				t.Fatal("Taylor enclosure", x)
			}
		}
	}
}
func TestCertifiedPartialRevolutionLimits(t *testing.T) {
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{}, {X: 2}, {X: 2, Y: 1}, {Y: 1}}
	for _, o := range []CertifiedRevolvePolygonOptions{{Deflection: .01, MaxSegments: 1}, {Deflection: .1, MaxTrigTerms: 1}, {Deflection: .1, MaxPredicates: 1}, {Deflection: .1, MaxTriangles: 1}} {
		m, e := RevolvePolygonCertified(context.Background(), frame, p, 1, o)
		if e == nil || len(m.Mesh.Vertices) != 0 {
			t.Fatal("partial budget", e)
		}
	}
	for _, angle := range []float64{0, 2 * math.Pi, 7} {
		if _, e := RevolvePolygonCertified(context.Background(), frame, p, angle, CertifiedRevolvePolygonOptions{Deflection: .1}); e == nil {
			t.Fatal("unsupported angle")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := RevolvePolygonCertified(ctx, frame, p, 1, CertifiedRevolvePolygonOptions{Deflection: .1}); e != context.Canceled {
		t.Fatal(e)
	}
}

func TestCertifiedRevolutionRoundedFrame(t *testing.T) {
	frame, e := geom.NewFrame(geom.Vec3{X: .1, Y: -.3, Z: .7}, geom.Vec3{X: 1, Y: -2, Z: 1}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	profile := []geom.Vec2{{X: 1}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}}
	m, e := RevolvePolygonCertified(context.Background(), frame, profile, 1.25, CertifiedRevolvePolygonOptions{Deflection: .2})
	if e != nil {
		t.Fatal(e)
	}
	positiveCapError := false
	for i, face := range m.OriginalFace {
		if face < 2 && m.SquaredErrors[i].Sign() > 0 {
			positiveCapError = true
		}
	}
	if !positiveCapError || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
		t.Fatal("rounded cap proof")
	}
	got, _ := m.SignedDisplayVolume.Float64()
	want := 1.25 * 1.5
	if math.Abs(got-want) > .002*want {
		t.Fatal(got, want)
	}
}
