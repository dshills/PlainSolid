package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"plainsolid/geom"
	"unicode/utf8"
)

// PlaneSupport is an unbounded analytic support, not a finite plane body.
// U and V are signed distances along Reference and Normal×Reference.
// Only perpendicular signed coordinate unit axes are accepted, without snapping.
type PlaneSupport struct {
	Name, PlacementName, LocationName, NormalName, ReferenceName string
	Location, Normal, Reference                                  geom.Vec3
}
type PlaneSupportModel struct {
	Name, SetName string
	LengthUnit    float64
	Surfaces      []PlaneSupport
}

func planeSupportUnit(v geom.Vec3) bool {
	n := 0
	for _, x := range []float64{v.X, v.Y, v.Z} {
		if x != 0 {
			if x != 1 && x != -1 {
				return false
			}
			n++
		}
	}
	return n == 1
}
func (s PlaneSupport) Geometry() (geom.PlaneSurface, error) {
	if !s.Location.IsFinite() || !planeSupportUnit(s.Normal) || !planeSupportUnit(s.Reference) || s.Normal.Dot(s.Reference) != 0 {
		return geom.PlaneSurface{}, geom.ErrDomain
	}
	f, e := geom.NewFrame(s.Location, s.Reference, s.Normal)
	if e != nil {
		return geom.PlaneSurface{}, e
	}
	return geom.NewPlaneSurface(f)
}
func planeSupportNumber(v Value) (float64, error) {
	r, e := liftSourceRat(v)
	if e != nil {
		return 0, e
	}
	x, exact := r.Float64()
	if !exact || !finiteNumber(x) {
		return 0, errors.New("step: plane support scalar would round")
	}
	return x, nil
}

