package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// RationalAreaInterval contains the exact real quantity in [Lower,Upper].
// Endpoints are caller-owned rationals; no quadrature estimate is substituted.
type RationalAreaInterval struct{ Lower, Upper *big.Rat }

// AffineQuadraticTrimLoopArea distinguishes the source UV area enclosure from
// the exact chord polygon UV area. ModelAreaVector is the signed area vector
// of the source boundary (one half of its line integral r×dr), in model units.
// Exact affine chart regularity and a simple CCW source loop make this also
// the oriented planar fill area vector. This is NOT a claim that the chord
// polygon and the source curve enclose the same region.
type AffineQuadraticTrimLoopArea struct {
	Boundary        CertifiedQuadraticTrimLoop
	SourceUVArea    RationalAreaInterval
	PolygonUVArea   *big.Rat
	ModelAreaVector [3]RationalAreaInterval
}

// BoundAffineQuadraticTrimLoopArea constructs the certified chord loop, proves
// the support is a regular affine chart, then bounds the exact Green integral
// over each original source interval. The positive homogeneous denominator and
// Bernstein numerator hull give rigorous real-arithmetic area intervals.
// Tessellation options control partition size, not a requested area accuracy.
// Refining the boundary generally tightens the enclosure; no sampled quadrature
// or equality of source and polygon fills is assumed. Currently one CCW loop.
func BoundAffineQuadraticTrimLoopArea(ctx context.Context, patch *geom.SurfacePatch, trims []SplineUV, o QuadraticTrimLoopOptions) (AffineQuadraticTrimLoopArea, error) {
	empty := AffineQuadraticTrimLoopArea{}
	if ctx == nil || patch == nil {
		return empty, geom.ErrInvalid
	}
	_, e := geom.NewAffineSurfaceChart(ctx, patch, geom.AffineSurfaceChartOptions{})
	if e != nil {
		return empty, e
	}
	boundary, e := TessellateQuadraticSplineTrimLoopCertified(ctx, patch, trims, o)
	if e != nil {
		return empty, e
	}
	area := RationalAreaInterval{new(big.Rat), new(big.Rat)}
	domain := patch.Domain()
	origin := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Add(ratInset(domain.UMin), ratInset(domain.UMax)), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Add(ratInset(domain.VMin), ratInset(domain.VMax)), big.NewRat(2, 1))}

	for _, segment := range boundary.Segments {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		part := trims[segment.Source]
		part.Reversed = segment.Start > segment.End
		part.Range = UVInterval{Min: segment.Start, Max: segment.End}
		if part.Reversed {
			part.Range.Min, part.Range.Max = part.Range.Max, part.Range.Min
		}
		h, e := quadraticTrimSourceControls(part)
		if e != nil {
			return empty, e
		}
		// A common translation changes each open integral by an endpoint
		// term whose sum is exactly zero around the proved closed loop.
		for i := range h {
			for axis := 0; axis < 2; axis++ {
				h[i][axis].Sub(h[i][axis], new(big.Rat).Mul(origin[axis], h[i][2]))
			}
		}
		interval := quadraticGreenInterval(h)
		area.Lower.Add(area.Lower, interval.Lower)
		area.Upper.Add(area.Upper, interval.Upper)
	}
	// The independently certified CCW source bounds a region of positive area.
	if area.Lower.Sign() < 0 {
		area.Lower.SetInt64(0)
	}
	polygon := new(big.Rat)
	for i, a := range boundary.ExactUV {
		b := boundary.ExactUV[(i+1)%len(boundary.ExactUV)]
		polygon.Add(polygon, new(big.Rat).Sub(new(big.Rat).Mul(a[0], b[1]), new(big.Rat).Mul(a[1], b[0])))
	}
	polygon.Quo(polygon, big.NewRat(2, 1))
	d := patch.Domain()
	p, e := patch.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMin), o.Tessellation.Evaluation)
	if e != nil {
		return empty, e
	}
	u, e := patch.EvaluateRational(ctx, ratInset(d.UMax), ratInset(d.VMin), o.Tessellation.Evaluation)
	if e != nil {
		return empty, e
	}
	v, e := patch.EvaluateRational(ctx, ratInset(d.UMin), ratInset(d.VMax), o.Tessellation.Evaluation)
	if e != nil {
		return empty, e
	}
	var du, dv [3]*big.Rat
	for k := 0; k < 3; k++ {
		du[k] = new(big.Rat).Quo(new(big.Rat).Sub(u.Point[k], p.Point[k]), new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)))
		dv[k] = new(big.Rat).Quo(new(big.Rat).Sub(v.Point[k], p.Point[k]), new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))
	}
	result := AffineQuadraticTrimLoopArea{Boundary: boundary, SourceUVArea: area, PolygonUVArea: polygon}
	for k := 0; k < 3; k++ {
		a, b := (k+1)%3, (k+2)%3
		normal := new(big.Rat).Sub(new(big.Rat).Mul(du[a], dv[b]), new(big.Rat).Mul(du[b], dv[a]))
		lo, hi := area.Lower, area.Upper
		if normal.Sign() < 0 {
			lo, hi = hi, lo
		}
		result.ModelAreaVector[k] = RationalAreaInterval{new(big.Rat).Mul(normal, lo), new(big.Rat).Mul(normal, hi)}
	}
	return result, nil
}

