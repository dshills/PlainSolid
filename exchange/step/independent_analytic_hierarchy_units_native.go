package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const normalizedAnalyticHierarchyFormat = "plainsolid-step-normalized-analytic-hierarchy"

type normalizedAnalyticHierarchyEnvelope struct {
	Format  string
	Version int
	Source  string
	Target  IndependentMaterialSIUnit
	Units   map[int64]IndependentMaterialSIUnit
	Proof   independentAnalyticDAGEnvelope
}

func normalizedAnalyticHierarchyEnvelopeFor(ctx context.Context, m NormalizedIndependentAnalyticHierarchy) (normalizedAnalyticHierarchyEnvelope, []IndependentExactMaterial, error) {
	q := normalizedAnalyticHierarchyEnvelope{Format: normalizedAnalyticHierarchyFormat, Version: 1, Target: m.TargetUnit, Units: m.OriginalUnits}
	if !independentNativeSourceBound(m.OriginalSource, 4096) || !independentNativeSourceBound(m.Hierarchy.Assembly.Source, 4096) {
		return q, nil, geom.ErrInvalid
	}
	raw, e := mappedPlanarSourceBytes(m.OriginalSource)
	if e != nil {
		return q, nil, e
	}
	q.Source = string(raw)
	raw, e = mappedPlanarSourceBytes(m.Hierarchy.Assembly.Source)
	if e != nil {
		return q, nil, e
	}
	p, c, e := independentAnalyticDAGEnvelopeFor(ctx, IndependentAnalyticDAGAssembly(m.Hierarchy), raw, true)
	q.Proof = p
	if e == nil && m.tolerance != p.Proof.Tolerance {
		return q, nil, geom.ErrInvalid
	}
	return q, c, e
}

// WriteNormalizedIndependentAnalyticHierarchyJSON re-proves the original mixed
// source once, comparing normalized geometry, paths, units and native claims.
// Previous hierarchy/native formats retain their same-unit meaning.
func WriteNormalizedIndependentAnalyticHierarchyJSON(ctx context.Context, w io.Writer, m NormalizedIndependentAnalyticHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	q, c, e := normalizedAnalyticHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewBufferString(q.Source), q.Target, Limits{}, q.Proof.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := normalizedAnalyticHierarchyEnvelopeFor(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, expected) || !reflect.DeepEqual(c, d) {
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

// ReadNormalizedIndependentAnalyticHierarchyJSON rebuilds solely from source
// and checks the separately tagged normalized/native claims atomically.
func ReadNormalizedIndependentAnalyticHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NormalizedIndependentAnalyticHierarchy, error) {
	bad := func() (NormalizedIndependentAnalyticHierarchy, error) {
		return NormalizedIndependentAnalyticHierarchy{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return bad()
	}
	var q normalizedAnalyticHierarchyEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != normalizedAnalyticHierarchyFormat || q.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewBufferString(q.Source), q.Target, l.Source, q.Proof.Proof.Tolerance)
	if e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	expected, _, e := normalizedAnalyticHierarchyEnvelopeFor(ctx, m)
	if e != nil {
		return NormalizedIndependentAnalyticHierarchy{}, e
	}
	for i := range q.Proof.Proof.Members {
		p := &q.Proof.Proof.Members[i]
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
