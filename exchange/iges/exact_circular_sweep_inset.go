package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ExactCircularSweepInset describes a host and strictly inset guest abutting
// at a common cap. Unit is metres per IGES length unit.
type ExactCircularSweepInset struct {
	Host, Guest trimmed.ExactCircularSweep
	Unit        float64
}

// ExactCircularSweepInsetIGES preserves the original host and guest face
// roles. Zero denotes the consumed guest cap; exposed floors are extra
// fragments of the original host cap.
type ExactCircularSweepInsetIGES struct {
	Descriptor                       ExactCircularSweepInset
	Native                           trimmed.CircularSweepInset
	SourceSolid, SourceShell         int
	SourceVertexList, SourceEdgeList int
	HostFaces                        [][]int
	GuestFaces                       []int
}

func buildCircularSweepInsetIGES(ctx context.Context, d ExactCircularSweepInset) (topologyAuthor, directedIGESRoles, trimmed.CircularSweepInset, error) {
	var no directedIGESRoles
	var empty trimmed.CircularSweepInset
	p, err := trimmed.JoinExactCircularSweepInset(ctx, d.Host, d.Guest, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return topologyAuthor{}, no, empty, err
	}
	supports := make([]directedIGESSupport, len(p.Solid.Faces()))
	circles := make([]trimmed.ExactCircularLoopSegment, len(p.Solid.Edges()))
	for i, src := range []trimmed.ExactCircularSweep{d.Host, d.Guest} {
		segments := append([]trimmed.ExactCircularLoopSegment(nil), src.Outer...)
		for _, h := range src.Holes {
			segments = append(segments, h...)
		}
		n := len(segments)
		offset := 0
		if i == 1 {
			offset = p.GuestEdgeOffset
		}
		for j, a := range segments {
			circles[offset+j], circles[offset+n+j] = a, a
		}
		for j := 0; j < n+2; j++ {
			s := directedIGESSupport{z: src.Bottom}
			switch {
			case j == 1:
				s.z = src.Top
			case j > 1:
				s.wall, s.segment = true, segments[j-2]
			}
			if i == 0 {
				for _, id := range p.HostFaces[j] {
					supports[id] = s
				}
			} else if id := p.GuestFaces[j]; id >= 0 {
				supports[id] = s
			}
		}
	}
	a, roles, err := buildDirectedIGESBoundary(ctx, p.Solid, supports, circles)
	return a, roles, p, err
}

// WriteExactCircularSweepInset emits the proved union as one analytic IGES
// type-186 solid with separate original caps, walls, and exposed floor wires.
func WriteExactCircularSweepInset(ctx context.Context, w io.Writer, d ExactCircularSweepInset) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	a, _, _, err := buildCircularSweepInsetIGES(ctx, d)
	if err != nil {
		return err
	}
	raw, err := a.file(d.Unit)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		return io.ErrShortWrite
	}
	return err
}

func directedInsetIGESRecord(records []topologyRecord, v parameter, kind int) (topologyRecord, bool) {
	id, err := intValue(v)
	if err != nil || id < 1 || id%2 != 1 || id/2 >= len(records) {
		return topologyRecord{}, false
	}
	r := records[id/2]
	return r, r.kind == kind
}

func directedInsetIGESCap(records []topologyRecord, f topologyRecord) (float64, bool) {
	p, ok := directedInsetIGESRecord(records, f.values[1], 190)
	if !ok || len(p.values) != 4 {
		return 0, false
	}
	n, ok := directedInsetIGESRecord(records, p.values[2], 123)
	if !ok || len(n.values) != 4 {
		return 0, false
	}
	d, err := igesPlanarCoordinates(n.values[1:])
	if err != nil || d[0].Sign() != 0 || d[1].Sign() != 0 || d[2].Cmp(compositeRat(1)) != 0 {
		return 0, false
	}
	origin, ok := directedInsetIGESRecord(records, p.values[1], 116)
	if !ok || len(origin.values) != 5 {
		return 0, false
	}
	q, err := igesPlanarCoordinates(origin.values[1:4])
	if err != nil || q[0].Sign() != 0 || q[1].Sign() != 0 {
		return 0, false
	}
	z, err := geom.RepresentedExactScalar(q[2])
	return z, err == nil
}

func directedInsetIGESRingCounts(records []topologyRecord, f topologyRecord) ([]int, int, error) {
	n, err := intValue(f.values[2])
	if err != nil || n < 1 || n > 21 || len(f.values) != 4+n {
		return nil, 0, geom.ErrDomain
	}
	var counts []int
	total := 0
	for _, id := range f.values[4:] {
		a, ok := directedInsetIGESRecord(records, id, 508)
		if !ok || len(a.values) < 2 {
			return nil, 0, geom.ErrDomain
		}
		k, err := intValue(a.values[1])
		if err != nil || k < 3 || k > 64-total {
			return nil, 0, geom.ErrDomain
		}
		counts = append(counts, k)
		total += k
	}
	return counts, total, nil
}

