package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// ObliqueCavityWitnessEnclosureOptions bounds rational arithmetic independently
// of the source distance solver. Defaults: 64 precision bits, 8192 coefficient
// bits, 10000 checked operations. Precision is at most 256; coefficient limit at
// most 65536. CoordinateTolerance bounds uncertainty of EACH extremum, not the
// geometric width of a circle family. No silent precision retries occur.
type ObliqueCavityWitnessEnclosureOptions struct {
	CoordinateTolerance                              *big.Rat
	PrecisionBits, MaxCoefficientBits, MaxOperations int
}

// ObliqueCavityWorldWitnessEnclosure contains rational bounds on every exact
// coordinate minimum/maximum of an attained witness. For points both extrema
// have the same enclosure. Circle families remain families. OriginalFaces are
// exact source chart owners of this witness, not an exhaustive nearest-face set.
// GloballyNearest requires a proved collapsed exact global squared distance.
type ObliqueCavityWorldWitnessEnclosure struct {
	Minimum, Maximum             [3][2]*big.Rat
	OriginalFaces                []int
	WholeCircle, GloballyNearest bool
}
type EnclosedPlacedObliqueCavityDistance struct {
	Distance            PlacedNativeObliqueTorusCavityDistance
	Witnesses           [4]ObliqueCavityWorldWitnessEnclosure
	EnclosureOperations int
}
type cavityWitnessWork struct {
	ctx                        context.Context
	bits, precision, max, work int
	err                        error
}

func (w *cavityWitnessWork) check(q *big.Rat) {
	if w.err != nil {
		return
	}
	if w.err = w.ctx.Err(); w.err != nil {
		return
	}
	w.work++
	if w.work > w.max || q == nil || q.Num().BitLen() > w.bits || q.Denom().BitLen() > w.bits {
		w.err = geom.ErrConvergence
	}
}
func (w *cavityWitnessWork) add(a, b *big.Rat) *big.Rat {
	w.check(a)
	w.check(b)
	if w.err != nil {
		return new(big.Rat)
	}
	q := new(big.Rat).Add(a, b)
	w.check(q)
	return q
}
func (w *cavityWitnessWork) mul(a, b *big.Rat) *big.Rat {
	w.check(a)
	w.check(b)
	if w.err != nil {
		return new(big.Rat)
	}
	q := new(big.Rat).Mul(a, b)
	w.check(q)
	return q
}
func (w *cavityWitnessWork) sqrt(q *big.Rat) [2]*big.Rat {
	zero := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	w.check(q)
	if w.err != nil {
		return zero
	}
	if q.Sign() < 0 {
		w.err = geom.ErrDomain
		return zero
	}
	if q.Num().BitLen()+2*w.precision > w.bits {
		w.err = geom.ErrConvergence
		return zero
	}
	scaled := new(big.Int).Lsh(new(big.Int).Set(q.Num()), uint(2*w.precision))
	floor := new(big.Int).Quo(new(big.Int).Set(scaled), q.Denom())
	root := new(big.Int).Sqrt(floor)
	den := new(big.Int).Lsh(big.NewInt(1), uint(w.precision))
	lo := new(big.Rat).SetFrac(root, den)
	hi := new(big.Rat).Set(lo)
	exact := new(big.Int).Mul(new(big.Int).Mul(root, root), q.Denom())
	if exact.Cmp(scaled) != 0 {
		hi.SetFrac(new(big.Int).Add(root, big.NewInt(1)), den)
	}
	w.check(lo)
	w.check(hi)
	return [2]*big.Rat{lo, hi}
}
func (w *cavityWitnessWork) scale(q [2]*big.Rat, c *big.Rat) [2]*big.Rat {
	a, b := w.mul(q[0], c), w.mul(q[1], c)
	if c.Sign() < 0 {
		a, b = b, a
	}
	return [2]*big.Rat{a, b}
}
func (w *cavityWitnessWork) intervalAdd(a, b [2]*big.Rat) [2]*big.Rat {
	return [2]*big.Rat{w.add(a[0], b[0]), w.add(a[1], b[1])}
}

