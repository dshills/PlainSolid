package parabolacircle

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PrepareNormalBridge gives the certified nonapex model bridge an equivalent
// Bernstein net for exact decimal transfer. Tensor composition can represent
// the source quartic as degree eight with nonterminating poles. A degree-five
// net has binomial factors 1, 5 and 10, so a dyadic power law has exact decimal
// poles. The model clock, both original pcurves and all topology stay intact.
func PrepareNormalBridge(ctx context.Context, b trimmed.ExactBezierBoundary, bridge int, proof edgecheck.Options, decimal func(*big.Rat) (string, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if bridge < 0 || bridge >= len(b.Edges) {
		return geom.ErrConvergence
	}
	model := &b.Edges[bridge].Model
	const targetDegree = 5
	degree := len(model.Control) - 1
	if model.Start != 0 || model.End != 1 || degree < 1 || degree > 8 {
		return geom.ErrConvergence
	}
	weight := model.Control[0][3]
	if weight.Sign() <= 0 {
		return geom.ErrConvergence
	}
	representable := func(row [4]*big.Rat) bool {
		for _, q := range row[:3] {
			if _, err := decimal(new(big.Rat).Quo(q, weight)); err != nil {
				return false
			}
		}
		return true
	}
	exact := true
	for _, row := range model.Control {
		if row[3].Cmp(weight) != 0 {
			return geom.ErrConvergence
		}
		exact = exact && representable(row)
	}
	if exact {
		return nil
	}
	if proof.MaxDegree > 0 && proof.MaxDegree < targetDegree {
		return geom.ErrConvergence
	}
	maxBits := proof.MaxBits
	if maxBits == 0 {
		// Match the default coefficient limit of native edge reproof.
		maxBits = 16384
	}
	power := bernsteinPower(model.Control)
	for _, row := range power[min(len(power), targetDegree+1):] {
		for _, coefficient := range row {
			if coefficient.Sign() != 0 {
				return geom.ErrUnrepresentable
			}
		}
	}
	net := make([][4]*big.Rat, targetDegree+1)
	for i := range net {
		for j := range net[i] {
			coefficient := new(big.Rat)
			for k := 0; k <= i && k < len(power); k++ {
				factor := new(big.Rat).SetFrac(binomial(i, k), binomial(targetDegree, k))
				coefficient.Add(coefficient, factor.Mul(factor, power[k][j]))
			}
			if coefficient.Num().BitLen() > maxBits || coefficient.Denom().BitLen() > maxBits {
				return geom.ErrConvergence
			}
			net[i][j] = coefficient
		}
		if !representable(net[i]) {
			return geom.ErrUnrepresentable
		}
	}
	// Match every homogeneous coefficient in the original clock. The extracted
	// span already has both owner compositions proved against this model law.
	candidatePower := bernsteinPower(net)
	for k, row := range candidatePower {
		for j, coefficient := range row {
			if k >= len(power) {
				if coefficient.Sign() != 0 {
					return geom.ErrConvergence
				}
			} else if coefficient.Cmp(power[k][j]) != 0 {
				return geom.ErrConvergence
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	model.Control = net
	return nil
}

func bernsteinPower(control [][4]*big.Rat) [][4]*big.Rat {
	degree := len(control) - 1
	diff := append([][4]*big.Rat(nil), control...)
	power := make([][4]*big.Rat, len(control))
	for k := range power {
		factor := new(big.Rat).SetInt(binomial(degree, k))
		for j := range power[k] {
			power[k][j] = new(big.Rat).Mul(diff[0][j], factor)
		}
		for i := 0; i < degree-k; i++ {
			for j := range diff[i] {
				diff[i][j] = new(big.Rat).Sub(diff[i+1][j], diff[i][j])
			}
		}
	}
	return power
}

func binomial(n, k int) *big.Int {
	return new(big.Int).Binomial(int64(n), int64(k))
}
