package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const independentSphericalHierarchyFormat = "plainsolid-step-independent-spherical-hierarchy"
const independentSphericalHierarchyClaimsFormat = "plainsolid-step-independent-spherical-hierarchy-claims"

func sphericalHierarchyEnvelope(ctx context.Context, m IndependentSphericalHierarchy) (independentAnalyticDAGEnvelope, []IndependentExactMaterial, error) {
	q := independentAnalyticDAGEnvelope{Format: independentSphericalHierarchyFormat, Version: 1, Instances: m.Instances}
	if !independentNativeSourceBound(m.Assembly.Source, 4096) || len(m.Instances) < 1 || len(m.Instances) > 64 {
		return q, nil, geom.ErrInvalid
	}
	for _, u := range m.Instances {
		if len(u.Path) < 1 || len(u.Path) > 8 || !planarSplineRigid(u.Transform) {
			return q, nil, geom.ErrInvalid
		}
	}
	b, e := mappedPlanarSourceBytes(m.Assembly.Source)
	if e != nil {
		return q, nil, e
	}
	p, c, e := makeIndependentTorusAssemblyEnvelope(ctx, IndependentTorusAssembly{Assembly: m.Assembly, Products: m.Products}, b, true, true, true, true)
	q.Proof = p
	return q, c, e
}

// WriteIndependentSphericalHierarchyJSON stores native local assets once and
// re-proves all source/occurrence/explicit sphere-frame claims before output.
func WriteIndependentSphericalHierarchyJSON(ctx context.Context, w io.Writer, m IndependentSphericalHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	q, c, e := sphericalHierarchyEnvelope(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), Limits{}, q.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := sphericalHierarchyEnvelope(ctx, proof)
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

// ReadIndependentSphericalHierarchyJSON uses a distinct version-one tag and
// rebuilds exclusively through one full source proof, never stored native data.
func ReadIndependentSphericalHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentSphericalHierarchy, error) {
	bad := func() (IndependentSphericalHierarchy, error) { return IndependentSphericalHierarchy{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	var q independentAnalyticDAGEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != independentSphericalHierarchyFormat || q.Version != 1 || q.Proof.Format != independentSphericalHierarchyClaimsFormat || len(q.Proof.Members) < 3 || len(q.Proof.Members) > 4 || len(q.Instances) > 64 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), l.Source, q.Proof.Tolerance)
	if e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	expected, _, e := sphericalHierarchyEnvelope(ctx, m)
	if e != nil {
		return IndependentSphericalHierarchy{}, e
	}
	for i := range q.Proof.Members {
		p := &q.Proof.Members[i]
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
