package step

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// AssemblyPart stores named local planar solids. An assembly-only product has
// no Bodies and at least one child occurrence. IDs are nonempty unique strings.
type AssemblyPart struct {
	ID, Name string
	Bodies   []Body
}

// AssemblyInstance explicitly places one Child product in Parent coordinates.
// Transform must be finite, rigid and orientation-preserving within 1e-10.
type AssemblyInstance struct {
	ID, Name, Parent, Child string
	Transform               geom.Affine
}
type AssemblyModel struct {
	LengthUnit  float64
	Products    []AssemblyPart
	Occurrences []AssemblyInstance
}

func rigidFrame(m geom.Affine) (geom.Frame, error) {
	fail := func() (geom.Frame, error) {
		return geom.Frame{}, errors.New("step: assembly placement must be proper rigid")
	}
	if !m.IsFinite() {
		return fail()
	}
	x, y, z := m.ApplyVector(geom.Vec3{X: 1}), m.ApplyVector(geom.Vec3{Y: 1}), m.ApplyVector(geom.Vec3{Z: 1})
	if math.Abs(x.Length()-1) > 1e-10 || math.Abs(y.Length()-1) > 1e-10 || math.Abs(z.Length()-1) > 1e-10 || math.Abs(x.Dot(y)) > 1e-10 || math.Abs(x.Dot(z)) > 1e-10 || math.Abs(y.Dot(z)) > 1e-10 || math.Abs(m.Determinant()-1) > 1e-10 {
		return fail()
	}
	return geom.NewFrame(m.Apply(geom.Vec3{}), x, z)
}
func assemblyValue(v Value, ids map[int64]int64) string {
	switch v.Kind {
	case Omitted:
		return "$"
	case Derived:
		return "*"
	case Number:
		return v.Text
	case String:
		return quote(v.Text)
	case Enumeration:
		return "." + v.Text + "."
	case Reference:
		return fmt.Sprintf("#%d", ids[v.Ref])
	case Aggregate, Typed:
		values := make([]string, len(v.Items))
		for i, x := range v.Items {
			values[i] = assemblyValue(x, ids)
		}
		prefix := ""
		if v.Kind == Typed {
			prefix = v.Text
		}
		return prefix + "(" + strings.Join(values, ",") + ")"
	}
	panic("unreachable parsed STEP value")
}
func assemblyEntity(e Entity, ids map[int64]int64) string {
	records := make([]string, len(e.Records))
	for i, r := range e.Records {
		args := make([]string, len(r.Args))
		for j, v := range r.Args {
			args[j] = assemblyValue(v, ids)
		}
		records[i] = r.Type + "(" + strings.Join(args, ",") + ")"
	}
	if len(records) == 1 {
		return records[0]
	}
	return "(" + strings.Join(records, " ") + ")"
}

