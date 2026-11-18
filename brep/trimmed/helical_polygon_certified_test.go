package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedHelicalPolygonMultipleTurns(t *testing.T) {
	slowtest.Skip(t)
	frame, _ := geom.NewFrame(geom.Vec3{X: .125, Y: -.25, Z: 2}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	p := []geom.Vec2{{X: 1}, {X: 1.25}, {X: 1.25, Y: .125}, {X: 1.125, Y: .125}, {X: 1.125, Y: .25}, {X: 1, Y: .25}}
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		o := CertifiedRevolvePolygonOptions{Deflection: .15}
		m, e := SweepHelicalPolygonCertified(context.Background(), frame, p, 1, -.25, 1.25, hand, o)
		if e != nil {
			t.Fatal(hand, e)
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !m.IdealSweepEmbedded || !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("embedding")
		}
		moment := .0517578125
		want := 3 * math.Pi * moment
		got, _ := m.SignedDisplayVolume.Float64()
		if math.Abs(got-want) > .003*want {
			t.Fatal(got, want)
		}
		if m.SourceImageSquaredDistance.Cmp(ratInset(.0225)) > 0 {
			t.Fatal("deflection")
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
		// Independent exact quarter-turn witnesses use sin/cos in {0,+/-1},
		// avoiding both Taylor and libm as the oracle. Check the actual
		// source-to-triangle barycentric correspondence on every side.
		for turn := 0; turn <= 4; turn++ {
			u := big.NewRat(int64(turn), 4)
			position := new(big.Rat).Mul(new(big.Rat).Add(u, big.NewRat(1, 4)), big.NewRat(int64(2*m.Segments), 3))
			integer := new(big.Int).Quo(position.Num(), position.Denom())
			cell := int(integer.Int64())
			alpha := new(big.Rat).Sub(position, new(big.Rat).SetInt(integer))
			sinValues := [5]int64{0, 1, 0, -1, 0}
			cosValues := [5]int64{1, 0, -1, 0, 1}
			for side, a := range p {
				b := p[(side+1)%len(p)]
				v := big.NewRat(1, 3)
				radius := new(big.Rat).Add(ratInset(a.X), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(b.X), ratInset(a.X))))
				height := new(big.Rat).Add(u, new(big.Rat).Add(ratInset(a.Y), new(big.Rat).Mul(v, new(big.Rat).Sub(ratInset(b.Y), ratInset(a.Y)))))
				local := [3]*big.Rat{new(big.Rat).Mul(radius, big.NewRat(cosValues[turn], 1)), new(big.Rat).Mul(radius, big.NewRat(sinValues[turn]*int64(hand), 1)), height}
				weights := [3]*big.Rat{}
				part := 0
				if v.Cmp(alpha) <= 0 {
					weights = [3]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), alpha), new(big.Rat).Sub(alpha, v), v}
				} else {
					part = 1
					weights = [3]*big.Rat{new(big.Rat).Sub(big.NewRat(1, 1), v), alpha, new(big.Rat).Sub(v, alpha)}
				}
				if hand == geom.LeftHanded {
					weights[1], weights[2] = weights[2], weights[1]
				}
				id := 2*(side*m.Segments+cell) + part
				tri := m.Mesh.Triangles[id]
				axes := [3][3]*big.Rat{exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis()), exactBoundPoint(frame.ZAxis())}
				origin := exactBoundPoint(frame.Origin())
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					source := new(big.Rat).Set(origin[axis])
					display := new(big.Rat)
					for k := 0; k < 3; k++ {
						source.Add(source, new(big.Rat).Mul(axes[k][axis], local[k]))
						point := exactBoundPoint(m.Mesh.Vertices[tri[k]])
						display.Add(display, new(big.Rat).Mul(weights[k], point[axis]))
					}
					d := new(big.Rat).Sub(source, display)
					squared.Add(squared, new(big.Rat).Mul(d, d))
				}
				if squared.Cmp(m.SquaredErrors[id]) > 0 {
					t.Fatal("exact quarter-turn correspondence", turn, side)
				}
			}
		}
		t.Log(hand, m.Segments, len(m.Mesh.Triangles), m.TrigTerms, got, want)
	}
}
func TestCertifiedHelicalPiAndLimits(t *testing.T) {
	work := 0
	mid, r, e := certifiedSweepPi(context.Background(), CertifiedRevolvePolygonOptions{MaxTrigTerms: 1000, MaxBits: 16384}, &work)
	if e != nil {
		t.Fatal(e)
	}
	reference, _ := new(big.Rat).SetString("3.141592653589793238462643383279502884197169399375105820974944592307816406")
	if new(big.Rat).Abs(new(big.Rat).Sub(mid, reference)).Cmp(r) > 0 || r.Sign() <= 0 || r.Cmp(big.NewRat(1, 1000000000000000000)) >= 0 {
		t.Fatal("pi interval")
	}
	frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := []geom.Vec2{{X: 1}, {X: 1.25}, {X: 1.25, Y: .25}, {X: 1, Y: .25}}
	for _, o := range []CertifiedRevolvePolygonOptions{{Deflection: .1, MaxSegments: 1}, {Deflection: .1, MaxTrigTerms: 1}, {Deflection: .1, MaxBits: 64}} {
		m, e := SweepHelicalPolygonCertified(context.Background(), frame, p, 1, 0, 1, geom.RightHanded, o)
		if e == nil || len(m.Mesh.Vertices) != 0 {
			t.Fatal("partial budget", e)
		}
	}
	if _, e := SweepHelicalPolygonCertified(context.Background(), frame, p, .25, 0, 1, geom.RightHanded, CertifiedRevolvePolygonOptions{Deflection: .1}); e == nil {
		t.Fatal("overlapping turns")
	}
	if _, e := SweepHelicalPolygonCertified(context.Background(), frame, p, 1, 0, 5, geom.RightHanded, CertifiedRevolvePolygonOptions{Deflection: .1}); e == nil {
		t.Fatal("turn budget")
	}
}
