package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

const affinePCurveHierarchyFormat = "plainsolid-step-affine-pcurve-rational-hierarchy"

type affinePCurveHierarchyEnvelope struct {
	Format              string
	Version             int
	OriginalSTEP        string
	OCCTFrameConvention bool
	Tolerance           float64
	Evidence            []AffinePCurveEvidence
	Proof               rationalSphericalHierarchyEnvelope
}

func affinePCurveEnvelope(ctx context.Context, m AffinePCurveHierarchy) (affinePCurveHierarchyEnvelope, error) {
	q := affinePCurveHierarchyEnvelope{Format: affinePCurveHierarchyFormat, Version: 1, OriginalSTEP: m.original, OCCTFrameConvention: m.vendor, Tolerance: m.tolerance, Evidence: m.AffinePCurves()}
	var e error
	q.Proof, _, e = rationalSphericalHierarchyEnvelopeFor(ctx, m.model, m.vendor)
	return q, e
}

// WriteAffinePCurveHierarchyJSON persists original source, exact affine evidence,
// and the canonical native proof view. The frame convention is an explicit tag.
func WriteAffinePCurveHierarchyJSON(ctx context.Context, w io.Writer, m AffinePCurveHierarchy) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	if m.original == "" {
		return geom.ErrInvalid
	}
	q, e := affinePCurveEnvelope(ctx, m)
	if e != nil {
		return e
	}
	// Reprove original ownership rather than trusting even private cached data.
	proof, e := readAffinePCurveHierarchy(ctx, bytes.NewBufferString(m.original), Limits{}, m.tolerance, m.vendor)
	if e != nil {
		return e
	}
	expected, e := affinePCurveEnvelope(ctx, proof)
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
		return io.ErrShortWrite
	}
	return e
}

// ReadAffinePCurveHierarchyJSON atomically reconstructs through the original
// source proof and checks every stored model, ancestry and coefficient claim.
func ReadAffinePCurveHierarchyJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (AffinePCurveHierarchy, error) {
	var zero AffinePCurveHierarchy
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
	if int64(len(b)) > l.MaxBytes {
		return zero, geom.ErrInvalid
	}
	var q affinePCurveHierarchyEnvelope
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != affinePCurveHierarchyFormat || q.Version != 1 || len(q.Evidence) > 4096 {
		return zero, geom.ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return zero, geom.ErrInvalid
	}
	m, e := readAffinePCurveHierarchy(ctx, bytes.NewBufferString(q.OriginalSTEP), l.Source, q.Tolerance, q.OCCTFrameConvention)
	if e != nil {
		return zero, e
	}
	expected, e := affinePCurveEnvelope(ctx, m)
	if e != nil {
		return zero, e
	}
	for i := range q.Proof.LocalProof.Proof.Members {
		p := &q.Proof.LocalProof.Proof.Members[i]
		p.Claims, e = independentCompact(p.Claims)
		if e != nil {
			return zero, e
		}
		p.Native, e = independentCompact(p.Native)
		if e != nil {
			return zero, e
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return zero, geom.ErrInvalid
	}
	return m, nil
}

// ProofHierarchy returns a freshly re-proved, caller-owned normalized hierarchy.
// Its Source is the affine LINE proof view, NOT the original authored STEP.
// OriginalSTEP retains the original splines and their knot domains unchanged.
func (m AffinePCurveHierarchy) ProofHierarchy(ctx context.Context) (RationalSphericalHierarchy, error) {
	p, e := readAffinePCurveHierarchy(ctx, bytes.NewBufferString(m.original), Limits{}, m.tolerance, m.vendor)
	if e != nil {
		return RationalSphericalHierarchy{}, e
	}
	return p.model, nil
}

// LocatePoint preserves original source face/edge/vertex and occurrence IDs.
func (m AffinePCurveHierarchy) LocatePoint(ctx context.Context, p [3]*big.Rat, o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
	if m.vendor {
		return LocateGeneralOCCTRationalHierarchyPoint(ctx, OCCTRationalSphericalHierarchy(m.model), p, o)
	}
	return LocateGeneralRationalHierarchyPoint(ctx, m.model, p, o)
}

// ClipLine includes exact coplanar holed-face spans; material is per occurrence,
// not the Boolean union of assembly instances.
func (m AffinePCurveHierarchy) ClipLine(ctx context.Context, p, d [3]*big.Rat, lo, hi *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	if m.vendor {
		return ClipCoplanarGeneralOCCTRationalHierarchyLine(ctx, OCCTRationalSphericalHierarchy(m.model), p, d, lo, hi, o)
	}
	return ClipCoplanarGeneralRationalHierarchyLine(ctx, m.model, p, d, lo, hi, o)
}
