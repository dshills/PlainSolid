package collision

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
)

// ExactMassOptions bounds the second-moment integration over a validated
// planar boundary. Zero selects two million rational operations and 16384
// bits per intermediate numerator or denominator.
type ExactMassOptions struct{ MaxOperations, MaxCoefficientBits int }

// ExactPlanarMass contains exact rational unit-density moments of the
// represented tessellation. SecondMoments[i][j] is integral(x_i*x_j dV) in
// world coordinates. The inertia tensors use the conventional sign -Q_ij
// off diagonal. An empty material has zero moments and an undefined (nil)
// centroid. All returned rational values belong to the caller.
type ExactPlanarMass struct {
	Volume                       *big.Rat
	FirstMoments, Centroid       [3]*big.Rat
	SecondMoments, InertiaOrigin [3][3]*big.Rat
	InertiaCentroid              [3][3]*big.Rat
	Operations                   int
}

// MeasureExactPlanarMass validates exact embedding and material orientation,
// then integrates the represented planar boundary. Prepare a PointLocator
// directly to reuse its collision and nesting proof across queries.
func MeasureExactPlanarMass(ctx context.Context, solid brep.Solid, validation LocationOptions, options ExactMassOptions) (ExactPlanarMass, error) {
	if ctx == nil {
		return ExactPlanarMass{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return ExactPlanarMass{}, err
	}
	loc, err := NewMeshPointLocator(ctx, solid.Tessellate(), solid.TriangleFaces(), validation)
	if err != nil {
		return ExactPlanarMass{}, err
	}
	return loc.MassPropertiesExact(ctx, options)
}

// MassPropertiesExact integrates exact volume, centroid, second moments and
// inertia from the same oriented triangles that prepared the locator.
func (l *PointLocator) MassPropertiesExact(ctx context.Context, o ExactMassOptions) (ExactPlanarMass, error) {
	if l == nil {
		return ExactPlanarMass{}, ErrInvalid
	}
	return exactPlanarMass(ctx, []*PointLocator{l}, o)
}

// MassPropertiesExact integrates the disjoint component interiors. The
// declared point contacts have zero volume, so they add no mass terms.
func (l *VertexContactAssemblyLocator) MassPropertiesExact(ctx context.Context, o ExactMassOptions) (ExactPlanarMass, error) {
	if l == nil {
		return ExactPlanarMass{}, ErrInvalid
	}
	parts := make([]*PointLocator, len(l.components))
	for i, component := range l.components {
		parts[i] = component.loc
	}
	return exactPlanarMass(ctx, parts, o)
}

type planarMassArithmetic struct {
	budget locationBudget
	bits   int
	err    error
}

func (a *planarMassArithmetic) check(v *big.Rat) *big.Rat {
	if a.err == nil {
		a.err = a.budget.tick(1)
		if a.err == nil && (v.Num().BitLen() > a.bits || v.Denom().BitLen() > a.bits) {
			a.err = ErrLimit
		}
	}
	return v
}

func (a *planarMassArithmetic) add(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	return a.check(new(big.Rat).Add(x, y))
}

func (a *planarMassArithmetic) sub(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	return a.check(new(big.Rat).Sub(x, y))
}

func (a *planarMassArithmetic) mul(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	return a.check(new(big.Rat).Mul(x, y))
}

func (a *planarMassArithmetic) div(x, y *big.Rat) *big.Rat {
	if a.err != nil {
		return new(big.Rat)
	}
	return a.check(new(big.Rat).Quo(x, y))
}

func newPlanarMassArithmetic(ctx context.Context, o ExactMassOptions) (planarMassArithmetic, error) {
	if ctx == nil || o.MaxOperations < 0 || o.MaxCoefficientBits < 0 {
		return planarMassArithmetic{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return planarMassArithmetic{}, err
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	return planarMassArithmetic{budget: locationBudget{ctx: ctx, limit: o.MaxOperations}, bits: o.MaxCoefficientBits}, nil
}

func exactPlanarMass(ctx context.Context, parts []*PointLocator, o ExactMassOptions) (ExactPlanarMass, error) {
	var zero ExactPlanarMass
	arith, err := newPlanarMassArithmetic(ctx, o)
	if err != nil {
		return zero, err
	}
	volume, validationVolume := new(big.Rat), new(big.Rat)
	var first [3]*big.Rat
	var second [3][3]*big.Rat
	for i := range first {
		first[i] = new(big.Rat)
		for j := i; j < 3; j++ {
			second[i][j] = new(big.Rat)
		}
	}
	var origin point
	fourth, sixth, twentieth := big.NewRat(1, 4), big.NewRat(1, 6), big.NewRat(1, 20)
	for _, l := range parts {
		for _, shell := range l.shells {
			validationVolume = arith.add(validationVolume, shell.SignedVolume)
		}
		for _, t := range l.triangles {
			if origin[0] == nil {
				origin = t.v[0]
			}
			var vertex [3]point
			var sum point
			for i := 0; i < 3; i++ {
				sum[i] = new(big.Rat)
				for j := 0; j < 3; j++ {
					vertex[j][i] = arith.sub(t.v[j][i], origin[i])
					sum[i] = arith.add(sum[i], vertex[j][i])
				}
			}
			det := new(big.Rat)
			for i := 0; i < 3; i++ {
				j, k := (i+1)%3, (i+2)%3
				minor := arith.sub(arith.mul(vertex[1][j], vertex[2][k]), arith.mul(vertex[1][k], vertex[2][j]))
				det = arith.add(det, arith.mul(vertex[0][i], minor))
			}
			v := arith.mul(det, sixth)
			if arith.err != nil {
				return zero, arith.err
			}
			if v.Sign() == 0 {
				continue
			}
			volume = arith.add(volume, v)
			v4, v20 := arith.mul(v, fourth), arith.mul(v, twentieth)
			for i := 0; i < 3; i++ {
				first[i] = arith.add(first[i], arith.mul(v4, sum[i]))
				for j := i; j < 3; j++ {
					// On a tetrahedron with its fourth vertex at zero, the
					// diagonal barycentric second moment is twice the cross one.
					q := arith.mul(sum[i], sum[j])
					for _, p := range vertex {
						q = arith.add(q, arith.mul(p[i], p[j]))
					}
					second[i][j] = arith.add(second[i][j], arith.mul(v20, q))
				}
			}
			if arith.err != nil {
				return zero, arith.err
			}
		}
	}
	if arith.err != nil {
		return zero, arith.err
	}
	if volume.Cmp(validationVolume) != 0 || (origin[0] != nil && volume.Sign() <= 0) {
		return zero, fmt.Errorf("%w: exact mass volume disagrees with the material boundary", ErrInvalid)
	}
	result := ExactPlanarMass{Volume: volume}
	if origin[0] == nil {
		for i := 0; i < 3; i++ {
			result.FirstMoments[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				result.SecondMoments[i][j] = new(big.Rat)
				result.InertiaOrigin[i][j] = new(big.Rat)
				result.InertiaCentroid[i][j] = new(big.Rat)
			}
		}
		result.Operations = arith.budget.work
		return result, nil
	}
	var central [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		result.FirstMoments[i] = arith.add(first[i], arith.mul(origin[i], volume))
		result.Centroid[i] = arith.add(origin[i], arith.div(first[i], volume))
		for j := i; j < 3; j++ {
			center := arith.sub(second[i][j], arith.div(arith.mul(first[i], first[j]), volume))
			central[i][j], central[j][i] = center, center
			shift := arith.add(arith.mul(origin[i], first[j]), arith.mul(origin[j], first[i]))
			world := arith.add(second[i][j], arith.add(shift, arith.mul(arith.mul(origin[i], origin[j]), volume)))
			result.SecondMoments[i][j], result.SecondMoments[j][i] = world, new(big.Rat).Set(world)
		}
	}
	inertia := func(q [3][3]*big.Rat) [3][3]*big.Rat {
		var result [3][3]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if i == j {
					result[i][j] = arith.add(q[(i+1)%3][(i+1)%3], q[(i+2)%3][(i+2)%3])
				} else {
					result[i][j] = arith.sub(new(big.Rat), q[i][j])
				}
			}
		}
		return result
	}
	result.InertiaOrigin = inertia(result.SecondMoments)
	result.InertiaCentroid = inertia(central)
	if arith.err != nil {
		return zero, arith.err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	result.Operations = arith.budget.work
	return result, nil
}
