package iges

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

const maxPeriodicSurface128Poles = 4096

type periodicSurface128 struct {
	expanded   *geom.BSplineSurface
	exact      *geom.ExactPeriodicSplineSurface
	axes       [2]bool
	polynomial bool
}

// ExactPeriodicGeometry returns the original decimal type-128 net as an
// immutable exact cyclic tensor in file coordinates. EvaluateRational uses
// physical file parameters and their derivatives. Each periodic axis's first
// and last returned knots give its original seam and period. Affine placement
// is available separately on the Entity.
func (s Surface) ExactPeriodicGeometry() (*geom.ExactPeriodicSplineSurface, error) {
	if s.periodic == nil {
		return nil, geom.ErrUnrepresentable
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s.periodic.exact, nil
}

func provePeriodicSurface128(expanded *geom.BSplineSurface, rawU, rawV, rawNet, rawBounds []parameter, counts, degrees [2]int, periodic [2]bool, polynomial bool) (*periodicSurface128, error) {
	var axes [2]geom.ExactTensorSplineAxis
	var phase [2]int
	for dir, raw := range [2][]parameter{rawU, rawV} {
		var err error
		axes[dir], phase[dir], err = periodicSurface128Axis(raw, rawBounds[2*dir:2*dir+2], counts[dir], degrees[dir], periodic[dir])
		if err != nil {
			return nil, fmt.Errorf("iges: surface axis %c: %w", 'U'+dir, err)
		}
	}
	n := counts[0] * counts[1]
	controls := make([][4]*big.Rat, axes[0].Count*axes[1].Count)
	var firstWeight *big.Rat
	for v := 0; v < counts[1]; v++ {
		for u := 0; u < counts[0]; u++ {
			i := v*counts[0] + u
			weight, err := igesPlanarExact(rawNet[i])
			if err != nil || weight.Sign() <= 0 {
				return nil, errors.New("iges: invalid exact surface weight")
			}
			if i == 0 {
				firstWeight = weight
			}
			if polynomial && weight.Cmp(firstWeight) != 0 {
				return nil, errors.New("iges: inconsistent exact polynomial weight")
			}
			h, err := igesPlanarCoordinates(rawNet[n+3*i : n+3*i+3])
			if err != nil {
				return nil, err
			}
			// The expanded basis starts its active controls at
			// span-degree. The cyclic basis starts them at span-first.
			cu := (u + phase[0]) % axes[0].Count
			cv := (v + phase[1]) % axes[1].Count
			owner := &controls[cu*axes[1].Count+cv]
			control := [4]*big.Rat{h[0].Mul(h[0], weight), h[1].Mul(h[1], weight), h[2].Mul(h[2], weight), weight}
			if owner[3] == nil {
				*owner = control
				continue
			}
			for j, q := range control {
				if q.Cmp(owner[j]) != 0 {
					return nil, errors.New("iges: inconsistent repeated periodic surface pole or weight")
				}
			}
		}
	}
	exact, err := geom.NewExactPeriodicSplineSurface(context.Background(), axes[0], axes[1], controls, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, err
	}
	return &periodicSurface128{expanded: expanded, exact: exact, axes: periodic, polynomial: polynomial}, nil
}

func periodicSurface128Axis(raw, rawBounds []parameter, count, degree int, periodic bool) (geom.ExactTensorSplineAxis, int, error) {
	var zero geom.ExactTensorSplineAxis
	knots := make([]*big.Rat, len(raw))
	for i, value := range raw {
		q, err := igesPlanarExact(value)
		if err != nil {
			return zero, 0, err
		}
		if i > 0 && knots[i-1].Cmp(q) > 0 {
			return zero, 0, errors.New("unordered exact knots")
		}
		knots[i] = q
	}
	for i, index := range [2]int{degree, count} {
		bound, err := igesPlanarExact(rawBounds[i])
		if err != nil || bound.Cmp(knots[index]) != 0 {
			return zero, 0, errors.New("periodic surface requires its full original active interval")
		}
	}
	if !periodic {
		return geom.ExactTensorSplineAxis{Degree: degree, Count: count, Knots: knots}, 0, nil
	}
	// In the finite expansion K[degree] is the last entry at the
	// start seam and K[count] the first at the end seam. The cyclic
	// basis stores the repeats at the start, at the original phase.
	first := degree
	for first > 0 && knots[first-1].Cmp(knots[degree]) == 0 {
		first--
	}
	cyclicCount := count - first
	period := new(big.Rat).Sub(knots[count], knots[first])
	if first == 0 || cyclicCount <= degree || knots[count-1].Cmp(knots[count]) >= 0 || period.Sign() <= 0 {
		return zero, 0, errors.New("invalid periodic seam or cyclic owner count")
	}
	var translated big.Rat
	for i := 0; i+cyclicCount < len(knots); i++ {
		if translated.Sub(knots[i+cyclicCount], knots[i]).Cmp(period) != 0 {
			return zero, 0, errors.New("inconsistent periodic knot translation")
		}
	}
	return geom.ExactTensorSplineAxis{Degree: degree, Count: cyclicCount, Periodic: true, Knots: knots[first : count+1]}, degree - first, nil
}
