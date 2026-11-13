package iges

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
)

// QuadraticOvalSeam pairs a hole ordinal of the base face with the index of its
// filling disk in Model.TrimmedSurfaces. ModelCurves are directed oppositely.
type QuadraticOvalSeam struct {
	Hole, Disk  int
	ModelCurves [2]int
}

// QuadraticOvalFilledSheet is a connected planar geometric sheet with one
// perforated affine face and one independently charted disk per hole. It does
// not assert IGES topological shell ownership or material closure.
type QuadraticOvalFilledSheet struct {
	Model Model
	Base  int
	Seams []QuadraticOvalSeam
}

// CertifyQuadraticOvalFilledSheet accepts 1..16 disjoint, strictly contained
// oval holes, each filled exactly once. The existing oval containment proof and
// exact reversed whole-curve identities establish coplanarity, winding and
// complementary face interiors. Every source entity and contour must be owned.
func CertifyQuadraticOvalFilledSheet(m Model) (QuadraticOvalFilledSheet, error) {
	fail := func() (QuadraticOvalFilledSheet, error) {
		return QuadraticOvalFilledSheet{}, fmt.Errorf("iges: unsupported or inconsistent filled oval sheet")
	}
	n := len(m.TrimmedSurfaces) - 1
	if n < 1 || n > 16 || len(m.Entities) != 3+5*n || len(m.CurvesOnSurfaces) != 1+2*n || m.LengthUnit <= 0 || math.IsNaN(m.LengthUnit) || math.IsInf(m.LengthUnit, 0) {
		return fail()
	}
	base := -1
	for i, t := range m.TrimmedSurfaces {
		if len(t.Inner) > 0 {
			if base >= 0 || len(t.Inner) != n {
				return fail()
			}
			base = i
		}
	}
	if base < 0 {
		return fail()
	}
	if _, _, e := m.QuadraticOvalBoundaries(m.TrimmedSurfaces[base]); e != nil {
		return QuadraticOvalFilledSheet{}, e
	}
	used := map[int]bool{}
	contours := map[int]bool{}
	for i, t := range m.TrimmedSurfaces {
		if i != base {
			if _, e := m.QuadraticOvalBoundary(t); e != nil {
				return QuadraticOvalFilledSheet{}, e
			}
		}
		if t.Surface < 0 || t.Surface >= len(m.Entities) || used[t.Surface] {
			return fail()
		}
		used[t.Surface] = true
		for _, id := range append([]int{t.Outer}, t.Inner...) {
			if id < 0 || id >= len(m.CurvesOnSurfaces) || contours[id] {
				return fail()
			}
			contours[id] = true
			c := m.CurvesOnSurfaces[id]
			for _, v := range []int{c.ParameterCurve, c.ModelCurve} {
				if v < 0 || v >= len(m.Entities) || used[v] {
					return fail()
				}
				used[v] = true
			}
		}
	}
	if len(used) != len(m.Entities) || len(contours) != len(m.CurvesOnSurfaces) {
		return fail()
	}
	for _, e := range m.Entities {
		if e.Transform != nil {
			return fail()
		}
	}
	curves := map[int]*geom.ExactSplineCurve{}
	for _, c := range m.CurvesOnSurfaces {
		q, e := geom.NewExactSplineCurve(m.Entities[c.ModelCurve].Spline.Curve, geom.SurfacePatchOptions{})
		if e != nil {
			return QuadraticOvalFilledSheet{}, e
		}
		curves[c.ModelCurve] = q
	}
	result := QuadraticOvalFilledSheet{Model: m, Base: base}
	disks := map[int]bool{}
	for hole, id := range m.TrimmedSurfaces[base].Inner {
		ca := m.CurvesOnSurfaces[id].ModelCurve
		match := -1
		cb := -1
		for i, t := range m.TrimmedSurfaces {
			if i == base {
				continue
			}
			candidate := m.CurvesOnSurfaces[t.Outer].ModelCurve
			equal, e := geom.CompareExactCurvesContext(context.Background(), curves[ca], curves[candidate], geom.ExactCurveComparisonOptions{ReverseB: true})
			if e != nil {
				return QuadraticOvalFilledSheet{}, e
			}
			if !equal.Complete {
				return fail()
			}
			if equal.Equal {
				if match >= 0 {
					return fail()
				}
				match = i
				cb = candidate
			}
		}
		if match < 0 || disks[match] {
			return fail()
		}
		disks[match] = true
		result.Seams = append(result.Seams, QuadraticOvalSeam{Hole: hole, Disk: match, ModelCurves: [2]int{ca, cb}})
	}
	return result, nil
}

// NewQuadraticOvalFilledSheet combines a perforated face and independent disk
// models. Disk order need not match hole order; exact matching determines it.
func NewQuadraticOvalFilledSheet(base Model, disks []Model) (QuadraticOvalFilledSheet, error) {
	if len(disks) < 1 || len(disks) > 16 || len(base.TrimmedSurfaces) != 1 || len(base.TrimmedSurfaces[0].Inner) != len(disks) || len(base.Entities) != 3+2*len(disks) || len(base.CurvesOnSurfaces) != 1+len(disks) {
		return QuadraticOvalFilledSheet{}, geom.ErrInvalid
	}
	m := Model{LengthUnit: base.LengthUnit}
	models := append([]Model{base}, disks...)
	for i, part := range models {
		if part.LengthUnit != m.LengthUnit || len(part.TrimmedSurfaces) != 1 || i > 0 && (len(part.Entities) != 3 || len(part.CurvesOnSurfaces) != 1 || len(part.TrimmedSurfaces[0].Inner) != 0) {
			return QuadraticOvalFilledSheet{}, geom.ErrInvalid
		}
		offset, co := len(m.Entities), len(m.CurvesOnSurfaces)
		m.Entities = append(m.Entities, part.Entities...)
		for _, c := range part.CurvesOnSurfaces {
			c.Surface += offset
			c.ParameterCurve += offset
			c.ModelCurve += offset
			m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, c)
		}
		for _, t := range part.TrimmedSurfaces {
			t.Surface += offset
			t.Outer += co
			t.Inner = append([]int(nil), t.Inner...)
			for j := range t.Inner {
				t.Inner[j] += co
			}
			m.TrimmedSurfaces = append(m.TrimmedSurfaces, t)
		}
	}
	return CertifyQuadraticOvalFilledSheet(m)
}

// ReadQuadraticOvalFilledSheet applies resource-bounded decoding and exact certification.
func ReadQuadraticOvalFilledSheet(r io.Reader, l Limits) (QuadraticOvalFilledSheet, error) {
	m, e := Read(r, l)
	if e != nil {
		return QuadraticOvalFilledSheet{}, e
	}
	return CertifyQuadraticOvalFilledSheet(m)
}

// WriteQuadraticOvalFilledSheet recertifies the model; cached seam data is ignored.
func WriteQuadraticOvalFilledSheet(w io.Writer, s QuadraticOvalFilledSheet) error {
	if _, e := CertifyQuadraticOvalFilledSheet(s.Model); e != nil {
		return e
	}
	return Write(w, s.Model)
}
