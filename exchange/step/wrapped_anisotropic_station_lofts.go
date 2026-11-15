package step

import (
	"context"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// stationScaleEmbedding keeps both represented coordinate scales. A homothetic
// band's two scales agree; no anisotropic scale is described as a circle radius.
type stationScaleEmbedding struct {
	CenterX, CenterY, LowerZ, UpperZ *big.Rat
	LowerScale, UpperScale           [2]*big.Rat
	Complete, Circular               bool
}

func certifyStationScaleEmbedding(w *geom.WrappedSurfacePatch, o geom.SurfacePatchOptions, anisotropic bool) (stationScaleEmbedding, error) {
	if anisotropic {
		p, e := geom.CertifyWrappedAnisotropicEmbedding(context.Background(), w, o)
		return stationScaleEmbedding{CenterX: p.Center[0], CenterY: p.Center[1], LowerZ: p.LowerZ, UpperZ: p.UpperZ, LowerScale: p.LowerScale, UpperScale: p.UpperScale, Complete: p.Complete}, e
	}
	p, e := geom.CertifyWrappedFrustumEmbedding(context.Background(), w, o)
	return stationScaleEmbedding{CenterX: p.CenterX, CenterY: p.CenterY, LowerZ: p.LowerZ, UpperZ: p.UpperZ, LowerScale: [2]*big.Rat{p.LowerRadius, p.LowerRadius}, UpperScale: [2]*big.Rat{p.UpperRadius, p.UpperRadius}, Complete: p.Complete, Circular: p.Circular}, e
}

// WrappedAnisotropicStationLoftBody retains 1..16 original weighted 9x2 spline
// bands with independent positive X/Y scales. Weights and scales do not imply
// circular or elliptical sections. Solid is a rederived cache, never authority.
type WrappedAnisotropicStationLoftBody struct {
	Name       string
	Walls      []WrappedSplineFace
	CapNames   [2]string
	Solid      trimmed.Solid
	LengthUnit float64
}

// ReadWrappedAnisotropicStationLoft proves complete model/pcurve correspondence,
// exact station rim identity and cap ownership, then the native anisotropic
// material. Existing circular and homothetic entrypoints remain unchanged.
func ReadWrappedAnisotropicStationLoft(r io.Reader, l Limits, o WrappedSurfaceOptions) (WrappedAnisotropicStationLoftBody, error) {
	m, e := readWrappedStationLoft(r, l, o, false, true)
	if e != nil {
		return WrappedAnisotropicStationLoftBody{}, e
	}
	return WrappedAnisotropicStationLoftBody{Name: m.Name, Walls: m.Walls, CapNames: m.CapNames, Solid: m.Solid, LengthUnit: m.LengthUnit}, nil
}

// WriteWrappedAnisotropicStationLoft retains exact source spline supports and
// full weighted rims, staging and re-proving all geometry before output.
func WriteWrappedAnisotropicStationLoft(w io.Writer, m WrappedAnisotropicStationLoftBody) error {
	if w == nil || !validLengthUnit(m.LengthUnit) || len(m.Walls) < 1 || len(m.Walls) > 16 {
		return geom.ErrInvalid
	}
	supports := make([]*geom.WrappedSurfacePatch, len(m.Walls))
	parts := WrappedFrustumModel{LengthUnit: m.LengthUnit}
	for i, wall := range m.Walls {
		supports[i] = wall.Geometry
		parts.Bodies = append(parts.Bodies, WrappedFrustumBody{Face: wall, CapNames: m.CapNames})
	}
	if _, _, e := trimmed.NewWrappedAnisotropicStationLoft(context.Background(), supports, 1e-10, geom.SurfacePatchOptions{}); e != nil {
		return e
	}
	return writeWrappedStationGraph(w, m.Name, parts, false, true)
}

// WrappedAnisotropicStationLoftFromSolid recovers authenticated original walls;
// independent scales and weights remain in those supports without resampling.
func WrappedAnisotropicStationLoftFromSolid(ctx context.Context, s trimmed.Solid, name string, unit float64, o geom.SurfacePatchOptions) (WrappedAnisotropicStationLoftBody, error) {
	var zero WrappedAnisotropicStationLoftBody
	if !validLengthUnit(unit) {
		return zero, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedAnisotropicStationLoft(ctx, o)
	if e != nil {
		return zero, e
	}
	if !p.Complete {
		return zero, geom.ErrConvergence
	}
	out := WrappedAnisotropicStationLoftBody{Name: name, Solid: s, LengthUnit: unit}
	faces := s.Faces()
	for _, id := range p.History.WallFaces {
		w, ok := faces[id].Surface.(*geom.WrappedSurfacePatch)
		if !ok {
			return zero, geom.ErrDomain
		}
		out.Walls = append(out.Walls, WrappedSplineFace{Geometry: w})
	}
	return out, nil
}

// The new dialect preserves authored decimal weights as well as the existing
// represented coefficient normalization gate; near-binary source weights fail.
func anisotropicStationSourceWeights(f *File) error {
	var visit func(Value) error
	visit = func(v Value) error {
		if v.Kind == Aggregate {
			for _, x := range v.Items {
				if e := visit(x); e != nil {
					return e
				}
			}
			return nil
		}
		q, e := liftSourceRat(v)
		if e != nil {
			return e
		}
		x, e := numeric(v)
		if e != nil || q.Sign() <= 0 {
			return geom.ErrInvalid
		}
		actual := new(big.Rat).SetFloat64(x)
		if actual == nil || q.Cmp(actual) != 0 {
			return geom.ErrUnrepresentable
		}
		return nil
	}
	for _, id := range f.Order {
		for _, r := range f.Entities[id].Records {
			if r.Type == "RATIONAL_B_SPLINE_SURFACE" || r.Type == "RATIONAL_B_SPLINE_CURVE" {
				if len(r.Args) != 1 {
					return geom.ErrInvalid
				}
				if e := visit(r.Args[0]); e != nil {
					return e
				}
				if e := exactSplineWeightNormalization(f, id, r.Type); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
