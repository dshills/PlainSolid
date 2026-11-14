package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

// IndependentMaterialJSONLimits separates envelope allocation from original STEP limits.
type IndependentMaterialJSONLimits struct {
	MaxBytes int64
	Source   Limits
}
type independentMaterialMemberEnvelope struct {
	Claims json.RawMessage
	Native json.RawMessage
}
type independentMaterialEnvelope struct {
	Format                string
	Version               int
	Source                string
	Tolerance, LengthUnit float64
	Members               []independentMaterialMemberEnvelope
}

func independentCompact(b []byte) (json.RawMessage, error) {
	var out bytes.Buffer
	if json.Compact(&out, b) != nil {
		return nil, geom.ErrInvalid
	}
	return out.Bytes(), nil
}

// Strip only native state after checking all duplicate payload solids agree.
// The remaining public/private metadata can then be compared structurally.
func independentMaterialClaims(m IndependentExactMaterial, cylinderMode ...bool) (IndependentExactMaterial, json.RawMessage, error) {
	bad := func() (IndependentExactMaterial, json.RawMessage, error) {
		return IndependentExactMaterial{}, nil, geom.ErrInvalid
	}
	if !independentNativeSourceBound(m.Source, 1024) || len(m.Name) > 1024 || len(m.ProductName) > 1024 {
		return bad()
	}
	raw, e := prismNativeBytes(m.Solid)
	if e != nil {
		return bad()
	}
	native, e := independentCompact(raw)
	if e != nil {
		return bad()
	}
	check := func(s trimmed.Solid) bool {
		b, e := prismNativeBytes(s)
		if e != nil {
			return false
		}
		v, e := independentCompact(b)
		return e == nil && bytes.Equal(native, v)
	}
	countFamilies := 0
	if m.Planar != nil {
		countFamilies++
	}
	if m.Elliptical != nil {
		countFamilies++
	}
	if m.Cylinder != nil {
		countFamilies++
	}
	if countFamilies != 1 || (m.Cylinder != nil && !(len(cylinderMode) == 1 && cylinderMode[0])) {
		return bad()
	}
	if m.Planar != nil {
		b := *m.Planar
		s, e := trimmed.FromPlanar(b.Solid, m.Solid.Tolerance())
		if e != nil || !check(s) {
			return bad()
		}
		b.Solid = brep.Solid{}
		m.Planar = &b
	}
	if m.Elliptical != nil {
		p := *m.Elliptical
		if !independentNativeSourceBound(p.Source, 1024) {
			return bad()
		}
		if !check(p.Solid) {
			return bad()
		}
		p.Solid = trimmed.Solid{}
		count := 0
		if p.Cardinal != nil {
			count++
			q := *p.Cardinal
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalCardinal || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.Cardinal = &q
		}
		if p.Pythagorean != nil {
			count++
			q := *p.Pythagorean
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalPythagorean || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.Pythagorean = &q
		}
		if p.ExactControl != nil {
			count++
			q := *p.ExactControl
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalExactControl || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.ExactControl = &q
		}
		if p.Tilted != nil {
			count++
			q := *p.Tilted
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalTilted || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.Tilted = &q
		}
		if p.HomotheticAnnular != nil {
			count++
			q := *p.HomotheticAnnular
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalHomotheticAnnular || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.HomotheticAnnular = &q
		}
		if p.NestedAnnular != nil {
			count++
			q := *p.NestedAnnular
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalNestedAnnular || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.NestedAnnular = &q
		}
		if p.OrthogonalAnnular != nil {
			count++
			q := *p.OrthogonalAnnular
			if !independentNativeSourceBound(q.Source, 1024) {
				return bad()
			}
			if p.Variant != EllipticalOrthogonalAnnular || !check(q.Solid) {
				return bad()
			}
			q.Solid = trimmed.Solid{}
			p.OrthogonalAnnular = &q
		}
		if count != 1 {
			return bad()
		}
		m.Elliptical = &p
	}
	m.Solid = trimmed.Solid{}
	return m, native, nil
}
func independentCollectionEnvelope(ctx context.Context, m IndependentExactMaterialCollection, source []byte, cylinderMode ...bool) (independentMaterialEnvelope, []IndependentExactMaterial, error) {
	q := independentMaterialEnvelope{Format: "plainsolid-step-independent-exact-materials", Version: 1, Source: string(source), Tolerance: m.tolerance, LengthUnit: m.LengthUnit}
	if len(cylinderMode) == 1 && cylinderMode[0] {
		q.Format = independentCylinderNativeFormat
	}
	claims := []IndependentExactMaterial{}
	if !finiteNumber(m.tolerance) || m.tolerance <= 0 || len(m.Members) < 1 || len(m.Members) > 16 {
		return q, nil, geom.ErrInvalid
	}
	for _, member := range m.Members {
		if e := ctx.Err(); e != nil {
			return q, nil, e
		}
		c, n, e := independentMaterialClaims(member, cylinderMode...)
		if e != nil {
			return q, nil, e
		}
		b, e := json.Marshal(c)
		if e != nil {
			return q, nil, e
		}
		q.Members = append(q.Members, independentMaterialMemberEnvelope{b, n})
		claims = append(claims, c)
	}
	return q, claims, nil
}