// ReadPlaneSupports accepts one generic REPRESENTATION/GEOMETRIC_SET graph,
// explicit units and uniquely owned plane supports. No trim, face, solid,
// transformation, product or assembly graph is interpreted by this codec.
func ReadPlaneSupports(r io.Reader, l Limits) (PlaneSupportModel, error) {
	l, e := l.defaults()
	if e != nil {
		return PlaneSupportModel{}, e
	}
	l.MaxEntities = min(l.MaxEntities, 512)
	f, e := Parse(r, l)
	if e != nil {
		return PlaneSupportModel{}, e
	}
	fail := func() (PlaneSupportModel, error) {
		return PlaneSupportModel{}, errors.New("step: unsupported plane support graph")
	}
	var repID int64
	for id, x := range f.Entities {
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
	name, e := textValue(rep.Args[0])
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
	setName, e := textValue(set.Args[0])
	if e != nil {
		return fail()
	}
	ids, e := listRefs(set.Args[1], 1)
	if e != nil || len(ids) > 64 {
		return fail()
	}
	ctx, e := ref(rep.Args[2])
	if e != nil {
		return fail()
	}
	unit, e := contextUnits(f, ctx)
	if e != nil {
		return fail()
	}
	used := map[int64]bool{repID: true, items[0]: true}
	allowed := map[string]bool{"GEOMETRIC_REPRESENTATION_CONTEXT": true, "GLOBAL_UNIT_ASSIGNED_CONTEXT": true, "REPRESENTATION_CONTEXT": true, "LENGTH_UNIT": true, "NAMED_UNIT": true, "SI_UNIT": true, "PLANE_ANGLE_UNIT": true, "SOLID_ANGLE_UNIT": true, "CONVERSION_BASED_UNIT": true, "LENGTH_MEASURE_WITH_UNIT": true, "MEASURE_WITH_UNIT": true, "DIMENSIONAL_EXPONENTS": true}
	visiting := map[int64]bool{}
	var mark func(int64) error
	mark = func(id int64) error {
		if visiting[id] {
			return geom.ErrDomain
		}
		if used[id] {
			return nil
		}
		x, ok := f.Entities[id]
		if !ok {
			return geom.ErrDomain
		}
		visiting[id] = true
		for _, rec := range x.Records {
			if !allowed[rec.Type] {
				return geom.ErrDomain
			}
			var walk func(Value) error
			walk = func(v Value) error {
				if v.Kind == Reference {
					return mark(v.Ref)
				}
				for _, a := range v.Items {
					if e := walk(a); e != nil {
						return e
					}
				}
				return nil
			}
			for _, v := range rec.Args {
				if e := walk(v); e != nil {
					return e
				}
			}
		}
		visiting[id] = false
		used[id] = true
		return nil
	}
	if e = mark(ctx); e != nil {
		return fail()
	}
	take := func(id int64, kind string, n int) (Record, error) {
		if used[id] {
			return Record{}, geom.ErrDomain
		}
		rec, e := simple(f, id, kind, n)
		if e == nil {
			used[id] = true
		}
		return rec, e
	}
	vector := func(id int64, kind string) (geom.Vec3, string, error) {
		x, e := take(id, kind, 2)
		if e != nil {
			return geom.Vec3{}, "", e
		}
		name, e := textValue(x.Args[0])
		if e != nil || x.Args[1].Kind != Aggregate || len(x.Args[1].Items) != 3 {
			return geom.Vec3{}, "", geom.ErrDomain
		}
		var a [3]float64
		for i, v := range x.Args[1].Items {
			a[i], e = planeSupportNumber(v)
			if e != nil {
				return geom.Vec3{}, "", e
			}
		}
		return geom.Vec3{X: a[0], Y: a[1], Z: a[2]}, name, nil
	}
	out := PlaneSupportModel{Name: name, SetName: setName, LengthUnit: unit}
	for _, id := range ids {
		x, e := take(id, "PLANE", 2)
		if e != nil {
			return fail()
		}
		var s PlaneSupport
		s.Name, e = textValue(x.Args[0])
		if e != nil {
			return fail()
		}
		pid, e := ref(x.Args[1])
		if e != nil {
			return fail()
		}
		p, e := take(pid, "AXIS2_PLACEMENT_3D", 4)
		if e != nil {
			return fail()
		}
		s.PlacementName, e = textValue(p.Args[0])
		if e != nil {
			return fail()
		}
		var refs [3]int64
		for i := range refs {
			refs[i], e = ref(p.Args[i+1])
			if e != nil {
				return fail()
			}
		}
		s.Location, s.LocationName, e = vector(refs[0], "CARTESIAN_POINT")
		if e != nil {
			return fail()
		}
		s.Normal, s.NormalName, e = vector(refs[1], "DIRECTION")
		if e != nil {
			return fail()
		}
		s.Reference, s.ReferenceName, e = vector(refs[2], "DIRECTION")
		if e != nil {
			return fail()
		}
		if _, e = s.Geometry(); e != nil {
			return fail()
		}
		out.Surfaces = append(out.Surfaces, s)
	}
	if len(used) != len(f.Entities) {
		return fail()
	}
	return out, nil
}

// WritePlaneSupports emits an unbounded generic geometry representation and
// stages strict read-back before I/O. It emits no product, topology or solid.
func WritePlaneSupports(w io.Writer, m PlaneSupportModel) error {
	if w == nil || len(m.Surfaces) < 1 || len(m.Surfaces) > 64 || !validLengthUnit(m.LengthUnit) {
		return geom.ErrDomain
	}
	names := []string{m.Name, m.SetName}
	for _, s := range m.Surfaces {
		if _, e := s.Geometry(); e != nil {
			return e
		}
		names = append(names, s.Name, s.PlacementName, s.LocationName, s.NormalName, s.ReferenceName)
	}
	for _, s := range names {
		if !utf8.ValidString(s) || len(s) > 1024 {
			return geom.ErrDomain
		}
	}
	var b bytes.Buffer
	fmt.Fprintln(&b, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Unbounded plane supports'),'2;1');\nFILE_NAME('supports.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('CONFIG_CONTROL_DESIGN'));\nENDSEC;\nDATA;")
	next := int64(0)
	emit := func(s string) int64 { next++; fmt.Fprintf(&b, "#%d=%s;\n", next, s); return next }
	length := emitLengthUnit(emit, m.LengthUnit)
	angle := emit("(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))")
	solid := emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
	ctx := emit(fmt.Sprintf("(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#%d,#%d,#%d)) REPRESENTATION_CONTEXT('',''))", length, angle, solid))
	vector := func(kind, name string, v geom.Vec3) int64 {
		return emit(fmt.Sprintf("%s(%s,(%s,%s,%s))", kind, quote(name), real(v.X), real(v.Y), real(v.Z)))
	}
	var ids []int64
	for _, s := range m.Surfaces {
		p := vector("CARTESIAN_POINT", s.LocationName, s.Location)
		a := vector("DIRECTION", s.NormalName, s.Normal)
		x := vector("DIRECTION", s.ReferenceName, s.Reference)
		frame := emit(fmt.Sprintf("AXIS2_PLACEMENT_3D(%s,#%d,#%d,#%d)", quote(s.PlacementName), p, a, x))
		ids = append(ids, emit(fmt.Sprintf("PLANE(%s,#%d)", quote(s.Name), frame)))
	}
	set := emit(fmt.Sprintf("GEOMETRIC_SET(%s,%s)", quote(m.SetName), references(ids)))
	emit(fmt.Sprintf("REPRESENTATION(%s,(#%d),#%d)", quote(m.Name), set, ctx))
	fmt.Fprintln(&b, "ENDSEC;\nEND-ISO-10303-21;")
	if _, e := ReadPlaneSupports(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		return e
	}
	n, e := w.Write(b.Bytes())
	if e == nil && n != b.Len() {
		return io.ErrShortWrite
	}
	return e
}