func directedInsetIGESProfiles(records []topologyRecord, vertices []geom.Vec3, counts []int, faceList []topologyRecord, vertexOffset, edgeOffset int) ([][]trimmed.ExactCircularLoopSegment, error) {
	out := make([][]trimmed.ExactCircularLoopSegment, len(counts))
	at := 0
	for h, n := range counts {
		for k := 0; k < n; k++ {
			i, j := at+k, at+(k+1)%n
			p, q := vertices[vertexOffset+i], vertices[vertexOffset+j]
			s := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: q.X, Y: q.Y}}
			c := records[1+edgeOffset+i]
			switch {
			case c.kind == 100 && len(c.values) == 8:
				w, ok := directedInsetIGESRecord(records, faceList[i].values[1], 192)
				if !ok || len(w.values) != 5 {
					return nil, geom.ErrDomain
				}
				if err := compositeSourceNumbers(100, c.values); err != nil {
					return nil, err
				}
				if err := compositeSourceNumbers(110, w.values[3:4]); err != nil {
					return nil, err
				}
				s.Center.X, _ = realValue(c.values[2])
				s.Center.Y, _ = realValue(c.values[3])
				s.Radius, _ = realValue(w.values[3])
				if s.Radius <= 0 {
					return nil, geom.ErrDomain
				}
			case c.kind == 110 && len(c.values) == 7:
			default:
				return nil, geom.ErrDomain
			}
			out[h] = append(out[h], s)
		}
		at += n
	}
	return out, nil
}

// ReadExactCircularSweepInset recovers both original sweeps from their
// unconsumed cap wires, then compares the entire re-proved IGES graph. It
// reads the canonical writer dialect, including reversed arcs and independent
// host floor wires.
func ReadExactCircularSweepInset(ctx context.Context, r io.Reader, l Limits) (ExactCircularSweepInsetIGES, error) {
	var zero ExactCircularSweepInsetIGES
	fail := func() (ExactCircularSweepInsetIGES, error) {
		return zero, fmt.Errorf("iges: unsupported exact circular inset source: %w", geom.ErrDomain)
	}
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
	records := doc.entities
	if len(records) < 2 || records[0].kind != 502 || len(records[0].values) < 2 {
		return fail()
	}
	var faces []topologyRecord
	for _, a := range records {
		if a.kind == 510 {
			if len(a.values) < 5 {
				return fail()
			}
			faces = append(faces, a)
		}
	}
	if len(faces) < 9 || len(faces) > 151 {
		return fail()
	}
	bottom, ok := directedInsetIGESCap(records, faces[0])
	if !ok {
		return fail()
	}
	top, ok := directedInsetIGESCap(records, faces[1])
	if !ok || top <= bottom {
		return fail()
	}
	next := 0
	var far float64
	for i := 2; i < len(faces); i++ {
		var yes bool
		if far, yes = directedInsetIGESCap(records, faces[i]); yes {
			next = i
			break
		}
	}
	if next < 5 || next > 66 || far >= bottom && far <= top {
		return fail()
	}
	hostFar, guestBottom, guestTop := 0, top, far
	if far < bottom {
		hostFar, guestBottom, guestTop = 1, far, bottom
	}
	hostCounts, hn, err := directedInsetIGESRingCounts(records, faces[hostFar])
	if err != nil || hn+2 != next {
		return fail()
	}
	guestCounts, gn, err := directedInsetIGESRingCounts(records, faces[next])
	if err != nil || len(faces) != next+gn+len(guestCounts) {
		return fail()
	}

	v := records[0].values
	count, err := intValue(v[1])
	total := hn + gn
	if err != nil || count != 2*total || len(v) != 2+3*count ||
		len(records) < 3*total+2 || records[3*total+1].kind != 504 {
		return fail()
	}
	points := make([]geom.Vec3, count)
	for i := range points {
		p, err := igesPlanarCoordinates(v[2+3*i : 5+3*i])
		if err != nil {
			return zero, err
		}
		points[i], err = p.represented()
		if err != nil {
			return zero, err
		}
	}
	h, err := directedInsetIGESProfiles(records, points, hostCounts, faces[2:next], 0, 0)
	if err != nil {
		return zero, err
	}
	g, err := directedInsetIGESProfiles(records, points, guestCounts, faces[next+1:next+1+gn], 2*hn, 3*hn)
	if err != nil {
		return zero, err
	}
	d := ExactCircularSweepInset{
		Host:  trimmed.ExactCircularSweep{Outer: h[0], Holes: h[1:], Bottom: bottom, Top: top},
		Guest: trimmed.ExactCircularSweep{Outer: g[0], Holes: g[1:], Bottom: guestBottom, Top: guestTop},
		Unit:  doc.unit,
	}
	a, ids, j, err := buildCircularSweepInsetIGES(ctx, d)
	if err != nil {
		return zero, err
	}
	check, err := a.file(doc.unit)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(bytes.ReplaceAll(doc.bytes, []byte("\r\n"), []byte("\n")), check) {
		return fail()
	}
	p := ExactCircularSweepInsetIGES{
		Descriptor: d, Native: j, SourceSolid: ids.solid, SourceShell: ids.shell, SourceVertexList: ids.vertices, SourceEdgeList: ids.edges,
		HostFaces: make([][]int, len(j.HostFaces)), GuestFaces: make([]int, len(j.GuestFaces)),
	}
	for i, f := range j.HostFaces {
		for _, k := range f {
			p.HostFaces[i] = append(p.HostFaces[i], ids.faceIDs[k])
		}
	}
	for i, k := range j.GuestFaces {
		if k >= 0 {
			p.GuestFaces[i] = ids.faceIDs[k]
		}
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return p, nil
}
