package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

type AlgebraicGraphOccurrenceFaceBounds struct {
	Face, Operand int
	Bounds        [3]geom.ExactParameterInterval
}

// AlgebraicGraphOccurrenceBounds encloses the world occurrence and each
// retained native face using its original graph support hull. These are exact
// rational enclosures of certified outward SOURCE bounds, not exact extrema or
// trim-tight bounds. They may include holes, discarded patches and empty space.
// Face IDs remain original native Boolean IDs. All returned rationals are owned.
type AlgebraicGraphOccurrenceBounds struct {
	Whole           [3]geom.ExactParameterInterval
	Faces           []AlgebraicGraphOccurrenceFaceBounds
	ExactOperations int
}

// BoundsRational applies the complete nonbinary rational pose without changing
// source geometry or rounding image coordinates. Original support hulls are
// sufficient because the prepared native common proved every retained face
// belongs to one of those supports. Material fibers map to segments between
// the two graph bounds, so their enclosing box also contains the solid interior.
// Empty/discarded regions create no face IDs.
func (o *AlgebraicGraphOccurrence) BoundsRational(ctx context.Context, limits geom.ExactCurvePlaneEventOptions) (out AlgebraicGraphOccurrenceBounds, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphOccurrenceBounds{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	limits, err = graphMaterialLimits(limits)
	if err != nil {
		return out, err
	}
	a := graphOccurrenceArithmetic{ctx: ctx, options: limits}
	clone := func(b [3]geom.ExactParameterInterval) (r [3]geom.ExactParameterInterval) {
		for i := range r {
			r[i] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(b[i].Lower), Upper: new(big.Rat).Set(b[i].Upper)}
		}
		return r
	}
	for owner, patch := range o.query.patches {
		b, e := patch.Bounds()
		if e != nil {
			return out, e
		}
		source := [3][2]float64{{b.Min.X, b.Max.X}, {b.Min.Y, b.Max.Y}, {b.Min.Z, b.Max.Z}}
		var image [3]geom.ExactParameterInterval
		for i := range image {
			lo, hi := new(big.Rat).Set(o.pose[i][3]), new(big.Rat).Set(o.pose[i][3])
			for j := 0; j < 3; j++ {
				if !finite(source[j][0]) || !finite(source[j][1]) {
					return out, geom.ErrConvergence
				}
				x := a.calc('*', o.pose[i][j], ratInset(source[j][0]))
				y := a.calc('*', o.pose[i][j], ratInset(source[j][1]))
				if x.Cmp(y) > 0 {
					x, y = y, x
				}
				lo = a.calc('+', lo, x)
				hi = a.calc('+', hi, y)
			}
			image[i] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
		}
		out.ExactOperations = a.used
		if a.err != nil {
			return out, a.err
		}
		if owner == 0 {
			out.Whole = clone(image)
		} else {
			for i := range out.Whole {
				if image[i].Lower.Cmp(out.Whole[i].Lower) < 0 {
					out.Whole[i].Lower.Set(image[i].Lower)
				}
				if image[i].Upper.Cmp(out.Whole[i].Upper) > 0 {
					out.Whole[i].Upper.Set(image[i].Upper)
				}
			}
		}
		ids := map[int]bool{}
		for _, region := range o.query.regions {
			ids[region.faces[owner]] = true
		}
		var sorted []int
		for id := range ids {
			sorted = append(sorted, id)
		}
		sort.Ints(sorted)
		for _, id := range sorted {
			out.Faces = append(out.Faces, AlgebraicGraphOccurrenceFaceBounds{Face: id, Operand: owner, Bounds: clone(image)})
		}
	}
	return out, nil
}
