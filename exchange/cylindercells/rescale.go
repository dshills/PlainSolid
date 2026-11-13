package cylindercells

import (
	"errors"
	"fmt"
	"math/big"
)

// ErrUnrepresentableScale reports a conversion that cannot preserve an exact
// represented scalar in binary64. No rounded asset is returned.
var ErrUnrepresentableScale = errors.New("cylindercells: exact unit conversion is not representable")

// RescaleSI returns physically identical geometry in the requested SI unit.
// Origin, radius and height scale by exact 10^(old-new); axes are dimensionless.
// Every scalar conversion is checked before either old or new native geometry
// is constructed. The original evidence is retained only for an identical unit.
// Otherwise the result is a new native-only asset; the input remains unchanged.
func RescaleSI(a Asset, target SIUnit) (Asset, error) {
	if _, e := a.Unit.scale(); e != nil {
		return Asset{}, e
	}
	if _, e := target.scale(); e != nil {
		return Asset{}, e
	}
	delta := a.Unit.MetresExponent - target.MetresExponent
	power := delta
	if power < 0 {
		power = -power
	}
	ten := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(power)), nil)
	factor := new(big.Rat).SetInt(ten)
	if delta < 0 {
		factor.Inv(factor)
	}
	values := []float64{a.Origin.X, a.Origin.Y, a.Origin.Z, a.Radius, a.Height}
	names := []string{"Origin.X", "Origin.Y", "Origin.Z", "Radius", "Height"}
	converted := make([]float64, len(values))
	for i, v := range values {
		q := new(big.Rat).SetFloat64(v)
		if q == nil {
			return Asset{}, ErrAsset
		}
		q.Mul(q, factor)
		x, exact := q.Float64()
		if !exact {
			return Asset{}, fmt.Errorf("%w: %s", ErrUnrepresentableScale, names[i])
		}
		converted[i] = x
	}
	if _, e := a.Validate(); e != nil {
		return Asset{}, e
	}
	if target == a.Unit {
		return a, nil
	}
	out := a
	out.Unit = target
	out.SourceSTEP = ""
	out.Origin.X = converted[0]
	out.Origin.Y = converted[1]
	out.Origin.Z = converted[2]
	out.Radius = converted[3]
	out.Height = converted[4]
	if _, e := out.Validate(); e != nil {
		return Asset{}, e
	}
	return out, nil
}
