package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// AnalyticSupport contains exactly one independently owned complete support.
// These are geometry definitions, not trimmed faces, occurrences or solids.
type AnalyticSupport struct {
	Plane    *PlaneSupport
	Cylinder *CylinderSupport
	Sphere   *SphereSupport
	Torus    *TorusSupport
}
type AnalyticSupportModel struct {
	Name, SetName string
	LengthUnit    float64
	Surfaces      []AnalyticSupport
}

func analyticSupportClosure(f *File, id int64) (map[int64]bool, error) {
	out, active := map[int64]bool{}, map[int64]bool{}
	var visit func(int64, int) error
	visit = func(id int64, depth int) error {
		if depth > 64 || active[id] {
			return errors.New("step: cyclic support dependency")
		}
		if out[id] {
			return nil
		}
		e, ok := f.Entities[id]
		if !ok {
			return errors.New("step: missing support dependency")
		}
		active[id] = true
		var value func(Value) error
		value = func(v Value) error {
			if v.Kind == Reference {
				return visit(v.Ref, depth+1)
			}
			for _, a := range v.Items {
				if e := value(a); e != nil {
					return e
				}
			}
			return nil
		}
		for _, r := range e.Records {
			for _, v := range r.Args {
				if e := value(v); e != nil {
					return e
				}
			}
		}
		active[id] = false
		out[id] = true
		return nil
	}
	e := visit(id, 0)
	return out, e
}
func analyticSupportRoots(f *File) (int64, int64, int64, []int64, error) {
	fail := func() (int64, int64, int64, []int64, error) {
		return 0, 0, 0, nil, errors.New("step: unsupported mixed support envelope")
	}
	var repID int64
	for _, id := range f.Order {
		x := f.Entities[id]
		if len(x.Records) == 1 && x.Records[0].Type == "REPRESENTATION" {
			if repID != 0 {
				return fail()
			}
			repID = id
		}
	}
	rep, e := simple(f, repID, "REPRESENTATION", 3)
	if e != nil {
		return fail()
	}
	items, e := listRefs(rep.Args[1], 1)
	if e != nil || len(items) != 1 {
		return fail()
	}
	set, e := simple(f, items[0], "GEOMETRIC_SET", 2)
	if e != nil {
		return fail()
	}
	roots, e := listRefs(set.Args[1], 1)
	if e != nil || len(roots) > 64 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	return repID, items[0], ctx, roots, nil
}
func analyticSupportHeader(b *bytes.Buffer) {
	fmt.Fprintln(b, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Independent analytic supports'),'2;1');\nFILE_NAME('supports.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;")
}
func analyticSupportSubset(f *File, keep map[int64]bool, setID int64, roots []int64) []byte {
	var b bytes.Buffer
	analyticSupportHeader(&b)
	identity := map[int64]int64{}
	for id := range keep {
		identity[id] = id
	}
	for _, id := range f.Order {
		if !keep[id] {
			continue
		}
		e := f.Entities[id]
		if id == setID {
			r := e.Records[0]
			r.Args = append([]Value(nil), r.Args...)
			r.Args[1] = Value{Kind: Aggregate}
			for _, sid := range roots {
				r.Args[1].Items = append(r.Args[1].Items, Value{Kind: Reference, Ref: sid})
			}
			e.Records = []Record{r}
		}
		fmt.Fprintf(&b, "#%d=%s;\n", id, assemblyEntity(e, identity))
	}
	fmt.Fprintln(&b, "ENDSEC;\nEND-ISO-10303-21;")
	return b.Bytes()
}

// ReadAnalyticSupports preserves source set order and dispatches only explicit
// PLANE/CYLINDRICAL_SURFACE/SPHERICAL_SURFACE/TOROIDAL_SURFACE roots. It proves
// disjoint source ownership before invoking the strict specialized codecs.
// Limits additionally cap bytes at 4 MiB, entities 512, values 8192 and depth 16.
func ReadAnalyticSupports(r io.Reader, l Limits) (AnalyticSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return AnalyticSupportModel{}, e
	}
	l.MaxBytes = min(l.MaxBytes, 4<<20)
	l.MaxEntities = min(l.MaxEntities, 512)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return AnalyticSupportModel{}, e
	}
	fail := func() (AnalyticSupportModel, error) {
		return AnalyticSupportModel{}, errors.New("step: unsupported mixed analytic supports")
	}
	for _, ent := range f.Entities {
		var check func(Value) bool
		check = func(v Value) bool {
			if v.Kind == String && len(v.Text) > 1024 {
				return false
			}
			for _, a := range v.Items {
				if !check(a) {
					return false
				}
			}
			return true
		}
		for _, rec := range ent.Records {
			for _, v := range rec.Args {
				if !check(v) {
					return fail()
				}
			}
		}
	}
	rep, set, ctx, roots, e := analyticSupportRoots(f)
	if e != nil {
		return fail()
	}
	unit, e := analyticSupportContextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	common, e := analyticSupportClosure(f, ctx)
	if e != nil {
		return fail()
	}
	if common[rep] || common[set] || rep == set {
		return fail()
	}
	common[rep] = true
	common[set] = true
	out := AnalyticSupportModel{LengthUnit: unit}
	out.Name, e = textValue(f.Entities[rep].Records[0].Args[0])
	if e != nil {
		return fail()
	}
	out.SetName, e = textValue(f.Entities[set].Records[0].Args[0])
	if e != nil {
		return fail()
	}
	owned := map[int64]bool{}
	for id := range common {
		owned[id] = true
	}
	closures := make([]map[int64]bool, len(roots))
	kinds := make([]string, len(roots))
	for i, id := range roots {
		x := f.Entities[id]
		if len(x.Records) != 1 {
			return fail()
		}
		kinds[i] = x.Records[0].Type
		switch kinds[i] {
		case "PLANE", "CYLINDRICAL_SURFACE", "SPHERICAL_SURFACE", "TOROIDAL_SURFACE":
		default:
			return fail()
		}
		c, e := analyticSupportClosure(f, id)
		if e != nil {
			return fail()
		}
		for dep := range c {
			if owned[dep] {
				return fail()
			}
			owned[dep] = true
		}
		closures[i] = c
	}
	if len(owned) != len(f.Entities) {
		return fail()
	}
	for i := range roots {
		keep := map[int64]bool{}
		for id := range common {
			keep[id] = true
		}
		for id := range closures[i] {
			keep[id] = true
		}
		b := analyticSupportSubset(f, keep, set, []int64{roots[i]})
		s := AnalyticSupport{}
		switch kinds[i] {
		case "PLANE":
			m, e := ReadPlaneSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Plane = &m.Surfaces[0]
		case "CYLINDRICAL_SURFACE":
			m, e := ReadCylinderSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Cylinder = &m.Surfaces[0]
		case "SPHERICAL_SURFACE":
			m, e := ReadSphereSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Sphere = &m.Surfaces[0]
		case "TOROIDAL_SURFACE":
			m, e := ReadTorusSupports(bytes.NewReader(b), Limits{})
			if e != nil || len(m.Surfaces) != 1 {
				return fail()
			}
			s.Torus = &m.Surfaces[0]
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	return out, nil
}

// WriteAnalyticSupports stages every specialized support, merges only disjoint
// owned geometry under one context, then strictly re-reads before caller I/O.
func WriteAnalyticSupports(w io.Writer, m AnalyticSupportModel) error {
	if w == nil || len(m.Surfaces) < 1 || len(m.Surfaces) > 64 {
		return errors.New("step: invalid mixed supports")
	}
	var first *File
	var firstSet int64
	var roots []int64
	var extras bytes.Buffer
	var next int64
	for _, s := range m.Surfaces {
		n := 0
		for _, v := range []bool{s.Plane != nil, s.Cylinder != nil, s.Sphere != nil, s.Torus != nil} {
			if v {
				n++
			}
		}
		if n != 1 {
			return errors.New("step: exactly one support variant required")
		}
		var b bytes.Buffer
		var e error
		switch {
		case s.Plane != nil:
			e = WritePlaneSupports(&b, PlaneSupportModel{Name: m.Name, SetName: m.SetName, LengthUnit: m.LengthUnit, Surfaces: []PlaneSupport{*s.Plane}})
		case s.Cylinder != nil:
			e = WriteCylinderSupports(&b, CylinderSupportModel{Name: m.Name, SetName: m.SetName, LengthUnit: m.LengthUnit, Surfaces: []CylinderSupport{*s.Cylinder}})
		case s.Sphere != nil:
			e = WriteSphereSupports(&b, SphereSupportModel{Name: m.Name, SetName: m.SetName, LengthUnit: m.LengthUnit, Surfaces: []SphereSupport{*s.Sphere}})
		case s.Torus != nil:
			e = WriteTorusSupports(&b, TorusSupportModel{Name: m.Name, SetName: m.SetName, LengthUnit: m.LengthUnit, Surfaces: []TorusSupport{*s.Torus}})
		}
		if e != nil {
			return e
		}
		f, e := Parse(&b, Limits{})
		if e != nil {
			return e
		}
		_, set, _, ids, e := analyticSupportRoots(f)
		if e != nil || len(ids) != 1 {
			return errors.New("step: staged support root")
		}
		if first == nil {
			first = f
			firstSet = set
			roots = append(roots, ids[0])
			for id := range f.Entities {
				next = max(next, id)
			}
			continue
		}
		c, e := analyticSupportClosure(f, ids[0])
		if e != nil {
			return e
		}
		remap := map[int64]int64{}
		for _, id := range f.Order {
			if c[id] {
				next++
				remap[id] = next
			}
		}
		for _, id := range f.Order {
			if c[id] {
				fmt.Fprintf(&extras, "#%d=%s;\n", remap[id], assemblyEntity(f.Entities[id], remap))
			}
		}
		roots = append(roots, remap[ids[0]])
	}
	keep := map[int64]bool{}
	for id := range first.Entities {
		keep[id] = true
	}
	for _, id := range roots {
		keep[id] = true
	}
	raw := analyticSupportSubset(first, keep, firstSet, roots)
	suffix := []byte("ENDSEC;\nEND-ISO-10303-21;\n")
	if !bytes.HasSuffix(raw, suffix) {
		return errors.New("step: internal support envelope")
	}
	raw = append(raw[:len(raw)-len(suffix)], extras.Bytes()...)
	raw = append(raw, suffix...)
	if _, e := ReadAnalyticSupports(bytes.NewReader(raw), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}
