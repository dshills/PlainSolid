package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const nestedAnisotropicNativeFormat = "plainsolid-step-nested-anisotropic-station-assembly"
const nestedAnisotropicBoreNativeFormat = "plainsolid-step-nested-anisotropic-station-bore-assembly"

type nestedAnisotropicNativeEnvelope struct {
	anisotropicAssemblyNativeEnvelope
	GroupID   int64
	Instances []WrappedAnisotropicAssemblyInstance
}

func nestedAnisotropicFilledClaim(ctx context.Context, m NestedWrappedAnisotropicStationAssembly) (nestedAnisotropicNativeEnvelope, error) {
	q, e := anisotropicAssemblyNativeClaim(ctx, m.WrappedAnisotropicAssemblySource, m.Body.Name, m.Body.CapNames, m.Body.LengthUnit, []WrappedAnisotropicStationLoftBody{m.Body}, m.Body.Solid, false, true)
	q.Format = nestedAnisotropicNativeFormat
	return nestedAnisotropicNativeEnvelope{q, m.GroupID, m.Instances}, e
}
func nestedAnisotropicBoreClaim(ctx context.Context, m NestedWrappedAnisotropicStationBoreAssembly) (nestedAnisotropicNativeEnvelope, error) {
	q, e := anisotropicAssemblyNativeClaim(ctx, m.WrappedAnisotropicAssemblySource, m.Body.Name, m.Body.CapNames, m.Body.LengthUnit, []WrappedAnisotropicStationLoftBody{m.Body.Outer, m.Body.Inner}, m.Body.Solid, true, true)
	q.Format = nestedAnisotropicBoreNativeFormat
	return nestedAnisotropicNativeEnvelope{q, m.GroupID, m.Instances}, e
}

// WriteNestedWrappedAnisotropicStationAssemblyJSON binds the complete original STEP,
// one local native material and all original support/occurrence claims by reproof.
func WriteNestedWrappedAnisotropicStationAssemblyJSON(ctx context.Context, w io.Writer, m NestedWrappedAnisotropicStationAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := nestedAnisotropicFilledClaim(ctx, m)
	if e != nil {
		return e
	}
	return writeNestedAnisotropicAssemblyNative(ctx, w, q, false)
}

// WriteNestedWrappedAnisotropicStationBoreAssemblyJSON uses a separate version-one tag
// retaining both original wall families and actual annular material claims.
func WriteNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, w io.Writer, m NestedWrappedAnisotropicStationBoreAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := nestedAnisotropicBoreClaim(ctx, m)
	if e != nil {
		return e
	}
	return writeNestedAnisotropicAssemblyNative(ctx, w, q, true)
}
func reproveNestedAnisotropicAssemblyNative(ctx context.Context, source string, l Limits, bore bool) (NestedWrappedAnisotropicStationAssembly, NestedWrappedAnisotropicStationBoreAssembly, nestedAnisotropicNativeEnvelope, error) {
	var filled NestedWrappedAnisotropicStationAssembly
	var hole NestedWrappedAnisotropicStationBoreAssembly
	var q nestedAnisotropicNativeEnvelope
	var e error
	if bore {
		hole, e = ReadNestedWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewBufferString(source), l, WrappedSurfaceOptions{})
		if e == nil {
			q, e = nestedAnisotropicBoreClaim(ctx, hole)
		}
	} else {
		filled, e = ReadNestedWrappedAnisotropicStationAssembly(ctx, bytes.NewBufferString(source), l, WrappedSurfaceOptions{})
		if e == nil {
			q, e = nestedAnisotropicFilledClaim(ctx, filled)
		}
	}
	return filled, hole, q, e
}
func writeNestedAnisotropicAssemblyNative(ctx context.Context, w io.Writer, q nestedAnisotropicNativeEnvelope, bore bool) error {
	_, _, want, e := reproveNestedAnisotropicAssemblyNative(ctx, q.Source, Limits{}, bore)
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

// ReadNestedWrappedAnisotropicStationAssemblyJSON regenerates from original source
// once; stored native geometry and graph/pose claims are never authorities.
func ReadNestedWrappedAnisotropicStationAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NestedWrappedAnisotropicStationAssembly, error) {
	a, _, e := readNestedAnisotropicAssemblyNative(ctx, r, l, false)
	return a, e
}

// ReadNestedWrappedAnisotropicStationBoreAssemblyJSON re-proves complete original bore
// geometry, source ownership and every rigid use with atomic failure.
func ReadNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NestedWrappedAnisotropicStationBoreAssembly, error) {
	_, a, e := readNestedAnisotropicAssemblyNative(ctx, r, l, true)
	return a, e
}
func readNestedAnisotropicAssemblyNative(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, bore bool) (NestedWrappedAnisotropicStationAssembly, NestedWrappedAnisotropicStationBoreAssembly, error) {
	var a NestedWrappedAnisotropicStationAssembly
	var h NestedWrappedAnisotropicStationBoreAssembly
	if ctx == nil || r == nil {
		return a, h, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return a, h, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return a, h, geom.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return a, h, e
	}
	if e = ctx.Err(); e != nil {
		return a, h, e
	}
	if int64(len(b)) > l.MaxBytes {
		return a, h, geom.ErrConvergence
	}
	var q nestedAnisotropicNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	tag := nestedAnisotropicNativeFormat
	if bore {
		tag = nestedAnisotropicBoreNativeFormat
	}
	if d.Decode(&q) != nil || q.Format != tag || q.Version != 1 || len(q.Source) > 1<<20 {
		return a, h, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return a, h, geom.ErrInvalid
	}
	filled, hole, want, e := reproveNestedAnisotropicAssemblyNative(ctx, q.Source, l.Source, bore)
	if e != nil {
		return a, h, e
	}
	q.Native, e = independentCompact(q.Native)
	if e != nil || !reflect.DeepEqual(q, want) {
		return a, h, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return a, h, e
	}
	return filled, hole, nil
}
