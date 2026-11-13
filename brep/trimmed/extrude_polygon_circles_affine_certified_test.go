package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedCircularHoleTiltedFrame(t *testing.T) {
	frame, e := geom.NewFrame(geom.Vec3{X: .1, Y: -.3, Z: .7}, geom.Vec3{X: 1, Y: -2, Z: 1}, geom.Vec3{X: 1, Y: 2, Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	outer := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	through := []ExtrusionCircle{{Center: geom.Vec2{X: 1, Y: 1}, Radius: .5}}
	blind := []ExtrusionBlindHole{{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 2.5}, Radius: .375}, Depth: .75}}
	axes := [3][3]*big.Rat{exactBoundPoint(frame.XAxis()), exactBoundPoint(frame.YAxis()), exactBoundPoint(frame.ZAxis())}
	for _, height := range []float64{2, -2} {
		m, e := ExtrudePolygonWithHolesCertified(context.Background(), frame, outer, through, blind, height, CertifiedRevolvePolygonOptions{Deflection: .05})
		if e != nil {
			t.Fatal(height, e)
		}
		if m.FrameDeterminant.Sign() <= 0 || m.PlanarNormSquaredUpper.Sign() <= 0 || m.DisplayRoundingSquaredUpper.Sign() <= 0 || m.IdealBoundarySquaredDistance.Cmp(ratInset(.0025)) > 0 {
			t.Fatal("affine proof metadata")
		}
		if e := m.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !m.Collision.Complete || len(m.Collision.Contacts) != 0 {
			t.Fatal("embedding")
		}
		// Independently test exact norm transport on a rational direction grid.
		for x := -5; x <= 5; x++ {
			for y := -5; y <= 5; y++ {
				dx, dy := big.NewRat(int64(x), 3), big.NewRat(int64(y), 7)
				squared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					v := new(big.Rat).Add(new(big.Rat).Mul(dx, axes[0][axis]), new(big.Rat).Mul(dy, axes[1][axis]))
					squared.Add(squared, new(big.Rat).Mul(v, v))
				}
				local := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
				if squared.Cmp(new(big.Rat).Mul(local, m.PlanarNormSquaredUpper)) > 0 {
					t.Fatal("Gram norm bound")
				}
			}
		}
		// Exact affine evaluation of original polygon corners checks the separate
		// roundoff enclosure without calling the production mapping helper.
		origin := exactBoundPoint(frame.Origin())
		for cap, z := range []float64{math.Min(0, height), math.Max(0, height)} {
			for i, p := range outer {
				display := exactBoundPoint(m.Mesh.Vertices[m.RimVertices[0][cap][i]])
				coordinates := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(z)}
				errorSquared := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					ideal := new(big.Rat).Set(origin[axis])
					for k := 0; k < 3; k++ {
						ideal.Add(ideal, new(big.Rat).Mul(axes[k][axis], coordinates[k]))
					}
					delta := new(big.Rat).Sub(ideal, display[axis])
					errorSquared.Add(errorSquared, new(big.Rat).Mul(delta, delta))
				}
				if errorSquared.Cmp(m.DisplayRoundingSquaredUpper) > 0 {
					t.Fatal("rounding enclosure")
				}
			}
		}
		det, _ := m.FrameDeterminant.Float64()
		want := det * (16*math.Abs(height) - math.Pi*.25*math.Abs(height) - math.Pi*.375*.375*.75)
		got, _ := m.SignedDisplayVolume.Float64()
		if math.Abs(got-want) > .002*want {
			t.Fatal(got, want)
		}
		t.Log(height, len(m.Mesh.Triangles), got, want, m.PlanarNormSquaredUpper.FloatString(18))
	}
}
func TestCertifiedCircularHoleSingularFrame(t *testing.T) {
	outer := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	m, e := ExtrudePolygonWithHolesCertified(context.Background(), geom.Frame{}, outer, nil, nil, 2, CertifiedRevolvePolygonOptions{Deflection: .05})
	if e == nil || len(m.Mesh.Vertices) != 0 {
		t.Fatal("singular affine frame accepted")
	}
}
