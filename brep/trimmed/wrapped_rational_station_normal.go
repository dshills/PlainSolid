package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedRationalStationNormalOptions bounds one original-boundary point
// classification separately from the aggregate normal arithmetic/evaluations.
// MaxOperations defaults to65536, maximum1M; coefficient bits default16384,
// maximum65536. Source retains its per-band/shared-edge authentication limits.
type WrappedRationalStationNormalOptions struct {
	Source                                                geom.SurfacePatchOptions
	MaxPointOperations, MaxOperations, MaxCoefficientBits int
}

// WrappedRationalStationFaceNormal is an exact nonzero outward direction, not
// a rounded unit vector. Direction/sqrt(SquaredLength) specifies its unit normal.
// At a crease each incident original face has its own one-sided normal.
type WrappedRationalStationFaceNormal struct {
	Face          int
	Direction     [3]*big.Rat
	SquaredLength *big.Rat
}

type WrappedRationalStationNormals struct {
	Point           [3]*big.Rat
	Normals         []WrappedRationalStationFaceNormal
	ExactOperations int
}

// WrappedRationalStationBoundaryNormalsRational returns every original face
// normal at an exact rational boundary point of the authenticated native loft.
// Inside/outside points are rejected. Canonical positive conic quadrants are
// regular, including their cardinal joins; terminal caps are +/-Z. An internal
// station disk is never a boundary face. Failures return no partial normals.
func (s Solid) WrappedRationalStationBoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationNormalOptions) (WrappedRationalStationNormals, error) {
	empty := WrappedRationalStationNormals{}
	if o.MaxOperations == 0 {
		o.MaxOperations = 65536
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPointOperations == 0 {
		o.MaxPointOperations = 8192
	}
	if ctx == nil || o.MaxOperations < 1 || o.MaxOperations > 1000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 65536 || o.MaxPointOperations < 1 || o.MaxPointOperations > 1000000 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	p, pointOptions, e := prepareWrappedRationalStationPointQuery(ctx, p, WrappedRationalStationPointOptions{
		Source: o.Source, MaxPointOperations: o.MaxPointOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if e != nil {
		return empty, e
	}
	proof, bands, e := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if e != nil {
		return empty, e
	}
	loc, e := locateWrappedRationalStationPoint(ctx, proof, bands, p, pointOptions)
	if e != nil {
		return empty, e
	}
	if loc.Location != parametric.Boundary || len(loc.Faces) == 0 {
		return empty, geom.ErrDomain
	}
	// Reuse the bounded rational ledger: exact support evaluations below debit
	// this same operation count, rather than resetting it at a station crease.
	b := stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	zero, two := big.NewRat(0, 1), big.NewRat(2, 1)
	result := WrappedRationalStationNormals{}
	for i := range p {
		result.Point[i] = new(big.Rat).Set(p[i])
		b.check(p[i])
	}
	x, y := b.sub(p[0], proof.Center[0]), b.sub(p[1], proof.Center[1])
	quadrant := 0
	if x.Sign() < 0 {
		if y.Sign() >= 0 {
			quadrant = 1
		} else {
			quadrant = 2
		}
	} else if y.Sign() < 0 {
		quadrant = 3
	}
	rotate := func(x, y *big.Rat) (*big.Rat, *big.Rat) {
		switch quadrant {
		case 1:
			return new(big.Rat).Set(y), b.sub(zero, x)
		case 2:
			return b.sub(zero, x), b.sub(zero, y)
		case 3:
			return b.sub(zero, y), new(big.Rat).Set(x)
		}
		return new(big.Rat).Set(x), new(big.Rat).Set(y)
	}
	for _, face := range loc.Faces {
		n := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		if face == proof.History.CapFaces[0] {
			n[2].SetInt64(-1)
		} else if face == proof.History.CapFaces[1] {
			n[2].SetInt64(1)
		} else {
			band := slices.Index(proof.History.WallFaces, face)
			if band < 0 {
				return empty, geom.ErrConvergence
			}
			patch := bands[band].FundamentalPatch()
			cuts, _ := patch.KnotBreaks()
			if len(cuts) != 5 {
				return empty, geom.ErrConvergence
			}
			u := b.div(b.add(ratInset(cuts[quadrant]), ratInset(cuts[quadrant+1])), two)
			if b.err != nil {
				return empty, b.err
			}
			remaining := b.limit - b.used
			if remaining < 1 {
				return empty, geom.ErrConvergence
			}
			jet, e := patch.EvaluateRational(ctx, u, ratInset(patch.Domain().VMin), geom.SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.bits})
			if e != nil {
				return empty, e
			}
			b.used += jet.ExactOperations
			// The whole-source proof establishes the homogeneous conic
			// (U+V-r)^2 = K*(r-U)*(r-V), K=4*w1²/(w0*w2)>0.
			// One exact interior point uniquely recovers K; this is algebraic
			// coefficient recovery within that proved family, not curve fitting.
			a, c := rotate(b.sub(jet.Point[0], proof.Center[0]), b.sub(jet.Point[1], proof.Center[1]))
			r0 := proof.Radii[band]
			sum := b.sub(b.add(a, c), r0)
			ra, rc := b.sub(r0, a), b.sub(r0, c)
			if b.err != nil {
				return empty, b.err
			}
			if sum.Sign() <= 0 || ra.Sign() <= 0 || rc.Sign() <= 0 {
				return empty, geom.ErrConvergence
			}
			k := b.div(b.mul(sum, sum), b.mul(ra, rc))
			slope := b.div(b.sub(proof.Radii[band+1], r0), b.sub(proof.Heights[band+1], proof.Heights[band]))
			radius := b.add(r0, b.mul(slope, b.sub(p[2], proof.Heights[band])))
			a, c = rotate(x, y)
			sum = b.sub(b.add(a, c), radius)
			residual := b.sub(b.mul(sum, sum), b.mul(k, b.mul(b.sub(radius, a), b.sub(radius, c))))
			if b.err != nil {
				return empty, b.err
			}
			if residual.Sign() != 0 || sum.Sign() < 0 {
				return empty, geom.ErrConvergence
			}
			du := b.add(b.mul(two, sum), b.mul(k, b.sub(radius, c)))
			dv := b.add(b.mul(two, sum), b.mul(k, b.sub(radius, a)))
			dr := b.sub(b.sub(zero, b.mul(two, sum)), b.mul(k, b.sub(b.mul(two, radius), b.add(a, c))))
			switch quadrant {
			case 0:
				n[0], n[1] = du, dv
			case 1:
				n[0], n[1] = b.sub(zero, dv), du
			case 2:
				n[0], n[1] = b.sub(zero, du), b.sub(zero, dv)
			case 3:
				n[0], n[1] = dv, b.sub(zero, du)
			}
			n[2] = b.mul(slope, dr)
			radial := b.add(b.mul(n[0], x), b.mul(n[1], y))
			if b.err != nil {
				return empty, b.err
			}
			if radial.Sign() <= 0 {
				return empty, geom.ErrConvergence
			}
		}
		squared := new(big.Rat)
		for _, v := range n {
			squared = b.add(squared, b.mul(v, v))
		}
		if b.err != nil {
			return empty, b.err
		}
		if squared.Cmp(zero) <= 0 {
			return empty, geom.ErrConvergence
		}
		result.Normals = append(result.Normals, WrappedRationalStationFaceNormal{Face: face, Direction: n, SquaredLength: squared})
	}
	result.ExactOperations = b.used
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
