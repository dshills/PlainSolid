package geom

import (
	"context"
	"math/big"
)

// ProjectiveSurfaceGraphOverlap describes the complete positive-area overlap
// of two proved Cartesian graph projections. Areas are the exact original
// UV boxes. BStationRatios and the reversals act on those normalized boxes in
// the same order as ExactSurfaceComparisonOptions. The orientation describes
// the A-to-B chart map. NoArea leaves boundary contact undecided.
type ProjectiveSurfaceGraphOverlap struct {
	Coincident, NoArea             bool
	Axes                           [2]int
	Projection                     [2]ExactParameterInterval
	Areas                          [2]ExactSurfaceRectangle
	SwapB, ReverseBU, ReverseBV    bool
	BStationRatios                 [2]*big.Rat
	Orientation                    int
	ProjectionOrientation          [2]int
	ComparedCells, ExactOperations int
}

type projectiveGraphAxis struct {
	raw   int
	ends  [2]*big.Rat
	ratio *big.Rat
}

// OriginalUVMap converts a proved overlapping chart map to physical original
// parameters. The returned map can transform an exact UV curve without
// changing its clock. Its denominators are positive throughout source Areas[0].
func (r ProjectiveSurfaceGraphOverlap) OriginalUVMap(o SurfacePatchOptions) (ProjectiveUVMap, error) {
	m, _, err := r.OriginalUVMapWithWork(o)
	return m, err
}

