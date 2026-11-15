package step

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"

	rigidplacement "plainsolid/brep/placement"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

const rationalPosedPCurveRepairFormat = "plainsolid-step-rational-posed-pcurve-convex-arc-source"
const rationalPosedGroupedPCurveRepairFormat = "plainsolid-step-rational-posed-grouped-pcurve-convex-arc-source"
const maxRationalPosedRepairJSONBytes = 16 << 20

type rationalPosedPCurveRepairEnvelope struct {
	Format      string
	Version     int
	Source      []byte
	MaxDistance float64
	Claims      json.RawMessage
}

type rationalPosedPCurveRepairMaterial interface {
	OriginalSTEP() string
	CorrectedModelSTEP() string
	Solid() trimmed.Solid
	LocalDescriptor() ExactCircularLoopExtrusion
	Pose() rigidplacement.RationalRigid
	ReferenceCircleID() int64
	Provenance() ConvexArcRepairProvenance
	Repairs() []ConvexArcRepairSite
	Tangencies() []ConvexArcTangentEvidence
	PCurves() []ConvexArcPCurveAgreement
	FiniteLineOwners() []FiniteConvexArcLineOwner
}

type rationalPosedGroupedCapClaim struct {
	GroupedCapFiniteTrim
	EndStation geom.ExactRootRecipe
}

type rationalPosedGroupedRepairClaim struct {
	AxialTrims   []ConvexArcAxialTrimRepair
	Reversals    []ConvexArcRepairReversal
	RationalCaps []GroupedRationalCapEvidence
	FiniteTrims  []rationalPosedGroupedCapClaim
}

func rationalPosedRepairDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func rationalPosedPCurveClaims(ctx context.Context, m rationalPosedPCurveRepairMaterial, distance float64) ([]byte, error) {
	if ctx == nil || m.OriginalSTEP() == "" || m.CorrectedModelSTEP() == "" {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	pose, e := m.Pose().Matrix()
	if e != nil {
		return nil, e
	}
	native, e := prismNativeBytes(m.Solid())
	if e != nil {
		return nil, e
	}
	d := m.LocalDescriptor()
	recipe, e := json.Marshal(exactLoopNativeRecipe{d.Name, frameTransform(d.Frame), d.Segments, d.Height, d.LengthUnit})
	if e != nil {
		return nil, e
	}
	claim := struct {
		SourceSHA256          string
		MaxDistance           float64
		Pose                  [3][4]*big.Rat
		ReferenceCircleID     int64
		CorrectedModelSHA256  string
		NativeSHA256          string
		LocalRecipeSHA256     string
		Provenance            ConvexArcRepairProvenance
		Repairs               []ConvexArcRepairSite
		Tangencies            []ConvexArcTangentEvidence
		AzimuthDirections     []int64
		PCurves               []ConvexArcPCurveAgreement
		FiniteLineOwners      []FiniteConvexArcLineOwner
		CylindricalSpanOwners []ConvexArcCylinderSpanOwner     `json:",omitempty"`
		Grouped               *rationalPosedGroupedRepairClaim `json:",omitempty"`
	}{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.OriginalSTEP())), MaxDistance: distance,
		Pose: pose, ReferenceCircleID: m.ReferenceCircleID(),
		CorrectedModelSHA256: rationalPosedRepairDigest([]byte(m.CorrectedModelSTEP())),
		NativeSHA256:         rationalPosedRepairDigest(native),
		LocalRecipeSHA256:    rationalPosedRepairDigest(recipe),
		Provenance:           m.Provenance(), Repairs: m.Repairs(), Tangencies: m.Tangencies(),
		PCurves: m.PCurves(), FiniteLineOwners: m.FiniteLineOwners(),
	}
	switch p := m.(type) {
	case RationalPosedPCurveRepairedOCCTConvexArcExtrusion:
		claim.AzimuthDirections = p.base.material.AzimuthDirections()
		claim.CylindricalSpanOwners = p.CylindricalSpanOwners()
	case RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion:
		claim.AzimuthDirections = p.base.material.AzimuthDirections()
		claim.CylindricalSpanOwners = p.CylindricalSpanOwners()
		group := &rationalPosedGroupedRepairClaim{
			AxialTrims: p.AxialTrims(), Reversals: p.Reversals(), RationalCaps: p.RationalCaps(),
		}
		for _, trim := range p.FiniteTrims() {
			root, e := trim.EndStation.Export(ctx, geom.ExactRootCodecOptions{MaxDegree: 2})
			if e != nil {
				return nil, e
			}
			group.FiniteTrims = append(group.FiniteTrims, rationalPosedGroupedCapClaim{GroupedCapFiniteTrim: trim, EndStation: root})
		}
		claim.Grouped = group
	default:
		return nil, geom.ErrInvalid
	}
	raw, e := json.Marshal(claim)
	if e != nil {
		return nil, e
	}
	return raw, ctx.Err()
}

type rationalPosedPCurveSourceReader[M rationalPosedPCurveRepairMaterial] func(context.Context, io.Reader, Limits, ConvexArcRepairOptions) (M, error)

