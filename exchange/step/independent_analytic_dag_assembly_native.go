package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const independentAnalyticDAGNativeFormat = "plainsolid-step-independent-analytic-dag-assembly"
const independentAnalyticDAGClaimsFormat = "plainsolid-step-independent-analytic-dag-claims"

type independentAnalyticDAGEnvelope struct {
	Format    string
	Version   int
	Proof     independentTorusAssemblyEnvelope
	Instances []IndependentAnalyticDAGInstance
}

func independentAnalyticDAGEnvelopeFor(ctx context.Context, m IndependentAnalyticDAGAssembly, source []byte) (independentAnalyticDAGEnvelope, []IndependentExactMaterial, error) {
	q := independentAnalyticDAGEnvelope{Format: independentAnalyticDAGNativeFormat, Version: 1, Instances: m.Instances}
	if len(m.Instances) != 5 {
		return q, nil, geom.ErrInvalid
	}
	for _, u := range m.Instances {
		if len(u.Path) < 1 || len(u.Path) > 2 || !planarSplineRigid(u.Transform) {
			return q, nil, geom.ErrInvalid
		}
	}
	p, c, e := makeIndependentTorusAssemblyEnvelope(ctx, IndependentTorusAssembly{Assembly: m.Assembly, Products: m.Products}, source, true, true)
	q.Proof = p
	return q, c, e
}

// WriteIndependentAnalyticDAGAssemblyJSON stores original source, each local
// material once, the original occurrence DAG, and every exact composed leaf path.
// One source proof checks all claims before any output; older native tags retain
// their flat-only meaning.
func WriteIndependentAnalyticDAGAssemblyJSON(ctx context.Context, w io.Writer, m IndependentAnalyticDAGAssembly) error {
	if ctx == nil || w == nil || !independentNativeSourceBound(m.Assembly.Source, 4096) {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	source, e := mappedPlanarSourceBytes(m.Assembly.Source)
	if e != nil {
		return e
	}
	actual, c, e := independentAnalyticDAGEnvelopeFor(ctx, m, source)
	if e != nil {
		return e
	}
	proof, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(source), Limits{}, actual.Proof.Tolerance)
	if e != nil {
		return e
	}
	expected, d, e := independentAnalyticDAGEnvelopeFor(ctx, proof, source)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
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

// ReadIndependentAnalyticDAGAssemblyJSON reconstructs the graph and local
// materials exclusively from the original STEP, then compares path ancestry,
// composed poses and canonical native encodings with the stored claims.
func ReadIndependentAnalyticDAGAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (IndependentAnalyticDAGAssembly, error) {
	bad := func() (IndependentAnalyticDAGAssembly, error) {
		return IndependentAnalyticDAGAssembly{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	var q independentAnalyticDAGEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != independentAnalyticDAGNativeFormat || q.Version != 1 || q.Proof.Format != independentAnalyticDAGClaimsFormat || q.Proof.Version != 1 || q.Proof.Assembly.Source != nil || len(q.Proof.Members) != 3 || len(q.Instances) != 5 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	proof, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader([]byte(q.Proof.Source)), l.Source, q.Proof.Tolerance)
	if e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	expected, _, e := independentAnalyticDAGEnvelopeFor(ctx, proof, []byte(q.Proof.Source))
	if e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	for i := range q.Proof.Members {
		q.Proof.Members[i].Claims, e = independentCompact(q.Proof.Members[i].Claims)
		if e != nil {
			return bad()
		}
		q.Proof.Members[i].Native, e = independentCompact(q.Proof.Members[i].Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return IndependentAnalyticDAGAssembly{}, e
	}
	return proof, nil
}
