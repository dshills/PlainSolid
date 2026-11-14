package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/ellipticprism"
	"strings"
)

// ConicCircularPrismMaterial authenticates a circular104 directrix +122 wall,
// not a192 cylinder. Original scalar records and source ancestry are retained.
type ConicCircularPrismMaterial struct {
	Topology            EllipticalPrismTopology
	Solid               trimmed.Solid
	NativeFaceSourceIDs [6]int
	NativeRimEdges      [2][4]int
	SourceRimCurveIDs   [2]int
}

func circularConicProofDescriptor(d EllipticalPrismDescriptor) (EllipticalPrismDescriptor, error) {
	c := d.Coefficients
	for _, v := range c {
		if !finite(v) {
			return d, geom.ErrInvalid
		}
	}
	if c[0] != c[2] || c[0] <= 0 {
		return d, geom.ErrInvalid
	}
	q := new(big.Rat).Mul(compositeRat(c[2]), big.NewRat(4, 1))
	v, ok := q.Float64()
	if !ok || !finite(v) {
		return d, geom.ErrUnrepresentable
	}
	d.Coefficients[2] = v
	if _, _, e := ellipseConicRadii(EllipseConic{Coefficients: d.Coefficients, Z: d.LowerZ}); e != nil {
		return d, e
	}
	return d, nil
}

// ReadConicCircularPrismMaterial accepts the canonical20-record circular conic
// graph. An exact invertible y/2 normalization provides the existing whole
// topology proof, while native geometry and retained source remain circular.
func ReadConicCircularPrismMaterial(ctx context.Context, r io.Reader, l Limits, tol float64) (ConicCircularPrismMaterial, error) {
	bad := func() (ConicCircularPrismMaterial, error) {
		return ConicCircularPrismMaterial{}, fmt.Errorf("iges: unsupported conic circular prism")
	}
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ConicCircularPrismMaterial{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 32768)
	l.MaxRecords = min(l.MaxRecords, 512)
	l.MaxEntities = min(l.MaxEntities, 20)
	l.MaxValues = min(l.MaxValues, 2048)
	doc, e := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, false, false, true, true, true, true, true)
	if e != nil || len(doc.entities) != 20 || mixedTopologyDirectory(doc.bytes) != nil {
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
	t, e := ReadEllipticalPrismTopology(ctx, bytes.NewReader(normalized), l)
	if e != nil {
		return bad()
	}
	a, _, e := ellipseConicRadii(EllipseConic{Coefficients: t.Descriptor.Coefficients, Z: t.Descriptor.LowerZ})
	if e != nil {
		return bad()
	}
	t.Descriptor.Coefficients[2] = t.Descriptor.Coefficients[0]
	t.Parameters = originals
	t.source = source
	h, e := cylinderExactSum(t.Descriptor.UpperZ, -t.Descriptor.LowerZ)
	if e != nil {
		return bad()
	}
	f, e := geom.NewFrame(geom.Vec3{Z: t.Descriptor.LowerZ}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if e != nil {
		return bad()
	}
	s, rim, e := ellipticprism.CirclePrism(ctx, f, a, h, tol)
	if e != nil {
		return ConicCircularPrismMaterial{}, e
	}
	return ConicCircularPrismMaterial{t, s, [6]int{35, 33, 31, 31, 31, 31}, rim, [2]int{9, 11}}, nil
}

func WriteConicCircularPrism(ctx context.Context, w io.Writer, d EllipticalPrismDescriptor, unit, tol float64) error {
	if w == nil {
		return geom.ErrInvalid
	}
	proof, e := circularConicProofDescriptor(d)
	if e != nil {
		return e
	}
	a, e := ellipticalPrismAuthor(proof)
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
		v[3].text = number(d.Coefficients[2])
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
	if _, e = ReadConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, tol); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

func normalizeCircularPrismSource(doc *topologySource) (map[int][]float64, error) {
	originals := map[int][]float64{}
	for i, rec := range doc.entities {
		if rec.transform != 0 || compositeSourceNumbers(110, rec.values) != nil {
			return nil, geom.ErrInvalid
		}
		originals[rec.id] = make([]float64, len(rec.values))
		for j, p := range rec.values {
			x, e := realValue(p)
			if e != nil {
				return nil, geom.ErrInvalid
			}
			originals[rec.id][j] = x
		}
		// y/2 leaves every nonconic authored coordinate invariant in this dialect:
		// all points, vertices, seam and terminal lie at y=0; plane normals are Z
		// and references X. Reject any counterexample before changing conic data.
		zero := []int{}
		switch rec.kind {
		case 104:
			if len(rec.values) != 12 {
				return nil, geom.ErrInvalid
			}
			v := originals[rec.id]
			if v[1] != v[3] || v[1] <= 0 || v[2] != 0 || v[4] != 0 || v[5] != 0 || v[9] != 0 || v[11] != 0 {
				return nil, geom.ErrInvalid
			}
			q := new(big.Rat).Mul(compositeRat(v[3]), big.NewRat(4, 1))
			x, ok := q.Float64()
			if !ok || !finite(x) {
				return nil, geom.ErrInvalid
			}
			rec.values = append([]parameter(nil), rec.values...)
			rec.values[3].text = number(x)
			doc.entities[i] = rec
		case 110:
			zero = []int{2, 5}
		case 116:
			zero = []int{2}
		case 123:
			zero = []int{2}
		case 122:
			zero = []int{3}
		case 502:
			if len(rec.values) != 8 && len(rec.values) != 14 {
				return nil, geom.ErrInvalid
			}
			zero = []int{3, 6}
			if len(rec.values) == 14 {
				zero = append(zero, 9, 12)
			}
		case 190, 186, 504, 508, 510, 514:
		default:
			return nil, geom.ErrInvalid
		}
		for _, j := range zero {
			if j >= len(rec.values) || originals[rec.id][j] != 0 {
				return nil, geom.ErrInvalid
			}
		}
	}
	return originals, nil
}