// BoundPlacedBoundaryDistanceEnclosed performs one native-authenticated source
// query of the original analytic occurrence (not its display triangles),
// preserves symbolic world witnesses, and encloses their extrema by
// exact dyadic square-root isolation. It never interprets interval overlap as
// equality or promotes an upper-bound witness to an exact minimizer.
func (p *PreparedObliqueTorusCavity) BoundPlacedBoundaryDistanceEnclosed(ctx context.Context, pose placement.RationalRigid, point [3]*big.Rat, distance geom.TorusCylinderDistanceOptions, o ObliqueCavityWitnessEnclosureOptions) (out EnclosedPlacedObliqueCavityDistance, err error) {
	defer func() {
		if err != nil {
			out = EnclosedPlacedObliqueCavityDistance{}
		}
	}()
	if ctx == nil || o.CoordinateTolerance == nil || o.CoordinateTolerance.Sign() <= 0 {
		return out, geom.ErrInvalid
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 10000
	}
	if o.PrecisionBits < 16 || o.PrecisionBits > 256 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 || o.MaxOperations < 1 || o.MaxOperations > 1000000 {
		return out, geom.ErrInvalid
	}
	w := cavityWitnessWork{ctx: ctx, bits: o.MaxCoefficientBits, precision: o.PrecisionBits, max: o.MaxOperations}
	w.check(o.CoordinateTolerance)
	if w.err != nil {
		return out, w.err
	}
	out.Distance, err = p.BoundPlacedBoundaryDistance(ctx, pose, point, distance)
	if err != nil {
		return out, err
	}
	for i, v := range out.Distance.Witnesses {
		bounds := &out.Witnesses[i]
		bounds.OriginalFaces = append([]int(nil), out.Distance.Base.WitnessFaces[i]...)
		bounds.WholeCircle = v.Base.WholeCircle
		bounds.GloballyNearest = out.Distance.Base.Lower.Cmp(out.Distance.Base.Upper) == 0 && out.Distance.Base.Supports[i].Upper.Cmp(out.Distance.Base.Lower) == 0
		h := w.scale(w.sqrt(v.Base.HeightRadicand), big.NewRat(int64(v.Base.HeightSign), 1))
		h = w.intervalAdd(h, [2]*big.Rat{v.Base.HeightBase, v.Base.HeightBase})
		var radial [2]*big.Rat
		if !v.Base.WholeCircle {
			w.check(v.Base.RadialSquared)
			w.check(v.QueryRadialSquared)
			if w.err != nil {
				return out, w.err
			}
			if v.QueryRadialSquared.Sign() <= 0 {
				return out, geom.ErrDomain
			}
			ratio := new(big.Rat).Quo(v.Base.RadialSquared, v.QueryRadialSquared)
			w.check(ratio)
			radial = w.sqrt(ratio)
		}
		for j := 0; j < 3; j++ {
			center := w.intervalAdd([2]*big.Rat{v.Origin[j], v.Origin[j]}, w.scale(h, v.Axis[j]))
			if v.Base.WholeCircle {
				sq := w.add(w.mul(v.CircleDirections[0][j], v.CircleDirections[0][j]), w.mul(v.CircleDirections[1][j], v.CircleDirections[1][j]))
				amp := w.sqrt(w.mul(v.Base.RadialSquared, sq))
				bounds.Minimum[j] = w.intervalAdd(center, w.scale(amp, big.NewRat(-1, 1)))
				bounds.Maximum[j] = w.intervalAdd(center, amp)
			} else {
				q := w.intervalAdd(center, w.scale(radial, v.RadialVector[j]))
				bounds.Minimum[j] = q
				bounds.Maximum[j] = [2]*big.Rat{new(big.Rat).Set(q[0]), new(big.Rat).Set(q[1])}
			}
			for _, q := range [][2]*big.Rat{bounds.Minimum[j], bounds.Maximum[j]} {
				width := w.add(q[1], w.mul(big.NewRat(-1, 1), q[0]))
				if w.err != nil {
					return out, w.err
				}
				if width.Cmp(o.CoordinateTolerance) > 0 {
					return out, geom.ErrConvergence
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.EnclosureOperations = w.work
	return out, nil
}
