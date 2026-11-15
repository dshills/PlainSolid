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

// ExactCircularSweepInsets describes independent guests inset on one or
// both host caps. Unit is metres per IGES length unit.
type ExactCircularSweepInsets struct {
	Host   trimmed.ExactCircularSweep
	Guests []trimmed.ExactCircularSweep
	Unit   float64
}

// ExactCircularSweepInsetsIGES gives the re-proved join and every original
// face's IGES IDs. Zero denotes each consumed guest cap.
type ExactCircularSweepInsetsIGES struct {
	Native                           trimmed.CircularSweepInsets
	SourceSolid, SourceShell         int
	SourceVertexList, SourceEdgeList int
	HostFaces                        [][]int
	GuestFaces                       [][]int
}

func buildCircularSweepInsetsIGES(ctx context.Context, d ExactCircularSweepInsets) ([]byte, directedIGESRoles, trimmed.CircularSweepInsets, error) {
	var roles directedIGESRoles
	var zero trimmed.CircularSweepInsets
	p, err := circularsweep.BuildInsets(ctx, d.Host, d.Guests)
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

// WriteExactCircularSweepInsets writes one analytic type-186 solid with
// independent floors on the original host cap supports.
func WriteExactCircularSweepInsets(ctx context.Context, w io.Writer, d ExactCircularSweepInsets) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	raw, _, _, err := buildCircularSweepInsetsIGES(ctx, d)
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

// ReadExactCircularSweepInsets compares the entire canonical IGES graph with
// the independently re-proved boundary from the supplied original sweeps.
func ReadExactCircularSweepInsets(ctx context.Context, r io.Reader, d ExactCircularSweepInsets, l Limits) (ExactCircularSweepInsetsIGES, error) {
	var zero ExactCircularSweepInsetsIGES
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
	l.MaxEntities = min(l.MaxEntities, 3500)
	l.MaxValues = min(l.MaxValues, 65536)
	doc, err := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if err != nil {
		return zero, err
	}
	check, ids, p, err := buildCircularSweepInsetsIGES(ctx, d)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(bytes.ReplaceAll(doc.bytes, []byte("\r\n"), []byte("\n")), check) {
		return zero, fmt.Errorf("iges: circular multi-inset source: %w", geom.ErrDomain)
	}
	out := ExactCircularSweepInsetsIGES{
		Native: p, SourceSolid: ids.solid, SourceShell: ids.shell, SourceVertexList: ids.vertices, SourceEdgeList: ids.edges,
		HostFaces: make([][]int, len(p.HostFaces)), GuestFaces: make([][]int, len(p.Guests)),
	}
	for i, source := range p.HostFaces {
		for _, j := range source {
			out.HostFaces[i] = append(out.HostFaces[i], ids.faceIDs[j])
		}
	}
	for i, g := range p.Guests {
		out.GuestFaces[i] = make([]int, len(g.Faces))
		for j, k := range g.Faces {
			if k >= 0 {
				out.GuestFaces[i][j] = ids.faceIDs[k]
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
