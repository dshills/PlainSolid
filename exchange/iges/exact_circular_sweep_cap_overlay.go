package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/circularsweep"
	"plainsolid/geom"
)

// ExactCircularSweepCapOverlay describes two original sweeps with one
// matching complete cap. Unit is metres per IGES length unit.
type ExactCircularSweepCapOverlay struct {
	Sweeps [2]trimmed.ExactCircularSweep
	Unit   float64
}

// ExactCircularSweepCapOverlayIGES authenticates the analytic IGES graph
// against the original sweeps. Faces retains each original face's output
// IGES fragments, with an empty slice for either consumed cap.
type ExactCircularSweepCapOverlayIGES struct {
	Native                           trimmed.CircularSweepCapOverlay
	SourceSolid, SourceShell         int
	SourceVertexList, SourceEdgeList int
	Faces                            [2][][]int
}

func buildCircularSweepCapOverlayIGES(ctx context.Context, d ExactCircularSweepCapOverlay) ([]byte, directedIGESRoles, trimmed.CircularSweepCapOverlay, error) {
	var roles directedIGESRoles
	var zero trimmed.CircularSweepCapOverlay
	p, err := circularsweep.BuildCapOverlay(ctx, d.Sweeps[0], d.Sweeps[1])
	if err != nil {
		return nil, roles, zero, err
	}
	supports := make([]directedIGESSupport, len(p.Supports))
	for i, s := range p.Supports {
		supports[i] = directedIGESSupport{z: s.Z, wall: s.Wall, segment: s.Segment}
	}
	a, roles, err := buildDirectedIGESBoundary(ctx, p.Native.Solid, supports, p.Rims)
	if err != nil {
		return nil, roles, zero, err
	}
	raw, err := a.file(d.Unit)
	if err != nil {
		return nil, roles, zero, err
	}
	return raw, roles, p.Native, nil
}

// WriteExactCircularSweepCapOverlay emits one analytic IGES type-186 solid
// with the common cap subdivision. The native overlay receipt separately
// keeps both unrefined source profiles and their parameter maps.
func WriteExactCircularSweepCapOverlay(ctx context.Context, w io.Writer, d ExactCircularSweepCapOverlay) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	raw, _, _, err := buildCircularSweepCapOverlayIGES(ctx, d)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

// ReadExactCircularSweepCapOverlay reads the canonical analytic dialect and
// authenticates it against two supplied original sweeps. A refined IGES rim
// alone does not uniquely identify the two original partitions.
func ReadExactCircularSweepCapOverlay(ctx context.Context, r io.Reader, d ExactCircularSweepCapOverlay, l Limits) (ExactCircularSweepCapOverlayIGES, error) {
	var zero ExactCircularSweepCapOverlayIGES
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxRecords = min(l.MaxRecords, 8192)
	l.MaxEntities = min(l.MaxEntities, 1400)
	l.MaxValues = min(l.MaxValues, 32768)
	doc, err := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if err != nil {
		return zero, err
	}
	check, ids, p, err := buildCircularSweepCapOverlayIGES(ctx, d)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(bytes.ReplaceAll(doc.bytes, []byte("\r\n"), []byte("\n")), check) {
		return zero, fmt.Errorf("iges: circular cap overlay source: %w", geom.ErrDomain)
	}
	out := ExactCircularSweepCapOverlayIGES{
		Native: p, SourceSolid: ids.solid, SourceShell: ids.shell, SourceVertexList: ids.vertices, SourceEdgeList: ids.edges,
	}
	for i, source := range p.Faces {
		out.Faces[i] = make([][]int, len(source))
		for j, split := range source {
			for _, k := range split {
				out.Faces[i][j] = append(out.Faces[i][j], ids.faceIDs[k])
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
