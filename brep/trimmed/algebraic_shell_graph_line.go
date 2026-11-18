//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphNormalLineOptions separates native preparation work from the
// cumulative query budget. Use a prepared query when issuing many lines.
type AlgebraicGraphNormalLineOptions struct {
	Preparation AlgebraicGraphMaterialOptions
	Exact       geom.ExactCurvePlaneEventOptions
}
type AlgebraicGraphFiberHit struct {
	Parameter     *big.Rat
	Point         [3]*big.Rat
	Faces         []int
	RimEdges      []int
	Before, After bool // Whole material along increasing t, not artificial clipping ends.
}
type AlgebraicGraphFiberLine struct {
	Complete, Present                      bool
	Parameters                             [2]*big.Rat              // Closed retained interval in original t units; nil if absent.
	EndpointOnBoundary                     [2]bool                  // False for a requested endpoint strictly inside material.
	Faces                                  [2]int                   // Bracketing native faces in operand order; -1 when no fiber exists.
	Hits                                   []AlgebraicGraphFiberHit // True source boundary events inside the closed request range.
	ExactOperations, PreparationOperations int
}

// IntersectGraphNormalLineRational is the native convenience path for a
// qualified graph-only Boolean common. It retains exact origin, signed scale,
// and finite parameter units; it never normalizes or rounds the line.
func (s AlgebraicShellBoolean) IntersectGraphNormalLineRational(ctx context.Context, origin [3]*big.Rat, scale *big.Rat, limits [2]*big.Rat, o AlgebraicGraphNormalLineOptions) (AlgebraicGraphFiberLine, error) {
	q, e := s.PrepareGraphMaterialQuery(ctx, o.Preparation)
	if e != nil {
		return AlgebraicGraphFiberLine{Faces: [2]int{-1, -1}}, e
	}
	out, e := q.IntersectNormalLineRational(ctx, origin, scale, limits, o.Exact)
	if e != nil {
		return out, e
	}
	out.PreparationOperations = q.PreparationOperations()
	return out, nil
}

// IntersectNormalLineRational clips p(t)=origin+t*scale*e_residual against
// retained material. Projection coordinates are fixed in original model axes.
// A proved graph has no boundary-coincident segment in this direction. A rim
// can yield a singleton with both adjacent native faces and no material on
// either open side. Requested endpoints may lie inside; they are never emitted
// as invented boundary hits. Zero scale and reversed limits reject atomically.
// Query work includes a conservative reservation of64 bounded rational ops for
// clipping/witness construction; all intermediate rationals obey the bit cap.
func (q *AlgebraicGraphMaterialQuery) IntersectNormalLineRational(ctx context.Context, origin [3]*big.Rat, scale *big.Rat, limits [2]*big.Rat, o geom.ExactCurvePlaneEventOptions) (out AlgebraicGraphFiberLine, err error) {
	out.Faces = [2]int{-1, -1}
	defer func() {
		if err != nil {
			out = AlgebraicGraphFiberLine{Faces: [2]int{-1, -1}, ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil || scale == nil || scale.Sign() == 0 || limits[0] == nil || limits[1] == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	o, err = graphMaterialLimits(o)
	if err != nil {
		return out, err
	}
	check := func(x *big.Rat) error {
		if x == nil {
			return geom.ErrInvalid
		}
		if x.Num().BitLen() > o.MaxCoefficientBits || x.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	for _, x := range []*big.Rat{origin[0], origin[1], origin[2], scale, limits[0], limits[1]} {
		if e := check(x); e != nil {
			return out, e
		}
	}
	if limits[0].Cmp(limits[1]) > 0 {
		return out, geom.ErrInvalid
	}
	const reserve = 64
	if o.MaxOperations <= reserve {
		return out, geom.ErrConvergence
	}
	queryOptions := o
	queryOptions.MaxOperations -= reserve
	fiber, e := q.locateRationalGraphMaterial(ctx, origin, queryOptions, true)
	out.ExactOperations = fiber.ExactOperations + reserve
	if e != nil {
		return out, e
	}
	if !fiber.Complete {
		return out, geom.ErrConvergence
	}
	out.Complete = true
	out.Faces = fiber.Faces
	if fiber.Lower == nil {
		return out, nil
	}
	parameters := [2]*big.Rat{}
	for i, x := range []*big.Rat{fiber.Lower, fiber.Upper} {
		d := new(big.Rat).Sub(x, origin[q.residual])
		if e := check(d); e != nil {
			return out, e
		}
		parameters[i] = new(big.Rat).Quo(d, scale)
		if e := check(parameters[i]); e != nil {
			return out, e
		}
	}
	faces := append([]int(nil), fiber.BoundaryFaces...)
	if scale.Sign() < 0 {
		parameters[0], parameters[1] = parameters[1], parameters[0]
		if len(faces) == 2 {
			faces[0], faces[1] = faces[1], faces[0]
		}
	}
	singleton := parameters[0].Cmp(parameters[1]) == 0
	lo, hi := new(big.Rat).Set(parameters[0]), new(big.Rat).Set(parameters[1])
	if lo.Cmp(limits[0]) < 0 {
		lo.Set(limits[0])
	}
	if hi.Cmp(limits[1]) > 0 {
		hi.Set(limits[1])
	}
	if lo.Cmp(hi) > 0 {
		return out, nil
	}
	out.Present = true
	out.Parameters = [2]*big.Rat{lo, hi}
	for i, x := range out.Parameters {
		out.EndpointOnBoundary[i] = x.Cmp(parameters[0]) == 0 || x.Cmp(parameters[1]) == 0
	}
	for i, t := range parameters {
		if singleton && i == 1 {
			break
		}
		if t.Cmp(limits[0]) < 0 || t.Cmp(limits[1]) > 0 {
			continue
		}
		hit := AlgebraicGraphFiberHit{Parameter: new(big.Rat).Set(t), Before: i == 1, After: i == 0}
		if singleton {
			hit.Before, hit.After = false, false
			hit.Faces = append([]int(nil), fiber.BoundaryFaces...)
			hit.RimEdges = append([]int(nil), fiber.RimEdges...)
		} else {
			if len(faces) != 2 {
				return out, geom.ErrConvergence
			}
			hit.Faces = []int{faces[i]}
		}
		for axis, x := range origin {
			hit.Point[axis] = new(big.Rat).Set(x)
		}
		delta := new(big.Rat).Mul(scale, t)
		if e := check(delta); e != nil {
			return out, e
		}
		hit.Point[q.residual].Add(hit.Point[q.residual], delta)
		if e := check(hit.Point[q.residual]); e != nil {
			return out, e
		}
		out.Hits = append(out.Hits, hit)
	}
	return out, nil
}
