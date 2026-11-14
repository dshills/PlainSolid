package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const weightedBoreAssemblyNativeFormat = "plainsolid-step-wrapped-rational-station-bore-assembly"

type weightedBoreSideClaim struct {
	Name       string
	CapNames   [2]string
	LengthUnit float64
	Walls      []weightedAssemblyWallClaim
	Native     json.RawMessage
}
type weightedBoreAssemblyNativeEnvelope struct {
	Proof                  weightedAssemblyNativeEnvelope
	Outer, Inner           weightedBoreSideClaim
	OuterFaces, InnerFaces []int64
}

func weightedBoreAssemblyNativeClaim(ctx context.Context, m WrappedRationalStationBoreAssembly) (weightedBoreAssemblyNativeEnvelope, error) {
	var q weightedBoreAssemblyNativeEnvelope
	if len(m.Body.Outer.Walls) < 1 || len(m.Body.Outer.Walls) > 16 || len(m.Body.Inner.Walls) < 1 || len(m.Body.Inner.Walls) > 16 {
		return q, geom.ErrInvalid
	}
	pseudo := WrappedRationalStationAssembly{Assembly: m.Assembly, Product: m.Product, SourceBodyID: m.SourceBodyID, SourceFaceIDs: m.SourceFaceIDs, Occurrences: m.Occurrences}
	pseudo.SourceWallFaceIDs = append(append([]int64(nil), m.SourceOuterWallFaceIDs...), m.SourceInnerWallFaceIDs...)
	pseudo.Body = WrappedRationalStationLoftBody{Name: m.Body.Name, CapNames: m.Body.CapNames, LengthUnit: m.Body.LengthUnit, Solid: m.Body.Solid, Walls: append(append([]WrappedSplineFace(nil), m.Body.Outer.Walls...), m.Body.Inner.Walls...)}
	var e error
	q.Proof, e = weightedAssemblyNativeClaimBound(ctx, pseudo, 32)
	if e != nil {
		return q, e
	}
	q.Proof.Format = weightedBoreAssemblyNativeFormat
	q.OuterFaces = append([]int64(nil), m.SourceOuterWallFaceIDs...)
	q.InnerFaces = append([]int64(nil), m.SourceInnerWallFaceIDs...)
	for i, side := range []WrappedRationalStationLoftBody{m.Body.Outer, m.Body.Inner} {
		pseudo.Body = side
		p, e := weightedAssemblyNativeClaimBound(ctx, pseudo, 32)
		if e != nil {
			return q, e
		}
		c := weightedBoreSideClaim{side.Name, side.CapNames, side.LengthUnit, p.Walls, p.Native}
		if i == 0 {
			q.Outer = c
		} else {
			q.Inner = c
		}
	}
	return q, nil
}

// WriteWrappedRationalStationBoreAssemblyJSON stores one local native asset and the
// original STEP graph. A complete source reproof binds every support and use
// before any output. This distinct version covers through bores only.
func WriteWrappedRationalStationBoreAssemblyJSON(ctx context.Context, w io.Writer, m WrappedRationalStationBoreAssembly) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	q, e := weightedBoreAssemblyNativeClaim(ctx, m)
	if e != nil {
		return e
	}
	proof, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewBufferString(q.Proof.Source), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		return e
	}
	expected, e := weightedBoreAssemblyNativeClaim(ctx, proof)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(q, expected) {
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

// ReadWrappedRationalStationBoreAssemblyJSON re-proves the original local geometry
// and each source occurrence once, comparing all stored claims atomically.
// Native geometry is regenerated from STEP, never trusted from the payload.
func ReadWrappedRationalStationBoreAssemblyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (WrappedRationalStationBoreAssembly, error) {
	var zero WrappedRationalStationBoreAssembly
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
	var q weightedBoreAssemblyNativeEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Proof.Format != weightedBoreAssemblyNativeFormat || q.Proof.Version != 1 || len(q.Proof.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := ReadWrappedRationalStationBoreAssembly(ctx, bytes.NewBufferString(q.Proof.Source), l.Source, WrappedSurfaceOptions{})
	if e != nil {
		return zero, e
	}
	expected, e := weightedBoreAssemblyNativeClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	q.Proof.Native, e = independentCompact(q.Proof.Native)
	if e != nil {
		return zero, e
	}
	q.Outer.Native, e = independentCompact(q.Outer.Native)
	if e != nil {
		return zero, e
	}
	q.Inner.Native, e = independentCompact(q.Inner.Native)
	if e != nil || !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
