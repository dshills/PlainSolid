package iges

import (
	"context"
	"errors"
	"io"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ConicalRimFilletBoreRepairOptions sets the maximum geometric displacement
// in the length unit of the original IGES file.
type ConicalRimFilletBoreRepairOptions struct {
	MaxDistance float64
}

// ConicalRimFilletBoreRepairEvidence identifies the original type 194
// support, its finite seam and the rounded occurrences. Geometric bounds,
// stations and slope are exact rational strings in source coordinates.
// OriginalAngleDegrees preserves the original IGES decimal.
type ConicalRimFilletBoreRepairEvidence struct {
	ConeFaceID, ConeSupportID, ConeSeamCurveID int
	ConeSeamEdgeIndex                          int
	ConeRimEdgeIndices, ConeVertexIndices      [2]int
	ConeRootPointID, ConeAxisID, ConePhaseID   int
	OriginalAngleDegrees, ExactSlope           string
	OriginalRimStations                        [2]string
	ConeDisplacementBound                      string
	ArcOccurrences                             []ConicalRimFilletBoreArcRepair
	TorusOccurrences                           []ConicalRimFilletBoreTorusRepair
}

// ConicalRimFilletBoreArcRepair bounds displacement on an original type
// 100 finite arc or complete circle under its type 124 affine occurrence.
type ConicalRimFilletBoreArcRepair struct {
	CurveID, TransformID, EdgeIndex int
	FaceIDs                         [2]int
	Meridian                        bool
	DisplacementBound               string
}

// ConicalRimFilletBoreTorusRepair records the phase displacement on one
// original type 198 surface with unchanged axis, translation and radii.
type ConicalRimFilletBoreTorusRepair struct {
	FaceID, SupportID, DirectionID int
	DisplacementBound              string
}

// RepairedConicalRimFilletBoreIGES binds the strict native reconstruction and
// its original entity ownership to the unchanged original IGES bytes.
type RepairedConicalRimFilletBoreIGES struct {
	maxDistance float64
	material    ConicalRimFilletBoreIGES
	evidence    ConicalRimFilletBoreRepairEvidence
}

// OriginalIGES returns a copy of the byte-preserved source.
func (m RepairedConicalRimFilletBoreIGES) OriginalIGES() []byte { return m.material.SourceIGES() }

// Material returns the proved native conical solid and original ownership.
func (m RepairedConicalRimFilletBoreIGES) Material() ConicalRimFilletBoreIGES {
	p := m.material
	p.NativeFaceSourceIDs = slices.Clone(p.NativeFaceSourceIDs)
	p.NativeCurveSourceIDs = slices.Clone(p.NativeCurveSourceIDs)
	p.NativeEdgeSourceIndices = slices.Clone(p.NativeEdgeSourceIndices)
	p.NativeVertexSourceIndices = slices.Clone(p.NativeVertexSourceIndices)
	return p
}

// Evidence returns the independently replayable repair claims.
func (m RepairedConicalRimFilletBoreIGES) Evidence() ConicalRimFilletBoreRepairEvidence {
	p := m.evidence
	p.ArcOccurrences = slices.Clone(p.ArcOccurrences)
	p.TorusOccurrences = slices.Clone(p.TorusOccurrences)
	return p
}

var errIGESConicalRimFilletBoreRepair = errors.New("iges: unsupported bounded conical rim fillet repair")

type igesConicalRepairUse struct {
	face, supportKind int
	forward           bool
}

type igesConicalRepairEdge struct {
	curve, start, end int
	uses              [2]igesConicalRepairUse
}

type igesConicalRepairFace struct {
	id, support, kind int
	face              igesBoreRimFace
}

type igesConicalRepairGraph struct {
	faces         []igesConicalRepairFace
	edges         []igesConicalRepairEdge
	vertices      []igesPlanarVec
	cone          int
	supportRefs   map[int]int
	transformRefs map[int]int
}

// RepairConicalRimFilletBore explicitly repairs one model-only OCCT conical
// stock with both rolling rims and one eccentric bore. It preserves all
// original vertices, edge geometry types, directed uses and owner IDs.
// The finite cone and corrected circle occurrences must pass the same exact
// incidence and native reconstruction as ReadConicalRimFilletBore.
func RepairConicalRimFilletBore(ctx context.Context, r io.Reader, l Limits, o ConicalRimFilletBoreRepairOptions) (RepairedConicalRimFilletBoreIGES, error) {
	var zero RepairedConicalRimFilletBoreIGES
	if ctx == nil || r == nil || !finite(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := conicalRimFilletBoreIGESLimits(l)
	if err != nil {
		return zero, err
	}
	doc, err := readTopologyRecordPolicyWithDirectory(r, l, true, false, false, false, false, false, true, false, false, igesConicalRepairDirectory)
	if err != nil {
		return zero, err
	}
	p, _, _, _, faces, err := conicalRimFilletBoreIGESGraph(doc)
	if err != nil {
		return zero, err
	}
	g, err := readIGESConicalRepairGraph(&p, doc.entities, faces)
	if err != nil {
		return zero, err
	}
	edits := map[int]topologyRecord{}
	budget := igesBoreRat(o.MaxDistance)
	evidence, err := repairIGESConicalSupport(&p, g, edits, budget)
	if err != nil {
		return zero, err
	}
	if err := repairIGESConicalOccurrences(&p, g, edits, budget, &evidence); err != nil {
		return zero, err
	}
	corrected := doc
	corrected.entities = make([]topologyRecord, 0, len(doc.entities)-3)
	for _, rec := range doc.entities {
		if edit, ok := edits[rec.id]; ok {
			rec = edit
		}
		if rec.id != 0 {
			corrected.entities = append(corrected.entities, rec)
		}
	}
	material, err := decodeConicalRimFilletBoreIGES(ctx, corrected, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, err
	}
	return RepairedConicalRimFilletBoreIGES{o.MaxDistance, material, evidence}, nil
}

func igesConicalRepairDirectory(first, second [9]int) bool {
	if first[0] == 194 {
		first[0], second[0] = 198, 198
		return igesBoreRimDirectory(first, second)
	}
	return igesBoreRimDirectory(first, second)
}

func readIGESConicalRepairGraph(p *igesBoreRimSource, records []topologyRecord, src []int) (igesConicalRepairGraph, error) {
	g := igesConicalRepairGraph{cone: -1, supportRefs: map[int]int{}, transformRefs: map[int]int{}}
	const edgeCount = 10
	var uses [edgeCount][]igesConicalRepairUse
	for _, rec := range records {
		if rec.transform != 0 {
			g.transformRefs[rec.transform]++
		}
	}
	for i := 0; i < len(src); i += 2 {
		fc, err := readIGESConicalRepairFace(p, src[i], src[i+1], g.supportRefs)
		if err != nil {
			return g, err
		}
		if fc.kind == 194 {
			if g.cone != -1 || !fc.face.forward || len(fc.face.loops) != 1 || len(fc.face.loops[0]) != 4 {
				return g, errIGESConicalRimFilletBoreRepair
			}
			g.cone = len(g.faces)
		}
		g.faces = append(g.faces, fc)
		for _, wire := range fc.face.loops {
			for _, u := range wire {
				if u.edge > edgeCount {
					return g, errIGESConicalRimFilletBoreRepair
				}
				uses[u.edge-1] = append(uses[u.edge-1], igesConicalRepairUse{fc.id, fc.kind, u.forward})
			}
		}
	}
	if g.cone < 0 {
		return g, errIGESConicalRimFilletBoreRepair
	}
	edges, err := p.topology(p.edges, 504)
	if err != nil || len(edges) != 2+5*edgeCount || edges[1] != edgeCount {
		return g, errIGESConicalRimFilletBoreRepair
	}
	vertices := edges[3]
	vertexRecord, err := p.entity(vertices, 502, true)
	if err != nil || len(vertexRecord.values) != 20 {
		return g, errIGESConicalRimFilletBoreRepair
	}
	n, err := intValue(vertexRecord.values[1])
	if err != nil || n != 6 {
		return g, errIGESConicalRimFilletBoreRepair
	}
	for i := 0; i < n; i++ {
		point, err := igesPlanarCoordinates(vertexRecord.values[2+3*i:])
		if err != nil {
			return g, err
		}
		g.vertices = append(g.vertices, point)
	}
	curves := map[int]bool{}
	for i, u := range uses {
		values := edges[2+5*i : 7+5*i]
		curve, a, b := values[0], values[2], values[4]
		if len(u) != 2 || u[0].forward == u[1].forward || curves[curve] || values[1] != vertices || values[3] != vertices ||
			a < 1 || a > n || b < 1 || b > n {
			return g, errIGESConicalRimFilletBoreRepair
		}
		curves[curve] = true
		g.edges = append(g.edges, igesConicalRepairEdge{curve: curve, start: a, end: b, uses: [2]igesConicalRepairUse{u[0], u[1]}})
	}
	return g, nil
}

func readIGESConicalRepairFace(p *igesBoreRimSource, id, sense int, supportRefs map[int]int) (igesConicalRepairFace, error) {
	var zero igesConicalRepairFace
	fail := errIGESConicalRimFilletBoreRepair
	face, err := p.topology(id, 510)
	if err != nil || sense < 0 || sense > 1 || len(face) < 5 || face[2] < 1 || face[2] > 2 || len(face) != 4+face[2] || face[3] != 1 {
		return zero, fail
	}
	kind := p.records[face[1]].kind
	r, err := p.entity(face[1], kind, true)
	if err != nil {
		return zero, err
	}
	arity := 0
	switch kind {
	case 190:
		arity = 4
	case 192:
		arity = 5
	case 194, 198:
		arity = 6
	}
	if arity == 0 || len(r.values) != arity {
		return zero, fail
	}
	for _, i := range []int{1, 2, arity - 1} {
		ref, err := intValue(r.values[i])
		if err != nil {
			return zero, err
		}
		supportRefs[ref]++
	}
	fc := igesConicalRepairFace{id: id, support: r.id, kind: kind, face: igesBoreRimFace{id: id, forward: sense == 1}}
	if err := p.readFaceLoops(&fc.face, face[4:]); err != nil {
		return zero, err
	}
	return fc, nil
}

func igesConicalRepairNumber(v parameter) (*big.Rat, error) {
	if len(v.text) > 64 {
		return nil, errIGESConicalRimFilletBoreRepair
	}
	return igesPlanarExact(v)
}
