package step

import (
	"errors"
	"io"
	"plainsolid/geom"
)

const MaxSplineFaceHoles = 16

// MultiHoledSplineFace retains an exact outer rectangle and one through
// MaxSplineFaceHoles pairwise disjoint, strictly interior rectangular holes.
// Holes retain source contour order, independent of the outer bound position.
type MultiHoledSplineFace struct {
	Name     string
	Geometry *geom.BSplineSurface
	Outer    geom.ParameterRectangle
	Holes    []geom.ParameterRectangle
}
type MultiHoledSplineFaceModel struct {
	Faces      []MultiHoledSplineFace
	LengthUnit float64
}

func validateSplineHoles(outer geom.ParameterRectangle, holes []geom.ParameterRectangle) error {
	if len(holes) == 0 || len(holes) > MaxSplineFaceHoles {
		return errors.New("step: spline hole count out of range")
	}
	for i, h := range holes {
		if !strictSplineHole(outer, h) {
			return errors.New("step: spline hole is not strictly interior")
		}
		for _, p := range holes[:i] {
			if !(h.UMax < p.UMin || p.UMax < h.UMin || h.VMax < p.VMin || p.VMax < h.VMin) {
				return errors.New("step: spline holes overlap, nest or touch")
			}
		}
	}
	return nil
}
func decodeMultiHoledSplineFace(f *File, id int64, maxPoints int) (MultiHoledSplineFace, error) {
	fail := func() (MultiHoledSplineFace, error) {
		return MultiHoledSplineFace{}, errors.New("step: invalid multi-hole spline face")
	}
	face, err := simple(f, id, "ADVANCED_FACE", 4)
	if err != nil {
		return fail()
	}
	bounds, err := listRefs(face.Args[1], 2)
	if err != nil || len(bounds) < 2 || len(bounds) > 1+MaxSplineFaceHoles {
		return fail()
	}
	result := MultiHoledSplineFace{}
	hasOuter := false
	seen := map[int64]bool{}
	for _, bid := range bounds {
		if seen[bid] {
			return fail()
		}
		seen[bid] = true
		view, e := splineBoundView(f, id, bid, false)
		if e != nil {
			return fail()
		}
		rectangle, e := decodeBoundedSplineFace(view, id, maxPoints)
		if e == nil {
			if hasOuter {
				return fail()
			}
			hasOuter = true
			result.Name = rectangle.Name
			result.Geometry = rectangle.Geometry
			result.Outer = rectangle.Domain
			continue
		}
		view, e = splineBoundView(f, id, bid, true)
		if e != nil {
			return fail()
		}
		rectangle, e = decodeBoundedSplineFace(view, id, maxPoints)
		if e != nil {
			return fail()
		}
		result.Holes = append(result.Holes, rectangle.Domain)
	}
	if !hasOuter {
		return fail()
	}
	if e := validateSplineHoles(result.Outer, result.Holes); e != nil {
		return fail()
	}
	return result, nil
}

// ReadMultiHoledSplineFaces accepts only the explicit bounded rectangular loop
// subset. Each loop passes the exact model/UV proof; strict parameter separation
// and outer-patch injectivity prove disjoint model boundaries. Old single-hole
// and hole-free readers retain their narrower contracts.
func ReadMultiHoledSplineFaces(r io.Reader, l Limits) (MultiHoledSplineFaceModel, error) {
	l, err := l.defaults()
	if err != nil {
		return MultiHoledSplineFaceModel{}, err
	}
	f, err := Parse(r, l)
	if err != nil {
		return MultiHoledSplineFaceModel{}, err
	}
	result := MultiHoledSplineFaceModel{}
	remainingUses := l.MaxEdgeUses
	m, err := readSplineFaceGraph(f, l, func(f *File, id int64, maxPoints int) (BoundedSplineFace, error) {
		record, e := simple(f, id, "ADVANCED_FACE", 4)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		bounds, e := listRefs(record.Args[1], 2)
		if e != nil || len(bounds) > remainingUses/4 {
			return BoundedSplineFace{}, errors.New("step: spline face edge-use limit")
		}
		face, e := decodeMultiHoledSplineFace(f, id, maxPoints)
		if e != nil {
			return BoundedSplineFace{}, e
		}
		uses := 4 * (1 + len(face.Holes))
		if uses > remainingUses {
			return BoundedSplineFace{}, errors.New("step: spline face edge-use limit")
		}
		remainingUses -= uses
		result.Faces = append(result.Faces, face)
		return BoundedSplineFace{Name: face.Name, Geometry: face.Geometry, Domain: face.Outer}, nil
	}, 0)
	if err != nil {
		return MultiHoledSplineFaceModel{}, err
	}
	result.LengthUnit = m.LengthUnit
	return result, nil
}

// WriteMultiHoledSplineFaces retains the original supports and emits lossless
// paired model and UV boundaries. Nested, touching or overlapping holes fail;
// no polygon merging or approximation is performed.
func WriteMultiHoledSplineFaces(w io.Writer, m MultiHoledSplineFaceModel) error {
	base := BoundedSplineFaceModel{LengthUnit: m.LengthUnit}
	holes := make([][]geom.ParameterRectangle, len(m.Faces))
	for i, f := range m.Faces {
		if e := validateSplineHoles(f.Outer, f.Holes); e != nil {
			return e
		}
		base.Faces = append(base.Faces, BoundedSplineFace{Name: f.Name, Geometry: f.Geometry, Domain: f.Outer})
		holes[i] = f.Holes
	}
	return writeSplineRectangles(w, base, holes)
}
