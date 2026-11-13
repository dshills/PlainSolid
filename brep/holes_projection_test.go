package brep

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func TestHoledTriangulationPreservesCollinearDiagonals(t *testing.T) {
	outer := rectangle(0, -50, 80, 0)
	var hole []geom.Vec3
	for i := 0; i < 16; i++ {
		a := -2 * math.Pi * float64(i) / 16
		hole = append(hole, geom.Vec3{X: 30 + 3*math.Cos(a), Y: -17 + 3*math.Sin(a)})
	}
	triangles, err := triangulateRings(outer, [][]geom.Vec3{hole})
	if err != nil {
		t.Fatal(err)
	}
	points := append(append([]geom.Vec3{}, outer...), hole...)
	for _, tri := range triangles {
		a, b, c := points[tri[0]], points[tri[1]], points[tri[2]]
		if b.Sub(a).Cross(c.Sub(a)).Z == 0 {
			t.Fatalf("degenerate physical triangle: %v %v %v", a, b, c)
		}
	}
	s, err := PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 80})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Tessellate().ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}

func TestHoledProjectionExtremeScales(t *testing.T) {
	for _, exponent := range []int{-1000, -500, 0, 500, 1000} {
		scale := math.Ldexp(1, exponent)
		outer := rectangle(-4*scale, -4*scale, 4*scale, 4*scale)
		hole := rectangle(-scale, -scale, scale, scale)
		reverseVecs := func(p []geom.Vec3) {
			for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
				p[i], p[j] = p[j], p[i]
			}
		}
		reverseVecs(hole)
		triangles, err := triangulateRings(outer, [][]geom.Vec3{hole})
		if err != nil {
			t.Fatalf("scale2^%d: %v", exponent, err)
		}
		if len(triangles) != 8 {
			t.Fatalf("scale2^%d: triangle count%d", exponent, len(triangles))
		}
		// Resolution remains relative to the full face, at every exponent.
		tiny := rectangle(scale, scale, scale*(1+1e-13), scale*(1+1e-13))
		reverseVecs(tiny)
		if _, err := triangulateRings(outer, [][]geom.Vec3{tiny}); err == nil {
			t.Fatalf("accepted unresolved hole at scale2^%d", exponent)
		}
	}
}

func TestHoledProjectionIgnoresDroppedAxisMagnitude(t *testing.T) {
	outer := rectangle(-4, -4, 4, 4)
	hole := rectangle(-1, -1, 1, 1)
	for i, j := 0, len(hole)-1; i < j; i, j = i+1, j-1 {
		hole[i], hole[j] = hole[j], hole[i]
	}
	for _, ring := range [][]geom.Vec3{outer, hole} {
		for i := range ring {
			ring[i].Z = math.MaxFloat64 / 2
		}
	}
	if _, err := triangulateRings(outer, [][]geom.Vec3{hole}); err != nil {
		t.Fatal(err)
	}
}
