package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
)

// EllipticalWire is one complete natural ellipse in a rooted geometric curve
// set. It is not a face or solid, and contains no finite authored trim interval.
// Geometry uses radians; the exact natural period is mathematical 2π.
// Source is caller-owned. Source re-export revalidates it before writing.
type EllipticalWire struct {
	Name                                                       string
	Geometry                                                   geom.Ellipse
	Location, Axis, Reference                                  geom.Vec3
	MajorRadius, MinorRadius, LengthUnit                       float64
	Source                                                     *File
	ProductID, RepresentationID, SetID, EllipseID, PlacementID int64
}

func ellipticalWireFailure() error {
	return fmt.Errorf("step: unsupported exact natural elliptical wire graph")
}

// ReadEllipticalWire accepts the original OCCT AP214 rooted single natural
// ellipse graph, with signed-cardinal frame, millimeter units and exact source
// coefficients. All 26 records must belong to the proved owner graph. Entity
// numbering and string metadata may differ; trims, maps and detached data fail.
func ReadEllipticalWire(r io.Reader, l Limits) (EllipticalWire, error) {
	fail := func() (EllipticalWire, error) { return EllipticalWire{}, ellipticalWireFailure() }
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 26)
	l.MaxValues = min(l.MaxValues, 2048)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return EllipticalWire{}, e
	}
	if len(f.Entities) != 26 {
		return fail()
	}
	var eid, root, protocol, category int64
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		switch en.Records[0].Type {
		case "ELLIPSE":
			if eid != 0 {
				return fail()
			}
			eid = id
		case "SHAPE_DEFINITION_REPRESENTATION":
			if root != 0 {
				return fail()
			}
			root = id
		case "APPLICATION_PROTOCOL_DEFINITION":
			if protocol != 0 {
				return fail()
			}
			protocol = id
		case "PRODUCT_RELATED_PRODUCT_CATEGORY":
			if category != 0 {
				return fail()
			}
			category = id
		}
	}
	er, e := simple(f, eid, "ELLIPSE", 4)
	if e != nil {
		return fail()
	}
	pid, e := ref(er.Args[1])
	if e != nil {
		return fail()
	}
	frame, e := mappedExactFrame(f, pid)
	if e != nil {
		return fail()
	}
	a, e := cylinderSupportNumber(er.Args[2])
	if e != nil {
		return fail()
	}
	b, e := cylinderSupportNumber(er.Args[3])
	if e != nil || a <= b || b <= 0 {
		return fail()
	}
	curve, e := geom.NewEllipse(frame, a, b)
	if e != nil {
		return fail()
	}
	var buf bytes.Buffer
	analyticSupportHeader(&buf)
	buf.WriteString(ellipseWireTemplate)
	fmt.Fprintln(&buf, "ENDSEC;END-ISO-10303-21;")
	want, e := Parse(&buf, Limits{})
	if e != nil {
		return fail()
	}
	// Expected geometry is reconstructed from exact decoded coefficients; frame
	// numbers are never normalized approximately. The template supplies topology.
	setVec := func(id int64, v geom.Vec3) {
		en := want.Entities[id]
		for i, x := range []float64{v.X, v.Y, v.Z} {
			en.Records[0].Args[1].Items[i].Text = strings.TrimRight(strings.TrimRight(new(big.Rat).SetFloat64(x).FloatString(1074), "0"), ".")
		}
		want.Entities[id] = en
	}
	setVec(18, frame.Origin())
	setVec(19, frame.ZAxis())
	setVec(20, frame.XAxis())
	en := want.Entities[16]
	en.Records[0].Args[2].Text = strings.TrimRight(strings.TrimRight(new(big.Rat).SetFloat64(a).FloatString(1074), "0"), ".")
	en.Records[0].Args[3].Text = strings.TrimRight(strings.TrimRight(new(big.Rat).SetFloat64(b).FloatString(1074), "0"), ".")
	want.Entities[16] = en
	mapping, reverse := map[int64]int64{}, map[int64]int64{}
	var matchEntity func(int64, int64) bool
	var matchValue func(Value, Value) bool
	matchValue = func(w, v Value) bool {
		if w.Kind != v.Kind {
			return false
		}
		switch w.Kind {
		case Reference:
			return matchEntity(w.Ref, v.Ref)
		case String:
			return len(v.Text) <= 1024
		case Number:
			wr, we := liftSourceRat(w)
			vr, ve := liftSourceRat(v)
			return we == nil && ve == nil && wr.Cmp(vr) == 0
		}
		if w.Text != v.Text || len(w.Items) != len(v.Items) {
			return false
		}
		for i := range w.Items {
			if !matchValue(w.Items[i], v.Items[i]) {
				return false
			}
		}
		return true
	}
	matchEntity = func(wid, id int64) bool {
		if old, ok := mapping[wid]; ok {
			return old == id
		}
		if _, ok := reverse[id]; ok {
			return false
		}
		w, ok := want.Entities[wid]
		if !ok {
			return false
		}
		v, ok := f.Entities[id]
		if !ok || len(w.Records) != len(v.Records) {
			return false
		}
		mapping[wid] = id
		reverse[id] = wid
		for i, wr := range w.Records {
			vr := v.Records[i]
			if wr.Type != vr.Type || len(wr.Args) != len(vr.Args) {
				return false
			}
			for j := range wr.Args {
				if !matchValue(wr.Args[j], vr.Args[j]) {
					return false
				}
			}
		}
		return true
	}
	if !matchEntity(3, root) || !matchEntity(1, protocol) || !matchEntity(26, category) || len(mapping) != 26 || mapping[16] != eid || mapping[17] != pid {
		return fail()
	}
	unit, e := contextUnits(f, mapping[21])
	if e != nil || unit != .001 {
		return fail()
	}
	pr, e := simple(f, mapping[7], "PRODUCT", 4)
	if e != nil {
		return fail()
	}
	return EllipticalWire{Name: pr.Args[1].Text, Geometry: curve, Location: frame.Origin(), Axis: frame.ZAxis(), Reference: frame.XAxis(), MajorRadius: a, MinorRadius: b, LengthUnit: unit, Source: f, ProductID: mapping[7], RepresentationID: mapping[10], SetID: mapping[15], EllipseID: eid, PlacementID: pid}, nil
}

