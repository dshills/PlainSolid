package iges

import (
	"context"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
)

// CurvedOvalSheet preserves two distinct curved IGES faces with an exactly
// matched, oppositely directed complete model seam. Each face is independently
// embedded; pairwise interior disjointness is not certified. This geometric
// adjacency does not assert IGES topological incidence or material closure.
type CurvedOvalSheet struct {
	Model           Model
	Annulus, Disk   int
	SeamModelCurves [2]int
}

// CertifyCurvedOvalSheet accepts exactly an annulus and the disk filling its
// sole hole, with no unowned entities, transforms or additional contours.
func CertifyCurvedOvalSheet(m Model) (CurvedOvalSheet, error) {
	fail := func() (CurvedOvalSheet, error) {
		return CurvedOvalSheet{}, fmt.Errorf("iges: unsupported or inconsistent curved two-face sheet")
	}
	if m.LengthUnit <= 0 || math.IsNaN(m.LengthUnit) || math.IsInf(m.LengthUnit, 0) || len(m.Entities) != 8 || len(m.TrimmedSurfaces) != 2 || len(m.CurvesOnSurfaces) != 3 {
		return fail()
	}
	a, b := 0, 1
	if len(m.TrimmedSurfaces[a].Inner) == 0 {
		a, b = b, a
	}
	ta, tb := m.TrimmedSurfaces[a], m.TrimmedSurfaces[b]
	if len(ta.Inner) != 1 || len(tb.Inner) != 0 || ta.Surface == tb.Surface {
		return fail()
	}
	if e := m.validateCurvedOvalTrim(ta); e != nil {
		return CurvedOvalSheet{}, e
	}
	if e := m.validateCurvedOvalTrim(tb); e != nil {
		return CurvedOvalSheet{}, e
	}
	used := map[int]bool{}
	contours := map[int]bool{}
	for _, t := range m.TrimmedSurfaces {
		used[t.Surface] = true
		for _, id := range append([]int{t.Outer}, t.Inner...) {
			if contours[id] || id < 0 || id >= len(m.CurvesOnSurfaces) {
				return fail()
			}
			contours[id] = true
			c := m.CurvesOnSurfaces[id]
			for _, v := range []int{c.ParameterCurve, c.ModelCurve} {
				if used[v] {
					return fail()
				}
				used[v] = true
			}
		}
	}
	if len(used) != len(m.Entities) {
		return fail()
	}
	for _, e := range m.Entities {
		if e.Transform != nil {
			return fail()
		}
	}
	ca, cb := m.CurvesOnSurfaces[ta.Inner[0]].ModelCurve, m.CurvesOnSurfaces[tb.Outer].ModelCurve
	x, e := geom.NewExactSplineCurve(m.Entities[ca].Spline.Curve, curvedOvalOptions())
	if e != nil {
		return CurvedOvalSheet{}, e
	}
	y, e := geom.NewExactSplineCurve(m.Entities[cb].Spline.Curve, curvedOvalOptions())
	if e != nil {
		return CurvedOvalSheet{}, e
	}
	same, e := geom.CompareExactCurvesContext(context.Background(), x, y, geom.ExactCurveComparisonOptions{ReverseB: true, MaxSpans: 128, MaxOperations: 1000000, MaxCoefficientBits: 8192})
	if e != nil {
		return CurvedOvalSheet{}, e
	}
	if !same.Complete || !same.Equal {
		return fail()
	}
	return CurvedOvalSheet{Model: m, Annulus: a, Disk: b, SeamModelCurves: [2]int{ca, cb}}, nil
}

// ReadCurvedOvalSheet applies bounded IGES decoding before exact certification.
func ReadCurvedOvalSheet(r io.Reader, l Limits) (CurvedOvalSheet, error) {
	m, e := readCurvedMode(r, l, true)
	if e != nil {
		return CurvedOvalSheet{}, e
	}
	return CertifyCurvedOvalSheet(m)
}

// WriteCurvedOvalSheet recertifies Model; cached seam indices are not trusted.
func WriteCurvedOvalSheet(w io.Writer, s CurvedOvalSheet) error {
	if _, e := CertifyCurvedOvalSheet(s.Model); e != nil {
		return e
	}
	return writeCurvedMode(w, s.Model, true)
}

// NewCurvedOvalSheet joins two independently constructed single-face models.
// Geometry remains separate: the returned seam is an exact geometric relation,
// not an IGES manifold topology entity. Input length units must match exactly.
func NewCurvedOvalSheet(annulus, disk Model) (CurvedOvalSheet, error) {
	if len(annulus.TrimmedSurfaces) != 1 || len(disk.TrimmedSurfaces) != 1 || len(annulus.Entities) != 5 || len(disk.Entities) != 3 || len(annulus.CurvesOnSurfaces) != 2 || len(disk.CurvesOnSurfaces) != 1 || annulus.LengthUnit != disk.LengthUnit {
		return CurvedOvalSheet{}, geom.ErrInvalid
	}
	m := Model{LengthUnit: annulus.LengthUnit}
	m.Entities = append(m.Entities, annulus.Entities...)
	m.Entities = append(m.Entities, disk.Entities...)
	m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, annulus.CurvesOnSurfaces...)
	for _, c := range disk.CurvesOnSurfaces {
		c.Surface += 5
		c.ParameterCurve += 5
		c.ModelCurve += 5
		m.CurvesOnSurfaces = append(m.CurvesOnSurfaces, c)
	}
	m.TrimmedSurfaces = append(m.TrimmedSurfaces, annulus.TrimmedSurfaces...)
	for _, t := range disk.TrimmedSurfaces {
		t.Surface += 5
		t.Outer += 2
		t.Inner = append([]int(nil), t.Inner...)
		for i := range t.Inner {
			t.Inner[i] += 2
		}
		m.TrimmedSurfaces = append(m.TrimmedSurfaces, t)
	}
	return CertifyCurvedOvalSheet(m)
}
