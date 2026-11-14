package geom

import (
	"context"
	"math/big"
)

// ProjectedGraphCusp is immutable evidence of one complete finite cusp section
// on the two original supports. The signed cusp parameter remains in normalized
// source-chart units; target UV is obtained through the exact affine map.
// No transverse tube or trimmed material topology is implied.
type ProjectedGraphCusp struct {
	sources                    [2]*SurfacePatch
	domains                    [2]ParameterRectangle
	uv                         [2][3][2]*big.Rat // cusp, negative-parameter terminal, positive terminal
	points                     [3][3]*big.Rat
	mapping                    [2][3]*big.Rat
	axes                       [2]int
	residual, side, operations int
	tau                        *big.Rat
}

func (c *ProjectedGraphCusp) valid() bool {
	return c != nil && c.sources[0] != nil && c.sources[1] != nil && c.tau != nil
}
func (c *ProjectedGraphCusp) Binds(a, b *SurfacePatch) bool {
	return c != nil && c.sources[0] == a && c.sources[1] == b && a != nil && b != nil
}
func (c *ProjectedGraphCusp) ExactOperations() int {
	if !c.valid() {
		return 0
	}
	return c.operations
}
func (c *ProjectedGraphCusp) Domain(owner int) (ParameterRectangle, error) {
	if !c.valid() || owner < 0 || owner > 1 {
		return ParameterRectangle{}, ErrInvalid
	}
	return c.domains[owner], nil
}

// Parameters returns cusp then negative/positive parameter terminal UVs.
func (c *ProjectedGraphCusp) Parameters(owner int) (out [3][2]*big.Rat, err error) {
	if !c.valid() || owner < 0 || owner > 1 {
		return out, ErrInvalid
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(c.uv[owner][i][j])
		}
	}
	return out, nil
}
func (c *ProjectedGraphCusp) Points() (out [3][3]*big.Rat, err error) {
	if !c.valid() {
		return out, ErrInvalid
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(c.points[i][j])
		}
	}
	return out, nil
}
func (c *ProjectedGraphCusp) TargetMap() (out [2][3]*big.Rat, err error) {
	if !c.valid() {
		return out, ErrInvalid
	}
	for i := range out {
		for j := range out[i] {
			out[i][j] = new(big.Rat).Set(c.mapping[i][j])
		}
	}
	return out, nil
}
func (c *ProjectedGraphCusp) ParameterRange() (lo, hi *big.Rat, err error) {
	if !c.valid() {
		return nil, nil, ErrInvalid
	}
	return new(big.Rat).Neg(c.tau), new(big.Rat).Set(c.tau), nil
}

// Residual returns projection axes, remaining model axis, and the sign of the
// coefficient multiplying the canonical cusp equation in A-B's residual.
func (c *ProjectedGraphCusp) Residual() (axes [2]int, axis, side int, err error) {
	if !c.valid() {
		return axes, 0, 0, ErrInvalid
	}
	return c.axes, c.residual, c.side, nil
}

// CertifyProjectedGraphSemicubicalCusp re-proves complete matching projected
// rectangles, independent signed/swapped/scaled original charts, and the full
// cusp identity. Center and k,tau have CertifyGraphSemicubicalCusp semantics
// in A's normalized chart. Partial overlap, shear, nonrepresentable remapped
// knots and other terminal regimes are rejected. Failure returns nil.
func CertifyProjectedGraphSemicubicalCusp(ctx context.Context, a, c *SurfacePatch, axes [2]int, r, s, k, tau *big.Rat, o GraphPointContactOptions) (*ProjectedGraphCusp, error) {
	if ctx == nil || a == nil || c == nil || r == nil || s == nil || k == nil || tau == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.MaxCells < 0 || o.MaxCells > 65536 || o.MaxDepth < 0 || o.MaxDepth > 128 {
		return nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, q := range []*big.Rat{r, s, k, tau} {
		if e := b.check(q); e != nil {
			return nil, e
		}
	}
	order, e := modelProjectionPermutation(axes)
	if e != nil {
		return nil, e
	}
	pa, e := permuteGraphPatch(a, order, b)
	if e != nil {
		return nil, e
	}
	pc, e := permuteGraphPatch(c, order, b)
	if e != nil {
		return nil, e
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	align, work, e := AlignAffineXYChartsWithWork(ctx, pa, pc, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += work
	if e != nil {
		return nil, e
	}
	remaining = o.MaxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	local := o
	local.MaxOperations = remaining
	proof, e := CertifyGraphSemicubicalCusp(ctx, pa, align.target, r, s, k, tau, local)
	b.operations += proof.ExactOperations
	if e != nil {
		return nil, e
	}
	out := &ProjectedGraphCusp{sources: [2]*SurfacePatch{a, c}, domains: [2]ParameterRectangle{a.Domain(), c.Domain()}, axes: axes, residual: order[2], side: proof.Side, tau: new(big.Rat).Set(tau)}
	out.uv[0] = [3][2]*big.Rat{proof.Parameters, proof.Terminals[0], proof.Terminals[1]}
	out.mapping, e = align.MapCoefficients()
	if e != nil {
		return nil, e
	}
	model := [3][3]*big.Rat{proof.Point, proof.TerminalPoints[0], proof.TerminalPoints[1]}
	for event := range out.uv[0] {
		for axis := range out.uv[1][event] {
			q := new(big.Rat).Set(out.mapping[axis][2])
			for j, v := range out.uv[0][event] {
				term := new(big.Rat).Mul(out.mapping[axis][j], v)
				if e = b.check(term); e != nil {
					return nil, e
				}
				q.Add(q, term)
				if e = b.check(q); e != nil {
					return nil, e
				}
			}
			out.uv[1][event][axis] = q
		}
		for i, j := range order {
			out.points[event][j] = model[event][i]
		}
		// Bind actual original target evaluation, independently of aligned storage.
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return nil, ErrConvergence
		}
		jet, e := c.EvaluateRational(ctx, out.uv[1][event][0], out.uv[1][event][1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return nil, e
		}
		for j := range jet.Point {
			if jet.Point[j].Cmp(out.points[event][j]) != 0 {
				return nil, ErrConvergence
			}
		}
	}
	out.operations = b.operations
	return out, nil
}
