package geom

import "context"

// ExactTensorSurface explicitly exposes an immutable exact positive-weight
// connected tensor support through the legacy numerical Surface interface. It never
// reconstructs rounded poles. Eval/Partials round rational jet results at the
// represented query parameters; ExactPatch retains the independent proof source.
// A regular chart or closed solid is not implied by this adapter.
type ExactTensorSurface struct {
	patch      *SurfacePatch
	operations int
}

// NewExactTensorSurface binds a validated immutable positive-weight patch.
// It bounds input poles, degrees, coefficient bits and inspection work, without
// rebuilding knots or dividing homogeneous controls into rounded poles.
func NewExactTensorSurface(ctx context.Context, p *SurfacePatch, o SurfacePatchOptions) (*ExactTensorSurface, error) {
	if ctx == nil || p == nil || p.degreeU < 1 || p.degreeV < 1 || p.degreeU > 64 || p.degreeV > 64 || !p.Domain().Valid() {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if len(p.net) > opts.MaxPoles {
		return nil, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	for _, h := range p.net {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for _, v := range h {
			if e := b.check(v); e != nil {
				return nil, e
			}
		}
	}
	return &ExactTensorSurface{patch: p, operations: b.operations}, nil
}

// ExactOperations reports the bounded constructor coefficient inspection work.
func (s *ExactTensorSurface) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// ExactPatch returns the immutable source, not a floating reconstruction.
func (s *ExactTensorSurface) ExactPatch() *SurfacePatch {
	if s == nil {
		return nil
	}
	return s.patch
}
func (s *ExactTensorSurface) Domain() ParameterRectangle {
	if s == nil {
		return (*SurfacePatch)(nil).Domain()
	}
	return s.patch.Domain()
}
func (s *ExactTensorSurface) Eval(u, v float64) (Vec3, error) {
	if s == nil || s.patch == nil {
		return Vec3{}, ErrInvalid
	}
	return s.patch.Eval(u, v)
}
func (s *ExactTensorSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if s == nil || s.patch == nil {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return s.patch.Partials(u, v)
}

var _ Surface = (*ExactTensorSurface)(nil)
