package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
)

const independentFrustumHierarchyFormat = "plainsolid-step-independent-frustum-hierarchy"
const independentFrustumHierarchyClaimsFormat = "plainsolid-step-independent-frustum-hierarchy-claims"

func frustumHierarchyEnvelope(ctx context.Context, m IndependentFrustumHierarchy) (independentAnalyticDAGEnvelope, []IndependentExactMaterial, error) {
	q := independentAnalyticDAGEnvelope{Format: independentFrustumHierarchyFormat, Version: 1, Instances: m.Instances}
	if !independentNativeSourceRecordBound(m.Assembly.Source, 4096, 7) || len(m.Instances) < 1 || len(m.Instances) > 64 {
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
	p, c, e := makeIndependentTorusAssemblyEnvelope(ctx, IndependentTorusAssembly{Assembly: m.Assembly, Products: m.Products}, b, true, true, true, true, true)
	q.Proof = p
	return q, c, e
}

// WriteIndependentFrustumHierarchyJSON stores native local assets once and
// re-proves all source/occurrence/explicit original support claims before output.
func WriteIndependentFrustumHierarchyJSON(ctx context.Context, w io.Writer, m IndependentFrustumHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	q, c, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), Limits{}, q.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := frustumHierarchyEnvelope(ctx, proof)
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

// ReadIndependentFrustumHierarchyJSON uses a distinct version-one tag and
// rebuilds exclusively through one full source proof, never stored native data.
func ReadIndependentFrustumHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentFrustumHierarchy, error) {
	bad := func() (IndependentFrustumHierarchy, error) { return IndependentFrustumHierarchy{}, geom.ErrInvalid }
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentFrustumHierarchy{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return IndependentFrustumHierarchy{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	var q independentAnalyticDAGEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != independentFrustumHierarchyFormat || q.Version != 1 || q.Proof.Format != independentFrustumHierarchyClaimsFormat || len(q.Proof.Members) < 4 || len(q.Proof.Members) > 5 || len(q.Instances) > 64 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(q.Proof.Source), l.Source, q.Proof.Tolerance)
	if e != nil {
		return IndependentFrustumHierarchy{}, e
	}
	expected, _, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return IndependentFrustumHierarchy{}, e
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

// Check duplicate native state and retain both canonical geometry and original
// side/corner IDs. Canonical STEP serialization validates the whole original
// rational support; it is evidence, not a replacement for source reconstruction.
func independentFrustumClaims(m IndependentExactMaterial) (IndependentExactMaterial, json.RawMessage, string, error) {
	bad := func() (IndependentExactMaterial, json.RawMessage, string, error) {
		return IndependentExactMaterial{}, nil, "", geom.ErrInvalid
	}
	if m.Frustum == nil || m.Planar != nil || m.Elliptical != nil || m.Cylinder != nil || m.Sphere != nil || m.Torus != nil || !independentNativeSourceRecordBound(m.Source, 1024, 7) || len(m.Name) > 1024 || len(m.ProductName) > 1024 {
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
	raw, e = prismNativeBytes(m.Frustum.Solid)
	if e != nil {
		return bad()
	}
	other, e := independentCompact(raw)
	if e != nil || !bytes.Equal(native, other) {
		return bad()
	}
	f := *m.Frustum
	for _, r := range []*big.Rat{f.Embedding.CenterX, f.Embedding.CenterY, f.Embedding.LowerZ, f.Embedding.UpperZ, f.Embedding.LowerRadius, f.Embedding.UpperRadius} {
		if r == nil || r.Num().BitLen() > 4096 || r.Denom().BitLen() > 4096 {
			return bad()
		}
	}
	if len(f.Name) > 1024 || len(f.Face.Name) > 1024 || len(f.CapNames[0]) > 1024 || len(f.CapNames[1]) > 1024 || f.Face.Geometry == nil {
		return bad()
	}
	var canonical bytes.Buffer
	if e = WriteWrappedFrusta(&canonical, WrappedFrustumModel{Bodies: []WrappedFrustumBody{f}, LengthUnit: m.LengthUnit}); e != nil {
		return bad()
	}
	if canonical.Len() > 1<<20 {
		return bad()
	}
	f.Solid = trimmed.Solid{}
	f.Face.Geometry = nil
	m.Frustum = &f
	m.Solid = trimmed.Solid{}
	return m, native, canonical.String(), nil
}
