package iges

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"plainsolid/geom"
)

const conicalRimFilletBoreRepairFormat = "plainsolid-iges-conical-rim-fillet-bore-repair-source"
const maxConicalRimFilletBoreRepairReceipt = 512 << 10

// ConicalRimFilletBoreRepairJSONLimits separates the envelope size from
// the source IGES parser limits.
type ConicalRimFilletBoreRepairJSONLimits struct {
	MaxBytes int64
	Source   Limits
}

type conicalRimFilletBoreRepairEnvelope struct {
	Format      string
	Version     int
	Source      []byte
	MaxDistance float64
	Claims      json.RawMessage
}

type conicalRimFilletBoreRepairClaim struct {
	SourceSHA256, NativeSHA256         string
	LengthUnit, MaxDistance            float64
	GroupID, SolidID, ShellID          int
	EdgeListID, VertexListID           int
	FaceIDs, CurveIDs, Edges, Vertices []int
	Evidence                           ConicalRimFilletBoreRepairEvidence
}

func conicalRimFilletBoreRepairReceipt(m RepairedConicalRimFilletBoreIGES) ([]byte, error) {
	p := m.material
	native, err := igesPrismBytes(p.Solid)
	if err != nil {
		return nil, err
	}
	claim := conicalRimFilletBoreRepairClaim{
		SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(p.source)), NativeSHA256: fmt.Sprintf("%x", sha256.Sum256(native)),
		LengthUnit: p.LengthUnit, MaxDistance: m.maxDistance, GroupID: p.SourceGroupID, SolidID: p.SourceSolidID, ShellID: p.SourceShellID,
		EdgeListID: p.SourceEdgeListID, VertexListID: p.SourceVertexListID, FaceIDs: p.NativeFaceSourceIDs, CurveIDs: p.NativeCurveSourceIDs,
		Edges: p.NativeEdgeSourceIndices, Vertices: p.NativeVertexSourceIndices, Evidence: m.evidence,
	}
	claims, err := json.Marshal(claim)
	if err != nil {
		return nil, err
	}
	return json.Marshal(conicalRimFilletBoreRepairEnvelope{conicalRimFilletBoreRepairFormat, 1, p.source, m.maxDistance, claims})
}

// WriteRepairedConicalRimFilletBoreJSON writes the original IGES bytes and
// deterministic proof claims after replaying the original repair.
func WriteRepairedConicalRimFilletBoreJSON(ctx context.Context, w io.Writer, m RepairedConicalRimFilletBoreIGES) error {
	if ctx == nil || w == nil || len(m.material.source) == 0 {
		return geom.ErrInvalid
	}
	replayed, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(m.material.source), Limits{}, ConicalRimFilletBoreRepairOptions{m.maxDistance})
	if err != nil {
		return err
	}
	data, err := conicalRimFilletBoreRepairReceipt(replayed)
	if err != nil {
		return err
	}
	want, err := conicalRimFilletBoreRepairReceipt(m)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, want) {
		return geom.ErrInvalid
	}
	if len(data)+1 > maxConicalRimFilletBoreRepairReceipt {
		return geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

// ReadRepairedConicalRimFilletBoreJSON reconstructs the native material
// from the original IGES bytes and verifies the stored canonical claims.
func ReadRepairedConicalRimFilletBoreJSON(ctx context.Context, r io.Reader, l ConicalRimFilletBoreRepairJSONLimits) (RepairedConicalRimFilletBoreIGES, error) {
	var zero RepairedConicalRimFilletBoreIGES
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > maxConicalRimFilletBoreRepairReceipt {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	limit := l.MaxBytes
	if limit == 0 {
		limit = maxConicalRimFilletBoreRepairReceipt
	}
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > limit {
		return zero, geom.ErrConvergence
	}
	var q conicalRimFilletBoreRepairEnvelope
	if json.Unmarshal(raw, &q) != nil || q.Format != conicalRimFilletBoreRepairFormat || q.Version != 1 || len(q.Source) == 0 || len(q.Source) > 128<<10 {
		return zero, geom.ErrInvalid
	}
	m, err := RepairConicalRimFilletBore(ctx, bytes.NewReader(q.Source), l.Source, ConicalRimFilletBoreRepairOptions{q.MaxDistance})
	if err != nil {
		return zero, err
	}
	want, err := conicalRimFilletBoreRepairReceipt(m)
	if err != nil {
		return zero, err
	}
	// Compare the compact envelope byte for byte to reject extra or
	// repeated fields and keep untrusted proof rationals opaque.
	var input bytes.Buffer
	if json.Compact(&input, raw) != nil || !bytes.Equal(input.Bytes(), want) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return m, nil
}
