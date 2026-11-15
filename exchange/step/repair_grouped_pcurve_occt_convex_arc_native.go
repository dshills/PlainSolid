package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
)

const groupedPCurveRepairFormat = "plainsolid-step-grouped-pcurve-convex-arc-repair"

type groupedPCurveRepairEnvelope struct {
	Format      string
	Version     int
	Source      string
	MaxDistance float64
	Claims      json.RawMessage
}

func groupedPCurveRepairClaim(ctx context.Context, m GroupedPCurveRepairedOCCTConvexArcExtrusion) ([]byte, error) {
	if ctx == nil || m.source == "" {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	native, e := prismNativeBytes(m.Solid())
	if e != nil {
		return nil, e
	}
	compact, e := independentCompact(native)
	if e != nil {
		return nil, e
	}
	d := m.Descriptor()
	claim := struct {
		CorrectedModel string
		Recipe         exactLoopNativeRecipe
		Provenance     ConvexArcRepairProvenance
		Repairs        []ConvexArcRepairSite
		Tangencies     []ConvexArcTangentEvidence
		FiniteProof    GroupedFiniteTrimEvidence
		Reversals      []ConvexArcRepairReversal
		Native         json.RawMessage
	}{m.CorrectedModelSTEP(), exactLoopNativeRecipe{d.Name, frameTransform(d.Frame), d.Segments, d.Height, d.LengthUnit}, m.Provenance(), m.Repairs(), m.Tangencies(), m.finite, m.Reversals(), compact}
	raw, e := json.Marshal(claim)
	if e != nil {
		return nil, e
	}
	return raw, ctx.Err()
}

// WriteGroupedPCurveRepairedOCCTConvexArcJSON retains all original authoritative
// pcurves and explicit repair evidence after one complete source reproof.
func WriteGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, w io.Writer, m GroupedPCurveRepairedOCCTConvexArcExtrusion) error {
	if w == nil {
		return geom.ErrInvalid
	}
	claim, e := groupedPCurveRepairClaim(ctx, m)
	if e != nil {
		return e
	}
	p, e := RepairGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewBufferString(m.source), Limits{}, ConvexArcRepairOptions{m.tolerance})
	if e != nil {
		return e
	}
	want, e := groupedPCurveRepairClaim(ctx, p)
	if e != nil {
		return e
	}
	if !bytes.Equal(claim, want) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(groupedPCurveRepairEnvelope{groupedPCurveRepairFormat, 1, m.source, m.tolerance, claim})
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

// ReadGroupedPCurveRepairedOCCTConvexArcJSON reconstructs the original source and
// compares canonical claims without parsing untrusted rational evidence.
func ReadGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (GroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	var zero GroupedPCurveRepairedOCCTConvexArcExtrusion
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
	var q groupedPCurveRepairEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != groupedPCurveRepairFormat || q.Version != 1 || len(q.Source) > 1<<20 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := RepairGroupedPCurveOCCTConvexArcExtrusion(ctx, bytes.NewBufferString(q.Source), l.Source, ConvexArcRepairOptions{q.MaxDistance})
	if e != nil {
		return zero, e
	}
	want, e := groupedPCurveRepairClaim(ctx, m)
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
