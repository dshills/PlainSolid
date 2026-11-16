package iges

import (
	"context"
	"errors"
	"io"
	"slices"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/borefillet"
	"plainsolid/exchange/internal/conicalfillet"
	"plainsolid/geom"
)

// ConicalRimFilletBoreIGES retains one proved type 186 body and the
// original directory and one-based list ownership. Complete original
// circles own four native quadrants. An added cardinal vertex has
// source index zero.
type ConicalRimFilletBoreIGES struct {
	Solid                                              trimmed.Solid
	LengthUnit                                         float64
	SourceGroupID, SourceSolidID, SourceShellID        int
	SourceEdgeListID, SourceVertexListID               int
	NativeFaceSourceIDs, NativeCurveSourceIDs          []int
	NativeEdgeSourceIndices, NativeVertexSourceIndices []int
	source                                             []byte
	display                                            conicalfillet.DisplayRecipe
}

// SourceIGES returns an independent copy of the proved original records.
func (m ConicalRimFilletBoreIGES) SourceIGES() []byte { return slices.Clone(m.source) }

// TessellateCertified uses the authenticated rolling and bore inputs to
// construct a closed mesh with a certified two-sided deflection bound.
func (m ConicalRimFilletBoreIGES) TessellateCertified(ctx context.Context, o trimmed.CertifiedWrappedFrustumMeshOptions) (trimmed.CertifiedFilletedFrustumBoreMesh, error) {
	return m.display.TessellateCertified(ctx, o)
}

// TessellateNormalsCertified proves continuous outward source normals
// against each represented facet along with the two-sided distance bound.
func (m ConicalRimFilletBoreIGES) TessellateNormalsCertified(ctx context.Context, angle float64, o trimmed.CertifiedWrappedFrustumMeshOptions) (trimmed.CertifiedFilletedFrustumBoreMesh, error) {
	return m.display.TessellateNormalsCertified(ctx, angle, o)
}

var errIGESConicalRimFilletBore = errors.New("iges: unsupported exact conical rim fillet with eccentric bore")

type conicalRimFilletBoreFaces struct {
	cone   igesAnnularChamferFace
	bore   igesBoreRimFace
	caps   [2]igesBoreRimFace
	toroid [2]igesBoreRimFace
}

