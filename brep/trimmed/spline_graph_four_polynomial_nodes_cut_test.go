package trimmed

import (
	"math/big"
)

func fourPolynomialCutBranchValue(power []*big.Rat, x *big.Rat) *big.Rat {
	value := new(big.Rat)
	for i := len(power) - 1; i >= 0; i-- {
		value.Mul(value, x).Add(value, power[i])
	}
	return value
}
