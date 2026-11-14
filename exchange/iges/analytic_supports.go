package iges

import (
	"bytes"
	"errors"
	"io"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// AnalyticSupport contains exactly one independent complete support definition.
// Cone45 is available through the explicit IGES-only read and write mode.
// Its generatrix parameter does not match the STEP cone contract.
type AnalyticSupport struct {
	Plane    *PlanarSupport
	Cylinder *CylindricalSurface
	Sphere   *SphericalSupport
	Torus    *ToroidalSupport
	Cone45   *Conical45Support
}
type AnalyticSupportModel struct {
	LengthUnit float64
	Surfaces   []AnalyticSupport
}

func analyticIGESReferenceIndices(kind int) []int {
	switch kind {
	case 190:
		return []int{1, 2, 3}
	case 192:
		return []int{1, 2, 4}
	case 194:
		return []int{1, 2, 5}
	case 196:
		return []int{1, 3, 4}
	case 198:
		return []int{1, 2, 5}
	}
	return nil
}
func analyticIGESAppend(a *topologyAuthor, d topologySource, keep map[int]bool) error {
	remap := map[int]int{}
	next := 2*len(a.records) + 1
	for _, r := range d.entities {
		if keep[r.id] {
			remap[r.id] = next
			next += 2
		}
	}
	for _, r := range d.entities {
		if !keep[r.id] {
			continue
		}
		if e := compositeSourceNumbers(110, r.values); e != nil {
			return e
		}
		values := append([]parameter(nil), r.values...)
		indices := analyticIGESReferenceIndices(r.kind)
		for _, i := range indices {
			if i >= len(values) {
				return geom.ErrDomain
			}
			old, e := intValue(values[i])
			if e != nil || remap[old] == 0 {
				return geom.ErrDomain
			}
			values[i].text = strconv.Itoa(remap[old])
		}
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = v.text
		}
		form := 0
		if indices != nil {
			form = 1
		}
		a.add(r.kind, form, strings.Join(parts, ",")+";")
		a.records[len(a.records)-1].label = r.label
		a.records[len(a.records)-1].subscript = r.subscript
	}
	return nil
}
func analyticIGESAuthor() topologyAuthor {
	p := geom.Vec3{}
	return topologyAuthor{geometry: Model{Entities: []Entity{{Point: &p}}}}
}

// ReadAnalyticSupports preserves root directory order and requires globally
// unique116/123 ownership. It stages unchanged exact source numeric tokens
// through the specialized strict codecs after whole-file graph partitioning.
func ReadAnalyticSupports(r io.Reader, l Limits) (AnalyticSupportModel, error) {
	return readAnalyticSupports(r, l, false)
}

// ReadAnalyticSupportsWithCones additionally admits original type194 form1
// supports with the strict 45-degree generatrix parameterization.
func ReadAnalyticSupportsWithCones(r io.Reader, l Limits) (AnalyticSupportModel, error) {
	return readAnalyticSupports(r, l, true)
}

