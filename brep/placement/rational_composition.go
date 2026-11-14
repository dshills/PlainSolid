package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Then applies r first, next second, without rounding. Four exact transforms
// and one proper-frame validation are bounded by maxBits (default 16384). No
// composition history or normalization is stored; coefficient growth fails.
func (r RationalRigid) Then(ctx context.Context, next RationalRigid, maxBits int) (RationalRigid, error) {
	if ctx == nil {
		return RationalRigid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return RationalRigid{}, e
	}
	original, e := r.Matrix()
	if e != nil {
		return RationalRigid{}, e
	}
	var out [3][4]*big.Rat
	for column := 0; column < 4; column++ {
		v := [3]*big.Rat{original[0][column], original[1][column], original[2][column]}
		var q [3]*big.Rat
		if column == 3 {
			q, e = next.TransformPoint(ctx, v, false, maxBits)
		} else {
			q, e = next.TransformVector(ctx, v, false, maxBits)
		}
		if e != nil {
			return RationalRigid{}, e
		}
		for row := 0; row < 3; row++ {
			out[row][column] = q[row]
		}
	}
	result, e := NewRationalRigid(out, maxBits)
	if e != nil {
		return RationalRigid{}, e
	}
	if e := ctx.Err(); e != nil {
		return RationalRigid{}, e
	}
	return result, nil
}

// Then composes the whole exact pose and returns to the ORIGINAL prepared mesh
// before rounding once. Thus it neither accumulates previous display rounding
// nor changes the inherited source proof. Fresh final embedding/nesting checks
// remain mandatory. The result retains no intermediate placement chain.
func (p *PlacedClosedDisplay) Then(ctx context.Context, next RationalRigid, o ClosedDisplayOptions) (*PlacedClosedDisplay, error) {
	if p == nil || p.source == nil {
		return nil, geom.ErrInvalid
	}
	limits, e := meshRoundingLimits(o.Rounding)
	if e != nil {
		return nil, e
	}
	pose, e := p.pose.Then(ctx, next, limits.MaxCoefficientBits)
	if e != nil {
		return nil, e
	}
	return p.source.Place(ctx, pose, o)
}