// WriteIndependentExactMaterialsJSON re-proves the complete shared source once,
// verifies all native and source ancestry claims, and stages an atomic envelope.
func WriteIndependentExactMaterialsJSON(ctx context.Context, w io.Writer, m IndependentExactMaterialCollection) error {
	return writeIndependentMaterialsJSON(ctx, w, m, false)
}
func writeIndependentMaterialsJSON(ctx context.Context, w io.Writer, m IndependentExactMaterialCollection, cylinder bool) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if !independentNativeSourceBound(m.Source, 16*1024) {
		return geom.ErrInvalid
	}
	source, e := mappedPlanarSourceBytes(m.Source)
	if e != nil {
		return e
	}
	read := ReadIndependentExactMaterials
	if cylinder {
		read = ReadIndependentCylinderMaterials
	}
	proof, e := read(ctx, bytes.NewReader(source), Limits{}, m.tolerance)
	if e != nil {
		return e
	}
	actual, claims, e := independentCollectionEnvelope(ctx, m, source, cylinder)
	if e != nil {
		return e
	}
	expected, proved, e := independentCollectionEnvelope(ctx, proof, source, cylinder)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(claims, proved) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(expected)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 16<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadIndependentExactMaterialsJSON reconstructs only from the original STEP
// graph, then verifies all envelope claims. Native bytes are never trusted as input geometry.
func ReadIndependentExactMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error) {
	return readIndependentMaterialsJSON(ctx, r, l, false)
}
func readIndependentMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, cylinder bool) (IndependentExactMaterialCollection, error) {
	bad := func() (IndependentExactMaterialCollection, error) {
		return IndependentExactMaterialCollection{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	format := "plainsolid-step-independent-exact-materials"
	read := ReadIndependentExactMaterials
	if cylinder {
		format = independentCylinderNativeFormat
		read = ReadIndependentCylinderMaterials
	}
	var q independentMaterialEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != format || q.Version != 1 || len(q.Members) < 1 || len(q.Members) > 16 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := read(ctx, bytes.NewReader([]byte(q.Source)), l.Source, q.Tolerance)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	expected, _, e := independentCollectionEnvelope(ctx, m, []byte(q.Source), cylinder)
	if e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	for i := range q.Members {
		q.Members[i].Claims, e = independentCompact(q.Members[i].Claims)
		if e != nil {
			return bad()
		}
		q.Members[i].Native, e = independentCompact(q.Members[i].Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentExactMaterialCollection{}, e
	}
	return m, nil
}

// Bound caller-owned graphs before recursive serialization, including cyclic
// Value.Items slices and overlong strings that a parsed source cannot contain.
func independentNativeSourceBound(f *File, maxEntities int) bool {
	if f == nil || len(f.Entities) < 1 || len(f.Entities) > maxEntities || len(f.Order) != len(f.Entities) || len(f.Header) > 64 {
		return false
	}
	budget, text := 16*16384, 2<<20
	var value func(Value, int) bool
	value = func(v Value, depth int) bool {
		budget--
		text -= len(v.Text)
		if budget < 0 || text < 0 || depth > 16 || len(v.Text) > 32768 {
			return false
		}
		for _, x := range v.Items {
			if !value(x, depth+1) {
				return false
			}
		}
		return true
	}
	record := func(r Record) bool {
		if len(r.Type) > 128 {
			return false
		}
		for _, v := range r.Args {
			if !value(v, 0) {
				return false
			}
		}
		return true
	}
	for _, r := range f.Header {
		if !record(r) {
			return false
		}
	}
	for id, en := range f.Entities {
		if id <= 0 || en.ID != id || len(en.Records) < 1 || len(en.Records) > 4 {
			return false
		}
		for _, r := range en.Records {
			if !record(r) {
				return false
			}
		}
	}
	return true
}
