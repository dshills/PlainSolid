//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffinePartitionSourceFillDistance(t *testing.T) {
	shear := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {.25, .125, 1, 0}}
	for _, placement := range []*geom.Affine{nil, &shear} {
		sub, line := affineAlgebraicPartitionFixture(t, placement)
		o := AffinePartitionFillOptions{Boundary: AlgebraicBoundaryMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .01}}}
		result, e := sub.TessellateAffinePartitionDistanceCertified(context.Background(), 1, line, o)
		if e != nil {
			t.Fatal(e)
		}
		if result.FillSquaredDistance.Sign() <= 0 || result.FillSquaredDistance.Cmp(big.NewRat(1, 1000)) > 0 {
			t.Fatal("bound", result.FillSquaredDistance)
		}
		if placement == nil && result.DisplayProjectionSquaredDistance.Sign() != 0 {
			t.Fatal("exact plane projection should be zero")
		}
		if placement != nil && result.DisplayProjectionSquaredDistance.Sign() == 0 {
			t.Fatal("fixture must exercise off-plane display rounding")
		}
		// Independent dense boundary witnesses on the exact ellipse, including the
		// two source subregions' shared vertical cut. Distance to the triangle union
		// is bounded above by distance to an explicit boundary chord point.
		limit, _ := result.FillSquaredDistance.Float64()
		for i := 0; i < 128; i++ {
			angle := 2 * math.Pi * float64(i) / 128
			p := geom.Vec3{X: .5 + .25*math.Cos(angle), Y: .5 + .125*math.Sin(angle), Z: 2}
			if placement != nil {
				p = placement.Apply(p)
			}
			best := math.Inf(1)
			for _, path := range result.Boundary.Boundary.Edges {
				for j := 0; j+1 < len(path); j++ {
					a, b := result.Boundary.Boundary.Vertices[path[j]], result.Boundary.Boundary.Vertices[path[j+1]]
					d := b.Sub(a)
					q := p.Sub(a).Dot(d) / d.Dot(d)
					if q < 0 {
						q = 0
					}
					if q > 1 {
						q = 1
					}
					v := p.Sub(a.Add(d.Scale(q)))
					squared := v.Dot(v)
					if squared < best {
						best = squared
					}
				}
			}
			if best > limit+1e-18 {
				t.Fatal("independent boundary sample exceeded fill bound", best, limit)
			}
		}
		t.Log("projection error", result.DisplayProjectionSquaredDistance.RatString(), "fill squared upper", limit)
	}
}
