package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ConicalRimFilletBoreRepairOptions sets an explicit maximum geometric
// displacement in the source STEP length unit.
type ConicalRimFilletBoreRepairOptions struct {
	MaxDistance float64
}

// ConicalRimFilletBoreRepairEvidence records the repairs to the original
// model-only graph. Bounds and stations are exact rational strings in source
// units, and the original angle is the source decimal in radians.
type ConicalRimFilletBoreRepairEvidence struct {
	ConeSupportID, ConeFaceID, SeamCurveID       int64
	ConeRimEdgeIDs, ConeSeamVertexIDs            [2]int64
	OriginalAngle, ExactSlope                    string
	OriginalRimStations                          [2]string
	ConeDisplacementBound, SeamDisplacementBound string
	Frames                                       []ConicalRimFilletBoreFrameRepair
	ReversedFaceID, ReversedBoundID              int64
	ReversedRimEdgeIDs                           [2]int64
	OriginalRimOwnerFaceIDs                      [2][2]int64
	OriginalRimUseSenses                         [2]bool
}

// ConicalRimFilletBoreFrameRepair binds the original frame to a certified
// displacement bound. EdgeID is present for a finite toroidal meridian.
type ConicalRimFilletBoreFrameRepair struct {
	SupportID, PlacementID, EdgeID int64
	CorrectedDirectionIDs          [2]int64
	DisplacementBound              string
}

// RepairedConicalRimFilletBoreSTEP binds the original bytes to an exact
// finite model-only STEP view and its strict native reconstruction.
type RepairedConicalRimFilletBoreSTEP struct {
	original, corrected string
	maxDistance         float64
	material            ConicalRimFilletBoreSTEP
	evidence            ConicalRimFilletBoreRepairEvidence
}

func (m RepairedConicalRimFilletBoreSTEP) OriginalSTEP() string       { return m.original }
func (m RepairedConicalRimFilletBoreSTEP) CorrectedModelSTEP() string { return m.corrected }
func (m RepairedConicalRimFilletBoreSTEP) Material() ConicalRimFilletBoreSTEP {
	p := m.material
	p.NativeFaceSourceIDs = slices.Clone(p.NativeFaceSourceIDs)
	p.NativeEdgeSourceIDs = slices.Clone(p.NativeEdgeSourceIDs)
	p.NativeVertexSourceIDs = slices.Clone(p.NativeVertexSourceIDs)
	return p
}
func (m RepairedConicalRimFilletBoreSTEP) Evidence() ConicalRimFilletBoreRepairEvidence {
	q := m.evidence
	q.Frames = slices.Clone(q.Frames)
	return q
}

var errConicalRimFilletBoreRepair = errors.New("step: unsupported bounded conical rim fillet repair")

type conicalRepairFace struct {
	boreRimSTEPFace
	support int64
	bounds  []int64
	kind    string
}

type conicalRepairEdge struct {
	start, end, curve int64
	sense             bool
}

type conicalRepairUse struct {
	face  int
	sense bool
}

type conicalRepairGraph struct {
	faces    []conicalRepairFace
	edges    map[int64]conicalRepairEdge
	uses     map[int64][]conicalRepairUse
	incoming map[int64]int
	cone     int
}

