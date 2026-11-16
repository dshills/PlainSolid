package iges

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"plainsolid/geom"
)

// NewExactPeriodicSurface128 constructs a standalone IGES type-128 surface
// from an exact cyclic tensor. It uses the full active rectangle and proves
// the emitted knot and homogeneous pole ownership through the strict reader.
// The finite decimal carrier must preserve every Cartesian pole and weight.
func NewExactPeriodicSurface128(g *geom.ExactPeriodicSplineSurface) (*Surface, error) {
	data, err := encodeExactPeriodicSurface128(g, nil)
	if err != nil {
		return nil, err
	}
	maxValues := maxPeriodicSurface128Poles * 8
	values, err := parseParameters(data, ',', ';', &maxValues)
	if err != nil {
		return nil, err
	}
	entity, err := decodeGeometry(128, values, maxPeriodicSurface128Poles, true)
	if err != nil {
		return nil, err
	}
	return entity.Surface, nil
}

type periodic128ExpandedAxis struct {
	knots        []*big.Rat
	count, phase int
}

func expandPeriodic128Axis(axis geom.ExactTensorSplineAxis) periodic128ExpandedAxis {
	if !axis.Periodic {
		return periodic128ExpandedAxis{knots: axis.Knots, count: axis.Count}
	}
	seamCopies := 1
	for axis.Knots[seamCopies].Cmp(axis.Knots[0]) == 0 {
		seamCopies++
	}
	// The last copy of the start seam must be at index Degree.
	// Prepend the translated ghost knots and preserve the seam phase
	// in the duplicated controls, undoing the finite basis shift.
	first := axis.Degree + 1 - seamCopies
	out := periodic128ExpandedAxis{count: axis.Count + first, phase: seamCopies - 1}
	out.knots = make([]*big.Rat, out.count+axis.Degree+1)
	period := new(big.Rat).Sub(axis.Knots[axis.Count], axis.Knots[0])
	for i := range out.knots {
		j := i - first
		switch {
		case j < 0:
			out.knots[i] = new(big.Rat).Sub(axis.Knots[j+axis.Count], period)
		case j >= axis.Count:
			out.knots[i] = new(big.Rat).Add(axis.Knots[j-axis.Count], period)
		default:
			out.knots[i] = axis.Knots[j]
		}
	}
	return out
}

// A scalar and its delimiter must fit within one IGES parameter record.
func periodic128Decimal(q *big.Rat) (string, error) {
	s, err := exactIGESSplineDecimal(q)
	if err != nil || len(s) > 63 {
		return "", fmt.Errorf("iges: type-128 scalar has no bounded exact decimal: %w", geom.ErrUnrepresentable)
	}
	return s, nil
}

func periodic128KnotTokens(knots []*big.Rat, degree int) ([]string, error) {
	out := make([]string, len(knots))
	var previous float64
	runStart := 0
	for i, q := range knots {
		var err error
		out[i], err = periodic128Decimal(q)
		if err != nil {
			return nil, err
		}
		x, _ := q.Float64()
		if i > 0 && q.Cmp(knots[i-1]) == 0 {
			// The pinned reader removes poles at an overfull interior knot.
			if i-runStart >= degree && runStart != 0 && q.Cmp(knots[len(knots)-1]) != 0 {
				return nil, fmt.Errorf("iges: type-128 interior multiplicity exceeds transfer degree: %w", geom.ErrUnrepresentable)
			}
		} else if i > 0 {
			runStart = i
			// The pinned consumer merges a distinct station this close
			// to the preceding binary64 knot when forming multiplicities.
			ulp := math.Abs(math.Nextafter(previous, math.Copysign(math.Inf(1), previous)) - previous)
			if x-previous <= ulp {
				return nil, fmt.Errorf("iges: distinct type-128 knots collapse in binary64 transfer: %w", geom.ErrUnrepresentable)
			}
		}
		previous = x
	}
	return out, nil
}

func encodeExactPeriodicSurface128(g *geom.ExactPeriodicSplineSurface, source *Surface) (string, error) {
	u, v, controls := g.Data()
	if len(controls) == 0 {
		return "", geom.ErrInvalid
	}
	// Match the maximum surface degree of the pinned IGES converter.
	if u.Degree > 25 || v.Degree > 25 {
		return "", fmt.Errorf("iges: type-128 degree exceeds transfer limit: %w", geom.ErrUnrepresentable)
	}
	eu, ev := expandPeriodic128Axis(u), expandPeriodic128Axis(v)
	if eu.count > maxPeriodicSurface128Poles/ev.count {
		return "", fmt.Errorf("iges: expanded periodic type-128 control point limit: %w", geom.ErrConvergence)
	}
	ku, err := periodic128KnotTokens(eu.knots, u.Degree)
	if err != nil {
		return "", err
	}
	kv, err := periodic128KnotTokens(ev.knots, v.Degree)
	if err != nil {
		return "", err
	}
	n := eu.count * ev.count
	weights, poles := make([]string, n), make([]string, 3*n)
	polynomial, distinguishable := true, false
	firstWeight := controls[eu.phase*v.Count+ev.phase][3]
	firstFloat, _ := firstWeight.Float64()
	lowest := firstFloat
	// Type 128 lists its controls with U varying fastest. Keep the
	// original cyclic phase in the finite expansion and do not rescale
	// homogeneous data to make a Cartesian coordinate representable.
	for j := 0; j < ev.count; j++ {
		for i := 0; i < eu.count; i++ {
			c := controls[((i+eu.phase)%u.Count)*v.Count+(j+ev.phase)%v.Count]
			k := j*eu.count + i
			weights[k], err = periodic128Decimal(c[3])
			if err != nil {
				return "", err
			}
			w, _ := c[3].Float64()
			lowest = min(lowest, w)
			polynomial = polynomial && c[3].Cmp(firstWeight) == 0
			distinguishable = distinguishable || math.Abs(w-firstFloat) > 1e-10
			for axis := range 3 {
				q := new(big.Rat).Quo(c[axis], c[3])
				poles[3*k+axis], err = periodic128Decimal(q)
				if err != nil {
					return "", err
				}
			}
		}
	}
	// The pinned reader treats weights within 1e-10 of the first as
	// polynomial and refuses rational weights below PConfusion().
	if !polynomial && (!distinguishable || lowest < 1e-9) {
		return "", fmt.Errorf("iges: type-128 rational weights lost in external transfer: %w", geom.ErrUnrepresentable)
	}
	closedU, closedV := u.Periodic, v.Periodic
	if source != nil {
		closedU, closedV, polynomial = source.ClosedU, source.ClosedV, source.Polynomial
	}
	p := []string{"128", strconv.Itoa(eu.count - 1), strconv.Itoa(ev.count - 1),
		strconv.Itoa(u.Degree), strconv.Itoa(v.Degree), flag(closedU), flag(closedV),
		flag(polynomial), flag(u.Periodic), flag(v.Periodic)}
	p = append(p, ku...)
	p = append(p, kv...)
	p = append(p, weights...)
	p = append(p, poles...)
	p = append(p, ku[u.Degree], ku[eu.count], kv[v.Degree], kv[ev.count], "0", "0")
	return strings.Join(p, ",") + ";", nil
}