// WriteEllipticalWireSource revalidates and re-emits the complete original
// source entity graph. It does not bake edits to the returned Geometry fields.
func WriteEllipticalWireSource(w io.Writer, f *File, l Limits) error {
	if w == nil || f == nil {
		return ellipticalWireFailure()
	}
	if len(f.Entities) != 26 || len(f.Order) != 26 || len(f.Header) > 64 {
		return ellipticalWireFailure()
	}
	budget, textBudget := 2048, 32768
	var bounded func(Value, int) bool
	bounded = func(v Value, depth int) bool {
		budget--
		textBudget -= len(v.Text)
		if budget < 0 || textBudget < 0 || depth > 16 || len(v.Text) > 1024 {
			return false
		}
		for _, a := range v.Items {
			if !bounded(a, depth+1) {
				return false
			}
		}
		return true
	}
	records := append([]Record(nil), f.Header...)
	for _, en := range f.Entities {
		if len(en.Records) > 4 {
			return ellipticalWireFailure()
		}
		records = append(records, en.Records...)
	}
	for _, rec := range records {
		if len(rec.Type) > 128 {
			return ellipticalWireFailure()
		}
		for _, v := range rec.Args {
			if !bounded(v, 0) {
				return ellipticalWireFailure()
			}
		}
	}
	raw, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}

	if _, e := ReadEllipticalWire(bytes.NewReader(raw), l); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return e
}

const ellipseWireTemplate = `

#1 = APPLICATION_PROTOCOL_DEFINITION('international standard',
  'automotive_design',2000,#2);
#2 = APPLICATION_CONTEXT(
  'core data for automotive mechanical design processes');
#3 = SHAPE_DEFINITION_REPRESENTATION(#4,#10);
#4 = PRODUCT_DEFINITION_SHAPE('','',#5);
#5 = PRODUCT_DEFINITION('design','',#6,#9);
#6 = PRODUCT_DEFINITION_FORMATION('','',#7);
#7 = PRODUCT('Open CASCADE STEP translator 8.1 1',
  'Open CASCADE STEP translator 8.1 1','',(#8));
#8 = PRODUCT_CONTEXT('',#2,'mechanical');
#9 = PRODUCT_DEFINITION_CONTEXT('part definition',#2,'design');
#10 = GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION('',(#11,#15),
  #21);
#11 = AXIS2_PLACEMENT_3D('',#12,#13,#14);
#12 = CARTESIAN_POINT('',(0.,0.,0.));
#13 = DIRECTION('',(0.,0.,1.));
#14 = DIRECTION('',(1.,0.,-0.));
#15 = GEOMETRIC_CURVE_SET('',(#16));
#16 = ELLIPSE('',#17,4.,2.);
#17 = AXIS2_PLACEMENT_3D('',#18,#19,#20);
#18 = CARTESIAN_POINT('',(1.,2.,3.));
#19 = DIRECTION('',(0.,1.,0.));
#20 = DIRECTION('',(0.,0.,-1.));
#21 = ( GEOMETRIC_REPRESENTATION_CONTEXT(3)
GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT((#25)) GLOBAL_UNIT_ASSIGNED_CONTEXT(
(#22,#23,#24)) REPRESENTATION_CONTEXT('Context #1',
  '3D Context with UNIT and UNCERTAINTY') );
#22 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );
#23 = ( NAMED_UNIT(*) PLANE_ANGLE_UNIT() SI_UNIT($,.RADIAN.) );
#24 = ( NAMED_UNIT(*) SI_UNIT($,.STERADIAN.) SOLID_ANGLE_UNIT() );
#25 = UNCERTAINTY_MEASURE_WITH_UNIT(LENGTH_MEASURE(1.E-07),#22,
  'distance_accuracy_value','confusion accuracy');
#26 = PRODUCT_RELATED_PRODUCT_CATEGORY('part',$,(#7));

`
