package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const nestedRationalAnisotropicBoreNativeFormat = "plainsolid-step-nested-rational-anisotropic-station-bore-assembly"

type nestedRationalAnisotropicBoreNativeEnvelope struct {
	rationalAnisotropicBoreNativeEnvelope
	GroupID   int64
	Instances []rationalHierarchyInstanceClaim
}

func nestedRationalAnisotropicBoreNativeClaim(ctx context.Context, m NestedRationalWrappedAnisotropicStationBoreAssembly) (nestedRationalAnisotropicBoreNativeEnvelope, error) {
	q, e := rationalAnisotropicBoreNativeClaim(ctx, m.RationalWrappedAnisotropicStationBoreAssembly, true)
	if e != nil {
		return nestedRationalAnisotropicBoreNativeEnvelope{}, e
	}
	q.Format = nestedRationalAnisotropicBoreNativeFormat
	out := nestedRationalAnisotropicBoreNativeEnvelope{rationalAnisotropicBoreNativeEnvelope: q, GroupID: m.GroupID}
	if len(m.Instances) < 2 || len(m.Instances) > 16 {
		return out, geom.ErrInvalid
	}
	for _, u := range m.Instances {
		if len(u.Path) != 2 {
			return out, geom.ErrInvalid
		}
		matrix, e := rationalHierarchyMatrixClaim(u.Matrix())
		if e != nil {
			return out, e
		}
		out.Instances = append(out.Instances, rationalHierarchyInstanceClaim{ProductID: u.ProductID, Path: u.Path, Matrix: matrix})
	}
	return out, ctx.Err()
}

// WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON authenticates the complete
// original STEP and all supplied local material, source and exact pose claims.
func WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, w io.Writer, m NestedRationalWrappedAnisotropicStationBoreAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := nestedRationalAnisotropicBoreNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(q.Source), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	want, e := nestedRationalAnisotropicBoreNativeClaim(ctx, proof)
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

// ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON re-proves source once and
// rejects forged material, occurrence ownership, rational matrices or metadata.
func ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NestedRationalWrappedAnisotropicStationBoreAssembly, error) {
	var zero NestedRationalWrappedAnisotropicStationBoreAssembly
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
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
	var q nestedRationalAnisotropicBoreNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != nestedRationalAnisotropicBoreNativeFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(q.Source), l.Source, WrappedSurfaceOptions{})
	if e != nil {
		return zero, e
	}
	want, e := nestedRationalAnisotropicBoreNativeClaim(ctx, m)
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
