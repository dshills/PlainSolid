package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// HelicalChartRoundoff bounds axial endpoints and angular coordinates in the stock wall chart.
// Exact input geometry and all original containment proofs are unchanged.
// It is not a bound for trig evaluation or arbitrary floating-point operations.
type HelicalChartRoundoff struct{ MaxAxialError, MaxTurnError float64 }

// GrooveWithBoundedChart permits explicitly bounded rounding of the stock seam phase
// and helical side endpoints in the stock wall chart. Strict constructors
// retain their exact-representability requirement. The bound must be less than
// one eighth of the native sewing tolerance; each contribution receives one
// quarter of that budget, with turns scaled by a conservative 2*pi*outerRadius.
// The reciprocal lead may also round in this bounded path; its angular drift
// is bounded over the full stock height and included in MaxTurnError. Strict
// constructors still require an exact reciprocal. NewSolid validation applies.
func GrooveWithBoundedChart(ctx context.Context, r HelicalGrooveRecipe, maxError float64) (Solid, HelicalChartRoundoff, error) {
	return grooveWithBoundedChart(ctx, r, nil, maxError, true)
}

// GrooveWithPhasedStarts keeps every start in the same finite axial window.
// InitialPhase is a physical turn about the original axis, independently of
// handedness. The original bounded chart and separation checks apply.
func GrooveWithPhasedStarts(ctx context.Context, r HelicalGrooveRecipe, initialPhase, maxError float64) (Solid, HelicalChartRoundoff, error) {
	if !finite(initialPhase) || initialPhase < 0 || initialPhase >= 1 {
		return Solid{}, HelicalChartRoundoff{}, geom.ErrDomain
	}
	return grooveWithBoundedChart(ctx, r, ratInset(initialPhase), maxError, true)
}

func grooveWithBoundedChart(ctx context.Context, r HelicalGrooveRecipe, phase *big.Rat, maxError float64, validateDisplay bool) (Solid, HelicalChartRoundoff, error) {
	var proof HelicalChartRoundoff
	if !finite(maxError) || maxError <= 0 || !finite(r.OuterRadius) || !finite(r.Height) || maxError > math.Max(r.OuterRadius, r.Height)*1e-11/8 || !finite(r.BoreRadius) || r.BoreRadius < 0 || r.BoreRadius >= r.OuterRadius {
		return Solid{}, proof, geom.ErrInvalid
	}
	turnError, twistError := new(big.Rat), new(big.Rat)
	upward := func(q *big.Rat) float64 {
		v, _ := q.Float64()
		if ratInset(v).Cmp(q) < 0 {
			v = math.Nextafter(v, math.Inf(1))
		}
		return v
	}
	recordTurns := func() { proof.MaxTurnError = upward(new(big.Rat).Add(turnError, twistError)) }
	roundValue := func(q *big.Rat, angular bool, span *big.Rat) (float64, *big.Rat, error) {
		v, _ := q.Float64()
		if !finite(v) {
			return 0, nil, geom.ErrUnrepresentable
		}
		delta := new(big.Rat).Abs(new(big.Rat).Sub(q, ratInset(v)))
		delta.Mul(delta, span)
		limit := maxError / 4
		if angular {
			limit /= math.Nextafter(2*math.Pi*r.OuterRadius, math.Inf(1))
		}
		if delta.Cmp(ratInset(limit)) > 0 {
			return 0, nil, geom.ErrUnrepresentable
		}
		return v, delta, nil
	}
	rounder := func(q *big.Rat, angular bool) (float64, error) {
		v, delta, e := roundValue(q, angular, big.NewRat(1, 1))
		if e != nil {
			return 0, e
		}
		if angular {
			if delta.Cmp(turnError) > 0 {
				turnError.Set(delta)
			}
			recordTurns()
		} else {
			proof.MaxAxialError = max(proof.MaxAxialError, upward(delta))
		}
		return v, nil
	}
	roundTwist := func(q *big.Rat) (float64, error) {
		v, delta, e := roundValue(q, true, ratInset(r.Height))
		if e != nil {
			return 0, e
		}
		twistError.Set(delta)
		recordTurns()
		return v, nil
	}

	stockRadius, outer := r.OuterRadius, 0.
	if r.BoreRadius > 0 {
		stockRadius, outer = r.BoreRadius, r.OuterRadius
	}
	s, e := grooveCylinderStarts(ctx, r.Frame, stockRadius, outer, r.Height, r.Profile, r.Lead, r.Start, r.End, r.Hand, r.Starts, phase, rounder, roundTwist, validateDisplay)
	return s, proof, e
}
