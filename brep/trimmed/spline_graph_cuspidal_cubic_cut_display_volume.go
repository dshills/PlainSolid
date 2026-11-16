package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// cuspidalCutPolynomialVolume integrates the certified height difference.
// Constant source weights make the full homogeneous coefficient identity a
// polynomial height law in physical XY. The original roof values at the
// certified exterior witness determine its amplitude exactly.
func cuspidalCutPolynomialVolume(ctx context.Context, cut *CuspidalCubicCut, roofs [2]*geom.SurfacePatch,
	d geom.ParameterRectangle) (*big.Rat, error) {
	p := cut.section
	origin := p.Vertices[0].Point
	delta := func(point [3]*big.Rat, axis int) *big.Rat {
		return new(big.Rat).Sub(point[axis], origin[axis])
	}
	xExit, yExit := delta(p.Vertices[2].Point, 0), delta(p.Vertices[2].Point, 1)
	k := new(big.Rat).Quo(cuspidalDisplayPower(yExit, 2), cuspidalDisplayPower(xExit, 3))

	var z [2]*big.Rat
	var unit *big.Rat
	for i, roof := range roofs {
		uv := p.Regions[0].UV[i]
		jet, err := roof.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			return nil, err
		}
		z[i] = jet.Point[2]
		if i == 0 {
			x, y := delta(jet.Point, 0), delta(jet.Point, 1)
			unit = new(big.Rat).Sub(cuspidalDisplayPower(y, 2), new(big.Rat).Mul(k, cuspidalDisplayPower(x, 3)))
		}
	}
	if unit.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}
	amplitude := new(big.Rat).Quo(new(big.Rat).Sub(z[0], z[1]), unit)
	// The transverse integral of b²-y² on [-b,b] is 4b³/3.
	// Substituting b=yExit*q³ and dx=2*xExit*q*dq contributes 2/11.
	lobe := new(big.Rat).Abs(amplitude)
	lobe.Mul(lobe, big.NewRat(8, 33)).Mul(lobe, xExit).Mul(lobe, cuspidalDisplayPower(yExit, 3))
	if cut.history.Region == 1 {
		if amplitude.Sign() >= 0 {
			return nil, geom.ErrConvergence
		}
		return lobe, nil
	}
	if amplitude.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}

	whole := [2]*big.Rat{}
	for i, bounds := range [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}} {
		low := new(big.Rat).Sub(ratInset(bounds[0]), origin[i])
		high := new(big.Rat).Sub(ratInset(bounds[1]), origin[i])
		integral := new(big.Rat).Sub(cuspidalDisplayPower(high, 4-i), cuspidalDisplayPower(low, 4-i))
		transverse := [2][2]float64{{d.VMin, d.VMax}, {d.UMin, d.UMax}}[i]
		width := new(big.Rat).Sub(ratInset(transverse[1]), ratInset(transverse[0]))
		whole[i] = integral.Mul(integral, width).Quo(integral, big.NewRat(int64(4-i), 1))
	}
	total := new(big.Rat).Sub(whole[1], new(big.Rat).Mul(k, whole[0]))
	total.Mul(total, amplitude).Add(total, lobe)
	if total.Sign() <= 0 {
		return nil, geom.ErrConvergence
	}
	return total, nil
}

func cuspidalDisplayPower(v *big.Rat, degree int) *big.Rat {
	out := big.NewRat(1, 1)
	for range degree {
		out.Mul(out, v)
	}
	return out
}
