package geom

import (
	"context"
	"math/big"
)

// VerifiedNormalOffsetAdjacency proves that two already verified offset
// patches meet exactly along one shared original edge. Coordinate is
// the exact value of their common separating model coordinate. The two
// interior signs describe the open halfspaces in that coordinate.
type VerifiedNormalOffsetAdjacency struct {
	Complete        bool
	Seam            SurfaceNormalOffsetSeam
	Axis            int
	Coordinate      *big.Rat
	InteriorSigns   [2]int
	ExactOperations int
}

// CertifyVerifiedNormalOffsetAdjacency checks the original seam and the
// projected differential proofs together. The two patches must have the
// same oriented offset and a common projection axis associated with their
// fixed edge directions. The original edge must have constant coordinate
// on that axis, with zero normal component. The strict signs of the two
// corresponding projected derivatives then put both interiors in opposite
// open halfspaces at every shared fixed displacement.
func CertifyVerifiedNormalOffsetAdjacency(ctx context.Context, a, b *VerifiedNormalOffsetPatch, edgeA, edgeB SurfaceEdge,
	o SurfaceNormalOffsetSeamOptions) (r VerifiedNormalOffsetAdjacency, err error) {
	if ctx == nil || a == nil || b == nil || a.source == nil || b.source == nil || a.distance == nil || b.distance == nil ||
		edgeA > SurfaceEdgeVMax || edgeB > SurfaceEdgeVMax ||
		(o.BStationRatio != nil && o.BStationRatio.Sign() <= 0) {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	limits, err := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return r, err
	}
	want := new(big.Rat).Set(a.distance)
	if o.FlipNormalA != o.FlipNormalB {
		want.Neg(want)
	}
	if b.distance.Cmp(want) != 0 {
		return r, nil
	}
	axisA, sideA := offsetEdgeInteriorSign(a, edgeA)
	axisB, sideB := offsetEdgeInteriorSign(b, edgeB)
	if axisA != axisB || sideA == sideB {
		return r, nil
	}
	r.Seam, err = CertifySurfaceNormalOffsetSeam(ctx, a.source, b.source, edgeA, edgeB, o)
	r.ExactOperations = r.Seam.ExactOperations
	if err != nil || !r.Seam.Complete {
		return r, err
	}
	if r.ExactOperations >= limits.MaxOperations {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations - r.ExactOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations += budget.operations }()
	jet, err := offsetEdgePowers(ctx, a.source, edgeA, false, nil, budget)
	if err != nil {
		return r, err
	}
	zero := new(big.Rat)
	first, err := polynomialValue(jet[0][axisA], zero, budget)
	if err != nil {
		return r, err
	}
	weight, err := polynomialValue(jet[0][3], zero, budget)
	if err != nil {
		return r, err
	}
	c := new(big.Rat).Quo(first, weight)
	if err := budget.check(c); err != nil {
		return r, err
	}
	difference, err := offsetPolynomialDet(jet[0][axisA], jet[0][3], rationalPolynomial{c}, rationalPolynomial{big.NewRat(1, 1)}, budget)
	if err != nil || len(difference) != 0 {
		return r, err
	}
	normal, err := offsetNormalPowers(ctx, jet, budget)
	if err != nil || len(normal[axisA]) != 0 {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Complete, r.Axis, r.Coordinate, r.InteriorSigns = true, axisA, c, [2]int{sideA, sideB}
	return r, nil
}

func offsetEdgeInteriorSign(p *VerifiedNormalOffsetPatch, edge SurfaceEdge) (axis, sign int) {
	fixed, sign := 0, 1
	if edge >= SurfaceEdgeVMin {
		fixed = 1
	}
	if edge == SurfaceEdgeUMax || edge == SurfaceEdgeVMax {
		sign = -sign
	}
	if p.projection.Reversed[fixed] {
		sign = -sign
	}
	return p.projection.Axes[fixed], sign
}
