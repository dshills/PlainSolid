package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// ConicCircularAnnularPrismMaterial retains two original circular104/122 walls
// and four full rim roles separately from exact native quarter partitions.
type ConicCircularAnnularPrismMaterial struct {
	Topology            EllipticalAnnularPrismTopology
	Solid               trimmed.Solid
	NativeFaceSourceIDs [10]int
	NativeRimEdges      [2][2][4]int
	SourceRimCurveIDs   [2][2]int
}

func ReadConicCircularAnnularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ConicCircularAnnularPrismMaterial, error) {
	bad := func() (ConicCircularAnnularPrismMaterial, error) {
		return ConicCircularAnnularPrismMaterial{}, fmt.Errorf("iges: unsupported circular conic annular prism")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ConicCircularAnnularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 28)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil || len(doc.entities) != 28 || mixedTopologyDirectory(doc.bytes) != nil {
		return bad()
	}
	source := append([]byte(nil), doc.bytes...)
	originals, e := normalizeCircularPrismSource(&doc)
	if e != nil {
		return bad()
	}
	normalized, e := mixedRoofFile(doc)
	if e != nil {
		return bad()
	}
	t, e := ReadEllipticalAnnularPrismTopology(ctx, bytes.NewReader(normalized), l)
	if e != nil {
		return bad()
	}
	d := t.Descriptor
	a, _, e := ellipseConicRadii(EllipseConic{Coefficients: d.OuterCoefficients, Z: d.LowerZ})
	if e != nil {
		return bad()
	}
	ia, _, e := ellipseConicRadii(EllipseConic{Coefficients: d.InnerCoefficients, Z: d.LowerZ})
	if e != nil || ia >= a {
		return bad()
	}
	t.Descriptor.OuterCoefficients[2] = t.Descriptor.OuterCoefficients[0]
	t.Descriptor.InnerCoefficients[2] = t.Descriptor.InnerCoefficients[0]
	t.Parameters = originals
	t.source = source
	h, e := cylinderExactSum(d.UpperZ, -d.LowerZ)
	if e != nil {
		return bad()
	}
	f, e := geom.NewFrame(geom.Vec3{Z: d.LowerZ}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return bad()
	}
	s, rims, e := ellipticprism.CircularAnnulus(ctx, f, a, ia, h, tol)
	if e != nil {
		return ConicCircularAnnularPrismMaterial{}, e
	}
	return ConicCircularAnnularPrismMaterial{t, s, [10]int{51, 49, 45, 45, 45, 45, 47, 47, 47, 47}, rims, [2][2]int{{23, 19}, {29, 25}}}, nil
}
func WriteConicCircularAnnularPrism(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	outer, e := circularConicProofDescriptor(EllipticalPrismDescriptor{d.OuterCoefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	inner, e := circularConicProofDescriptor(EllipticalPrismDescriptor{d.InnerCoefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	a, e := ellipticalAnnularPrismAuthor(EllipticalAnnularPrismDescriptor{outer.Coefficients, inner.Coefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	for i, rec := range a.records {
		if rec.kind != 104 {
			continue
		}
		budget := 32
		v, e := parseParameters(rec.data, ',', ';', &budget)
		if e != nil {
			return e
		}
		v[3] = v[1]
		parts := make([]string, len(v))
		for j, p := range v {
			parts[j] = p.text
		}
		a.records[i].data = strings.Join(parts, ",") + ";"
	}
	raw, e := a.file(unit)
	if e != nil {
		return e
	}
	if _, e = ReadConicCircularAnnularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}
