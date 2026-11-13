package step

import (
	"bytes"
	"context"
	"errors"
	"io"
	"plainsolid/geom"
)

// WrappedSurfaceOptions bounds exact closure certification. ContinuityOrder0
// proves position closure;1 additionally proves matching first partials.
// Zero limits select64 surfaces,4096 poles,4M operations and16384 coefficient bits.
// STEP closure flags do not encode continuity order or periodic knot conventions.
type WrappedSurfaceOptions struct{ ContinuityOrder, MaxSurfaces, MaxPoles, MaxOperations, MaxCoefficientBits int }

func (o WrappedSurfaceOptions) defaults() (WrappedSurfaceOptions, error) {
	if o.MaxSurfaces == 0 {
		o.MaxSurfaces = 64
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.ContinuityOrder < 0 || o.ContinuityOrder > 1 || o.MaxSurfaces < 1 || o.MaxPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// WrappedSplineSurface is one clamped fundamental spline patch with exactly
// certified seams. It is neither a trimmed face nor a closed solid.
type WrappedSplineSurface struct {
	Name          string
	Geometry      *geom.WrappedSurfacePatch
	SelfIntersect string
}
type WrappedSurfaceModel struct {
	Surfaces   []WrappedSplineSurface
	LengthUnit float64
}

func certifyWrappedSurface(ctx context.Context, s SplineSurface, o WrappedSurfaceOptions) (WrappedSplineSurface, int, error) {
	fail := func() (WrappedSplineSurface, int, error) {
		return WrappedSplineSurface{}, 0, errors.New("step: closed surface flags/seam unsupported")
	}
	if s.ClosedU != "T" && s.ClosedU != "F" || s.ClosedV != "T" && s.ClosedV != "F" || s.ClosedU != "T" && s.ClosedV != "T" {
		return fail()
	}
	p, e := geom.NewSurfacePatch(s.Geometry, geom.SurfacePatchOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return WrappedSplineSurface{}, 0, e
	}
	left := o.MaxOperations - p.ExactOperations()
	if left < 1 {
		return WrappedSplineSurface{}, 0, geom.ErrConvergence
	}
	w, proof, e := geom.NewWrappedSurfacePatch(ctx, p, geom.WrappedSurfaceOptions{WrapU: s.ClosedU == "T", WrapV: s.ClosedV == "T", ContinuityOrder: o.ContinuityOrder, MaxPoles: o.MaxPoles, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return WrappedSplineSurface{}, 0, e
	}
	return WrappedSplineSurface{Name: s.Name, Geometry: w, SelfIntersect: s.SelfIntersect}, p.ExactOperations() + proof.ExactOperations, nil
}

// DecodeWrappedSurface certifies a directly referenced surface entity without
// importing any surrounding face topology. Closed axes come only from explicit
// STEP T flags; unknown logical closure and source weight rounding are rejected.
func DecodeWrappedSurface(ctx context.Context, f *File, id int64, o WrappedSurfaceOptions) (WrappedSplineSurface, error) {
	if ctx == nil {
		return WrappedSplineSurface{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedSplineSurface{}, e
	}
	o, e := o.defaults()
	if e != nil {
		return WrappedSplineSurface{}, e
	}
	if f == nil {
		return WrappedSplineSurface{}, geom.ErrInvalid
	}
	if e := exactSplineWeightNormalization(f, id, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
		return WrappedSplineSurface{}, e
	}
	s, e := DecodeBSplineSurface(f, id, o.MaxPoles)
	if e != nil {
		return WrappedSplineSurface{}, e
	}
	w, _, e := certifyWrappedSurface(ctx, s, o)
	return w, e
}

// ReadWrappedSurfaces accepts the existing standalone GEOMETRIC_SET surface
// graph, with an additional exact seam proof. Generic ReadSurfaces is unchanged.
func ReadWrappedSurfaces(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedSurfaceModel, error) {
	return ReadWrappedSurfacesContext(context.Background(), r, l, o)
}
func ReadWrappedSurfacesContext(ctx context.Context, r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedSurfaceModel, error) {
	if ctx == nil {
		return WrappedSurfaceModel{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return WrappedSurfaceModel{}, e
	}
	o, e := o.defaults()
	if e != nil {
		return WrappedSurfaceModel{}, e
	}
	l, e = l.defaults()
	if e != nil {
		return WrappedSurfaceModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return WrappedSurfaceModel{}, e
	}
	count := 0
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "B_SPLINE_SURFACE_WITH_KNOTS"); ok {
			count++
			if count > o.MaxSurfaces {
				return WrappedSurfaceModel{}, geom.ErrConvergence
			}
			if e := exactSplineWeightNormalization(f, id, "RATIONAL_B_SPLINE_SURFACE"); e != nil {
				return WrappedSurfaceModel{}, e
			}
		}
	}
	m, e := readSurfacesFile(f, l)
	if e != nil {
		return WrappedSurfaceModel{}, e
	}
	out := WrappedSurfaceModel{LengthUnit: m.LengthUnit}
	remaining := o.MaxOperations
	for _, s := range m.Surfaces {
		if remaining < 1 {
			return WrappedSurfaceModel{}, geom.ErrConvergence
		}
		options := o
		options.MaxOperations = remaining
		w, used, e := certifyWrappedSurface(ctx, s, options)
		if e != nil {
			return WrappedSurfaceModel{}, e
		}
		remaining -= used
		out.Surfaces = append(out.Surfaces, w)
	}
	return out, nil
}

// WriteWrappedSurfaces exports a lossless clamped fundamental patch with explicit
// u_closed/v_closed flags. It does not convert to OCCT periodic knot storage.
// STEP does not carry the wrapper's requested seam derivative order; readers
// choose that order and independently re-prove it. No geometry is rounded.
func WriteWrappedSurfaces(w io.Writer, m WrappedSurfaceModel) error {
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > 64 {
		return geom.ErrInvalid
	}
	plain := SurfaceModel{LengthUnit: m.LengthUnit}
	for _, s := range m.Surfaces {
		if s.Geometry == nil {
			return geom.ErrInvalid
		}
		recipe, e := s.Geometry.ExportRecipe()
		if e != nil {
			return e
		}
		u, v := "F", "F"
		if recipe.Options.WrapU {
			u = "T"
		}
		if recipe.Options.WrapV {
			v = "T"
		}
		self := s.SelfIntersect
		if self == "" {
			self = "U"
		}
		plain.Surfaces = append(plain.Surfaces, SplineSurface{Name: s.Name, Geometry: recipe.Surface, ClosedU: u, ClosedV: v, SelfIntersect: self})
	}
	var output bytes.Buffer
	if e := WriteSurfaces(&output, plain); e != nil {
		return e
	}
	if _, e := ReadWrappedSurfaces(bytes.NewReader(output.Bytes()), Limits{}, WrappedSurfaceOptions{}); e != nil {
		return e
	}
	n, e := w.Write(output.Bytes())
	if e == nil && n != output.Len() {
		e = io.ErrShortWrite
	}
	return e
}