// ReadConicalRimFilletBore strictly imports one ascending conical stock
// with two external rolling rims and one eccentric through-bore. Its
// source has an exact rational type 128 cone, original type 198 toroids
// and model-only directed trims. The solid remains in original file units.
func ReadConicalRimFilletBore(ctx context.Context, r io.Reader, l Limits, o geom.SurfacePatchOptions) (ConicalRimFilletBoreIGES, error) {
	var empty ConicalRimFilletBoreIGES
	if ctx == nil || r == nil {
		return empty, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	l, err := conicalRimFilletBoreIGESLimits(l)
	if err != nil {
		return empty, err
	}
	doc, err := readTopologyRecordPolicyWithDirectory(r, l, true, false, false, false, false, false, true, false, false, conicalRimBoreDirectory)
	if err != nil {
		return empty, err
	}
	return decodeConicalRimFilletBoreIGES(ctx, doc, o)
}

func conicalRimFilletBoreIGESLimits(l Limits) (Limits, error) {
	l, err := l.defaults()
	if err != nil {
		return l, err
	}
	l.MaxBytes, l.MaxRecords = min(l.MaxBytes, 128<<10), min(l.MaxRecords, 1024)
	l.MaxEntities, l.MaxValues = min(l.MaxEntities, 128), min(l.MaxValues, 4096)
	if l.MaxControlPoints < 18 {
		return l, geom.ErrConvergence
	}
	return l, nil
}

func decodeConicalRimFilletBoreIGES(ctx context.Context, doc topologySource, o geom.SurfacePatchOptions) (ConicalRimFilletBoreIGES, error) {
	var empty ConicalRimFilletBoreIGES
	p, group, root, shellID, faceList, err := conicalRimFilletBoreIGESGraph(doc)
	if err != nil {
		return empty, err
	}
	faces, err := readConicalRimFilletBoreIGESFaces(&p, faceList)
	if err != nil {
		return empty, err
	}
	native, err := conicalfillet.Reconstruct(ctx, faces.sourceGeometry(), o)
	if err != nil {
		return empty, err
	}
	owners := make([]igesBoreRimFace, len(native.Graph.Faces))
	owners[native.Cone], owners[native.Bore] = faces.cone.igesBoreRimFace, faces.bore
	for end := range 2 {
		owners[native.Caps[end]], owners[native.Blends[end]] = faces.caps[end], faces.toroid[end]
	}
	boundary, err := p.proveCircularBoundary(ctx, native.Graph, owners)
	if err != nil {
		return empty, err
	}
	if len(boundary.pieces) != 11 || len(p.claimed) != len(p.records) {
		return empty, errIGESConicalRimFilletBore
	}
	mapped := make([]int, len(owners))
	for i, f := range owners {
		if !matchIGESBoreRimFace(f, native.Graph.Faces[i], boundary.pieces) {
			return empty, errIGESConicalRimFilletBore
		}
		mapped[i] = f.id
	}
	return ConicalRimFilletBoreIGES{
		Solid: native.Solid, display: native.Display, LengthUnit: doc.unit, source: doc.bytes,
		SourceGroupID: group, SourceSolidID: root, SourceShellID: shellID, SourceEdgeListID: p.edges, SourceVertexListID: p.vertices,
		NativeFaceSourceIDs: mapped, NativeCurveSourceIDs: boundary.curves,
		NativeEdgeSourceIndices: boundary.edges, NativeVertexSourceIndices: boundary.vertices,
	}, nil
}

func conicalRimFilletBoreIGESGraph(doc topologySource) (igesBoreRimSource, int, int, int, []int, error) {
	p := igesBoreRimSource{records: make(map[int]topologyRecord, len(doc.entities)), claimed: make(map[int]bool)}
	var group, root int
	for _, rec := range doc.entities {
		p.records[rec.id] = rec
		switch rec.kind {
		case 402:
			if group != 0 {
				return p, 0, 0, 0, nil, errIGESConicalRimFilletBore
			}
			group = rec.id
		case 186:
			if root != 0 {
				return p, 0, 0, 0, nil, errIGESConicalRimFilletBore
			}
			root = rec.id
		}
	}
	if group != 0 {
		rec, err := p.topology(group, 402)
		if err != nil || len(rec) != 3 || rec[1] != 1 || rec[2] != root {
			return p, 0, 0, 0, nil, errIGESConicalRimFilletBore
		}
	}
	body, err := p.topology(root, 186)
	if err != nil || len(body) != 4 || body[2] != 1 || body[3] != 0 || p.records[root].boreGroupMember != (group != 0) {
		return p, 0, 0, 0, nil, errIGESConicalRimFilletBore
	}
	shell, err := p.topology(body[1], 514)
	if err != nil || len(shell) != 14 || shell[1] != 6 {
		return p, 0, 0, 0, nil, errIGESConicalRimFilletBore
	}
	return p, group, root, body[1], shell[2:], nil
}

func conicalRimBoreDirectory(first, second [9]int) bool {
	if first[0] == 128 {
		return igesAnnularChamferDirectory(first, second)
	}
	return igesBoreRimDirectory(first, second)
}

func readConicalRimFilletBoreIGESFaces(p *igesBoreRimSource, src []int) (conicalRimFilletBoreFaces, error) {
	var out conicalRimFilletBoreFaces
	for i := 0; i < len(src); i += 2 {
		id, sense := src[i], src[i+1]
		rec := p.records[id]
		if rec.kind != 510 || len(rec.values) < 2 {
			return out, errIGESConicalRimFilletBore
		}
		support, err := intValue(rec.values[1])
		if err != nil {
			return out, err
		}
		if p.records[support].kind == 128 {
			if out.cone.id != 0 || sense != 1 {
				return out, errIGESConicalRimFilletBore
			}
			cone, err := readIGESAnnularChamferFace(p, id, sense)
			if err != nil || cone.station[0].Radius >= cone.station[1].Radius {
				return out, errIGESConicalRimFilletBore
			}
			out.cone = cone
			continue
		}
		f, err := p.readFace(id, sense)
		if err != nil {
			return out, err
		}
		end := 0
		switch f.kind {
		case borefillet.Plane:
			if f.forward == (f.axis.Z > 0) {
				end = 1
			}
			if out.caps[end].id != 0 || len(f.loops[0]) != 1 || len(f.loops[1]) != 1 {
				return out, errIGESConicalRimFilletBore
			}
			out.caps[end] = f
		case borefillet.Cylinder:
			if out.bore.id != 0 || f.forward || f.axis.Z != 1 || len(f.loops[0]) != 4 {
				return out, errIGESConicalRimFilletBore
			}
			out.bore = f
		case borefillet.Torus:
			if f.axis.Z > 0 {
				end = 1
			}
			if out.toroid[end].id != 0 || !f.forward || len(f.loops[0]) != 4 {
				return out, errIGESConicalRimFilletBore
			}
			out.toroid[end] = f
		default:
			return out, errIGESConicalRimFilletBore
		}
	}
	if out.cone.id == 0 || out.bore.id == 0 || out.caps[0].id == 0 || out.caps[1].id == 0 ||
		out.toroid[0].id == 0 || out.toroid[1].id == 0 {
		return out, errIGESConicalRimFilletBore
	}
	return out, nil
}

func (f conicalRimFilletBoreFaces) sourceGeometry() conicalfillet.Source {
	s := conicalfillet.Source{
		ConeCenter: geom.Vec2{X: f.cone.center.X, Y: f.cone.center.Y}, ConeStations: f.cone.station,
		BoreCenter: geom.Vec2{X: f.bore.center.X, Y: f.bore.center.Y}, BoreRadius: f.bore.major,
	}
	for end := range 2 {
		s.CapZ[end] = f.caps[end].center.Z
		t := f.toroid[end]
		s.Toroids[end] = conicalfillet.Toroid{Center: t.center, Major: t.major, Minor: t.minor}
	}
	return s
}