// WriteAssembly emits a named rigid AP214 product/occurrence graph containing
// local planar advanced BReps. Repeated/nested products are retained; occurrences
// are never flattened. Geometry staging uses the bounded STEP parser defaults.
func WriteAssembly(w io.Writer, m AssemblyModel) error {
	counts := make([]int, len(m.Products))
	for i, p := range m.Products {
		counts[i] = len(p.Bodies)
	}
	return writeAssemblyGeometry(w, m, counts, func(i int) (*File, []int64, error) {
		var b bytes.Buffer
		if e := WriteAdvanced(&b, Model{Bodies: m.Products[i].Bodies, LengthUnit: m.LengthUnit}); e != nil {
			return nil, nil, e
		}
		return stagedAssemblyBodies(&b)
	})
}
func stagedAssemblyBodies(r io.Reader) (*File, []int64, error) {
	f, e := Parse(r, Limits{})
	if e != nil {
		return nil, nil, e
	}
	ids := []int64{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) == 1 && ent.Records[0].Type == "MANIFOLD_SOLID_BREP" {
			ids = append(ids, id)
		}
	}
	return f, ids, nil
}
func writeAssemblyGeometry(w io.Writer, m AssemblyModel, bodyCounts []int, stage func(int) (*File, []int64, error)) error {
	fail := func() error { return errors.New("step: invalid or unsupported assembly model") }
	if w == nil || len(m.Products) == 0 || len(m.Occurrences) == 0 {
		return fail()
	}
	if !validLengthUnit(m.LengthUnit) {
		return errors.New("step: invalid length unit")
	}
	products := map[string]int{}
	for i, p := range m.Products {
		if p.ID == "" || !utf8.ValidString(p.ID) || !utf8.ValidString(p.Name) {
			return fail()
		}
		if _, ok := products[p.ID]; ok {
			return fail()
		}
		products[p.ID] = i
	}
	incoming := make([]int, len(m.Products))
	children := make([][]int, len(m.Products))
	frames := make([]geom.Frame, len(m.Occurrences))
	seen := map[string]bool{}
	for i, o := range m.Occurrences {
		p, pok := products[o.Parent]
		c, cok := products[o.Child]
		if !pok || !cok || p == c || o.ID == "" || seen[o.ID] || !utf8.ValidString(o.ID) || !utf8.ValidString(o.Name) {
			return fail()
		}
		seen[o.ID] = true
		frame, e := rigidFrame(o.Transform)
		if e != nil {
			return e
		}
		frames[i] = frame
		incoming[c]++
		children[p] = append(children[p], c)
	}
	queue := []int{}
	for i := range incoming {
		if incoming[i] == 0 {
			queue = append(queue, i)
		}
		if len(children[i]) == 0 && bodyCounts[i] == 0 {
			return fail()
		}
	}
	visited := 0
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		visited++
		for _, c := range children[i] {
			incoming[c]--
			if incoming[c] == 0 {
				queue = append(queue, c)
			}
		}
	}
	if visited != len(m.Products) {
		return fail()
	}
	sources := make([]*File, len(m.Products))
	bodyRefs := make([][]int64, len(m.Products))
	for i := range m.Products {
		if bodyCounts[i] == 0 {
			continue
		}
		f, ids, e := stage(i)
		if e != nil {
			return e
		}
		if len(ids) != bodyCounts[i] {
			return errors.New("step: staged assembly body count mismatch")
		}
		sources[i], bodyRefs[i] = f, ids
	}
	out := bufio.NewWriter(w)
	var writeErr error
	line := func(format string, args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(out, format+"\n", args...)
		}
	}
	next := int64(1)
	emit := func(s string) int64 { id := next; next++; line("#%d=%s;", id, s); return id }
	line("ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('PlainSolid rigid assembly'),'2;1');\nFILE_NAME('assembly.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	length := emitLengthUnit(emit, m.LengthUnit)
	angle := emit("(NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.))")
	solid := emit("(NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT())")
	ctx := emit(fmt.Sprintf("(GEOMETRIC_REPRESENTATION_CONTEXT(3) GLOBAL_UNIT_ASSIGNED_CONTEXT((#%d,#%d,#%d)) REPRESENTATION_CONTEXT('',''))", length, angle, solid))
	app := emit("APPLICATION_CONTEXT('core data for automotive mechanical design processes')")
	emit(fmt.Sprintf("APPLICATION_PROTOCOL_DEFINITION('international standard','automotive_design',2000,#%d)", app))
	pc := emit(fmt.Sprintf("PRODUCT_CONTEXT('',#%d,'mechanical')", app))
	dc := emit(fmt.Sprintf("PRODUCT_DEFINITION_CONTEXT('part definition',#%d,'design')", app))
	place := func(f geom.Frame) int64 {
		o, x, z := f.Origin(), f.XAxis(), f.ZAxis()
		p := emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(o.X), real(o.Y), real(o.Z)))
		axis := emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(z.X), real(z.Y), real(z.Z)))
		ref := emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(x.X), real(x.Y), real(x.Z)))
		return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, axis, ref))
	}
	identity, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	origin := place(identity)
	items := make([][]int64, len(m.Products))
	for i := range items {
		items[i] = []int64{origin}
	}
	targets := make([]int64, len(frames))
	for i, f := range frames {
		targets[i] = place(f)
		p := products[m.Occurrences[i].Parent]
		items[p] = append(items[p], targets[i])
	}
	defs, reps := make([]int64, len(m.Products)), make([]int64, len(m.Products))
	for i, p := range m.Products {
		if f := sources[i]; f != nil {
			reachable := map[int64]bool{}
			todo := append([]int64(nil), bodyRefs[i]...)
			var refs func(Value)
			refs = func(v Value) {
				if v.Kind == Reference {
					todo = append(todo, v.Ref)
				}
				for _, x := range v.Items {
					refs(x)
				}
			}
			for len(todo) > 0 {
				id := todo[len(todo)-1]
				todo = todo[:len(todo)-1]
				if reachable[id] {
					continue
				}
				reachable[id] = true
				for _, r := range f.Entities[id].Records {
					for _, v := range r.Args {
						refs(v)
					}
				}
			}
			remap := map[int64]int64{}
			for _, id := range f.Order {
				if reachable[id] {
					remap[id] = next
					next++
				}
			}
			for _, id := range f.Order {
				if reachable[id] {
					line("#%d=%s;", remap[id], assemblyEntity(f.Entities[id], remap))
				}
			}
			for _, id := range bodyRefs[i] {
				items[i] = append(items[i], remap[id])
			}
		}
		kind := "SHAPE_REPRESENTATION"
		if bodyCounts[i] > 0 {
			kind = "ADVANCED_BREP_SHAPE_REPRESENTATION"
		}
		reps[i] = emit(fmt.Sprintf("%s('',%s,#%d)", kind, references(items[i]), ctx))
		product := emit(fmt.Sprintf("PRODUCT(%s,%s,'',(#%d))", quote(p.ID), quote(p.Name), pc))
		formation := emit(fmt.Sprintf("PRODUCT_DEFINITION_FORMATION('','',#%d)", product))
		defs[i] = emit(fmt.Sprintf("PRODUCT_DEFINITION('design','',#%d,#%d)", formation, dc))
		shape := emit(fmt.Sprintf("PRODUCT_DEFINITION_SHAPE('','',#%d)", defs[i]))
		emit(fmt.Sprintf("SHAPE_DEFINITION_REPRESENTATION(#%d,#%d)", shape, reps[i]))
	}
	for i, o := range m.Occurrences {
		p, c := products[o.Parent], products[o.Child]
		n := emit(fmt.Sprintf("NEXT_ASSEMBLY_USAGE_OCCURRENCE(%s,%s,'',#%d,#%d,$)", quote(o.ID), quote(o.Name), defs[p], defs[c]))
		shape := emit(fmt.Sprintf("PRODUCT_DEFINITION_SHAPE('','',#%d)", n))
		tr := emit(fmt.Sprintf("ITEM_DEFINED_TRANSFORMATION('','',#%d,#%d)", origin, targets[i]))
		rel := emit(fmt.Sprintf("(REPRESENTATION_RELATIONSHIP('','',#%d,#%d) REPRESENTATION_RELATIONSHIP_WITH_TRANSFORMATION(#%d) SHAPE_REPRESENTATION_RELATIONSHIP())", reps[c], reps[p], tr))
		emit(fmt.Sprintf("CONTEXT_DEPENDENT_SHAPE_REPRESENTATION(#%d,#%d)", rel, shape))
	}
	line("ENDSEC;\nEND-ISO-10303-21;")
	if writeErr != nil {
		return writeErr
	}
	return out.Flush()
}
