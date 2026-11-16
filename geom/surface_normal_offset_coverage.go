package geom

import (
	"context"
	"errors"
	"math/big"
)

// NormalOffsetCoverageOptions bounds proof of regularity along the
// displacement at each source parameter. Zero uses 16 partition levels
// and 4,096 visited cells with the default exact patch work limits.
type NormalOffsetCoverageOptions struct {
	Patch              SurfacePatchOptions
	MaxDepth, MaxCells int
}

// NormalOffsetCoverage partitions the original parameter domain. Proven
// cells each include their boundary from their own side; unresolved cells
// may contain singularities or simply require tighter bounds.
type NormalOffsetCoverage struct {
	Proven, Unresolved []ParameterRectangle
	Complete           bool
	ExactOperations    int
}

// CertifyNormalOffsetCoverage proves a nonsingular differential for
// S + t*n, for every parameter in a patch and every t between zero and
// distance. Each original knot piece is treated independently. It returns
// ErrConvergence with an explicit remaining cover if the available bounds
// cannot prove the entire patch. It does not certify continuity across a
// crease, injectivity, collision freedom, or topology of the final offset.
func (p *SurfacePatch) CertifyNormalOffsetCoverage(ctx context.Context, distance *big.Rat,
	o NormalOffsetCoverageOptions) (result NormalOffsetCoverage, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || distance == nil || o.MaxDepth < 0 || o.MaxDepth > 64 ||
		o.MaxCells < 0 || o.MaxCells > 1<<18 {
		return result, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 16
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	limits, err := patchOptions(o.Patch)
	if err != nil {
		return result, err
	}
	if len(p.net) > limits.MaxPoles || len(p.net) > limits.MaxOperations/4 {
		result.Unresolved = []ParameterRectangle{p.Domain()}
		return result, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	root := *p
	root.options.MaxPoles = min(root.options.MaxPoles, limits.MaxPoles)
	defer func() { result.ExactOperations = b.operations }()
	type cell struct {
		patch *SurfacePatch
		depth int
	}
	remaining := []cell{{&root, 0}}
	defer func() {
		// Include every unprocessed cell, including the current one if
		// exact arithmetic stopped before any atomic subdivision.
		for i := len(remaining) - 1; i >= 0; i-- {
			result.Unresolved = append(result.Unresolved, remaining[i].patch.Domain())
		}
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := b.check(distance); err != nil {
		return result, err
	}
	for visited := 0; len(remaining) > 0; visited++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if visited == o.MaxCells {
			return result, ErrConvergence
		}
		i := len(remaining) - 1
		c := remaining[i]
		t, direction, knot := patchSplitChoice(c.patch)
		if !knot {
			ok, err := offsetCellFocalFree(ctx, c.patch, distance, b)
			if err != nil {
				return result, err
			}
			if ok {
				result.Proven = append(result.Proven, c.patch.Domain())
				remaining = remaining[:i]
				continue
			}
			// If the wide direction cannot be split into distinct float
			// parameters, try the other direction before giving up.
			d := c.patch.Domain()
			if direction && !(d.UMin < t && t < d.UMax) {
				t, direction = d.VMin/2+d.VMax/2, false
			} else if !direction && !(d.VMin < t && t < d.VMax) {
				t, direction = d.UMin/2+d.UMax/2, true
			}
			if (direction && !(d.UMin < t && t < d.UMax)) || (!direction && !(d.VMin < t && t < d.VMax)) {
				result.Unresolved = append(result.Unresolved, d)
				remaining = remaining[:i]
				continue
			}
		}
		if c.depth == o.MaxDepth {
			result.Unresolved = append(result.Unresolved, c.patch.Domain())
			remaining = remaining[:i]
			continue
		}
		left, right, err := c.patch.split(t, direction, b)
		if err != nil {
			return result, err
		}
		remaining = append(remaining[:i], cell{right, c.depth + 1}, cell{left, c.depth + 1})
	}
	if len(result.Unresolved) != 0 {
		return result, ErrConvergence
	}
	result.Complete = true
	return result, nil
}

func offsetCellFocalFree(ctx context.Context, p *SurfacePatch, distance *big.Rat, b *polynomialBudget) (bool, error) {
	h, err := offsetHomogeneousRanges(ctx, p, b)
	if err != nil {
		return false, err
	}
	a := offsetIntervalArithmetic{b: b}
	w := h[0][0][3]
	if w.lo.Sign() <= 0 {
		return false, ErrInvalid
	}
	var d [3][3][3]exactInterval
	for total := 0; total <= 2; total++ {
		for i := 0; i <= total; i++ {
			j := total - i
			for axis := range 3 {
				value := h[i][j][axis]
				for u := 0; u <= i; u++ {
					for v := 0; v <= j; v++ {
						if u+v == 0 {
							continue
						}
						term := a.mul(h[u][v][3], d[i-u][j-v][axis])
						if i == 2 && u == 1 || j == 2 && v == 1 {
							term = a.add(term, term)
						}
						value = a.sub(value, term)
					}
				}
				d[i][j][axis] = a.div(value, w)
			}
		}
	}
	du, dv := d[1][0], d[0][1]
	var normal [3]exactInterval
	for i := range normal {
		j, k := (i+1)%3, (i+2)%3
		normal[i] = a.sub(a.mul(du[j], dv[k]), a.mul(du[k], dv[j]))
	}
	dot := func(x, y [3]exactInterval) exactInterval {
		return a.add(a.add(a.mul(x[0], y[0]), a.mul(x[1], y[1])), a.mul(x[2], y[2]))
	}
	norm2 := func(v [3]exactInterval) exactInterval {
		return a.add(a.add(a.square(v[0]), a.square(v[1])), a.square(v[2]))
	}
	area2 := norm2(normal)
	if a.err != nil || area2.lo.Sign() <= 0 {
		return false, a.err
	}
	e, f, g := norm2(du), dot(du, dv), norm2(dv)
	l, m, n := dot(d[2][0], normal), dot(d[1][1], normal), dot(d[0][2], normal)
	displacement, zero := exactInterval{distance, distance}, offsetZeroInterval()
	// A lower bound for E*sqrt(area2) - distance*L.
	first := a.sub(zero, a.mul(displacement, l))
	scalars := boundedRatArithmetic{b: b}
	if a.err != nil || scalars.radicalSign(first.lo, e.lo, area2.lo) <= 0 || scalars.err != nil {
		return false, errors.Join(a.err, scalars.err)
	}
	fm := a.mul(f, m)
	trace := a.sub(a.add(a.mul(g, l), a.mul(e, n)), a.add(fm, fm))
	det := a.sub(a.mul(l, n), a.square(m))
	x := a.add(a.square(area2), a.mul(a.square(displacement), det))
	y := a.sub(zero, a.mul(displacement, trace))
	if a.err != nil {
		return false, a.err
	}
	squared := area2.lo
	if y.lo.Sign() < 0 {
		squared = area2.hi
	}
	ok := scalars.radicalSign(x.lo, y.lo, squared) > 0
	return ok, scalars.err
}

func offsetHomogeneousRanges(ctx context.Context, p *SurfacePatch, b *polynomialBudget) (h [3][3][4]exactInterval, err error) {
	domain := p.Domain()
	controls := make([]*big.Rat, len(p.net))
	for axis := range 4 {
		if err := ctx.Err(); err != nil {
			return h, err
		}
		for i, point := range p.net {
			controls[i] = point[axis]
			if err := b.check(controls[i]); err != nil {
				return h, err
			}
		}
		h[0][0][axis] = coefficientRange(controls)
		u, err := scalarPartial(controls, p.countU, p.countV, true, domain, b)
		if err != nil {
			return h, err
		}
		v, err := scalarPartial(controls, p.countU, p.countV, false, domain, b)
		if err != nil {
			return h, err
		}
		h[1][0][axis], h[0][1][axis] = coefficientRange(u), coefficientRange(v)
		h[2][0][axis], h[0][2][axis] = offsetZeroInterval(), offsetZeroInterval()
		if p.degreeU > 1 {
			h[2][0][axis], err = scalarPartialRange(u, p.countU-1, p.countV, true, domain, b)
			if err != nil {
				return h, err
			}
		}
		if p.degreeV > 1 {
			h[0][2][axis], err = scalarPartialRange(v, p.countU, p.countV-1, false, domain, b)
			if err != nil {
				return h, err
			}
		}
		h[1][1][axis], err = scalarPartialRange(u, p.countU-1, p.countV, false, domain, b)
		if err != nil {
			return h, err
		}
	}
	return h, nil
}

func offsetZeroInterval() exactInterval {
	z := new(big.Rat)
	return exactInterval{z, z}
}

type offsetIntervalArithmetic struct {
	b   *polynomialBudget
	err error
}

func (a *offsetIntervalArithmetic) apply(op func(exactInterval, exactInterval, *polynomialBudget) (exactInterval, error),
	x, y exactInterval) exactInterval {
	if a.err != nil {
		return offsetZeroInterval()
	}
	var out exactInterval
	out, a.err = op(x, y, a.b)
	if a.err != nil {
		return offsetZeroInterval()
	}
	return out
}

func (a *offsetIntervalArithmetic) add(x, y exactInterval) exactInterval {
	return a.apply(func(x, y exactInterval, b *polynomialBudget) (exactInterval, error) {
		return boundedExactInterval(new(big.Rat).Add(x.lo, y.lo), new(big.Rat).Add(x.hi, y.hi), b)
	}, x, y)
}

func (a *offsetIntervalArithmetic) sub(x, y exactInterval) exactInterval {
	return a.apply(intervalSubtract, x, y)
}
func (a *offsetIntervalArithmetic) mul(x, y exactInterval) exactInterval {
	return a.apply(intervalMultiply, x, y)
}
func (a *offsetIntervalArithmetic) div(x, y exactInterval) exactInterval {
	return a.apply(intervalDivide, x, y)
}

func (a *offsetIntervalArithmetic) square(x exactInterval) exactInterval {
	return a.apply(func(x, _ exactInterval, b *polynomialBudget) (exactInterval, error) {
		lo, hi := new(big.Rat).Mul(x.lo, x.lo), new(big.Rat).Mul(x.hi, x.hi)
		if hi.Cmp(lo) < 0 {
			lo, hi = hi, lo
		}
		if x.lo.Sign() <= 0 && x.hi.Sign() >= 0 {
			lo.SetInt64(0)
		}
		return boundedExactInterval(lo, hi, b)
	}, x, x)
}
