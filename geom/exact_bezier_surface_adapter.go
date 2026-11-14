package geom

// ExactBezierSurface explicitly exposes an immutable exact positive-weight
// single-cell support through the legacy numerical Surface interface. It never
// reconstructs rounded poles. Eval/Partials round rational jet results at the
// represented query parameters; ExactPatch retains the independent proof source.
// A regular chart or closed solid is not implied by this adapter.
type ExactBezierSurface struct{ patch *SurfacePatch }

func NewExactBezierSurface(p *SurfacePatch) (*ExactBezierSurface, error) {
	if p == nil || p.degreeU < 1 || p.degreeV < 1 || p.degreeU > 64 || p.degreeV > 64 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) != p.countU*p.countV || !p.Domain().Valid() {
		return nil, ErrInvalid
	}
	return &ExactBezierSurface{patch: p}, nil
}

// ExactPatch returns the immutable source, not a floating reconstruction.
func (s *ExactBezierSurface) ExactPatch() *SurfacePatch {
	if s == nil {
		return nil
	}
	return s.patch
}
func (s *ExactBezierSurface) Domain() ParameterRectangle {
	if s == nil {
		return (*SurfacePatch)(nil).Domain()
	}
	return s.patch.Domain()
}
func (s *ExactBezierSurface) Eval(u, v float64) (Vec3, error) {
	if s == nil || s.patch == nil {
		return Vec3{}, ErrInvalid
	}
	return s.patch.Eval(u, v)
}
func (s *ExactBezierSurface) Partials(u, v float64) (Vec3, Vec3, error) {
	if s == nil || s.patch == nil {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return s.patch.Partials(u, v)
}

var _ Surface = (*ExactBezierSurface)(nil)
