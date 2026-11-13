package geom

import (
	"context"
	"math/big"
)

// TubeCornerEndpoint retains two simultaneous original chart sides at one
// exact corner. The other original chart is strictly interior. This proves
// endpoint ownership/local tube continuation, not a shell vertex fan.
type TubeCornerEndpoint struct {
	tube    VerifiedSurfaceTube
	domains [2]ParameterRectangle
	chart   int
	sides   [2]SurfaceEdge
	upper   bool
	proof   TubeEndpointCertificate
}

// Matches requires the original immutable tube proof, including its anchor.
func (c *TubeCornerEndpoint) Matches(t VerifiedSurfaceTube) bool {
	return c != nil && c.tube.a == t.a && c.tube.b == t.b && c.tube.anchor == t.anchor && c.tube.axis == t.axis && c.tube.tangentSign == t.tangentSign
}
func (c *TubeCornerEndpoint) Chart() int                     { return c.chart }
func (c *TubeCornerEndpoint) Sides() [2]SurfaceEdge          { return c.sides }
func (c *TubeCornerEndpoint) Domains() [2]ParameterRectangle { return c.domains }
func (c *TubeCornerEndpoint) Parameters() [4]*big.Rat {
	var r [4]*big.Rat
	for i := range r {
		r[i] = new(big.Rat).Set(c.proof.Parameters[i])
	}
	return r
}
func (c *TubeCornerEndpoint) Point() [3]*big.Rat {
	var r [3]*big.Rat
	for i := range r {
		r[i] = new(big.Rat).Set(c.proof.Point[i])
	}
	return r
}

// CertifyCornerEndpoint requires an existing immutable boundary anchor. The
// original domains must contain both tube boxes. The selected endpoint must
// meet two sides of one original chart and neither side of the other chart.
// Existing anchored-tube proofs already establish regular inward continuation
// through the whole free interval; no sampled corner or widened box is used.
func (t VerifiedSurfaceTube) CertifyCornerEndpoint(ctx context.Context, upper bool, chart int, domains [2]ParameterRectangle, o TubeEndpointOptions) (*TubeCornerEndpoint, error) {
	if ctx == nil || t.a == nil || t.b == nil || chart < 0 || chart > 1 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if t.anchor == nil {
		return nil, ErrConvergence
	}
	ranges := pairParameterRanges(SurfacePairParameterBox{t.a.Domain(), t.b.Domain()})
	end := 0
	if upper {
		end = 1
	}
	if t.anchor.free != ranges[t.axis][end] {
		return nil, ErrDomain
	}
	out := &TubeCornerEndpoint{tube: t, domains: domains, chart: chart, upper: upper}
	for i, d := range domains {
		if !d.Valid() {
			return nil, ErrInvalid
		}
		lows, highs := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
		for axis := 0; axis < 2; axis++ {
			k := 2*i + axis
			if ranges[k][0] < lows[axis] || ranges[k][1] > highs[axis] {
				return nil, ErrDomain
			}
			q := t.anchor.parameters[k]
			lo, hi := q.Cmp(new(big.Rat).SetFloat64(lows[axis])), q.Cmp(new(big.Rat).SetFloat64(highs[axis]))
			if i == chart {
				if lo != 0 && hi != 0 {
					return nil, ErrDomain
				}
				out.sides[axis] = SurfaceEdge(2 * axis)
				if hi == 0 {
					out.sides[axis]++
				}
			} else if lo <= 0 || hi >= 0 {
				return nil, ErrDomain
			}
		}
	}
	p, e := t.CertifyEndpoint(ctx, upper, t.anchor.parameters, o)
	if e != nil {
		return nil, e
	}
	if !p.Proven {
		return nil, ErrConvergence
	}
	out.proof = p
	return out, nil
}
