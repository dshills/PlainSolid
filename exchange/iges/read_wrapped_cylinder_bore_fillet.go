package iges

import (
	"context"
	"errors"
	"io"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/geom"
)

// WrappedCylinderBoreRimFilletIGES retains source directory and list indices
// for the exact native material. Edge and vertex entries use one-based source
// list indices. A vertex index of zero identifies an exact added cardinal
// point on an original full circle.
type WrappedCylinderBoreRimFilletIGES struct {
	Solid                                              trimmed.Solid
	LengthUnit                                         float64
	SourceGroupID, SourceSolidID, SourceShellID        int
	SourceEdgeListID, SourceVertexListID               int
	NativeFaceSourceIDs, NativeCurveSourceIDs          []int
	NativeEdgeSourceIndices, NativeVertexSourceIndices []int
	source                                             []byte
}

// SourceIGES returns an independent copy of the original proved IGES bytes.
func (m WrappedCylinderBoreRimFilletIGES) SourceIGES() []byte {
	return append([]byte(nil), m.source...)
}

var errIGESBoreRim = errors.New("iges: unsupported exact model-only circular bore rim fillet")

type igesBoreRimUse struct {
	edge    int
	forward bool
}

type igesBoreRimFace struct {
	id           int
	kind         borefillet.SupportKind
	center, axis geom.Vec3
	major, minor float64
	forward      bool
	loops        [][]igesBoreRimUse
}

type igesBoreRimSource struct {
	records         map[int]topologyRecord
	claimed         map[int]bool
	edges, vertices int
}

