package iges

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ExactDirectedCircularLoopIGES retains all original topology and curve
// directory pointers for the canonical type-186 directed extrusion. RingOffsets
// separate the exterior and holes in the ordered rim, vertical and wall slices.
type ExactDirectedCircularLoopIGES struct {
	Descriptor                        ExactDirectedCircularLoopExtrusion
	Native                            trimmed.Solid
	SourceSolid, SourceShell          int
	SourceVertices, SourceEdges       int
	SourceCaps                        [2]int
	SourceCapLoops, SourceRimCurves   [2][]int
	SourceWalls, SourceVerticalCurves []int
	RingOffsets                       []int
}

// ReadExactDirectedCircularLoopExtrusion re-proves the exact canonical
// type-186 graph. Its scope is the PlainSolid model-only writer dialect;
// foreign charts, reordering and additional metadata are rejected.
func ReadExactDirectedCircularLoopExtrusion(ctx context.Context, r io.Reader, l Limits) (ExactDirectedCircularLoopIGES, error) {
	var zero ExactDirectedCircularLoopIGES
	fail := func() (ExactDirectedCircularLoopIGES, error) {
		return zero, fmt.Errorf("iges: unsupported exact directed extrusion: %w", geom.ErrDomain)
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
	l.MaxEntities = min(l.MaxEntities, 640)
	l.MaxValues = min(l.MaxValues, 16384)
	doc, err := readPlacedTopologyRecords(r, l, true, false, false, false, false, false, false, true)
	if err != nil {
		return zero, err
	}
	records := doc.entities
	if len(records) < 2 || records[0].kind != 502 || len(records[0].values) < 2 {
		return fail()
	}
	vertices := records[0].values
	vc, err := intValue(vertices[1])
	if err != nil || vc < 6 || vc > 128 || vc%2 != 0 || len(vertices) != 2+3*vc {
		return fail()
	}
	n := vc / 2
	if len(records) < 3*n+2 || records[1+3*n].kind != 504 {
		return fail()
	}
	xyz := func(index int) (geom.Vec3, error) {
		q := vertices[2+3*index : 5+3*index]
		if e := compositeSourceNumbers(110, q); e != nil {
			return geom.Vec3{}, e
		}
		x, _ := realValue(q[0])
		y, _ := realValue(q[1])
		z, _ := realValue(q[2])
		return geom.Vec3{X: x, Y: y, Z: z}, nil
	}
	points := make([]geom.Vec3, vc)
	for i := range points {
		points[i], err = xyz(i)
		if err != nil {
			return zero, err
		}
	}
	lookup := func(id parameter, kind int) (topologyRecord, bool) {
		x, e := intValue(id)
		if e != nil || x < 1 || x%2 != 1 || x/2 >= len(records) {
			return topologyRecord{}, false
		}
		a := records[x/2]
		return a, a.kind == kind
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
	if len(faces) != n+2 {
		return fail()
	}
	cap := faces[0].values
	loops, err := intValue(cap[2])
	if err != nil || loops < 1 || loops > 21 || len(cap) != 4+loops {
		return fail()
	}
	d := ExactDirectedCircularLoopExtrusion{Bottom: points[0].Z, Top: points[n].Z, Unit: doc.unit}
	offset := 0
	for _, id := range cap[4:] {
		a, ok := lookup(id, 508)
		if !ok || len(a.values) < 2 {
			return fail()
		}
		count, e := intValue(a.values[1])
		if e != nil || count < 3 || count > n-offset {
			return fail()
		}
		ring := make([]trimmed.ExactCircularLoopSegment, count)
		for j := range ring {
			i, next := offset+j, offset+(j+1)%count
			p, q := points[i], points[next]
			s := trimmed.ExactCircularLoopSegment{Start: geom.Vec2{X: p.X, Y: p.Y}, End: geom.Vec2{X: q.X, Y: q.Y}}
			curve := records[1+i]
			switch {
			case curve.kind == 100 && len(curve.values) == 8:
				wall, ok := lookup(faces[i+2].values[1], 192)
				if !ok || len(wall.values) != 5 {
					return fail()
				}
				if err = compositeSourceNumbers(100, curve.values); err != nil {
					return zero, err
				}
				if err = compositeSourceNumbers(110, wall.values[3:4]); err != nil {
					return zero, err
				}
				s.Center.X, _ = realValue(curve.values[2])
				s.Center.Y, _ = realValue(curve.values[3])
				s.Radius, _ = realValue(wall.values[3])
				if s.Radius <= 0 {
					return fail()
				}
			case curve.kind == 110 && len(curve.values) == 7:
			default:
				return fail()
			}
			ring[j] = s
		}
		if offset == 0 {
			d.Outer = ring
		} else {
			d.Holes = append(d.Holes, ring)
		}
		offset += count
	}
	if offset != n {
		return fail()
	}
	a, ids, native, err := buildDirectedIGES(ctx, d)
	if err != nil {
		return zero, err
	}
	b, err := a.file(doc.unit)
	if err != nil {
		return zero, err
	}
	if !bytes.Equal(bytes.ReplaceAll(doc.bytes, []byte("\r\n"), []byte("\n")), b) {
		return fail()
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return ExactDirectedCircularLoopIGES{
		Descriptor: d, Native: native, SourceSolid: ids.solid, SourceShell: ids.shell,
		SourceVertices: ids.vertices, SourceEdges: ids.edges, SourceCaps: ids.caps, SourceCapLoops: ids.capLoops,
		SourceRimCurves: ids.rims, SourceWalls: ids.walls, SourceVerticalCurves: ids.verticals, RingOffsets: ids.ringOffsets,
	}, nil
}
