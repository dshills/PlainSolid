package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const posedOCCTLoopNativeFormat = "plainsolid-step-posed-occt-exact-circular-loop-extrusion"

type posedOCCTLoopNativeEnvelope struct {
	exactLoopNativeEnvelope
	NormalizationRotation geom.Affine
	ReferenceCircleID     int64
}

func posedOCCTLoopNativeClaim(ctx context.Context, m PosedOCCTExactCircularLoopExtrusion) (posedOCCTLoopNativeEnvelope, error) {
	q, e := exactLoopNativeClaim(ctx, m.ExactCircularLoopExtrusionProduct, m.Product, true)
	q.Format = posedOCCTLoopNativeFormat
	return posedOCCTLoopNativeEnvelope{q, m.NormalizationRotation, m.ReferenceCircleID}, e
}

// WritePosedOCCTExactCircularLoopExtrusionJSON re-proves original model coordinates,
// local native geometry, source roles and the exact normalized world placement.
// This distinct version-one format does not change the unposed vendor envelope.
func WritePosedOCCTExactCircularLoopExtrusionJSON(ctx context.Context, w io.Writer, m PosedOCCTExactCircularLoopExtrusion) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := posedOCCTLoopNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewBufferString(q.Source), Limits{})
	if e != nil {
		return e
	}
	want, e := posedOCCTLoopNativeClaim(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, want) {
		return geom.ErrInvalid
	}
	b, e := json.Marshal(q)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > 8<<20 {
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

// ReadPosedOCCTExactCircularLoopExtrusionJSON authenticates stored claims by one
// complete original STEP reproof. It never trusts serialized native geometry.
func ReadPosedOCCTExactCircularLoopExtrusionJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (PosedOCCTExactCircularLoopExtrusion, error) {
	var zero PosedOCCTExactCircularLoopExtrusion
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 8 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 8<<20 {
		return zero, geom.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var q posedOCCTLoopNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != posedOCCTLoopNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadPosedOCCTExactCircularLoopExtrusion(ctx, bytes.NewBufferString(q.Source), l.Source)
	if e != nil {
		return zero, e
	}
	want, e := posedOCCTLoopNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	q.Native, e = independentCompact(q.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
