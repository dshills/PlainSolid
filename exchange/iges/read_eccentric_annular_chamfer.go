package iges

import (
	"cmp"
	"context"
	"errors"
	"io"
	"math/big"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/annularchamfer"
	"plainsolid/geom"
)

// EccentricAnnularChamferIGES retains the original type 186 body and its
// source identities. A complete original rim owns four native quadrant
// edges. Added cardinal vertices have original list index zero.
type EccentricAnnularChamferIGES struct {
	Solid                                              trimmed.Solid
	LengthUnit                                         float64
	SourceGroupID, SourceSolidID, SourceShellID        int
	SourceEdgeListID, SourceVertexListID               int
	NativeFaceSourceIDs, NativeCurveSourceIDs          []int
	NativeEdgeSourceIndices, NativeVertexSourceIndices []int
	source                                             []byte
}

// SourceIGES returns the proved original IGES bytes.
func (m EccentricAnnularChamferIGES) SourceIGES() []byte {
	return slices.Clone(m.source)
}

// ReadEccentricAnnularChamfer strictly imports one complete four-rim
// chamfered eccentric tapered annulus in a model-only IGES graph. Its
// bounded type 128 wall charts must certify two signed meridian chains
// with exact circular sections, full model rims and paired original
// seam uses. The native solid remains in original file units.
func ReadEccentricAnnularChamfer(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (EccentricAnnularChamferIGES, error) {
	var out EccentricAnnularChamferIGES
	if ctx == nil || r == nil {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	l, err := l.defaults()
	if err != nil {
		return out, err
	}
	l.MaxBytes, l.MaxRecords = min(l.MaxBytes, 128<<10), min(l.MaxRecords, 1024)
	l.MaxEntities, l.MaxValues = min(l.MaxEntities, 128), min(l.MaxValues, 4096)
	if l.MaxControlPoints < 6*18 {
		return out, geom.ErrConvergence
	}
	doc, err := readTopologyRecordPolicyWithDirectory(r, l, true, false, false, false, false, false, true, false, false, igesAnnularChamferDirectory)
	if err != nil {
		return out, err
	}
	p := igesBoreRimSource{records: make(map[int]topologyRecord, len(doc.entities)), claimed: make(map[int]bool)}
	var group, root int
	for _, rec := range doc.entities {
		p.records[rec.id] = rec
		switch rec.kind {
		case 402:
			if group != 0 {
				return out, errIGESAnnularChamfer
			}
			group = rec.id
		case 186:
			if root != 0 {
				return out, errIGESAnnularChamfer
			}
			root = rec.id
		}
	}
	if group != 0 {
		rec, err := p.topology(group, 402)
		if err != nil || len(rec) != 3 || rec[1] != 1 || rec[2] != root {
			return out, errIGESAnnularChamfer
		}
	}
	body, err := p.topology(root, 186)
	if err != nil || len(body) != 4 || body[2] != 1 || body[3] != 0 || p.records[root].boreGroupMember != (group != 0) {
		return out, errIGESAnnularChamfer
	}
	shell, err := p.topology(body[1], 514)
	if err != nil || len(shell) != 18 || shell[1] != 8 {
		return out, errIGESAnnularChamfer
	}
	faces := make([]igesAnnularChamferFace, 0, 8)
	wires := make([]igesBoreRimFace, 0, 8)
	for i := 2; i < len(shell); i += 2 {
		face, err := readIGESAnnularChamferFace(&p, shell[i], shell[i+1])
		if err != nil {
			return out, err
		}
		faces = append(faces, face)
		wires = append(wires, face.igesBoreRimFace)
	}
	native, err := reconstructIGESAnnularChamfer(ctx, faces, o)
	if err != nil {
		return out, err
	}
	g, err := annularchamfer.Extract(ctx, native, o)
	if err != nil {
		return out, err
	}
	boundary, err := p.proveCircularBoundary(ctx, g.Graph, wires)
	if err != nil {
		return out, err
	}
	if len(boundary.pieces) != 15 || len(p.claimed) != len(p.records) {
		return out, errIGESAnnularChamfer
	}
	mapped, err := matchIGESAnnularChamferFaces(ctx, faces, g, boundary.pieces)
	if err != nil {
		return out, err
	}
	return EccentricAnnularChamferIGES{
		Solid: native, LengthUnit: doc.unit, source: doc.bytes, SourceGroupID: group, SourceSolidID: root, SourceShellID: body[1],
		SourceEdgeListID: p.edges, SourceVertexListID: p.vertices, NativeFaceSourceIDs: mapped,
		NativeCurveSourceIDs: boundary.curves, NativeEdgeSourceIndices: boundary.edges, NativeVertexSourceIndices: boundary.vertices,
	}, nil
}

var errIGESAnnularChamfer = errors.New("iges: unsupported exact eccentric annular chamfer")

func igesAnnularChamferDirectory(first, second [9]int) bool {
	if first[0] == 128 {
		return first[3] == 0 && first[4] == 0 && first[6] == 0 && first[8] == 10000 &&
			second[1] == 0 && second[2] == 0 && second[4] == 0 && second[8] == 0
	}
	return first[0] != 192 && first[0] != 198 && igesBoreRimDirectory(first, second)
}

type igesAnnularChamferFace struct {
	igesBoreRimFace
	station [2]trimmed.WrappedLoftStation
}

func readIGESAnnularChamferFace(p *igesBoreRimSource, id, sense int) (igesAnnularChamferFace, error) {
	var f igesAnnularChamferFace
	face, err := p.topology(id, 510)
	if err != nil || sense < 0 || sense > 1 || len(face) < 5 || len(face) > 6 || face[2] != len(face)-4 || face[3] != 1 {
		return f, errIGESAnnularChamfer
	}
	f.id, f.forward = id, sense == 1
	support, err := p.entity(face[1], p.records[face[1]].kind, true)
	if err != nil {
		return f, err
	}
	if err := p.readFaceLoops(&f.igesBoreRimFace, face[4:]); err != nil {
		return f, err
	}
	switch support.kind {
	case 190:
		if len(support.values) != 4 || len(f.loops) != 2 || len(f.loops[0]) != 1 || len(f.loops[1]) != 1 {
			return f, errIGESAnnularChamfer
		}
		f.center, err = p.vector(support.values[1], 116)
		if err != nil {
			return f, err
		}
		f.axis, err = p.vector(support.values[2], 123)
		if err != nil || f.axis != (geom.Vec3{Z: 1}) && f.axis != (geom.Vec3{Z: -1}) {
			return f, errIGESAnnularChamfer
		}
		x, err := p.vector(support.values[3], 123)
		if err != nil || x != (geom.Vec3{X: 1}) {
			return f, errIGESAnnularChamfer
		}
	case 128:
		if len(f.loops) != 1 || len(f.loops[0]) != 4 {
			return f, errIGESAnnularChamfer
		}
		f.center, f.station, err = igesAnnularChamferNet(support.values)
		if err != nil {
			return f, err
		}
	default:
		return f, errIGESAnnularChamfer
	}
	return f, nil
}

func reconstructIGESAnnularChamfer(ctx context.Context, faces []igesAnnularChamferFace, o geom.SurfacePatchOptions) (trimmed.Solid, error) {
	var caps [2]*igesAnnularChamferFace
	var walls [2][]igesAnnularChamferFace
	for i := range faces {
		f := &faces[i]
		if f.station[0].Radius == 0 {
			end := 0
			if f.forward == (f.axis.Z > 0) {
				end = 1
			}
			if caps[end] != nil {
				return trimmed.Solid{}, errIGESAnnularChamfer
			}
			caps[end] = f
			continue
		}
		side := 0
		if !f.forward {
			side = 1
		}
		walls[side] = append(walls[side], *f)
	}
	if caps[0] == nil || caps[1] == nil || caps[0].center.Z >= caps[1].center.Z {
		return trimmed.Solid{}, errIGESAnnularChamfer
	}
	var lofts [2]trimmed.Solid
	for side, wall := range walls {
		if len(wall) != 3 {
			return trimmed.Solid{}, errIGESAnnularChamfer
		}
		slices.SortFunc(wall, func(a, b igesAnnularChamferFace) int {
			return cmp.Compare(a.station[0].Z, b.station[0].Z)
		})
		center := geom.Vec2{X: wall[0].center.X, Y: wall[0].center.Y}
		stations := []trimmed.WrappedLoftStation{wall[0].station[0]}
		for i, f := range wall {
			if f.center.X != center.X || f.center.Y != center.Y || f.station[0] != stations[i] {
				return trimmed.Solid{}, errIGESAnnularChamfer
			}
			stations = append(stations, f.station[1])
		}
		if stations[0].Z != caps[0].center.Z || stations[3].Z != caps[1].center.Z {
			return trimmed.Solid{}, errIGESAnnularChamfer
		}
		var err error
		lofts[side], _, err = trimmed.LoftWrappedCircularStations(ctx, center, stations, 1e-9, o)
		if err != nil {
			return trimmed.Solid{}, err
		}
	}
	native, _, err := trimmed.CutWrappedEccentricCircularStationLofts(ctx, lofts[0], lofts[1], o)
	return native, err
}

func matchIGESAnnularChamferFaces(ctx context.Context, source []igesAnnularChamferFace,
	g annularchamfer.Graph, pieces [][]igesBoreRimUse) ([]int, error) {
	ids := make([]int, len(g.Faces))
	for _, f := range source {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		match := -1
		for i, face := range g.Faces {
			if ids[i] != 0 || !sameIGESAnnularChamferFace(f, g.Bands[i], face.Origin[2], face.Reversed) ||
				!matchIGESBoreRimFace(f.igesBoreRimFace, face, pieces) {
				continue
			}
			if match != -1 {
				return nil, errIGESAnnularChamfer
			}
			match = i
		}
		if match == -1 {
			return nil, errIGESAnnularChamfer
		}
		ids[match] = f.id
	}
	return ids, nil
}

func sameIGESAnnularChamferFace(f igesAnnularChamferFace, band annularchamfer.Band, capZ *big.Rat, reversed bool) bool {
	if band.Patch == nil {
		return f.station[0].Radius == 0 && capZ.Cmp(igesBoreRat(f.center.Z)) == 0 &&
			(f.forward == (f.axis.Z > 0)) != reversed
	}
	if f.station[0].Radius == 0 || f.forward == reversed || band.X.Cmp(igesBoreRat(f.center.X)) != 0 ||
		band.Y.Cmp(igesBoreRat(f.center.Y)) != 0 {
		return false
	}
	for i, s := range f.station {
		if band.Z[i].Cmp(igesBoreRat(s.Z)) != 0 || band.R[i].Cmp(igesBoreRat(s.Radius)) != 0 {
			return false
		}
	}
	return true
}