// Green's numerator is Hu*Hv'-Hv*Hu'; W' cancels identically. The
// Bernstein convex hull encloses its cubic polynomial on [0,1], while
// positive control weights enclose W². Corner division bounds all signs.
func quadraticGreenInterval(h [3][3]*big.Rat) RationalAreaInterval {
	var power [2][3]*big.Rat
	for k := 0; k < 2; k++ {
		power[k][0] = h[0][k]
		power[k][1] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(h[1][k], h[0][k]))
		power[k][2] = new(big.Rat).Add(new(big.Rat).Sub(h[0][k], new(big.Rat).Mul(big.NewRat(2, 1), h[1][k])), h[2][k])
	}
	numerator := [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}
	for i := 0; i < 3; i++ {
		for j := 1; j < 3; j++ {
			term := new(big.Rat).Sub(new(big.Rat).Mul(power[0][i], power[1][j]), new(big.Rat).Mul(power[1][i], power[0][j]))
			term.Mul(term, big.NewRat(int64(j), 1))
			numerator[i+j-1].Add(numerator[i+j-1], term)
		}
	}
	choose := func(n, k int) int64 {
		r := int64(1)
		for j := 1; j <= k; j++ {
			r = r * int64(n-j+1) / int64(j)
		}
		return r
	}
	var lo, hi *big.Rat
	for i := 0; i < 4; i++ {
		c := new(big.Rat)
		for j := 0; j <= i; j++ {
			c.Add(c, new(big.Rat).Mul(numerator[j], big.NewRat(choose(i, j), choose(3, j))))
		}
		if lo == nil || c.Cmp(lo) < 0 {
			lo = new(big.Rat).Set(c)
		}
		if hi == nil || c.Cmp(hi) > 0 {
			hi = new(big.Rat).Set(c)
		}
	}
	minW, maxW := h[0][2], h[0][2]
	for i := 1; i < 3; i++ {
		if h[i][2].Cmp(minW) < 0 {
			minW = h[i][2]
		}
		if h[i][2].Cmp(maxW) > 0 {
			maxW = h[i][2]
		}
	}
	minD, maxD := new(big.Rat).Mul(minW, minW), new(big.Rat).Mul(maxW, maxW)
	result := RationalAreaInterval{}
	for _, n := range []*big.Rat{lo, hi} {
		for _, d := range []*big.Rat{minD, maxD} {
			q := new(big.Rat).Quo(n, d)
			q.Quo(q, big.NewRat(2, 1))
			if result.Lower == nil || q.Cmp(result.Lower) < 0 {
				result.Lower = new(big.Rat).Set(q)
			}
			if result.Upper == nil || q.Cmp(result.Upper) > 0 {
				result.Upper = new(big.Rat).Set(q)
			}
		}
	}
	return result
}
