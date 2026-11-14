package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"plainsolid/geom"
	"reflect"
)

const normalizedRoundFormat = "plainsolid-step-normalized-independent-round-materials"

type normalizedRoundEnvelope struct {
	Format         string
	Version        int
	OriginalSource string
	TargetUnit     RoundMaterialSIUnit
	OriginalUnits  []RoundMaterialSIUnit
	Normalized     independentMaterialEnvelope
}

func normalizedRoundClaims(ctx context.Context, m NormalizedIndependentRoundMaterials, planarMode ...bool) (normalizedRoundEnvelope, error) {
	if len(m.OriginalUnits) != len(m.Members) || len(m.Members) < 1 || len(m.Members) > 16 {
		return normalizedRoundEnvelope{}, geom.ErrInvalid
	}
	if !independentNativeSourceBound(m.OriginalSource, 16*1024) || !independentNativeSourceBound(m.NormalizedSource, 16*1024) {
		return normalizedRoundEnvelope{}, geom.ErrInvalid
	}
	original, e := mappedPlanarSourceBytes(m.OriginalSource)
	if e != nil {
		return normalizedRoundEnvelope{}, e
	}
	normalized, e := mappedPlanarSourceBytes(m.NormalizedSource)
	if e != nil {
		return normalizedRoundEnvelope{}, e
	}
	unit := 1.0
	if m.TargetUnit == RoundMillimetre {
		unit = .001
	} else if m.TargetUnit != RoundMetre {
		return normalizedRoundEnvelope{}, geom.ErrInvalid
	}
	c := IndependentExactMaterialCollection{Source: m.NormalizedSource, Members: m.Members, LengthUnit: unit, tolerance: m.tolerance}
	claims, _, e := independentCollectionEnvelope(ctx, c, normalized, true, true, len(planarMode) == 3 && planarMode[2])
	if e != nil {
		return normalizedRoundEnvelope{}, e
	}
	claims.Format = "plainsolid-step-normalized-round-material-claims"
	format := normalizedRoundFormat
	if len(planarMode) >= 1 && planarMode[0] {
		format = normalizedPlanarFormat
		claims.Format = "plainsolid-step-normalized-planar-material-claims"
	}
	if len(planarMode) >= 2 && planarMode[1] {
		format = normalizedMixedFormat
		claims.Format = "plainsolid-step-normalized-heterogeneous-material-claims"
	}
	if len(planarMode) == 3 && planarMode[2] {
		found := false
		for _, p := range m.Members {
			found = found || p.Torus != nil
		}
		if !found {
			return normalizedRoundEnvelope{}, geom.ErrInvalid
		}
		format = normalizedToroidalFormat
		claims.Format = "plainsolid-step-normalized-toroidal-material-claims"
	}
	return normalizedRoundEnvelope{format, 1, string(original), m.TargetUnit, append([]RoundMaterialSIUnit(nil), m.OriginalUnits...), claims}, nil
}

// WriteNormalizedIndependentRoundMaterialsJSON re-proves normalization from the
// original graph, then compares both source graphs, units, native shapes and
// original role IDs. No output is written until all claims have been checked.
func WriteNormalizedIndependentRoundMaterialsJSON(ctx context.Context, w io.Writer, m NormalizedIndependentRoundMaterials) error {
	return writeNormalizedMaterialsJSON(ctx, w, m, false)
}
func writeNormalizedMaterialsJSON(ctx context.Context, w io.Writer, m NormalizedIndependentRoundMaterials, planar bool, mixedMode ...bool) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	mixed := len(mixedMode) >= 1 && mixedMode[0]
	torus := mixed && len(mixedMode) == 2 && mixedMode[1]
	actual, e := normalizedRoundClaims(ctx, m, planar, mixed, torus)
	if e != nil {
		return e
	}
	proof, e := readNormalizedIndependentUnits(ctx, bytes.NewReader([]byte(actual.OriginalSource)), m.TargetUnit, Limits{}, m.tolerance, planar, mixed, torus)
	if e != nil {
		return e
	}
	expected, e := normalizedRoundClaims(ctx, proof, planar, mixed, torus)
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(actual, expected) {
		return geom.ErrInvalid
	}
	raw, e := json.Marshal(expected)
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

// ReadNormalizedIndependentRoundMaterialsJSON runs the bounded normalization
// pipeline once from OriginalSource. Stored normalized/native data are compared
// with the proof, never used as reconstruction geometry. Source limits apply to
// original and normalized STEP validation separately; MaxBytes bounds the envelope.
func ReadNormalizedIndependentRoundMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (NormalizedIndependentRoundMaterials, error) {
	return readNormalizedMaterialsJSON(ctx, r, l, false)
}
func readNormalizedMaterialsJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, planar bool, mixedMode ...bool) (NormalizedIndependentRoundMaterials, error) {
	bad := func() (NormalizedIndependentRoundMaterials, error) {
		return NormalizedIndependentRoundMaterials{}, geom.ErrInvalid
	}
	if ctx == nil || r == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 16 << 20
	}
	if l.MaxBytes < 1 || l.MaxBytes > 16<<20 {
		return bad()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	format, claimsFormat := normalizedRoundFormat, "plainsolid-step-normalized-round-material-claims"
	if planar {
		format = normalizedPlanarFormat
		claimsFormat = "plainsolid-step-normalized-planar-material-claims"
	}
	mixed := len(mixedMode) >= 1 && mixedMode[0]
	torus := mixed && len(mixedMode) == 2 && mixedMode[1]
	if mixed {
		format = normalizedMixedFormat
		claimsFormat = "plainsolid-step-normalized-heterogeneous-material-claims"
	}
	if torus {
		format = normalizedToroidalFormat
		claimsFormat = "plainsolid-step-normalized-toroidal-material-claims"
	}
	var q normalizedRoundEnvelope
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil || q.Format != format || q.Version != 1 || len(q.OriginalUnits) < 1 || len(q.OriginalUnits) > 16 || len(q.Normalized.Members) != len(q.OriginalUnits) || q.Normalized.Format != claimsFormat || q.Normalized.Version != 1 {
		return bad()
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return bad()
	}
	proof, e := readNormalizedIndependentUnits(ctx, bytes.NewReader([]byte(q.OriginalSource)), q.TargetUnit, l.Source, q.Normalized.Tolerance, planar, mixed, torus)
	if e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	expected, e := normalizedRoundClaims(ctx, proof, planar, mixed, torus)
	if e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	for i := range q.Normalized.Members {
		q.Normalized.Members[i].Claims, e = independentCompact(q.Normalized.Members[i].Claims)
		if e != nil {
			return bad()
		}
		q.Normalized.Members[i].Native, e = independentCompact(q.Normalized.Members[i].Native)
		if e != nil {
			return bad()
		}
	}
	if !reflect.DeepEqual(q, expected) {
		return bad()
	}
	if e = ctx.Err(); e != nil {
		return NormalizedIndependentRoundMaterials{}, e
	}
	return proof, nil
}
