package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
)

const posedGroupedPCurveRepairFormat = "plainsolid-step-posed-grouped-pcurve-convex-arc-repair"

type posedGroupedPCurveRepairEnvelope struct {
	Format      string
	Version     int
	Source      string
	MaxDistance float64
	Claims      json.RawMessage
}

func posedGroupedPCurveRepairClaim(ctx context.Context, m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) ([]byte, error) {
	base, e := groupedPCurveRepairClaim(ctx, m.base)
	if e != nil {
		return nil, e
	}
	return json.Marshal(struct {
		Proof             json.RawMessage
		Rotation          geom.Affine
		ReferenceCircleID int64
	}{base, m.rotation, m.reference})
}

// WritePosedGroupedPCurveRepairedOCCTConvexArcJSON retains all original authoritative
// pcurves and explicit repair evidence after one complete source reproof.
func WritePosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, w io.Writer, m PosedGroupedPCurveRepairedOCCTConvexArcExtrusion) error {
	if w == nil {
		return geom.ErrInvalid
	}
	claim, e := posedGroupedPCurveRepairClaim(ctx, m)
	if e != nil {
		return e
	}
	p, e := RepairPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewBufferString(m.base.source), Limits{}, ConvexArcRepairOptions{m.base.tolerance})
	if e != nil {
		return e
	}
	want, e := posedGroupedPCurveRepairClaim(ctx, p)
	if e != nil {
		return e
	}
	if !bytes.Equal(claim, want) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(posedGroupedPCurveRepairEnvelope{posedGroupedPCurveRepairFormat, 1, m.base.source, m.base.tolerance, claim})
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > 16<<20 {
		return geom.ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(raw)
	if e == nil && n != len(raw) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadPosedGroupedPCurveRepairedOCCTConvexArcJSON reconstructs the original source and
// compares canonical claims without parsing untrusted rational evidence.
func ReadPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (PosedGroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero PosedGroupedPCurveRepairedOCCTConvexArcExtrusion
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
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var q posedGroupedPCurveRepairEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != posedGroupedPCurveRepairFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := RepairPosedGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewBufferString(q.Source), l.Source, ConvexArcRepairOptions{q.MaxDistance})
	if e != nil {
		return zero, e
	}
	want, e := posedGroupedPCurveRepairClaim(ctx, m)
	if e != nil {
		return zero, e
	}
	claim, e := independentCompact(q.Claims)
	if e != nil || !bytes.Equal(claim, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}