func readAnalyticSupports(r io.Reader, l Limits, cones bool) (AnalyticSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return AnalyticSupportModel{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 4<<20)
	l.MaxRecords = min(l.MaxRecords, 1024)
	l.MaxEntities = min(l.MaxEntities, 256)
	l.MaxValues = min(l.MaxValues, 8192)
	d, e := readConeSupportRecords(r, l, false, true, true, true, true, cones)
	if e != nil {
		return AnalyticSupportModel{}, e
	}
	fail := func() (AnalyticSupportModel, error) {
		return AnalyticSupportModel{}, errors.New("iges: unsupported mixed analytic supports")
	}
	byID := map[int]topologyRecord{}
	for _, x := range d.entities {
		if e := compositeSourceNumbers(110, x.values); e != nil {
			return fail()
		}
		byID[x.id] = x
	}
	used := map[int]bool{}
	var roots []topologyRecord
	var groups []map[int]bool
	for _, x := range d.entities {
		indices := analyticIGESReferenceIndices(x.kind)
		if indices == nil {
			continue
		}
		if len(roots) >= 64 || used[x.id] {
			return fail()
		}
		used[x.id] = true
		group := map[int]bool{x.id: true}
		for j, i := range indices {
			if i >= len(x.values) {
				return fail()
			}
			id, e := intValue(x.values[i])
			dep, ok := byID[id]
			kind := 123
			if j == 0 {
				kind = 116
			}
			if e != nil || !ok || dep.kind != kind || used[id] {
				return fail()
			}
			used[id] = true
			group[id] = true
		}
		roots = append(roots, x)
		groups = append(groups, group)
	}
	if len(roots) == 0 || len(used) != len(d.entities) {
		return fail()
	}
	out := AnalyticSupportModel{LengthUnit: d.unit}
	for i, root := range roots {
		a := analyticIGESAuthor()
		if e = analyticIGESAppend(&a, d, groups[i]); e != nil {
			return fail()
		}
		b, e := a.file(d.unit)
		if e != nil {
			return fail()
		}
		s := AnalyticSupport{}
		switch root.kind {
		case 190:
			m, e := ReadPlanarSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Plane = &m.Surfaces[0]
		case 192:
			m, e := ReadCylindricalSurfaces(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Cylinder = &m.Surfaces[0]
		case 194:
			m, e := ReadConical45Supports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Cone45 = &m.Surfaces[0]
		case 196:
			m, e := ReadSphericalSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Sphere = &m.Surfaces[0]
		case 198:
			m, e := ReadToroidalSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Torus = &m.Surfaces[0]
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	return out, nil
}

// WriteAnalyticSupports stages every specialized support and merges only its
// uniquely owned graph. Whole-file validation completes before caller I/O.
func WriteAnalyticSupports(w io.Writer, m AnalyticSupportModel) error {
	return writeAnalyticSupports(w, m, false)
}

// WriteAnalyticSupportsWithCones stages and re-proves mixed IGES supports,
// including the strict native type194 form1 45-degree cone.
func WriteAnalyticSupportsWithCones(w io.Writer, m AnalyticSupportModel) error {
	return writeAnalyticSupports(w, m, true)
}

func writeAnalyticSupports(w io.Writer, m AnalyticSupportModel, cones bool) error {
	if w == nil || len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return geom.ErrDomain
	}
	a := analyticIGESAuthor()
	for _, s := range m.Surfaces {
		n := 0
		for _, v := range []bool{s.Plane != nil, s.Cylinder != nil, s.Sphere != nil, s.Torus != nil, s.Cone45 != nil} {
			if v {
				n++
			}
		}
		if n != 1 || s.Cone45 != nil && !cones {
			return geom.ErrDomain
		}
		var b bytes.Buffer
		var e error
		switch {
		case s.Plane != nil:
			e = WritePlanarSupports(&b, PlanarSupportModel{LengthUnit: m.LengthUnit, Surfaces: []PlanarSupport{*s.Plane}})
		case s.Cylinder != nil:
			e = WriteCylindricalSurfaces(&b, CylindricalSurfaceModel{LengthUnit: m.LengthUnit, Surfaces: []CylindricalSurface{*s.Cylinder}})
		case s.Cone45 != nil:
			e = WriteConical45Supports(&b, Conical45SupportModel{LengthUnit: m.LengthUnit, Surfaces: []Conical45Support{*s.Cone45}})
		case s.Sphere != nil:
			e = WriteSphericalSupports(&b, SphericalSupportModel{LengthUnit: m.LengthUnit, Surfaces: []SphericalSupport{*s.Sphere}})
		case s.Torus != nil:
			e = WriteToroidalSupports(&b, ToroidalSupportModel{LengthUnit: m.LengthUnit, Surfaces: []ToroidalSupport{*s.Torus}})
		}
		if e != nil {
			return e
		}
		d, e := readConeSupportRecords(&b, Limits{}, false, true, true, true, true, cones)
		if e != nil {
			return e
		}
		keep := map[int]bool{}
		for _, r := range d.entities {
			keep[r.id] = true
		}
		if e = analyticIGESAppend(&a, d, keep); e != nil {
			return e
		}
	}
	b, e := a.file(m.LengthUnit)
	if e != nil {
		return e
	}
	if _, e = readAnalyticSupports(bytes.NewReader(b), Limits{}, cones); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return e
}