func writeRationalPosedPCurveRepairJSON[M rationalPosedPCurveRepairMaterial](ctx context.Context, w io.Writer, m M, format string, distance float64, reconstruct rationalPosedPCurveSourceReader[M]) error {
	if ctx == nil || w == nil || !finiteNumber(distance) || distance <= 0 {
		return geom.ErrInvalid
	}
	claim, e := rationalPosedPCurveClaims(ctx, m, distance)
	if e != nil {
		return e
	}
	source := []byte(m.OriginalSTEP())
	p, e := reconstruct(ctx, bytes.NewReader(source), Limits{}, ConvexArcRepairOptions{MaxDistance: distance})
	if e != nil {
		return e
	}
	want, e := rationalPosedPCurveClaims(ctx, p, distance)
	if e != nil {
		return e
	}
	if !bytes.Equal(claim, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{Format: format, Version: 1, Source: source, MaxDistance: distance, Claims: claim})
}

func writeRationalPosedRepairEnvelope(ctx context.Context, w io.Writer, q rationalPosedPCurveRepairEnvelope) error {
	raw, e := json.Marshal(q)
	if e != nil {
		return e
	}
	raw = append(raw, '\n')
	if len(raw) > maxRationalPosedRepairJSONBytes {
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

func readRationalPosedPCurveRepairEnvelope(raw []byte) (rationalPosedPCurveRepairEnvelope, error) {
	var q rationalPosedPCurveRepairEnvelope
	var source json.RawMessage
	fields := map[string]any{
		"Format": &q.Format, "Version": &q.Version, "Source": &source,
		"MaxDistance": &q.MaxDistance, "Claims": &q.Claims,
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	start, e := d.Token()
	if e != nil || start != json.Delim('{') {
		return q, geom.ErrInvalid
	}
	for d.More() {
		key, e := d.Token()
		if e != nil {
			return q, geom.ErrInvalid
		}
		name, ok := key.(string)
		field := fields[name]
		if !ok || field == nil || d.Decode(field) != nil {
			return q, geom.ErrInvalid
		}
		delete(fields, name)
	}
	end, e := d.Token()
	if e != nil || end != json.Delim('}') || len(fields) != 0 || d.Decode(new(any)) != io.EOF {
		return q, geom.ErrInvalid
	}
	if len(source) == 0 || source[0] != '"' || json.Unmarshal(source, &q.Source) != nil {
		return q, geom.ErrInvalid
	}
	return q, nil
}

func readRationalPosedRepairEnvelope(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, format string) (rationalPosedPCurveRepairEnvelope, error) {
	var zero rationalPosedPCurveRepairEnvelope
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = maxRationalPosedRepairJSONBytes
	}
	if l.MaxBytes < 1 || l.MaxBytes > maxRationalPosedRepairJSONBytes {
		return zero, geom.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	q, e := readRationalPosedPCurveRepairEnvelope(raw)
	if e != nil || q.Format != format || q.Version != 1 || len(q.Source) == 0 || len(q.Source) > 1<<20 ||
		!finiteNumber(q.MaxDistance) || q.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	return q, nil
}

func readRationalPosedPCurveRepairJSON[M rationalPosedPCurveRepairMaterial](ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits, format string, reconstruct rationalPosedPCurveSourceReader[M]) (M, error) {
	var zero M
	q, e := readRationalPosedRepairEnvelope(ctx, r, l, format)
	if e != nil {
		return zero, e
	}
	m, e := reconstruct(ctx, bytes.NewReader(q.Source), l.Source, ConvexArcRepairOptions{MaxDistance: q.MaxDistance})
	if e != nil {
		return zero, e
	}
	want, e := rationalPosedPCurveClaims(ctx, m, q.MaxDistance)
	if e != nil {
		return zero, e
	}
	// Proof claims stay opaque until comparison so a supplied rational
	// cannot ask math/big to allocate memory or drive proof work.
	claim, e := independentCompact(q.Claims)
	if e != nil || !bytes.Equal(claim, want) {
		return zero, geom.ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return m, nil
}

// WriteRationalPosedPCurveRepairedOCCTConvexArcJSON independently re-proves
// the original single-fillet source and stores only source and proof claims.
func WriteRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, w io.Writer, m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) error {
	return writeRationalPosedPCurveRepairJSON(ctx, w, m, rationalPosedPCurveRepairFormat, m.base.tolerance, RepairRationalPosedPCurveOCCTConvexArcExtrusion)
}

// ReadRationalPosedPCurveRepairedOCCTConvexArcJSON reconstructs the native
// single-fillet material from the original STEP and checks every stored claim.
func ReadRationalPosedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalPosedPCurveRepairedOCCTConvexArcExtrusion, error) {
	return readRationalPosedPCurveRepairJSON(ctx, r, l, rationalPosedPCurveRepairFormat, RepairRationalPosedPCurveOCCTConvexArcExtrusion)
}

// WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON independently
// re-proves the original grouped source and stores only source and proof claims.
func WriteRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, w io.Writer, m RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion) error {
	return writeRationalPosedPCurveRepairJSON(ctx, w, m, rationalPosedGroupedPCurveRepairFormat, m.base.tolerance, RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion)
}

// ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON reconstructs the
// native grouped material from the original STEP and checks every stored claim.
func ReadRationalPosedGroupedPCurveRepairedOCCTConvexArcJSON(ctx context.Context, r io.Reader, l IndependentMaterialJSONLimits) (RationalPosedGroupedPCurveRepairedOCCTConvexArcExtrusion, error) {
	return readRationalPosedPCurveRepairJSON(ctx, r, l, rationalPosedGroupedPCurveRepairFormat, RepairRationalPosedGroupedPCurveOCCTConvexArcExtrusion)
}
