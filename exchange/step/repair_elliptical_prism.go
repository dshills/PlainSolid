package step

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// RepairedEllipticalPrismShell retains the authenticated original closed STEP
// graph and a reconstructed three-face boundary. Faces are lateral, lower cap,
// upper cap; shared local edges are seam, upper rim, lower rim. This is a shell
// exchange descriptor, not a trimmed.Solid or a general material classifier.
// The only repair is the lateral periodic seam described by Side.Bounds.
// Side.Source is an independently revalidated normalized proof partition; its
// IDs are local to that partition. Source and Source*IDs retain original roles.
type RepairedEllipticalPrismShell struct {
	SourceEdgeIDs               [3]int64
	SourceSeamPCurveIDs         [2]int64
	Name                        string
	Faces                       [3]trimmed.Face
	Side                        RepairedEllipseExtrusionFace
	Source                      *File
	SourceBodyID, SourceShellID int64
	SourceFaceIDs               [3]int64
	SourceRimPCurveIDs          [2][2]int64 // upper/lower, lateral/cap
}

// RepairRoundedEllipticalPrismShell accepts one original 121-record OCCT prism
// with two exact ellipse disk caps sharing both natural model rims. Every cap
// pcurve is proved to lift to its shared original ellipse for every angle;
// shell ownership, multiplicities and outward source senses are authenticated.
// It returns no partial result and does not reinterpret source uncertainty as
// permission to repair. Caller MaxDistance bounds only the periodic side repair.
func RepairRoundedEllipticalPrismShell(r io.Reader, l Limits, o EllipticalWireRepairOptions) (RepairedEllipticalPrismShell, error) {
	fail := func() (RepairedEllipticalPrismShell, error) {
		return RepairedEllipticalPrismShell{}, fmt.Errorf("step: unsupported elliptical prism shell reconstruction")
	}
	if r == nil {
		return fail()
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 121)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return RepairedEllipticalPrismShell{}, e
	}
	if len(f.Entities) != 121 {
		return fail()
	}
	parseTemplate := func(s string) (*File, error) {
		var b bytes.Buffer
		analyticSupportHeader(&b)
		b.WriteString(s)
		b.WriteString("ENDSEC;END-ISO-10303-21;")
		return Parse(&b, Limits{})
	}
	want, e := parseTemplate(ellipticalPrismTemplate)
	if e != nil {
		return fail()
	}
	roots := [][2]int64{{1, 0}, {3, 0}, {121, 0}}
	kinds := []string{"APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_RELATED_PRODUCT_CATEGORY"}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		for i, k := range kinds {
			if en.Records[0].Type == k {
				if roots[i][1] != 0 {
					return fail()
				}
				roots[i][1] = id
			}
		}
	}
	roles, ok := ellipsePrismDiscovery(want, f, roots)
	if !ok {
		return fail()
	}
	// Map the authentic side subgraph into the already-qualified face codec.
	// This is a private normalized proof partition, not a replacement source.
	sideTemplate, e := parseTemplate(ellipseExtrusionTemplate)
	if e != nil {
		return fail()
	}
	role := map[int64]int64{}
	for i := int64(1); i <= 67; i++ {
		role[i] = i
	}
	for i := int64(68); i <= 83; i++ {
		role[i] = i + 12
	}
	for i := int64(84); i <= 89; i++ {
		role[i] = i + 32
	}
	inverse := map[int64]int64{}
	for local, full := range role {
		inverse[roles[full]] = local
	}
	var copyValue func(Value) (Value, bool)
	copyValue = func(v Value) (Value, bool) {
		if v.Kind == Reference {
			id, ok := inverse[v.Ref]
			if !ok {
				return Value{}, false
			}
			v.Ref = id
		}
		items := make([]Value, len(v.Items))
		for i, x := range v.Items {
			var ok bool
			items[i], ok = copyValue(x)
			if !ok {
				return Value{}, false
			}
		}
		v.Items = items
		return v, true
	}
	for local, full := range role {
		actual := f.Entities[roles[full]]
		en := sideTemplate.Entities[local]
		for i, rec := range actual.Records {
			for j, v := range rec.Args {
				// Adapt only declared topology wrapper, owner lists and source face senses.
				if local == 15 && j == 1 {
					v = Value{Kind: Aggregate, Items: []Value{v}}
				}
				if local == 16 && j == 1 {
					v.Items = v.Items[:1]
				}
				if (local == 55 || local == 71) && j == 2 {
					v.Items = v.Items[:1]
				}
				if (local == 17 || local == 18) && j == len(rec.Args)-1 {
					continue
				}
				// Wrapper type changes have the same argument/reference layout.
				cv, ok := copyValue(v)
				if !ok {
					return fail()
				}
				en.Records[i].Args[j] = cv
			}
		}
		sideTemplate.Entities[local] = en
	}
	sideRaw, e := mappedPlanarSourceBytes(sideTemplate)
	if e != nil {
		return fail()
	}
	side, e := RepairRoundedEllipseExtrusionFace(bytes.NewReader(sideRaw), Limits{}, o)
	if e != nil {
		return RepairedEllipticalPrismShell{}, e
	}
	// All variable numeric fields retained in the side projection were proved by
	// that codec. Transfer ONLY scalar leaves into the complete expected graph;
	// references, topology flags and cap ownership remain the original template.
	var scalars func(Value, Value) Value
	scalars = func(w, v Value) Value {
		if w.Kind == Number || w.Kind == String {
			return v
		}
		for i := range w.Items {
			w.Items[i] = scalars(w.Items[i], v.Items[i])
		}
		return w
	}
	for local, full := range role {
		dst := want.Entities[full]
		src := sideTemplate.Entities[local]
		for i := range dst.Records {
			for j := range dst.Records[i].Args {
				if (local == 55 || local == 71) && j == 2 || (local == 15 || local == 16) && j == 1 {
					continue
				}
				dst.Records[i].Args[j] = scalars(dst.Records[i].Args[j], src.Records[i].Args[j])
			}
		}
		want.Entities[full] = dst
	}
	// Plane cap frame equals its model rim frame, and its UV ellipse has the
	// same radii and canonical zero origin/reference. This is continuous lift
	// identity, not a sampled cap fit.
	for dst, src := range map[int64]int64{71: 58, 72: 59, 73: 60, 99: 86, 100: 87, 101: 88} {
		d := want.Entities[dst]
		d.Records[0].Args[1] = want.Entities[src].Records[0].Args[1]
		want.Entities[dst] = d
	}
	for _, id := range []int64{75, 103} {
		d := want.Entities[id]
		for j := 2; j <= 3; j++ {
			d.Records[0].Args[j] = want.Entities[33].Records[0].Args[j]
		}
		want.Entities[id] = d
	}
	proved, ok := ellipseFaceGraph(want, f, roots)
	if !ok {
		return fail()
	}
	for id, v := range roles {
		if proved[id] != v {
			return fail()
		}
	}
	lowerFrame, e := mappedExactFrame(f, roles[98])
	if e != nil {
		return fail()
	}
	upperFrame, e := mappedExactFrame(f, roles[70])
	if e != nil {
		return fail()
	}
	lower, e := geom.NewPlaneSurface(lowerFrame)
	if e != nil {
		return fail()
	}
	upper, e := geom.NewPlaneSurface(upperFrame)
	if e != nil {
		return fail()
	}
	ellipse := trimmed.EllipseUV{Cos: geom.Vec2{X: side.MajorRadius}, Sin: geom.Vec2{Y: side.MinorRadius}, To: 2 * math.Pi}
	use := func(edge int) []trimmed.Use {
		return []trimmed.Use{{Edge: edge, UVStart: geom.Vec2{X: side.MajorRadius}, UVEnd: geom.Vec2{X: side.MajorRadius}, Curve: ellipse}}
	}
	lateral := side.Face
	lateral.Reversed = true
	return RepairedEllipticalPrismShell{SourceEdgeIDs: [3]int64{roles[21], roles[54], roles[82]}, SourceSeamPCurveIDs: [2]int64{roles[31], roles[46]}, Name: side.Name, Faces: [3]trimmed.Face{lateral, {Surface: lower, Outer: use(2), Reversed: true}, {Surface: upper, Outer: use(1)}}, Side: side, Source: f, SourceBodyID: roles[15], SourceShellID: roles[16], SourceFaceIDs: [3]int64{roles[17], roles[108], roles[112]}, SourceRimPCurveIDs: [2][2]int64{{roles[61], roles[68]}, {roles[89], roles[96]}}}, nil
}

// WriteEllipticalPrismShellSource revalidates and emits the ORIGINAL source,
// including its rounded seam. It does not serialize repaired mathematical π.
func WriteEllipticalPrismShellSource(w io.Writer, f *File, l Limits, o EllipticalWireRepairOptions) error {
	if w == nil || !ellipseOwnedSourceBound(f, 121) {
		return fmt.Errorf("step: invalid prism source")
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = RepairRoundedEllipticalPrismShell(bytes.NewReader(b), l, o); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
