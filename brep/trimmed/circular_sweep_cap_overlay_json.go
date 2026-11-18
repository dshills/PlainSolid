//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/geom"
)

const circularSweepOverlayFormat = "plainsolid-exact-circular-sweep-cap-overlay"
const circularSweepOverlayBytes = 8 << 20

// CircularSweepOverlayJSONLimits bounds total receipt bytes and original
// segments in each member. A zero limit selects the library default.
type CircularSweepOverlayJSONLimits struct {
	MaxBytes    int64
	MaxSegments int
}

// CircularSweepOverlayRecord contains both original authored sweeps and the
// exact re-proved face, vertex, and edge maps.
type CircularSweepOverlayRecord struct {
	Sources [2]ExactCircularSweep
	Join    CircularSweepCapOverlay
}

type circularSweepOverlayEnvelope struct {
	Format         string
	Version        int
	Sources        [2]ExactCircularSweep
	Caps           [2]int
	Faces          [2][][]int
	Edges          [2][][]CircularSweepEdgeFragment
	Vertices       [2][]int
	SharedRimEdges []int
	Native         json.RawMessage
}

func buildCircularSweepOverlayRecord(ctx context.Context, sources [2]ExactCircularSweep) (CircularSweepOverlayRecord, []byte, error) {
	var zero CircularSweepOverlayRecord
	p, err := JoinExactCircularSweepCapOverlay(ctx, sources[0], sources[1], ExactCircularLoopOptions{})
	if err != nil {
		return zero, nil, err
	}
	var native bytes.Buffer
	if err = WriteJSON(&native, p.Solid); err != nil {
		return zero, nil, err
	}
	claim := circularSweepOverlayEnvelope{
		Format: circularSweepOverlayFormat, Version: 1, Sources: sources, Caps: p.Caps,
		Faces: p.Faces, Edges: p.Edges, Vertices: p.Vertices, SharedRimEdges: p.SharedRimEdges, Native: native.Bytes(),
	}
	b, err := json.Marshal(claim)
	if err != nil {
		return zero, nil, err
	}
	if len(b) >= circularSweepOverlayBytes {
		return zero, nil, geom.ErrConvergence
	}
	if err = ctx.Err(); err != nil {
		return zero, nil, err
	}
	return CircularSweepOverlayRecord{Sources: sources, Join: p}, b, nil
}

// WriteExactCircularSweepCapOverlayJSON re-proves both original members,
// stores their exact parameter maps and native graph, and writes canonical
// bytes only after source and topology certification.
func WriteExactCircularSweepCapOverlayJSON(ctx context.Context, w io.Writer, a, b ExactCircularSweep) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	_, claim, err := buildCircularSweepOverlayRecord(ctx, [2]ExactCircularSweep{a, b})
	if err != nil {
		return err
	}
	claim = append(claim, '\n')
	n, err := w.Write(claim)
	if err == nil && n != len(claim) {
		return io.ErrShortWrite
	}
	return err
}

// ReadExactCircularSweepCapOverlayJSON rebuilds only from the original
// authored profiles. Every stored edge map and native entity is compared to
// the re-proved result; insignificant JSON whitespace is accepted.
func ReadExactCircularSweepCapOverlayJSON(ctx context.Context, r io.Reader, l CircularSweepOverlayJSONLimits) (CircularSweepOverlayRecord, error) {
	var zero CircularSweepOverlayRecord
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = circularSweepOverlayBytes
	}
	if l.MaxSegments == 0 {
		l.MaxSegments = 64
	}
	if l.MaxBytes < 1 || l.MaxBytes > circularSweepOverlayBytes || l.MaxSegments < 3 || l.MaxSegments > 64 {
		return zero, geom.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	var header struct {
		Format  string
		Version int
		Sources [2]ExactCircularSweep
	}
	if err = json.Unmarshal(raw, &header); err != nil || header.Format != circularSweepOverlayFormat || header.Version != 1 {
		return zero, geom.ErrInvalid
	}
	for _, src := range header.Sources {
		n := len(src.Outer)
		if len(src.Holes) > 20 {
			return zero, geom.ErrConvergence
		}
		for _, h := range src.Holes {
			n += len(h)
		}
		if n > l.MaxSegments {
			return zero, geom.ErrConvergence
		}
	}
	p, want, err := buildCircularSweepOverlayRecord(ctx, header.Sources)
	if err != nil {
		return zero, err
	}
	var compact bytes.Buffer
	if err = json.Compact(&compact, raw); err != nil || !bytes.Equal(compact.Bytes(), want) {
		return zero, geom.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return p, nil
}
