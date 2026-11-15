package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeMappedChainShellTrim struct {
	Controls     [][]string
	RationalEnds bool `json:",omitempty"`
}

type nativeMappedChainShellFace struct {
	Surface  json.RawMessage `json:",omitempty"`
	Outer    []nativeMappedChainShellTrim
	Reversed bool
}

type nativeMappedChainShell struct {
	Version  int
	Kind     string
	Boundary json.RawMessage
	Curved   []nativeMappedChainShellFace
	Planar   []nativeMappedChainShellFace
	Seams    [][]MappedChainClosedUse
}

// MappedChainShellNativeOptions bounds the standalone source-shell format.
// MaxBytes defaults to 1 MiB (maximum 32 MiB). Chain and Shell bound the
// rebuilt source proofs. Each planar support allows up to 4096 controls.
type MappedChainShellNativeOptions struct {
	MaxBytes int64
	Chain    edgecheck.Options
	Shell    MappedChainShellProofOptions
}

// WriteJSON stores the original support charts, spline owners, UV trims and
// coedge pairings. It re-proves those source records under the reader's limits
// before writing. Original support-joint ownership, where present, is
// checked against the source. Tessellation data is absent from the format.
func (r MappedSplineChainShell) WriteJSON(ctx context.Context, w io.Writer, o MappedChainShellNativeOptions) error {
	if ctx == nil || w == nil || r.topology.Faces < 3 {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	maxBytes, err := mappedChainNativeMax(o.MaxBytes)
	if err != nil {
		return err
	}
	var boundary bytes.Buffer
	if err := r.boundary.WriteJSON(ctx, &boundary, MappedChainNativeOptions{MaxBytes: maxBytes, Proof: o.Chain}); err != nil {
		return err
	}
	record := nativeMappedChainShell{
		Version: 1, Kind: "mapped-spline-chain-shell", Boundary: boundary.Bytes(),
		Curved: make([]nativeMappedChainShellFace, 2), Planar: make([]nativeMappedChainShellFace, len(r.planar)),
		Seams: make([][]MappedChainClosedUse, len(r.seams)),
	}
	for i, closing := range r.closing {
		outer, err := encodeMappedChainShellTrims(closing)
		if err != nil {
			return err
		}
		record.Curved[i] = nativeMappedChainShellFace{Outer: outer, Reversed: r.reversed[i]}
	}
	for i, p := range r.planar {
		support, err := parametric.EncodeSurface(p.Surface)
		if err != nil {
			return err
		}
		outer, err := encodeMappedChainShellTrims(p.Outer)
		if err != nil {
			return err
		}
		record.Planar[i] = nativeMappedChainShellFace{Surface: support, Outer: outer, Reversed: p.Reversed}
	}
	for i, seam := range r.seams {
		record.Seams[i] = []MappedChainClosedUse{seam[0], seam[1]}
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if int64(len(raw)) > maxBytes {
		return geom.ErrConvergence
	}
	if _, err := ReadMappedSplineChainShellJSON(ctx, bytes.NewReader(raw), o); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadMappedSplineChainShellJSON re-proves the original mapped chain, source
// trim domains and loops, affine charts, continuous seam agreement, and closed
// source topology. Tessellation is a separate operation on the result.
func ReadMappedSplineChainShellJSON(ctx context.Context, r io.Reader, o MappedChainShellNativeOptions) (MappedSplineChainShell, error) {
	var zero MappedSplineChainShell
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	maxBytes, err := mappedChainNativeMax(o.MaxBytes)
	if err != nil {
		return zero, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > maxBytes {
		return zero, geom.ErrConvergence
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record nativeMappedChainShell
	if err := decoder.Decode(&record); err != nil {
		return zero, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return zero, geom.ErrInvalid
	}
	if record.Version != 1 || record.Kind != "mapped-spline-chain-shell" || len(record.Curved) != 2 ||
		len(record.Planar) < 1 || len(record.Planar) > 14 || len(record.Seams) > 64 {
		return zero, geom.ErrInvalid
	}
	chain, err := ReadMappedSplineChainBoundaryJSON(ctx, bytes.NewReader(record.Boundary), MappedChainNativeOptions{MaxBytes: maxBytes, Proof: o.Chain})
	if err != nil {
		return zero, fmt.Errorf("mapped shell source chain: %w", err)
	}
	var closing [2][]PCurve
	var reversed [2]bool
	for i, face := range record.Curved {
		if len(face.Surface) != 0 {
			return zero, geom.ErrInvalid
		}
		closing[i], err = decodeMappedChainShellTrims(face.Outer)
		if err != nil {
			return zero, fmt.Errorf("mapped shell source face %d: %w", i, err)
		}
		reversed[i] = face.Reversed
	}
	planar := make([]MappedChainPlanarFace, len(record.Planar))
	for i, face := range record.Planar {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		p, err := parametric.DecodeSurface(face.Surface, 4096)
		if err != nil {
			return zero, fmt.Errorf("mapped shell source face %d support: %w", i+2, err)
		}
		outer, err := decodeMappedChainShellTrims(face.Outer)
		if err != nil {
			return zero, fmt.Errorf("mapped shell source face %d: %w", i+2, err)
		}
		planar[i] = MappedChainPlanarFace{Surface: p, Outer: outer, Reversed: face.Reversed}
	}
	seams := make([][2]MappedChainClosedUse, len(record.Seams))
	for i, seam := range record.Seams {
		if len(seam) != 2 {
			return zero, geom.ErrInvalid
		}
		seams[i] = [2]MappedChainClosedUse{seam[0], seam[1]}
	}
	return CertifyMappedSplineChainShell(ctx, chain, closing, reversed, planar, seams, o.Shell)
}

func encodeMappedChainShellTrims(loop []PCurve) ([]nativeMappedChainShellTrim, error) {
	out := make([]nativeMappedChainShellTrim, len(loop))
	for i, c := range loop {
		h, err := mixedExactControls(c)
		if err != nil {
			return nil, err
		}
		if q, ok := c.(ExactQuadraticUV); ok {
			out[i].RationalEnds = q.rationalEndpoints
		}
		for _, p := range h {
			if p[0] == nil || p[1] == nil || p[2] == nil {
				return nil, geom.ErrInvalid
			}
			out[i].Controls = append(out[i].Controls, []string{p[0].RatString(), p[1].RatString(), p[2].RatString()})
		}
	}
	return out, nil
}

func decodeMappedChainShellTrims(loop []nativeMappedChainShellTrim) ([]PCurve, error) {
	if len(loop) < 1 || len(loop) > 16 {
		return nil, geom.ErrInvalid
	}
	out := make([]PCurve, len(loop))
	for i, t := range loop {
		if len(t.Controls) < 3 || len(t.Controls) > 5 || (t.RationalEnds && len(t.Controls) != 3) {
			return nil, geom.ErrInvalid
		}
		h := make([][3]*big.Rat, len(t.Controls))
		for j, row := range t.Controls {
			if len(row) != 3 {
				return nil, geom.ErrInvalid
			}
			for k, value := range row {
				var err error
				h[j][k], err = parseNativeExactTrimCoefficient(value)
				if err != nil {
					return nil, err
				}
			}
		}
		var err error
		if len(h) == 3 {
			out[i], err = newExactQuadraticUV([3][3]*big.Rat(h), t.RationalEnds)
		} else {
			out[i], err = NewExactBezierUV(h)
		}
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
