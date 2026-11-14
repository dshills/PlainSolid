package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// SplineCuspSection is a complete finite cusp with two regular half-germs.
// Germ 0 has t<0 and germ 1 has t>0; both run outward from the cusp to an
// original UMax terminal. It does not represent a transverse tube or solid.
type SplineCuspSection struct {
	owners [2]Patch
	exact  [2]*geom.SurfacePatch
	domain geom.ParameterRectangle
	center [2]*big.Rat
	k, tau *big.Rat
	work   int
}

// CuspSectionSample contains detached exact original UV and XYZ values.
// Germ is -1 at the cusp, otherwise 0/1. Terminal identifies the finite UMax
// endpoint, not a clipping approximation. Parameters use each original owner.
type CuspSectionSample struct {
	Parameters      [2][2]*big.Rat
	Point           [3]*big.Rat
	Germ            int
	Terminal        bool
	ExactOperations int
}

func (s *SplineCuspSection) Complete() bool {
	return s != nil && s.exact[0] != nil && s.exact[1] != nil
}
func (s *SplineCuspSection) Owner(i int) (Patch, error) {
	if !s.Complete() || i < 0 || i > 1 {
		return Patch{}, geom.ErrInvalid
	}
	return s.owners[i], nil
}
func (s *SplineCuspSection) ExactOperations() int {
	if !s.Complete() {
		return 0
	}
	return s.work
}

// ParameterRange returns the exact signed cusp parameter interval.
func (s *SplineCuspSection) ParameterRange() (lo, hi *big.Rat, err error) {
	if !s.Complete() {
		return nil, nil, geom.ErrInvalid
	}
	return new(big.Rat).Neg(s.tau), new(big.Rat).Set(s.tau), nil
}

// CertifySplinePairCuspSection re-proves the actual original finite supports.
// r,s are normalized chart coordinates; the original charts must coincide in
// UV and have a common injective XY projection. Geometry proof quotas are
// cumulative; support recovery uses the separately bounded p stages.
func CertifySplinePairCuspSection(ctx context.Context, a, b Patch, r, v, k, tau *big.Rat, p geom.SurfacePatchOptions, o geom.GraphPointContactOptions) (*SplineCuspSection, error) {
	if ctx == nil || !a.valid() || !b.valid() {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	pa, e := exactSplinePatch(a, p)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	pb, e := exactSplinePatch(b, p)
	if e != nil {
		return nil, e
	}
	proof, e := geom.CertifyGraphSemicubicalCusp(ctx, pa, pb, r, v, k, tau, o)
	if e != nil {
		return nil, e
	}
	if !proof.Proven {
		return nil, ErrUnresolved
	}
	return &SplineCuspSection{owners: [2]Patch{a, b}, exact: [2]*geom.SurfacePatch{pa, pb}, domain: proof.Domain, center: [2]*big.Rat{new(big.Rat).Set(r), new(big.Rat).Set(v)}, k: new(big.Rat).Set(k), tau: new(big.Rat).Set(tau), work: proof.ExactOperations}, nil
}

// SampleRational evaluates a signed parameter in the complete proved range.
// Every sample independently evaluates both original exact supports; no float
// inverse or rounded source parameter is used. Failure publishes only work.
func (s *SplineCuspSection) SampleRational(ctx context.Context, t *big.Rat, o geom.SurfacePatchEvaluationOptions) (out CuspSectionSample, err error) {
	if !s.Complete() || ctx == nil || t == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 {
		return out, geom.ErrInvalid
	}
	work := 0
	defer func() {
		if err != nil {
			out = CuspSectionSample{}
		}
		out.ExactOperations = work
	}()
	check := func(q *big.Rat) error {
		work++
		if work > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	for _, q := range []*big.Rat{t, s.k, s.tau, s.center[0], s.center[1]} {
		if e := check(q); e != nil {
			return out, e
		}
	}
	abs := new(big.Rat).Abs(t)
	if abs.Cmp(s.tau) > 0 {
		return out, geom.ErrDomain
	}
	x := new(big.Rat).Mul(t, t)
	if e := check(x); e != nil {
		return out, e
	}
	x.Quo(x, s.k)
	if e := check(x); e != nil {
		return out, e
	}
	y := new(big.Rat).Mul(t, x)
	if e := check(y); e != nil {
		return out, e
	}
	norm := [2]*big.Rat{new(big.Rat).Add(s.center[0], x), new(big.Rat).Add(s.center[1], y)}
	ends := [2][2]float64{{s.domain.UMin, s.domain.UMax}, {s.domain.VMin, s.domain.VMax}}
	var uv [2]*big.Rat
	for i := range uv {
		if e := check(norm[i]); e != nil {
			return out, e
		}
		lo := new(big.Rat).SetFloat64(ends[i][0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ends[i][1]), lo)
		if e := check(width); e != nil {
			return out, e
		}
		uv[i] = new(big.Rat).Mul(width, norm[i])
		if e := check(uv[i]); e != nil {
			return out, e
		}
		uv[i].Add(uv[i], lo)
		if e := check(uv[i]); e != nil {
			return out, e
		}
	}
	var points [2][3]*big.Rat
	for i, patch := range s.exact {
		remaining := o.MaxOperations - work
		if remaining < 1 {
			return out, geom.ErrConvergence
		}
		local := o
		local.MaxOperations = remaining
		jet, e := patch.EvaluateRational(ctx, uv[0], uv[1], local)
		work += jet.ExactOperations
		if e != nil {
			return out, e
		}
		points[i] = jet.Point
		for j := range uv {
			out.Parameters[i][j] = new(big.Rat).Set(uv[j])
		}
	}
	for j := range points[0] {
		if points[0][j].Cmp(points[1][j]) != 0 {
			return out, ErrUnresolved
		}
	}
	out.Point = points[0]
	out.Germ = -1
	if t.Sign() < 0 {
		out.Germ = 0
	} else if t.Sign() > 0 {
		out.Germ = 1
	}
	out.Terminal = abs.Cmp(s.tau) == 0
	return out, nil
}
