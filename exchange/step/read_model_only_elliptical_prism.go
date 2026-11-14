package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ModelOnlyEllipticalPrism is an exact import of the bounded natural-model
// ellipse prism dialect. Unlike RepairedEllipticalPrismShell it contains no
// periodic repair, invented pcurve, or tolerance comparison to altered geometry.
// Tolerance belongs only to native validation; every source scalar and the
// shared model edge topology are proved exactly before construction.
type ModelOnlyEllipticalPrism struct {
	Name                                         string
	Frame                                        geom.Frame
	MajorRadius, MinorRadius, Height, LengthUnit float64
	Solid                                        trimmed.Solid
	Source                                       *File
	SourceBodyID, SourceShellID                  int64
	SourceFaceIDs                                [3]int64 // side,lower,upper
	SourceEdgeIDs                                [3]int64 // seam,upper rim,lower rim
	NativeFaceSourceIDs                          [6]int64
	NativeRimEdges                               [2][4]int
}

// ReadModelOnlyEllipticalPrism strictly accepts the 76-record rooted analytic
// model-only prism. Two closed natural ellipse edges bound the finite side,
// one shared LINE seam occurs in opposite directions, and both exact plane
// caps share those rims with opposite effective boundary orientations. No
// rounded parameter/trim or unknown ownership is admitted. The exact quarter
// native builder provides continuous edge identity and constructive embedding.
func ReadModelOnlyEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64) (ModelOnlyEllipticalPrism, error) {
	fail := func() (ModelOnlyEllipticalPrism, error) {
		return ModelOnlyEllipticalPrism{}, fmt.Errorf("step: unsupported exact model-only elliptical prism")
	}
	if ctx == nil || r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 76)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	if len(f.Entities) != 76 {
		return fail()
	}
	var buf bytes.Buffer
	analyticSupportHeader(&buf)
	buf.WriteString(modelOnlyEllipticalPrismTemplate)
	buf.WriteString("ENDSEC;END-ISO-10303-21;")
	want, e := Parse(&buf, Limits{})
	if e != nil {
		return fail()
	}
	roots := [][2]int64{{1, 0}, {3, 0}, {121, 0}}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		for i, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_RELATED_PRODUCT_CATEGORY"} {
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
	frame, e := mappedExactFrame(f, roles[34])
	if e != nil {
		return fail()
	}
	profile, _ := simple(f, roles[33], "ELLIPSE", 4)
	a, e := cylinderSupportNumber(profile.Args[2])
	if e != nil {
		return fail()
	}
	b, e := cylinderSupportNumber(profile.Args[3])
	if e != nil || b <= 0 || a <= b {
		return fail()
	}
	top, e := mappedExactCoordinate(f, roles[58], "CARTESIAN_POINT", 3)
	if e != nil {
		return fail()
	}
	values := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	origin := values(frame.Origin())
	axis := values(frame.ZAxis())
	height := new(big.Rat)
	for i, c := range values(top) {
		d := new(big.Rat).Sub(new(big.Rat).SetFloat64(c), new(big.Rat).SetFloat64(origin[i]))
		height.Add(height, d.Mul(d, new(big.Rat).SetFloat64(axis[i])))
	}
	h, exact := height.Float64()
	if !exact || !finiteNumber(h) || h <= 0 {
		return fail()
	}
	point := func(x, z float64) (geom.Vec3, bool) {
		v := [3]float64{}
		xx := values(frame.XAxis())
		for i, c := range origin {
			q := new(big.Rat).SetFloat64(c)
			q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(xx[i]), new(big.Rat).SetFloat64(x)))
			q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(axis[i]), new(big.Rat).SetFloat64(z)))
			var good bool
			v[i], good = q.Float64()
			if !good || !finiteNumber(v[i]) {
				return geom.Vec3{}, false
			}
		}
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, true
	}
	topCenter, ok := point(0, h)
	if !ok {
		return fail()
	}
	bottomVertex, ok := point(a, 0)
	if !ok {
		return fail()
	}
	topVertex, ok := point(a, h)
	if !ok {
		return fail()
	}
	number := func(x float64) Value {
		s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(x))
		if e != nil {
			return Value{}
		}
		return Value{Kind: Number, Text: s}
	}
	set := func(id int64, v geom.Vec3) {
		en := want.Entities[id]
		vv := values(v)
		en.Records[0].Args[1].Items = []Value{number(vv[0]), number(vv[1]), number(vv[2])}
		want.Entities[id] = en
	}
	for _, x := range []struct {
		id int64
		v  geom.Vec3
	}{{23, bottomVertex}, {25, topVertex}, {28, bottomVertex}, {30, frame.ZAxis()}, {35, frame.Origin()}, {36, frame.ZAxis()}, {37, frame.XAxis()}, {39, frame.ZAxis().Scale(-1)}, {58, topCenter}, {59, frame.ZAxis()}, {60, frame.XAxis()}, {71, topCenter}, {72, frame.ZAxis()}, {73, frame.XAxis()}, {86, frame.Origin()}, {87, frame.ZAxis()}, {88, frame.XAxis()}, {99, frame.Origin()}, {100, frame.ZAxis()}, {101, frame.XAxis()}} {
		set(x.id, x.v)
	}
	for _, id := range []int64{33, 56, 84} {
		en := want.Entities[id]
		en.Records[0].Args[2] = profile.Args[2]
		en.Records[0].Args[3] = profile.Args[3]
		want.Entities[id] = en
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
	unit, e := contextUnits(f, roles[116])
	if e != nil || unit != .001 {
		return fail()
	}
	solid, rims, e := sewExactEllipticalPrism(ctx, frame, a, b, h, tolerance)
	if e != nil {
		return ModelOnlyEllipticalPrism{}, e
	}
	product, _ := simple(f, roles[7], "PRODUCT", 4)
	return ModelOnlyEllipticalPrism{Name: product.Args[1].Text, Frame: frame, MajorRadius: a, MinorRadius: b, Height: h, LengthUnit: unit, Solid: solid, Source: f, SourceBodyID: roles[15], SourceShellID: roles[16], SourceFaceIDs: [3]int64{roles[17], roles[108], roles[112]}, SourceEdgeIDs: [3]int64{roles[21], roles[54], roles[82]}, NativeFaceSourceIDs: [6]int64{roles[108], roles[112], roles[17], roles[17], roles[17], roles[17]}, NativeRimEdges: rims}, nil
}

// WriteModelOnlyEllipticalPrismSource re-proves and retains the exact source.
// It serializes no repair metadata and does not accept edits to decoded fields.
func WriteModelOnlyEllipticalPrismSource(ctx context.Context, w io.Writer, source *File, l Limits, tolerance float64) error {
	if w == nil || !ellipseOwnedSourceBound(source, 76) {
		return geom.ErrInvalid
	}
	b, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadModelOnlyEllipticalPrism(ctx, bytes.NewReader(b), l, tolerance); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
