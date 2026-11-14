package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PreparedObliqueTorusCavity binds an immutable original body to its independently
// proved analytic source. Preparation authenticates actual supports, topology,
// orientations and canonical IDs once; subsequent distance and line queries do not re-sew it.
type PreparedObliqueTorusCavity struct {
	body       Solid
	source     geom.ObliqueCylinderTorusCavityMaterial
	major      *big.Rat
	dimensions [5]float64
}

func PrepareObliqueTorusCavity(ctx context.Context, s Solid, R, r, c, lower, upper float64, pose [3][4]*big.Rat, o geom.SurfacePatchOptions) (*PreparedObliqueTorusCavity, error) {
	q, e := bindObliqueTorusCavitySource(ctx, s, R, r, c, lower, upper, pose, o)
	if e != nil {
		return nil, e
	}
	return &PreparedObliqueTorusCavity{body: s, source: q, major: ratInset(R), dimensions: [5]float64{R, r, c, lower, upper}}, nil
}

// NativeObliqueTorusCavityDistance retains original support masks and exact
// attained-witness chart ownership. WitnessFaces are NOT a complete nearest
// face set: a support witness is an upper bound unless independently exact.
// Torus quarter-boundary witnesses retain both/all adjacent original faces.
// Chart assignment adds two exact integer square roots bounded by the source
// coefficient-bit limit and at most16 quadrant incidence tests.
type NativeObliqueTorusCavityDistance struct {
	geom.ObliqueCylinderTorusCavityDistance
	WitnessFaces [4][]int
}

func (p *PreparedObliqueTorusCavity) BoundBoundaryDistance(ctx context.Context, point [3]*big.Rat, o geom.TorusCylinderDistanceOptions) (NativeObliqueTorusCavityDistance, error) {
	var out NativeObliqueTorusCavityDistance
	if p == nil || p.major == nil {
		return out, geom.ErrInvalid
	}
	base, e := p.source.BoundBoundaryDistance(ctx, point, o)
	if e != nil {
		return out, e
	}
	w := base.Supports[0].Witness
	// The meridian sampler authors rational rho and stores rho² exactly.
	n, d := new(big.Int).Sqrt(w.RadialSquared.Num()), new(big.Int).Sqrt(w.RadialSquared.Denom())
	if new(big.Int).Mul(n, n).Cmp(w.RadialSquared.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(w.RadialSquared.Denom()) != 0 {
		return out, geom.ErrDomain
	}
	rho := new(big.Rat).SetFrac(n, d)
	radialSign := new(big.Rat).Sub(rho, p.major).Sign()
	heightSign := w.HeightSign
	if w.HeightRadicand.Sign() == 0 {
		heightSign = 0
	}
	if w.HeightBase.Sign() != 0 {
		return out, geom.ErrDomain
	}
	quarters := func(x, y int) []int {
		var result []int
		for i, sign := range [][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
			if (x == 0 || x == sign[0]) && (y == 0 || y == sign[1]) {
				result = append(result, i)
			}
		}
		return result
	}
	azimuth := quarters(base.TorusQuery[0].Sign(), base.TorusQuery[1].Sign())
	meridian := quarters(radialSign, heightSign)
	out.ObliqueCylinderTorusCavityDistance = base
	for _, u := range azimuth {
		for _, v := range meridian {
			out.WitnessFaces[0] = append(out.WitnessFaces[0], 3+4*u+v)
		}
	}
	out.WitnessFaces[1] = []int{0}
	out.WitnessFaces[2] = []int{1}
	out.WitnessFaces[3] = []int{2}
	return out, nil
}
