package cuspidal

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ElevateWallPCurves preserves a strict cuspidal transfer when a cubic
// affine-wall pcurve has nonterminating interior poles and its single
// degree elevation has exact decimals. The affine support and the edge
// clock are unchanged. Already representable and unsupported uses are
// left for the ordinary writer.
func ElevateWallPCurves(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options, decimal func(*big.Rat) (string, error)) error {
	for _, f := range b.Faces {
		if f.Affine == nil {
			continue
		}
		for _, loop := range f.Loops {
			for i := range loop {
				if err := ctx.Err(); err != nil {
					return err
				}
				net, err := exactQuarticWallPCurve(loop[i].UV, proof, decimal)
				if err != nil {
					return err
				}
				if net != nil {
					loop[i].UV.Control = net
				}
			}
		}
	}
	return nil
}

func exactQuarticWallPCurve(span geom.ExactBezierSpan, proof edgecheck.Options, decimal func(*big.Rat) (string, error)) ([][4]*big.Rat, error) {
	const targetDegree = 4
	control := span.Control
	if len(control) != targetDegree || span.Start != 0 || span.End != 1 {
		return nil, nil
	}
	weight := control[0][3]
	if weight.Sign() <= 0 {
		return nil, nil
	}
	for _, row := range control {
		if row[2].Sign() != 0 || row[3].Cmp(weight) != 0 {
			return nil, nil
		}
	}
	representable := func(row [4]*big.Rat) bool {
		for _, coordinate := range row[:2] {
			if _, err := decimal(new(big.Rat).Quo(coordinate, weight)); err != nil {
				return false
			}
		}
		return true
	}
	if !representable(control[0]) || !representable(control[3]) ||
		(representable(control[1]) && representable(control[2])) {
		return nil, nil
	}
	quartic := make([][4]*big.Rat, targetDegree+1)
	quartic[0], quartic[targetDegree] = control[0], control[3]
	for i := 1; i < targetDegree; i++ {
		priorWeight := big.NewRat(int64(i), targetDegree)
		nextWeight := big.NewRat(int64(targetDegree-i), targetDegree)
		for j := range quartic[i] {
			left := new(big.Rat).Mul(control[i-1][j], priorWeight)
			right := new(big.Rat).Mul(control[i][j], nextWeight)
			quartic[i][j] = left.Add(left, right)
		}
		if !representable(quartic[i]) {
			return nil, nil
		}
	}
	if proof.MaxDegree > 0 && proof.MaxDegree < targetDegree {
		return nil, geom.ErrConvergence
	}
	bits := proof.MaxBits
	if bits == 0 {
		bits = 16384 // Same default coefficient budget as the source proof.
	}
	for _, row := range quartic {
		for _, q := range row {
			if q.Num().BitLen() > bits || q.Denom().BitLen() > bits {
				return nil, geom.ErrConvergence
			}
		}
	}
	return quartic, nil
}
