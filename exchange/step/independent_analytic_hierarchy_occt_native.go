package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const occtAnalyticHierarchyFormat = "plainsolid-step-occt-normalized-analytic-hierarchy"

type occtAnalyticHierarchyEnvelope struct {
	Format   string
	Version  int
	Proof    normalizedAnalyticHierarchyEnvelope
	Mappings []OCCTHierarchyMapping
}

func occtAnalyticHierarchyEnvelopeFor(ctx context.Context, m OCCTNormalizedAnalyticHierarchy) (occtAnalyticHierarchyEnvelope, []IndependentExactMaterial, error) {
	p, c, e := normalizedAnalyticHierarchyEnvelopeFor(ctx, m.Normalized)
	p.Format = occtAnalyticHierarchyFormat + "-claims"
	return occtAnalyticHierarchyEnvelope{occtAnalyticHierarchyFormat, 1, p, m.Mappings}, c, e
}

// WriteOCCTNormalizedAnalyticHierarchyJSON stores original source and IDs plus
// separately authenticated derived relationship frames under a distinct tag.
func WriteOCCTNormalizedAnalyticHierarchyJSON(ctx context.Context, w io.Writer, m OCCTNormalizedAnalyticHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	q, c, e := occtAnalyticHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), q.Proof.Target, Limits{}, q.Proof.Proof.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := occtAnalyticHierarchyEnvelopeFor(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, expected) || !reflect.DeepEqual(c, d) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(q)
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

// ReadOCCTNormalizedAnalyticHierarchyJSON re-proves the original source once;
// neither saved native geometry nor a claimed derived transform is trusted.
func ReadOCCTNormalizedAnalyticHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (OCCTNormalizedAnalyticHierarchy, error) {
	bad := func() (OCCTNormalizedAnalyticHierarchy, error) {
		return OCCTNormalizedAnalyticHierarchy{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return OCCTNormalizedAnalyticHierarchy{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return OCCTNormalizedAnalyticHierarchy{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	var q occtAnalyticHierarchyEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != occtAnalyticHierarchyFormat || q.Version != 1 || len(q.Mappings) > 64 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), q.Proof.Target, l.Source, q.Proof.Proof.Proof.Tolerance)
	if e != nil {
		return OCCTNormalizedAnalyticHierarchy{}, e
	}
	expected, _, e := occtAnalyticHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return OCCTNormalizedAnalyticHierarchy{}, e
	}
	for i := range q.Proof.Proof.Proof.Members {
		p := &q.Proof.Proof.Proof.Members[i]
		p.Claims, e = independentCompact(p.Claims)
		if e != nil {
			return bad()
		}
		p.Native, e = independentCompact(p.Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	return m, nil
}

// WriteOCCTNormalizedAnalyticHierarchySource re-proves and emits original
// records. Emit Normalized.Hierarchy.Assembly.Source using the existing strict
// hierarchy source writer to transfer the explicitly derived neutral frames.
func WriteOCCTNormalizedAnalyticHierarchySource(ctx context.Context, w io.Writer, source *File, target IndependentMaterialSIUnit, l Limits, tol float64) error {
	if ctx == nil || w == nil || !independentNativeSourceBound(source, 4096) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	b, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadOCCTNormalizedAnalyticHierarchy(ctx, bytes.NewReader(b), target, l, tol); e != nil {
		return e
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
