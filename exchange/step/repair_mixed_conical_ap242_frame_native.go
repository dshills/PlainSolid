package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	"plainsolid/geom"
)

const mixedConicalAP242FrameRepairFormat = "plainsolid-step-mixed-conical-ap242-occurrence-frame-repair-source"

type mixedConicalFrameRepairClaims struct {
	SourceSHA256, CorrectedSHA256 string
	LengthUnit                    float64
	Options                       MixedConicalAP242FrameRepairOptions
	Evidence                      MixedConicalAP242FrameRepairEvidence
	Occurrences                   []nestedMixedConicalAP242OccurrenceClaim
	Instances                     []mixedConicalAP242DAGInstanceClaim
}

func mixedConicalAP242FrameRepairClaims(m RepairedMixedConicalAP242Frame) ([]byte, error) {
	a := m.assembly
	c := mixedConicalFrameRepairClaims{
		SourceSHA256: rationalPosedRepairDigest([]byte(m.original)), CorrectedSHA256: rationalPosedRepairDigest([]byte(m.corrected)),
		LengthUnit: a.Assembly.LengthUnit, Options: m.options, Evidence: m.evidence,
	}
	for _, u := range a.Occurrences {
		c.Occurrences = append(c.Occurrences, nestedMixedConicalAP242OccurrenceClaim{u, u.Matrix()})
	}
	for _, x := range a.Instances {
		c.Instances = append(c.Instances, mixedConicalAP242DAGInstanceClaim{x.ProductID, x.Path, x.Matrix()})
	}
	return json.Marshal(c)
}

// WriteRepairedMixedConicalAP242FrameJSON retains the exact original file
// and all inferred frame, motion and hierarchy claims. It re-proves the
// source before publishing the receipt.
func WriteRepairedMixedConicalAP242FrameJSON(ctx context.Context, w io.Writer, m RepairedMixedConicalAP242Frame) error {
	if ctx == nil || w == nil || m.original == "" {
		return geom.ErrInvalid
	}
	replayed, err := RepairMixedConicalAP242Frame(ctx, strings.NewReader(m.original), MixedConicalAP242DAGLimits{}, m.options)
	if err != nil {
		return err
	}
	claims, err := mixedConicalAP242FrameRepairClaims(m)
	if err != nil {
		return err
	}
	want, err := mixedConicalAP242FrameRepairClaims(replayed)
	if err != nil {
		return err
	}
	if !bytes.Equal(claims, want) {
		return geom.ErrInvalid
	}
	return writeRationalPosedRepairEnvelope(ctx, w, rationalPosedPCurveRepairEnvelope{
		Format: mixedConicalAP242FrameRepairFormat, Version: 1, Source: []byte(m.original), MaxDistance: m.options.MaxDisplacement, Claims: claims,
	})
}

// ReadRepairedMixedConicalAP242FrameJSON replays the actual axis inference,
// the exact scope and all displacement bounds from the retained source.
func ReadRepairedMixedConicalAP242FrameJSON(ctx context.Context, r io.Reader, l MixedConicalAP242DAGRepairJSONLimits) (RepairedMixedConicalAP242Frame, error) {
	var zero RepairedMixedConicalAP242Frame
	q, err := readRationalPosedRepairEnvelope(ctx, r, IndependentMaterialJSONLimits{MaxBytes: l.MaxBytes, Source: l.Source.Source}, mixedConicalAP242FrameRepairFormat)
	if err != nil {
		return zero, err
	}
	var request struct {
		Options MixedConicalAP242FrameRepairOptions
	}
	if err := json.Unmarshal(q.Claims, &request); err != nil || q.MaxDistance != request.Options.MaxDisplacement {
		return zero, geom.ErrInvalid
	}
	m, err := RepairMixedConicalAP242Frame(ctx, bytes.NewReader(q.Source), l.Source, request.Options)
	if err != nil {
		return zero, err
	}
	claims, err := independentCompact(q.Claims)
	if err != nil {
		return zero, err
	}
	want, err := mixedConicalAP242FrameRepairClaims(m)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(claims, want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