// OriginalUVMapWithWork also reports the exact arithmetic work performed.
func (r ProjectiveSurfaceGraphOverlap) OriginalUVMapWithWork(o SurfacePatchOptions) (out ProjectiveUVMap, operations int, err error) {
	if !r.Coincident || r.NoArea {
		return out, 0, ErrDomain
	}
	limits, err := patchOptions(o)
	if err != nil {
		return out, 0, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	a := boundedRatArithmetic{b: b}
	var boxes [2][2]ExactParameterInterval
	for owner, p := range r.Areas {
		boxes[owner] = [2]ExactParameterInterval{p.U, p.V}
		for _, x := range boxes[owner] {
			if x.Lower == nil || x.Upper == nil || x.Lower.Cmp(x.Upper) >= 0 {
				return ProjectiveUVMap{}, 0, ErrInvalid
			}
			for _, q := range [2]*big.Rat{x.Lower, x.Upper} {
				if err := b.check(q); err != nil {
					return ProjectiveUVMap{}, 0, err
				}
			}
		}
	}
	for from, flip := range [2]bool{r.ReverseBU, r.ReverseBV} {
		to := from
		if r.SwapB {
			to = 1 - from
		}
		lo, hi := boxes[0][from], boxes[1][to]
		ratio := r.BStationRatios[from]
		if ratio == nil || ratio.Sign() <= 0 {
			return ProjectiveUVMap{}, 0, ErrInvalid
		}
		if err := b.check(ratio); err != nil {
			return ProjectiveUVMap{}, 0, err
		}
		// Scale numerator and denominator by the width of the original
		// source domain so both are affine in its raw physical coordinate.
		oldWidth := a.sub(lo.Upper, lo.Lower)
		newWidth := a.sub(hi.Upper, hi.Lower)
		k := a.sub(ratio, big.NewRat(1, 1))
		den := [2]*big.Rat{k, a.sub(oldWidth, a.mul(k, lo.Lower))}
		num := [2]*big.Rat{ratio, a.sub(new(big.Rat), a.mul(ratio, lo.Lower))}
		for i := range num {
			if flip {
				num[i] = a.sub(den[i], num[i])
			}
			num[i] = a.add(a.mul(hi.Lower, den[i]), a.mul(newWidth, num[i]))
		}
		if a.err != nil {
			return ProjectiveUVMap{}, 0, a.err
		}
		out[to] = ProjectiveUVAxisMap{from, num, den}
	}
	return out, 0, nil
}

// CertifyProjectiveSurfaceGraphOverlap discovers signed-axis correspondence
// when each selected Cartesian coordinate is a nonconstant linear-fractional
// function of exactly one original normalized parameter. It proves the full
// projections and compares the lifted surfaces on their exact projected
// overlap. Nonidentity leaves the intersection unresolved.
func CertifyProjectiveSurfaceGraphOverlap(ctx context.Context, source, target *SurfacePatch,
	axes [2]int, o SurfacePatchOptions) (r ProjectiveSurfaceGraphOverlap, err error) {
	if ctx == nil || source == nil || target == nil || len(source.net) == 0 || len(target.net) == 0 ||
		axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	limits, err := patchOptions(o)
	if err != nil {
		return r, err
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	var charts [2][2]projectiveGraphAxis
	var domains [2][2]exactInterval
	for i, p := range [2]*SurfacePatch{source, target} {
		charts[i], err = certifyProjectiveGraphProjection(ctx, p, axes, limits.MaxPoles, budget)
		if err != nil {
			return r, err
		}
		domains[i] = projectiveParameterIntervals(p.Domain())
	}
	r.Axes = axes
	for owner, chart := range charts {
		sign := chart[0].ends[1].Cmp(chart[0].ends[0]) * chart[1].ends[1].Cmp(chart[1].ends[0])
		if chart[0].raw == 1 {
			sign = -sign
		}
		r.ProjectionOrientation[owner] = sign
	}
	r.Orientation = r.ProjectionOrientation[0] * r.ProjectionOrientation[1]
	r.SwapB = charts[0][0].raw != charts[1][0].raw
	a := boundedRatArithmetic{b: budget}
	one := big.NewRat(1, 1)
	var areas [2][2]ExactParameterInterval
	var reversed [2]bool
	for world := range axes {
		x, y := charts[0][world], charts[1][world]
		lo, hi := orderedProjectiveEnds(x.ends)
		otherLo, otherHi := orderedProjectiveEnds(y.ends)
		if otherLo.Cmp(lo) > 0 {
			lo = otherLo
		}
		if otherHi.Cmp(hi) < 0 {
			hi = otherHi
		}
		if lo.Cmp(hi) >= 0 {
			r.NoArea = true
			return r, nil
		}
		r.Projection[world] = ExactParameterInterval{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
		var local [2]*big.Rat
		for owner, axis := range [2]projectiveGraphAxis{x, y} {
			w := a.sub(axis.ends[1], axis.ends[0])
			k := a.sub(axis.ratio, one)
			var s [2]*big.Rat
			for i, end := range [2]*big.Rat{lo, hi} {
				z := a.quo(a.sub(end, axis.ends[0]), w)
				s[i] = a.quo(z, a.sub(axis.ratio, a.mul(k, z)))
			}
			s0, s1 := orderedProjectiveEnds(s)
			local[owner] = a.quo(a.add(one, a.mul(k, s1)), a.add(one, a.mul(k, s0)))
			d := domains[owner][axis.raw]
			rawWidth := a.sub(d.hi, d.lo)
			areas[owner][axis.raw] = ExactParameterInterval{a.add(d.lo, a.mul(rawWidth, s0)), a.add(d.lo, a.mul(rawWidth, s1))}
		}
		reversed[x.raw] = x.ends[1].Cmp(x.ends[0]) != y.ends[1].Cmp(y.ends[0])
		if reversed[x.raw] {
			// Reversing B replaces its local fractional ratio by its
			// reciprocal before matching the two normalized world stations.
			r.BStationRatios[x.raw] = a.mul(local[0], local[1])
		} else {
			r.BStationRatios[x.raw] = a.quo(local[0], local[1])
		}
		if a.err != nil {
			return r, a.err
		}
	}
	r.ReverseBU, r.ReverseBV = reversed[0], reversed[1]
	r.Areas = [2]ExactSurfaceRectangle{{areas[0][0], areas[0][1]}, {areas[1][0], areas[1][1]}}
	if budget.operations >= budget.maxOperations {
		return r, ErrConvergence
	}
	proof, err := CompareExactSurfaceRectangles(ctx, source, target, r.Areas, ExactSurfaceComparisonOptions{
		SwapB: r.SwapB, ReverseBU: r.ReverseBU, ReverseBV: r.ReverseBV, BStationRatios: r.BStationRatios,
		MaxOperations: budget.maxOperations - budget.operations, MaxCoefficientBits: budget.maxBits})
	budget.operations += proof.ExactOperations
	r.ComparedCells = proof.ComparedCells
	if err != nil {
		return r, err
	}
	if !proof.Complete || !proof.Equal {
		return r, ErrConvergence
	}
	r.Coincident = true
	return r, nil
}

func orderedProjectiveEnds(ends [2]*big.Rat) (*big.Rat, *big.Rat) {
	if ends[0].Cmp(ends[1]) > 0 {
		return ends[1], ends[0]
	}
	return ends[0], ends[1]
}

func projectiveParameterIntervals(d ParameterRectangle) [2]exactInterval {
	return [2]exactInterval{
		{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)},
		{new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)},
	}
}

func certifyProjectiveGraphProjection(ctx context.Context, p *SurfacePatch, axes [2]int, maxPoles int,
	b *polynomialBudget) (out [2]projectiveGraphAxis, err error) {
	if len(p.net) > maxPoles {
		return out, ErrConvergence
	}
	d := projectiveParameterIntervals(p.Domain())
	var m [2]*big.Rat
	for i, span := range d {
		m[i] = new(big.Rat).Quo(new(big.Rat).Add(span.lo, span.hi), big.NewRat(2, 1))
	}
	sample := func(uv [2]*big.Rat) ([3]*big.Rat, error) {
		if b.operations >= b.maxOperations {
			return [3]*big.Rat{}, ErrConvergence
		}
		jet, err := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{
			MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
		b.operations += jet.ExactOperations
		return jet.Point, err
	}
	middle, err := sample(m)
	if err != nil {
		return out, err
	}
	var end [2][2][3]*big.Rat
	for i := range end {
		for j, value := range [2]*big.Rat{d[i].lo, d[i].hi} {
			uv := m
			uv[i] = value
			end[i][j], err = sample(uv)
			if err != nil {
				return out, err
			}
		}
	}
	a := boundedRatArithmetic{b: b}
	for i, axis := range axes {
		var raw int
		matches := 0
		for j := range end {
			if end[j][0][axis].Cmp(end[j][1][axis]) != 0 {
				raw = j
				matches++
			}
		}
		if matches != 1 {
			return out, ErrDomain
		}
		lo, hi := end[raw][0][axis], end[raw][1][axis]
		fromLo, toHi := a.sub(middle[axis], lo), a.sub(hi, middle[axis])
		if a.err != nil {
			return out, a.err
		}
		if fromLo.Sign()*toHi.Sign() != 1 {
			return out, ErrDomain
		}
		out[i] = projectiveGraphAxis{raw, [2]*big.Rat{lo, hi}, a.quo(fromLo, toHi)}
	}
	if a.err != nil {
		return out, a.err
	}
	if out[0].raw == out[1].raw {
		return out, ErrDomain
	}
	// The three samples only propose a map. Compare the projection of
	// every original homogeneous control piece against the proposed
	// parameterization of a plane, including all original knots.
	zero, one := new(big.Rat), big.NewRat(1, 1)
	actual := *p
	actual.options.MaxPoles = min(p.options.MaxPoles, maxPoles)
	actual.net = make([]exactHomogeneous, len(p.net))
	for i, h := range p.net {
		actual.net[i] = exactHomogeneous{h[axes[0]], h[axes[1]], zero, h[3]}
	}
	if err := actual.computeBounds(b); err != nil {
		return out, err
	}
	expected := SurfacePatch{degreeU: 1, degreeV: 1, countU: 2, countV: 2,
		knotsU: []float64{0, 0, 1, 1}, knotsV: []float64{0, 0, 1, 1}, options: actual.options}
	for u := range 2 {
		for v := range 2 {
			station := [2]int{u, v}
			expected.net = append(expected.net, exactHomogeneous{out[0].ends[station[out[0].raw]], out[1].ends[station[out[1].raw]], zero, one})
		}
	}
	if err := expected.computeBounds(b); err != nil {
		return out, err
	}
	if b.operations >= b.maxOperations {
		return out, ErrConvergence
	}
	var ratios [2]*big.Rat
	for _, x := range out {
		ratios[x.raw] = x.ratio
	}
	proof, err := CompareExactSurfaces(ctx, &actual, &expected, ExactSurfaceComparisonOptions{NormalizeDomains: true,
		BStationRatios: ratios, MaxOperations: b.maxOperations - b.operations, MaxCoefficientBits: b.maxBits})
	b.operations += proof.ExactOperations
	if err != nil {
		return out, err
	}
	if !proof.Complete || !proof.Equal {
		return out, ErrDomain
	}
	return out, nil
}
