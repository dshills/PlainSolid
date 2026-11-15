package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

const circularSweepInsetsFormat = "plainsolid-exact-circular-sweep-insets"
const circularSweepInsetsJSONBytes = 16 << 20

// CircularSweepInsetsJSONLimits bounds a source-backed inset receipt. Zero
// selects the maximum supported bytes, guests, or total original segments.
type CircularSweepInsetsJSONLimits struct {
	MaxBytes               int64
	MaxGuests, MaxSegments int
}

// CircularSweepInsetsRecord contains the exact source sweeps and their
// reconstructed joined support and ownership graph.
type CircularSweepInsetsRecord struct {
	Host   ExactCircularSweep
	Guests []ExactCircularSweep
	Join   CircularSweepInsets
}

type circularSweepInsetsEnvelope struct {
	Format    string
	Version   int
	Host      ExactCircularSweep
	Guests    []ExactCircularSweep
	HostFaces [][]int
	Members   []CircularSweepInsetGuest
	Native    json.RawMessage
}

func buildCircularSweepInsetsRecord(ctx context.Context, host ExactCircularSweep, guests []ExactCircularSweep) (CircularSweepInsetsRecord, []byte, error) {
	var zero CircularSweepInsetsRecord
	joined, err := JoinExactCircularSweepInsets(ctx, host, guests, ExactCircularLoopOptions{})
	if err != nil {
		return zero, nil, err
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, joined.Solid); err != nil {
		return zero, nil, err
	}
	wire := circularSweepInsetsEnvelope{Format: circularSweepInsetsFormat, Version: 1, Host: host,
		Guests: guests, HostFaces: joined.HostFaces, Members: joined.Guests, Native: native.Bytes()}
	raw, err := json.Marshal(wire)
	if err != nil {
		return zero, nil, err
	}
	if len(raw) >= circularSweepInsetsJSONBytes {
		return zero, nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return zero, nil, err
	}
	return CircularSweepInsetsRecord{Host: host, Guests: guests, Join: joined}, raw, nil
}

// WriteExactCircularSweepInsetsJSON writes a receipt containing the source
// sweeps and exact inset ownership, after proving the entire joined shell.
func WriteExactCircularSweepInsetsJSON(ctx context.Context, w io.Writer, host ExactCircularSweep, guests []ExactCircularSweep) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	_, raw, err := buildCircularSweepInsetsRecord(ctx, host, guests)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

// ReadExactCircularSweepInsetsJSON reconstructs the join from its sources.
// Stored ownership, model and chart claims must match the new proof exactly.
func ReadExactCircularSweepInsetsJSON(ctx context.Context, r io.Reader, l CircularSweepInsetsJSONLimits) (CircularSweepInsetsRecord, error) {
	var zero CircularSweepInsetsRecord
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = circularSweepInsetsJSONBytes
	}
	if l.MaxGuests == 0 {
		l.MaxGuests = maxCircularSweepInsetGuests
	}
	if l.MaxSegments == 0 {
		l.MaxSegments = maxCircularSweepInsetSegments
	}
	if l.MaxBytes < 1 || l.MaxBytes > circularSweepInsetsJSONBytes ||
		l.MaxGuests < 1 || l.MaxGuests > maxCircularSweepInsetGuests ||
		l.MaxSegments < 6 || l.MaxSegments > maxCircularSweepInsetSegments {
		return zero, geom.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var claim struct {
		Format  string
		Version int
		Host    ExactCircularSweep
		Guests  []ExactCircularSweep
	}
	if err := json.Unmarshal(raw, &claim); err != nil || claim.Format != circularSweepInsetsFormat || claim.Version != 1 || len(claim.Guests) == 0 {
		return zero, geom.ErrInvalid
	}
	if len(claim.Guests) > l.MaxGuests {
		return zero, geom.ErrConvergence
	}
	if err := checkCircularSweepInsetSegments(claim.Host, claim.Guests, l.MaxSegments); err != nil {
		return zero, err
	}
	p, expected, err := buildCircularSweepInsetsRecord(ctx, claim.Host, claim.Guests)
	if err != nil {
		return zero, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil || !bytes.Equal(compact.Bytes(), expected) {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return p, nil
}