// ReadWrappedCylinderBoreRimFillet imports one exact model-only IGES solid
// with original cap, circular stock and bore supports, and one or both inner
// rolling rims. It checks every untrimmed support, model edge, vertex and
// effective loop incidence. Complete source circles may become four exact
// native edges. The coordinates remain in the original file units.
func ReadWrappedCylinderBoreRimFillet(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (WrappedCylinderBoreRimFilletIGES, error) {
	fail := func(err error) (WrappedCylinderBoreRimFilletIGES, error) {
		return WrappedCylinderBoreRimFilletIGES{}, err
	}
	if ctx == nil || r == nil {
		return fail(geom.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	l, err := l.defaults()
	if err != nil {
		return fail(err)
	}
	l.MaxBytes, l.MaxRecords = min(l.MaxBytes, 128<<10), min(l.MaxRecords, 1024)
	l.MaxEntities, l.MaxValues = min(l.MaxEntities, 128), min(l.MaxValues, 4096)
	doc, err := readTopologyRecordPolicyWithBore(r, l, true, false, false, false, false, false, true, false, false, true)
	if err != nil {
		return fail(err)
	}
	p := igesBoreRimSource{records: make(map[int]topologyRecord, len(doc.entities)), claimed: make(map[int]bool)}
	for _, rec := range doc.entities {
		p.records[rec.id] = rec
	}
	m := WrappedCylinderBoreRimFilletIGES{LengthUnit: doc.unit, source: doc.bytes}
	for _, rec := range doc.entities {
		switch rec.kind {
		case 402:
			if m.SourceGroupID != 0 {
				return fail(errIGESBoreRim)
			}
			m.SourceGroupID = rec.id
		case 186:
			if m.SourceSolidID != 0 {
				return fail(errIGESBoreRim)
			}
			m.SourceSolidID = rec.id
		}
	}
	if m.SourceGroupID != 0 {
		group, err := p.topology(m.SourceGroupID, 402)
		if err != nil || len(group) != 3 || group[1] != 1 || group[2] != m.SourceSolidID {
			return fail(errIGESBoreRim)
		}
	}
	body, err := p.topology(m.SourceSolidID, 186)
	if err != nil || len(body) != 4 || body[2] != 1 || body[3] != 0 ||
		p.records[m.SourceSolidID].boreGroupMember != (m.SourceGroupID != 0) {
		return fail(errIGESBoreRim)
	}
	m.SourceShellID = body[1]
	shell, err := p.topology(body[1], 514)
	if err != nil || len(shell) < 2 || shell[1] < 5 || shell[1] > 6 || len(shell) != 2+2*shell[1] {
		return fail(errIGESBoreRim)
	}
	faces := make([]igesBoreRimFace, 0, shell[1])
	for i := 2; i < len(shell); i += 2 {
		face, err := p.readFace(shell[i], shell[i+1])
		if err != nil {
			return fail(err)
		}
		faces = append(faces, face)
	}
	solid, faceSources, err := buildIGESBoreRimNative(ctx, faces, o)
	if err != nil {
		return fail(err)
	}
	curves, edges, vertices, err := p.proveIncidence(ctx, solid, faceSources, faces, o)
	if err != nil {
		return fail(err)
	}
	if len(p.claimed) != len(p.records) {
		return fail(errIGESBoreRim)
	}
	m.Solid, m.NativeFaceSourceIDs = solid, faceSources
	m.SourceEdgeListID, m.SourceVertexListID = p.edges, p.vertices
	m.NativeCurveSourceIDs, m.NativeEdgeSourceIndices, m.NativeVertexSourceIndices = curves, edges, vertices
	return m, nil
}

func (p *igesBoreRimSource) entity(id, kind int, unique bool) (topologyRecord, error) {
	r, ok := p.records[id]
	if !ok || r.kind != kind || len(r.values) == 0 || unique && p.claimed[id] {
		return topologyRecord{}, errIGESBoreRim
	}
	n, err := intValue(r.values[0])
	if err != nil || n != kind {
		return topologyRecord{}, errIGESBoreRim
	}
	p.claimed[id] = true
	return r, nil
}

func (p *igesBoreRimSource) topology(id, kind int) ([]int, error) {
	r, err := p.entity(id, kind, true)
	if err != nil {
		return nil, err
	}
	a := make([]int, len(r.values))
	for i, v := range r.values {
		a[i], err = intValue(v)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (p *igesBoreRimSource) readFace(id, sense int) (igesBoreRimFace, error) {
	var f igesBoreRimFace
	rec, err := p.topology(id, 510)
	if err != nil || sense < 0 || sense > 1 || len(rec) < 4 || rec[2] < 1 || rec[2] > 2 || len(rec) != 4+rec[2] || rec[3] != 1 {
		return f, errIGESBoreRim
	}
	r, err := p.entity(rec[1], p.records[rec[1]].kind, true)
	if err != nil {
		return f, err
	}
	f.id, f.forward = id, sense == 1
	if len(r.values) < 4 {
		return f, errIGESBoreRim
	}
	last := 3
	switch r.kind {
	case 190:
		f.kind = borefillet.Plane
		if len(r.values) != 4 || rec[2] != 2 {
			return f, errIGESBoreRim
		}
	case 192:
		f.kind, last = borefillet.Cylinder, 4
		if len(r.values) != 5 || rec[2] != 1 {
			return f, errIGESBoreRim
		}
	case 198:
		f.kind, last = borefillet.Torus, 5
		if (len(r.values) != 6 && len(r.values) != 8) || rec[2] != 1 {
			return f, errIGESBoreRim
		}
		for _, x := range r.values[6:] {
			n, err := intValue(x)
			if err != nil || n != 0 {
				return f, errIGESBoreRim
			}
		}
	default:
		return f, errIGESBoreRim
	}
	f.center, err = p.vector(r.values[1], 116)
	if err != nil {
		return f, err
	}
	f.axis, err = p.vector(r.values[2], 123)
	if err != nil || f.axis.X != 0 || f.axis.Y != 0 || f.axis.Z != 1 && f.axis.Z != -1 {
		return f, errIGESBoreRim
	}
	x, err := p.vector(r.values[last], 123)
	if err != nil || x != (geom.Vec3{X: 1}) {
		return f, errIGESBoreRim
	}
	if last >= 4 {
		f.major, err = igesBoreRimScalar(r.values[3])
		if err != nil || f.major <= 0 {
			return f, errIGESBoreRim
		}
	}
	if last == 5 {
		f.minor, err = igesBoreRimScalar(r.values[4])
		if err != nil || f.minor <= 0 || f.minor >= f.major {
			return f, errIGESBoreRim
		}
	}
	for _, loop := range rec[4:] {
		a, err := p.topology(loop, 508)
		if err != nil || len(a) < 2 || a[1] < 1 || a[1] > 10 || len(a) != 2+5*a[1] {
			return f, errIGESBoreRim
		}
		wire := make([]igesBoreRimUse, a[1])
		for i := range wire {
			v := a[2+5*i:]
			if v[0] != 0 || v[1] <= 0 || p.edges != 0 && p.edges != v[1] || v[2] < 1 ||
				v[3] < 0 || v[3] > 1 || v[4] != 0 {
				return f, errIGESBoreRim
			}
			p.edges = v[1]
			j := i
			if !f.forward {
				j = len(wire) - 1 - i
			}
			wire[j] = igesBoreRimUse{edge: v[2], forward: (v[3] == 1) == f.forward}
		}
		f.loops = append(f.loops, wire)
	}
	return f, nil
}

func (p *igesBoreRimSource) vector(v parameter, kind int) (geom.Vec3, error) {
	id, err := intValue(v)
	if err != nil {
		return geom.Vec3{}, err
	}
	r, err := p.entity(id, kind, false)
	if err != nil {
		return geom.Vec3{}, err
	}
	if kind == 116 {
		if len(r.values) != 5 {
			return geom.Vec3{}, errIGESBoreRim
		}
		display, err := intValue(r.values[4])
		if err != nil || display != 0 {
			return geom.Vec3{}, errIGESBoreRim
		}
	} else if len(r.values) != 4 {
		return geom.Vec3{}, errIGESBoreRim
	}
	q, err := igesPlanarCoordinates(r.values[1:4])
	if err != nil {
		return geom.Vec3{}, err
	}
	return q.represented()
}

func igesBoreRimScalar(v parameter) (float64, error) {
	q, err := igesPlanarExact(v)
	if err != nil {
		return 0, err
	}
	return geom.RepresentedExactScalar(q)
}

func buildIGESBoreRimNative(ctx context.Context, faces []igesBoreRimFace, o geom.SurfacePatchOptions) (trimmed.Solid, []int, error) {
	fail := func(err error) (trimmed.Solid, []int, error) { return trimmed.Solid{}, nil, err }
	var caps, cylinders, blends [2]*igesBoreRimFace
	for i := range faces {
		f := &faces[i]
		switch f.kind {
		case borefillet.Plane:
			end := 0
			if f.forward == (f.axis.Z > 0) {
				end = 1
			}
			if caps[end] != nil {
				return fail(errIGESBoreRim)
			}
			caps[end] = f
		case borefillet.Cylinder:
			side := 0
			if !f.forward {
				side = 1
			}
			if cylinders[side] != nil {
				return fail(errIGESBoreRim)
			}
			cylinders[side] = f
		}
	}
	if caps[0] == nil || caps[1] == nil || cylinders[0] == nil || cylinders[1] == nil || caps[0].center.Z >= caps[1].center.Z {
		return fail(errIGESBoreRim)
	}
	hole := cylinders[1]
	var radii [2]float64
	for i := range faces {
		f := &faces[i]
		if f.kind != borefillet.Torus {
			continue
		}
		if !f.forward || f.center.X != hole.center.X || f.center.Y != hole.center.Y ||
			new(big.Rat).Add(igesBoreRat(hole.major), igesBoreRat(f.minor)).Cmp(igesBoreRat(f.major)) != 0 {
			return fail(errIGESBoreRim)
		}
		end := -1
		for j, cap := range caps {
			if igesBoreRimShare(f.loops[0], cap.loops[1]) {
				if end >= 0 {
					return fail(errIGESBoreRim)
				}
				end = j
			}
		}
		if end < 0 || blends[end] != nil {
			return fail(errIGESBoreRim)
		}
		d := new(big.Rat).Sub(igesBoreRat(f.center.Z), igesBoreRat(caps[end].center.Z))
		if end == 1 {
			d.Neg(d)
		}
		if d.Cmp(igesBoreRat(f.minor)) != 0 {
			return fail(errIGESBoreRim)
		}
		blends[end], radii[end] = f, f.minor
	}
	var walls [2]*geom.WrappedSurfacePatch
	for i, f := range cylinders {
		w, err := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{X: f.center.X, Y: f.center.Y}, f.major, 1, o)
		if err != nil {
			return fail(err)
		}
		walls[i], err = geom.ReheightWrappedCylinder(ctx, w, igesBoreRat(caps[0].center.Z), igesBoreRat(caps[1].center.Z), o)
		if err != nil {
			return fail(err)
		}
	}
	stock, _, err := trimmed.NewWrappedEccentricCircularFrustumBore(ctx, walls[0], walls[1], 1e-9, o)
	if err != nil {
		return fail(err)
	}
	solid, history, err := trimmed.FilletWrappedCylinderBoreRims(ctx, stock, radii[0], radii[1], o)
	if err != nil {
		return fail(err)
	}
	sources := make([]int, len(faces))
	sources[history.Outer], sources[history.Bore] = cylinders[0].id, cylinders[1].id
	for end, cap := range caps {
		sources[history.Caps[end]] = cap.id
		if blends[end] != nil {
			sources[history.Blends[end]] = blends[end].id
		}
	}
	return solid, sources, nil
}

func igesBoreRimShare(a, b []igesBoreRimUse) bool {
	for _, x := range a {
		for _, y := range b {
			if x.edge == y.edge {
				return true
			}
		}
	}
	return false
}

func igesBoreRat(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
