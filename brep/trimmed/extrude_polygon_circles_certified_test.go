package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedCircularHoleExtrusionMixed(t *testing.T) {
	frame, _ := geom.NewFrame(geom.Vec3{X: 2, Y: -3, Z: 1}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	outer := []geom.Vec2{{}, {X: 6}, {X: 6, Y: 3}, {X: 4, Y: 4}, {Y: 4}}
	through := []ExtrusionCircle{{Center: geom.Vec2{X: 1, Y: 1}, Radius: .5}, {Center: geom.Vec2{X: 4.5, Y: 1}, Radius: .25}}
	blind := []ExtrusionBlindHole{{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 2.5}, Radius: .375}, Depth: .75}}
	for _, height := range []float64{2.5, -2.5} {
		m, e := ExtrudePolygonWithHolesCertified(context.Background(), frame, outer, through, blind, height, CertifiedRevolvePolygonOptions{Deflection: .05})
		if e != nil {
			t.Fatal(height, e)
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("embedding")
		}
		if len(m.OriginalFace) != len(m.Mesh.Triangles) || len(m.RimVertices) != 4 || m.IdealBoundarySquaredDistance.Cmp(ratInset(.0025)) > 0 {
			t.Fatal("certificate")
		}
		owners := map[[2]int]map[int]bool{}
		for i, tri := range m.Mesh.Triangles {
			if m.OriginalFace[i] < 0 || m.OriginalFace[i] >= len(m.Solid.faces) {
				t.Fatal("face ancestry")
			}
			for j, a := range tri {
				b := tri[(j+1)%3]
				if a > b {
					a, b = b, a
				}
				key := [2]int{a, b}
				if owners[key] == nil {
					owners[key] = map[int]bool{}
				}
				owners[key][m.OriginalFace[i]] = true
			}
		}
		if len(m.Mesh.Vertices)-len(owners)+len(m.Mesh.Triangles) != -2 {
			t.Fatal("two through handles")
		}
		for loop, pair := range m.RimVertices {
			for end, ring := range pair {
				for i, a := range ring {
					b := ring[(i+1)%len(ring)]
					if a > b {
						a, b = b, a
					}
					got := owners[[2]int{a, b}]
					wall, cap := 2+i, end
					if loop > 0 {
						wall = 2 + len(outer) + 4*(loop-1) + i/(m.Segments/4)
					}
					if loop > len(through) {
						entry := 0
						if height < 0 {
							entry = 1
						}
						if end != entry {
							cap = 2 + len(outer) + 4*(len(through)+len(blind)) + loop - len(through) - 1
						}
					}
					if len(got) != 2 || !got[wall] || !got[cap] {
						t.Fatal("shared original cap/wall/floor lineage")
					}
				}
			}
		}
		// A chord midpoint lies strictly inside the source circular hole,
		// while belonging to the displayed cap boundary. The fill certificate
		// must cover this symmetric-difference case, not only outside chords.
		ring := m.RimVertices[1][0]
		pa, pb := exactBoundPoint(m.Mesh.Vertices[ring[0]]), exactBoundPoint(m.Mesh.Vertices[ring[1]])
		origin := exactBoundPoint(frame.Origin())
		axes := [2][3]*big.Rat{exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis())}
		var uv [2]*big.Rat
		for axis := 0; axis < 2; axis++ {
			uv[axis] = new(big.Rat)
			for k := 0; k < 3; k++ {
				mid := new(big.Rat).Sub(new(big.Rat).Quo(new(big.Rat).Add(pa[k], pb[k]), big.NewRat(2, 1)), origin[k])
				uv[axis].Add(uv[axis], new(big.Rat).Mul(mid, axes[axis][k]))
			}
		}
		d2 := new(big.Rat)
		for _, x := range uv {
			d := new(big.Rat).Sub(x, big.NewRat(1, 1))
			d2.Add(d2, new(big.Rat).Mul(d, d))
		}
		r := big.NewRat(1, 2)
		r2 := big.NewRat(1, 4)
		if d2.Cmp(r2) >= 0 || d2.Cmp(big.NewRat(1, 16)) < 0 {
			t.Fatal("strict source-hole witness")
		}
		distanceUpper := new(big.Rat).Quo(new(big.Rat).Sub(r2, d2), new(big.Rat).Mul(big.NewRat(3, 2), r))
		if new(big.Rat).Mul(distanceUpper, distanceUpper).Cmp(m.UVFillSquaredDistance) > 0 {
			t.Fatal("holed fill bound")
		}
		want := 23*math.Abs(height) - math.Pi*(.5*.5+.25*.25)*math.Abs(height) - math.Pi*.375*.375*.75
		got, _ := m.SignedDisplayVolume.Float64()
		if math.Abs(got-want) > .002*want {
			t.Fatal(got, want)
		}
		t.Log(height, len(m.Mesh.Vertices), len(m.Mesh.Triangles), m.Segments, m.TrigTerms, m.Predicates, got, want)
	}
}
func TestCertifiedCircularHoleExtrusionLimits(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	outer := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	holes := []ExtrusionCircle{{Center: geom.Vec2{X: 2, Y: 2}, Radius: .5}}
	for _, o := range []CertifiedRevolvePolygonOptions{{Deflection: .05, MaxSegments: 4}, {Deflection: .05, MaxPredicates: 1}, {Deflection: .05, MaxTriangles: 1}, {Deflection: .05, MaxTrigTerms: 1}} {
		m, e := ExtrudePolygonWithHolesCertified(context.Background(), f, outer, holes, nil, 2, o)
		if e == nil || len(m.Mesh.Vertices) != 0 {
			t.Fatal("partial budget", e)
		}
	}
	bad := []ExtrusionCircle{{Center: geom.Vec2{X: .5, Y: 2}, Radius: .5}}
	if _, e := ExtrudePolygonWithHolesCertified(context.Background(), f, outer, bad, nil, 2, CertifiedRevolvePolygonOptions{Deflection: .05}); e == nil {
		t.Fatal("tangent hole")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ExtrudePolygonWithHolesCertified(ctx, f, outer, holes, nil, 2, CertifiedRevolvePolygonOptions{Deflection: .05}); e != context.Canceled {
		t.Fatal(e)
	}
}