// RepairConicalRimFilletBore repairs the bounded model-only OCCT dialect with
// a rounded cone angle, a line seam, rounded circular frames and the inverted
// lower toroidal bound. Original vertices, units and topology IDs are retained.
// The corrected view must independently pass ReadConicalRimFilletBore.
func RepairConicalRimFilletBore(ctx context.Context, r io.Reader, l Limits, o ConicalRimFilletBoreRepairOptions) (RepairedConicalRimFilletBoreSTEP, error) {
	var zero RepairedConicalRimFilletBoreSTEP
	if ctx == nil || r == nil || !finiteNumber(o.MaxDistance) || o.MaxDistance <= 0 {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	l.MaxBytes = min(l.MaxBytes, 128<<10)
	l.MaxEntities = min(l.MaxEntities, 384)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, rep, unit, items, err := parseCanonicalCircularExtrusions(bytes.NewReader(raw), l)
	if err != nil {
		return zero, err
	}
	representation, _ := simple(f, rep, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err := occtLoopContextMode(f, representation.Args[2].Ref, true); err != nil {
		return zero, err
	}
	body, err := modelOnlyCircularSTEPBody(f, rep, items, map[string]bool{"CONICAL_SURFACE": true})
	if err != nil {
		return zero, err
	}
	g, err := readConicalRepairGraph(f, body)
	if err != nil {
		return zero, err
	}
	fixed := cloneOvalFile(f)
	fixed.Order = slices.Clone(f.Order)
	budget := new(big.Rat).SetFloat64(o.MaxDistance)
	evidence, obsolete, err := repairConicalSupport(f, fixed, g, budget)
	if err != nil {
		return zero, err
	}
	if err := repairConicalCircularFrames(f, fixed, g, budget, &evidence); err != nil {
		return zero, err
	}
	if err := repairLowerConicalToroidBound(f, fixed, g, &evidence); err != nil {
		return zero, err
	}
	if err := pruneConicalRepairSupports(f, fixed, rep, obsolete); err != nil {
		return zero, err
	}
	corrected, err := mappedPlanarSourceBytes(fixed)
	if err != nil {
		return zero, err
	}
	m, err := ReadConicalRimFilletBore(ctx, bytes.NewReader(corrected), l, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, fmt.Errorf("step: verify corrected conical source: %w", err)
	}
	if m.LengthUnit != unit || m.SourceBodyID != body {
		return zero, errConicalRimFilletBoreRepair
	}
	return RepairedConicalRimFilletBoreSTEP{
		original: string(raw), corrected: string(corrected), maxDistance: o.MaxDistance, material: m, evidence: evidence,
	}, nil
}

func readConicalRepairGraph(f *File, bodyID int64) (conicalRepairGraph, error) {
	g, err := readModelOnlyConicalRepairGraph(f, bodyID, 6, 10, 6)
	if err != nil {
		return g, err
	}
	for i, fc := range g.faces {
		if fc.kind != "CONICAL_SURFACE" {
			continue
		}
		if g.cone != -1 || !fc.sense || len(fc.loops) != 1 || len(fc.loops[0]) != 4 {
			return g, errConicalRimFilletBoreRepair
		}
		g.cone = i
	}
	if g.cone < 0 {
		return g, errConicalRimFilletBoreRepair
	}
	return g, nil
}

func readModelOnlyConicalRepairGraph(f *File, bodyID int64, faces, edges, points int) (conicalRepairGraph, error) {
	g := conicalRepairGraph{
		cone: -1, edges: map[int64]conicalRepairEdge{}, incoming: conicalRepairIncoming(f),
		uses: map[int64][]conicalRepairUse{},
	}
	body, err := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if err != nil || body.Args[1].Kind != Reference {
		return g, errConicalRimFilletBoreRepair
	}
	shell, err := simple(f, body.Args[1].Ref, "CLOSED_SHELL", 2)
	if err != nil {
		return g, errConicalRimFilletBoreRepair
	}
	faceIDs, err := listRefs(shell.Args[1], faces)
	if err != nil || len(faceIDs) != faces {
		return g, errConicalRimFilletBoreRepair
	}
	owned := map[int64]bool{}
	claim := func(id int64) bool {
		if owned[id] || g.incoming[id] != 1 {
			return false
		}
		owned[id] = true
		return true
	}
	for i, id := range faceIDs {
		fc, err := readConicalRepairFace(f, id, claim)
		if err != nil {
			return g, err
		}
		g.faces = append(g.faces, fc)
		for _, loop := range fc.loops {
			for _, use := range loop {
				g.uses[use.id] = append(g.uses[use.id], conicalRepairUse{i, use.sense})
			}
		}
	}
	if len(g.uses) != edges {
		return g, errConicalRimFilletBoreRepair
	}
	vertices := map[int64]bool{}
	for id, uses := range g.uses {
		if len(uses) != 2 || g.incoming[id] != 2 {
			return g, errConicalRimFilletBoreRepair
		}
		edge, err := simple(f, id, "EDGE_CURVE", 5)
		if err != nil {
			return g, err
		}
		refs, err := namedReferences(edge, 1, 2, 3)
		if err != nil || g.incoming[refs[2]] != 1 {
			return g, errConicalRimFilletBoreRepair
		}
		sense, err := boolean(edge.Args[4])
		if err != nil {
			return g, err
		}
		g.edges[id] = conicalRepairEdge{refs[0], refs[1], refs[2], sense}
		vertices[refs[0]], vertices[refs[1]] = true, true
	}
	if len(vertices) != points {
		return g, errConicalRimFilletBoreRepair
	}
	return g, nil
}

func readConicalRepairFace(f *File, id int64, claim func(int64) bool) (conicalRepairFace, error) {
	fc := conicalRepairFace{boreRimSTEPFace: boreRimSTEPFace{id: id}}
	r, err := simple(f, id, "ADVANCED_FACE", 4)
	if err != nil || !claim(id) {
		return fc, errConicalRimFilletBoreRepair
	}
	fc.sense, err = boolean(r.Args[3])
	if err != nil {
		return fc, err
	}
	fc.support, err = ref(r.Args[2])
	if err != nil || !claim(fc.support) || len(f.Entities[fc.support].Records) != 1 {
		return fc, errConicalRimFilletBoreRepair
	}
	fc.kind = f.Entities[fc.support].Records[0].Type
	fc.bounds, err = listRefs(r.Args[1], 1)
	if err != nil || len(fc.bounds) > 2 {
		return fc, errConicalRimFilletBoreRepair
	}
	err = readModelOnlyCircularSTEPWires(f, fc.bounds, &fc.boreRimSTEPFace, claim)
	return fc, err
}

func conicalRepairIncoming(f *File) map[int64]int {
	out := map[int64]int{}
	var count func(Value)
	count = func(v Value) {
		if v.Kind == Reference {
			out[v.Ref]++
		}
		for _, a := range v.Items {
			count(a)
		}
	}
	for _, r := range f.Header {
		for _, a := range r.Args {
			count(a)
		}
	}
	for _, e := range f.Entities {
		for _, r := range e.Records {
			for _, a := range r.Args {
				count(a)
			}
		}
	}
	return out
}

func pruneConicalRepairSupports(original, fixed *File, rep int64, obsolete []int64) error {
	roots := []int64{rep}
	for _, id := range original.Order {
		for _, r := range original.Entities[id].Records {
			switch r.Type {
			case "APPLICATION_PROTOCOL_DEFINITION", "PRODUCT_RELATED_PRODUCT_CATEGORY", "SHAPE_DEFINITION_REPRESENTATION":
				roots = append(roots, id)
			}
		}
	}
	keep, err := nauoClosure(fixed, roots, 16)
	if err != nil {
		return err
	}
	old, err := nauoClosure(original, obsolete, 16)
	if err != nil {
		return err
	}
	order := make([]int64, 0, len(keep))
	for _, id := range fixed.Order {
		if keep[id] {
			order = append(order, id)
			continue
		}
		if !old[id] {
			return errConicalRimFilletBoreRepair
		}
		delete(fixed.Entities, id)
	}
	fixed.Order = order
	return nil
}

func addConicalRepairEntity(f *File, r Record) (int64, error) {
	var largest int64
	for _, id := range f.Order {
		largest = max(largest, id)
	}
	if largest == math.MaxInt64 {
		return 0, errConicalRimFilletBoreRepair
	}
	id := largest + 1
	ovalSet(f, id, r)
	f.Order = append(f.Order, id)
	return id, nil
}
