package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// MappedChainSTEPCoedge records the original source entities for one roof owner.
type MappedChainSTEPCoedge struct {
	FaceID, PCurveID, TrimmedCurveID, BasisID int64
}

// MappedChainSTEPCell identifies one original shared STEP model edge and its
// two independent roof pcurve owners in model-station order.
type MappedChainSTEPCell struct {
	EdgeID, ModelCurveID int64
	Owners               [2]MappedChainSTEPCoedge
}

// MappedChainSTEPSupportCrossing records one original STEP edge junction
// and its two adjacent certified native edges. RawParameters and
// OwnersChangingCells follow the original owners. StoredModelKnot distinguishes
// an authored model spline knot from an inferred owner-only support cut.
type MappedChainSTEPSupportCrossing struct {
	EdgeID               int64
	SourceModelParameter *big.Rat
	ModelStation         *big.Rat
	RawParameters        [2]*big.Rat
	OwnersChangingCells  [2]bool
	StoredModelKnot      bool
	NativeEdges          [2]int
}

// MappedChainSTEPJointSupportCrossing records the exact support event at an
// existing STEP vertex and which original charts change cells there. Each
// pair is ordered incoming then outgoing. OwnerParameters and
// OwnersChangingCells follow the original roof order.
type MappedChainSTEPJointSupportCrossing struct {
	SourceVertexID        int64
	SourceEdges           [2]int64
	SourceModelParameters [2]*big.Rat
	ModelStation          *big.Rat
	OwnerParameters       [2][2]*big.Rat
	OwnersChangingCells   [2]bool
	NativeEdges           [2]int
}

// MappedChainSTEPModel contains the rebuilt source certificate, embedded native
// material, and original roof ownership. FaceIDs follows the native face order.
type MappedChainSTEPModel struct {
	Name       string
	LengthUnit float64
	Shell      trimmed.MappedSplineChainShell
	Native     trimmed.MappedChainRectangularSolid
	FaceIDs    []int64
	Chain      []MappedChainSTEPCell
	Planes     []MappedChainSTEPPlane
	// Crossings are ordered by native model station. Crossing is populated
	// when the original edge has exactly one repair.
	Crossings []MappedChainSTEPSupportCrossing
	Crossing  *MappedChainSTEPSupportCrossing
	// SupportJoints retains the original vertices in model order, separately
	// from the new cuts. SupportJoint is populated for the two-cell form.
	SupportJoints []MappedChainSTEPJointSupportCrossing
	SupportJoint  *MappedChainSTEPJointSupportCrossing
	original      string
	foreign       bool
}

func (m MappedChainSTEPModel) OriginalSTEP() string { return m.original }

type mappedChainStepUseIndex struct {
	face, trim int
	sense      bool
}

type mappedChainStepSource struct {
	file     *File
	faces    []mappedChainStepFace
	uses     map[int64][]mappedChainStepUseIndex
	vertices map[int64]bool
	edgeEnds map[int64][2]int64
	pcurves  map[int64]bool
	curves   map[int64]bool
	bases    map[int64]*geom.BSpline
	name     string
	unit     float64
	foreign  bool
}

// ReadMappedChainRectangularShell authenticates a standard single seven-face
// source graph in canonical roof/closing face order. Shared original spline
// segments (two through eight) lead both roof loops in model-station order.
// It verifies every
// source surface, stored model curve, original pcurve, vertex, and seam before
// building the independently embedded native material. The two roofs have
// whole original cubic or quartic bases restricted by parameter-valued trims.
// The independent and mixed basis opt-ins admit separate full coedge bases
// on both roofs or one roof, respectively. The three-cut mixed-basis opt-in
// also admits the canonical original source graph before repair.
func ReadMappedChainRectangularShell(ctx context.Context, r io.Reader, l Limits, o MappedChainShellSTEPOptions) (MappedChainSTEPModel, error) {
	return readMappedChainRectangularShell(ctx, r, l, o, false)
}

// ReadOCCTMappedChainRectangularShell authenticates an independent seven-face
// AP242 shell with two original spline roof charts and five signed-axis
// analytic planes. It discovers the two roofs and the cyclic shared chain.
// Each direct original roof pcurve retains its individual source spline basis.
// The opt-in unsplit route also admits both roof chains as parameter-valued
// restrictions on their original complete bases.
// The mixed-basis opt-in admits one roof in each form, and can combine with
// the three-support-knot opt-in for one certified unsplit original edge.
func ReadOCCTMappedChainRectangularShell(ctx context.Context, r io.Reader, l Limits, o MappedChainShellSTEPOptions) (MappedChainSTEPModel, error) {
	return readMappedChainRectangularShell(ctx, r, l, o, true)
}

func readMappedChainRectangularShell(ctx context.Context, r io.Reader, l Limits, o MappedChainShellSTEPOptions, foreign bool) (MappedChainSTEPModel, error) {
	var zero MappedChainSTEPModel
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if !o.AllowMultiSpanRoofs && (o.AllowTwoInteriorRoofKnots || foreign && o.AllowUnsplitSupportKnot) {
		return zero, geom.ErrInvalid
	}
	if foreign && o.AllowAsynchronousSupportKnots && !o.AllowUnsplitSupportKnot {
		return zero, geom.ErrInvalid
	}
	if (o.AllowSameCellAsynchronousSupportKnots || o.AllowInferredSharedSupportKnot) &&
		(!o.AllowMultiSpanRoofs || !o.AllowUnsplitSupportKnot || !o.AllowMixedOwnerBases) {
		return zero, geom.ErrInvalid
	}
	if o.AllowThreeJoinedSupportKnotCells && !o.AllowJoinedSupportKnotCells {
		return zero, geom.ErrInvalid
	}
	if o.AllowJoinedSupportKnotCells && (!o.AllowMultiSpanRoofs || !o.AllowTwoInteriorRoofKnots ||
		!o.AllowUnsplitSupportKnot || !o.AllowMixedOwnerBases) {
		return zero, geom.ErrInvalid
	}
	if foreign && (o.AllowMixedSupportKnots || o.AllowThreeSupportKnots || o.AllowFourSupportKnots || o.AllowUpToSixSupportKnots ||
		o.AllowMultipleSupportKnotCells || o.AllowThreeSupportKnotCells || o.AllowSeparatedSupportKnotCells) &&
		(!o.AllowUnsplitSupportKnot || !o.AllowTwoInteriorRoofKnots) {
		return zero, geom.ErrInvalid
	}
	if (o.AllowFourSupportKnots || o.AllowUpToSixSupportKnots || o.AllowMultipleSupportKnotCells || o.AllowThreeSupportKnotCells ||
		o.AllowSeparatedSupportKnotCells) && !o.AllowTwoInteriorRoofKnots {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 2048)
	l.MaxValues = min(l.MaxValues, 32768)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return zero, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return zero, err
	}
	maxRoofKnots := 0
	if o.AllowMultiSpanRoofs {
		maxRoofKnots = 1
	}
	if o.AllowTwoInteriorRoofKnots {
		maxRoofKnots = 2
	}
	if o.AllowFourSupportKnots || o.AllowMultipleSupportKnotCells {
		maxRoofKnots = 3
	}
	if o.AllowUpToSixSupportKnots || o.AllowThreeSupportKnotCells || o.AllowThreeJoinedSupportKnotCells {
		maxRoofKnots = 4
	}
	source, err := openMappedChainSTEPSource(f, l, foreign, maxRoofKnots)
	if err != nil {
		return zero, err
	}
	out, err := source.prove(ctx, o)
	if err != nil {
		return zero, err
	}
	sourceVertices := len(source.vertices) + len(out.Crossings)
	if sourceVertices > l.MaxVertices || sourceVertices != len(out.Native.Solid.Vertices()) ||
		2*len(out.Native.Solid.Edges()) > l.MaxEdgeUses {
		return zero, geom.ErrConvergence
	}
	out.original = string(raw)
	out.foreign = foreign
	return out, nil
}

func openMappedChainSTEPSource(f *File, l Limits, foreign bool, maxRoofKnots int) (mappedChainStepSource, error) {
	var zero mappedChainStepSource
	var rid int64
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "ADVANCED_BREP_SHAPE_REPRESENTATION"); ok {
			if rid != 0 {
				return zero, geom.ErrInvalid
			}
			rid = id
		}
	}
	rep, err := simple(f, rid, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err != nil || rep.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	items, err := listRefs(rep.Args[1], 1)
	if err != nil || len(items) != 1 && !foreign || len(items) != 2 && foreign {
		return zero, geom.ErrInvalid
	}
	var bodyID int64
	for _, id := range items {
		if _, ok := component(f.Entities[id], "MANIFOLD_SOLID_BREP"); ok {
			if bodyID != 0 {
				return zero, geom.ErrInvalid
			}
			bodyID = id
			continue
		}
		p, err := readMappedChainForeignFrame(f, id)
		if !foreign || err != nil || p.normal != [3]int{0, 0, 1} || p.dir[0] != [3]int{1, 0, 0} {
			return zero, geom.ErrInvalid
		}
		for _, q := range p.origin {
			if q.Sign() != 0 {
				return zero, geom.ErrInvalid
			}
		}
	}
	body, err := simple(f, bodyID, "MANIFOLD_SOLID_BREP", 2)
	if err != nil || body.Args[0].Kind != String {
		return zero, geom.ErrInvalid
	}
	fids, err := bodyFaceIDs(f, bodyID)
	if err != nil || len(fids) != 7 || len(fids) > l.MaxFaces {
		return zero, geom.ErrInvalid
	}
	cid, err := ref(rep.Args[2])
	if err != nil {
		return zero, err
	}
	unit, err := contextUnits(f, cid)
	if err != nil {
		return zero, err
	}
	if err := validateMappedChainSTEPClosure(f, rid, foreign); err != nil {
		return zero, err
	}
	source := mappedChainStepSource{
		file: f, name: body.Args[0].Text, unit: unit, foreign: foreign,
		uses: make(map[int64][]mappedChainStepUseIndex), vertices: make(map[int64]bool), pcurves: make(map[int64]bool),
		edgeEnds: make(map[int64][2]int64), curves: make(map[int64]bool), bases: make(map[int64]*geom.BSpline),
	}
	supports := make(map[int64]bool)
	used := 0
	for _, id := range fids {
		face, err := parseMappedChainStepFace(f, id, foreign, maxRoofKnots)
		if err != nil {
			return zero, fmt.Errorf("mapped STEP face #%d: %w", id, err)
		}
		if supports[face.supportID] {
			return zero, geom.ErrInvalid
		}
		supports[face.supportID] = true
		source.faces = append(source.faces, face)
	}
	if foreign {
		source.faces, err = orderMappedChainForeignFaces(source.faces)
		if err != nil {
			return zero, err
		}
	}
	for i, face := range source.faces {
		if i >= 2 && mappedChainInteriorSupportKnots(face.surface) != 0 {
			return zero, geom.ErrConvergence
		}
		for j, u := range face.uses {
			source.uses[u.id] = append(source.uses[u.id], mappedChainStepUseIndex{i, j, u.sense})
		}
		used += len(face.uses)
	}
	if used > l.MaxEdgeUses || len(source.uses) > 64 {
		return zero, geom.ErrConvergence
	}
	return source, nil
}

func validateMappedChainSTEPClosure(f *File, rid int64, foreign bool) error {
	closure, err := nauoClosure(f, []int64{rid}, 32)
	if err != nil {
		return err
	}
	for id, entity := range f.Entities {
		for _, r := range entity.Records {
			switch r.Type {
			case "MANIFOLD_SOLID_BREP", "CLOSED_SHELL", "ADVANCED_FACE", "FACE_OUTER_BOUND", "EDGE_LOOP", "ORIENTED_EDGE",
				"EDGE_CURVE", "VERTEX_POINT", "SURFACE_CURVE", "PCURVE", "DEFINITIONAL_REPRESENTATION", "TRIMMED_CURVE",
				"CARTESIAN_POINT", "BOUNDED_CURVE", "B_SPLINE_CURVE", "B_SPLINE_CURVE_WITH_KNOTS", "CURVE",
				"GEOMETRIC_REPRESENTATION_ITEM", "RATIONAL_B_SPLINE_CURVE", "REPRESENTATION_ITEM", "BOUNDED_SURFACE",
				"B_SPLINE_SURFACE", "B_SPLINE_SURFACE_WITH_KNOTS", "RATIONAL_B_SPLINE_SURFACE", "SURFACE":
				if !closure[id] {
					return geom.ErrInvalid
				}
			case "PLANE", "LINE", "VECTOR", "DIRECTION", "AXIS2_PLACEMENT_3D", "FACE_BOUND":
				if !foreign || !closure[id] {
					return geom.ErrInvalid
				}
			case "ADVANCED_BREP_SHAPE_REPRESENTATION", "GEOMETRIC_REPRESENTATION_CONTEXT", "GLOBAL_UNIT_ASSIGNED_CONTEXT",
				"GLOBAL_UNCERTAINTY_ASSIGNED_CONTEXT", "UNCERTAINTY_MEASURE_WITH_UNIT", "REPRESENTATION_CONTEXT", "PARAMETRIC_REPRESENTATION_CONTEXT",
				"APPLICATION_CONTEXT", "APPLICATION_PROTOCOL_DEFINITION", "MECHANICAL_CONTEXT", "DESIGN_CONTEXT", "PRODUCT",
				"PRODUCT_DEFINITION_FORMATION_WITH_SPECIFIED_SOURCE", "PRODUCT_DEFINITION", "PRODUCT_DEFINITION_SHAPE", "SHAPE_DEFINITION_REPRESENTATION",
				"PRODUCT_CONTEXT", "PRODUCT_DEFINITION_CONTEXT", "PRODUCT_DEFINITION_FORMATION", "PRODUCT_RELATED_PRODUCT_CATEGORY",
				"NAMED_UNIT", "SI_UNIT", "LENGTH_UNIT", "PLANE_ANGLE_UNIT", "SOLID_ANGLE_UNIT", "CONVERSION_BASED_UNIT",
				"DIMENSIONAL_EXPONENTS", "LENGTH_MEASURE_WITH_UNIT", "MEASURE_WITH_UNIT":
			default:
				return fmt.Errorf("mapped STEP source entity %s: %w", r.Type, geom.ErrInvalid)
			}
		}
	}
	return nil
}

func (g *mappedChainStepSource) prove(ctx context.Context, o MappedChainShellSTEPOptions) (MappedChainSTEPModel, error) {
	var zero MappedChainSTEPModel
	canonicalOriginal := !g.foreign && o.allowsUnsplitMixedOwnerBases()
	var canonicalReversed bool
	var chainIDs []int64
	for i, use := range g.faces[0].uses {
		owners := g.uses[use.id]
		if len(owners) != 2 {
			return zero, geom.ErrInvalid
		}
		if owners[1].face != 1 {
			continue
		}
		if len(chainIDs) != i || owners[1].trim != i || use.sense != owners[1].sense || (!g.foreign && !canonicalOriginal && !use.sense) {
			return zero, geom.ErrInvalid
		}
		canonicalReversed = canonicalReversed || !g.foreign && !use.sense
		chainIDs = append(chainIDs, use.id)
	}
	n := len(chainIDs)
	if n < 2 || n > 8 || g.faces[0].reversed == g.faces[1].reversed {
		return zero, fmt.Errorf("mapped STEP shared chain has unsupported source topology: %w", geom.ErrConvergence)
	}
	var closing [2][]trimmed.PCurve
	for i := range closing {
		closing[i] = make([]trimmed.PCurve, len(g.faces[i].uses)-n)
	}
	planar := make([]trimmed.MappedChainPlanarFace, len(g.faces)-2)
	for i := range planar {
		f := g.faces[i+2]
		planar[i] = trimmed.MappedChainPlanarFace{Surface: f.surface, Outer: make([]trimmed.PCurve, len(f.uses)), Reversed: f.reversed}
	}
	var parts [2][]trimmed.MappedSplineUV
	var chain []MappedChainSTEPCell
	var stations []*big.Rat
	var crossings []MappedChainSTEPSupportCrossing
	var foreignTrimmed [2]bool
	var foreignMixed bool
	allowUnsplit := g.foreign && o.AllowUnsplitSupportKnot
	knotsA, knotsB := mappedChainInteriorSupportKnots(g.faces[0].surface), mappedChainInteriorSupportKnots(g.faces[1].surface)
	allowAsync := g.foreign && o.AllowAsynchronousSupportKnots && knotsA == 1 && knotsB == 1
	var bezierRepairs mappedChainBezierRepairs
	if (g.foreign || canonicalOriginal) && knotsA == 1 && knotsB == 1 {
		if o.AllowSameCellAsynchronousSupportKnots {
			bezierRepairs |= mappedChainBezierAsynchronous
		}
		if o.AllowInferredSharedSupportKnot {
			bezierRepairs |= mappedChainBezierShared
		}
	}
	allowMixed := g.foreign && o.AllowMixedSupportKnots && min(knotsA, knotsB) == 1 && max(knotsA, knotsB) == 2
	allowThree := (g.foreign || canonicalOriginal) && o.AllowThreeSupportKnots && knotsA == 2 && knotsB == 2
	canUseTwoCells := (g.foreign || canonicalOriginal) && o.AllowMultipleSupportKnotCells && knotsA == 3 && knotsB == 3
	canUseThreeCells := (g.foreign || canonicalOriginal) && o.AllowThreeSupportKnotCells && knotsA == 4 && knotsB == 4
	canUseSeparatedCells := (g.foreign || canonicalOriginal) && o.AllowSeparatedSupportKnotCells && knotsA == 2 && knotsB == 2
	v8Cuts := 0
	if (g.foreign || canonicalOriginal) && min(knotsA, knotsB) >= 2 && max(knotsA, knotsB) <= 4 {
		count := knotsA + knotsB - 1
		if count == 4 && o.AllowFourSupportKnots || 4 <= count && count <= 6 && o.AllowUpToSixSupportKnots {
			v8Cuts = count
		}
	}
	maxCrossings, maxInferredSupportKnots := 3, 0
	if v8Cuts != 0 {
		maxCrossings, maxInferredSupportKnots = v8Cuts, max(knotsA, knotsB)
	} else if allowMixed || allowThree {
		maxInferredSupportKnots = 2
	}
	maxModelPieces := 1
	if canonicalOriginal && (allowThree || v8Cuts != 0 || canUseTwoCells || canUseThreeCells || canUseSeparatedCells) {
		maxModelPieces = 2
	} else if allowUnsplit {
		maxModelPieces = 2
		if o.AllowTwoInteriorRoofKnots {
			maxModelPieces = 3
		}
	}
	sourceEdges := make([]mappedChainStepEdge, len(chainIDs))
	withStoredKnots := 0
	for i, id := range chainIDs {
		e, err := g.edge(ctx, id, o.Geometry, maxModelPieces)
		if err != nil {
			return zero, fmt.Errorf("mapped STEP edge #%d: %w", id, err)
		}
		sourceEdges[i] = e
		if len(e.joins) != 0 {
			withStoredKnots++
		}
	}
	if withStoredKnots == 0 && (g.foreign || canonicalOriginal) && o.AllowInferredSharedSupportKnot &&
		o.AllowTwoInteriorRoofKnots && knotsA == 2 && knotsB == 2 {
		bezierRepairs |= mappedChainBezierSharedPair
	}
	localCells := 0
	if canUseTwoCells && withStoredKnots == 2 || canUseThreeCells && withStoredKnots == 3 {
		localCells = withStoredKnots
	}
	cellLocal := localCells != 0
	separatedCandidate := canUseSeparatedCells && withStoredKnots == 1
	threeJoined := o.AllowThreeJoinedSupportKnotCells && len(chainIDs) == 3 && withStoredKnots == 0 &&
		min(knotsA, knotsB) >= 2 && min(knotsA, knotsB) <= 3 && max(knotsA, knotsB) == 4
	asymmetricJoined := threeJoined && min(knotsA, knotsB) == 2
	joinedCandidate := threeJoined || o.AllowJoinedSupportKnotCells && len(chainIDs) == 2 && withStoredKnots == 0 && knotsA == 2 && knotsB == 2
	expectedPerEdge := maxCrossings
	if cellLocal {
		maxCrossings, expectedPerEdge = localCells+2, 2
	}
	for i, id := range chainIDs {
		e := sourceEdges[i]
		var err error
		if g.foreign && len(chain) == 0 {
			var forms [2]bool
			for owner, p := range e.pcurves {
				_, forms[owner] = component(g.file.Entities[p.curve], "TRIMMED_CURVE")
			}
			if o.AllowMixedOwnerBases && forms[0] != forms[1] {
				foreignTrimmed, foreignMixed = forms, true
			} else if allowUnsplit && forms[0] {
				foreignTrimmed = [2]bool{true, true}
			}
		}
		var edgeBezierRepairs mappedChainBezierRepairs
		mixedOriginal := foreignMixed || canonicalOriginal
		if mixedOriginal {
			edgeBezierRepairs = bezierRepairs
		}
		allowJoined := joinedCandidate && mixedOriginal
		if edgeBezierRepairs != 0 && len(e.joins) != 0 {
			return zero, fmt.Errorf("inferred support repair requires an original model Bézier without a stored knot: %w", geom.ErrConvergence)
		}
		if (foreignMixed || canonicalOriginal || cellLocal || separatedCandidate) && len(e.joins) != 0 &&
			(!(allowThree || v8Cuts != 0 || cellLocal || separatedCandidate) || len(e.joins) != 1) {
			return zero, geom.ErrConvergence
		}
		if g.foreign && foreignTrimmed == [2]bool{} {
			if len(stations) == 0 {
				stations = append(stations, new(big.Rat))
			}
			stations = append(stations, big.NewRat(int64(len(stations)), int64(n)))
		} else {
			if len(stations) == 0 {
				stations = append(stations, e.lo)
			}
			if stations[len(stations)-1].Cmp(e.lo) != 0 {
				return zero, geom.ErrInvalid
			}
			stations = append(stations, e.hi)
		}
		reverse := !g.uses[id][0].sense
		row := MappedChainSTEPCell{EdgeID: id, ModelCurveID: e.modelID}
		var mapped [2]trimmed.MappedSplineUV
		var ownerCuts [2][]*big.Rat
		for owner, p := range e.pcurves {
			var source trimmed.SplineUV
			_, trimmedPCurve := component(g.file.Entities[p.curve], "TRIMMED_CURVE")
			if (allowUnsplit || foreignMixed) && trimmedPCurve != foreignTrimmed[owner] {
				return zero, geom.ErrConvergence
			}
			if g.foreign && !foreignTrimmed[owner] {
				source, err = g.foreignRoofSource(e, owner, reverse)
				row.Owners[owner] = MappedChainSTEPCoedge{FaceID: g.faces[owner].id, PCurveID: p.id, BasisID: p.curve}
			} else {
				basis, span, reversed, err := mappedChainStepRawUse(g.file, p.curve)
				if err != nil {
					return zero, geom.ErrInvalid
				}
				spline, err := g.ownerBasis(basis)
				if err != nil {
					return zero, err
				}
				source = trimmed.SplineUV{Curve: spline, Range: span, Reversed: reversed != reverse}
				row.Owners[owner] = MappedChainSTEPCoedge{g.faces[owner].id, p.id, p.curve, basis}
			}
			if err != nil {
				return zero, fmt.Errorf("mapped STEP edge #%d owner #%d: %w", id, g.faces[owner].id, err)
			}
			if threeJoined && mixedOriginal {
				model, modelErr := mappedChainSTEPBezierInTraversal(e.model, reverse, o.Geometry)
				if modelErr != nil {
					return zero, modelErr
				}
				var station *big.Rat
				mapped[owner], station, err = trimmed.ProveMappedSplineJoinedSupportTransfer(ctx, g.faces[owner].surface, source, model, o.Geometry)
				if station != nil {
					ownerCuts[owner] = []*big.Rat{station}
				}
			} else {
				mapped[owner], ownerCuts[owner], err = proveMappedChainSTEPTransfer(ctx, g.faces[owner].surface, source, e, reverse,
					allowAsync && !foreignMixed || separatedCandidate || allowJoined, cellLocal || separatedCandidate || allowJoined,
					edgeBezierRepairs, maxInferredSupportKnots, o.Geometry)
			}
			if err != nil {
				return zero, fmt.Errorf("mapped STEP edge #%d owner #%d: %w", id, g.faces[owner].id, err)
			}
			parts[owner] = append(parts[owner], mapped[owner])
		}
		interval := [2]*big.Rat{stations[len(stations)-2], stations[len(stations)-1]}
		edgeCrossings, err := g.edgeCrossings(ctx, id, e, reverse, interval, mapped, ownerCuts, edgeBezierRepairs, allowThree, allowJoined, v8Cuts, o.Geometry)
		if err != nil {
			return zero, fmt.Errorf("mapped STEP edge #%d: %w", id, err)
		}
		if len(crossings)+len(edgeCrossings) > maxCrossings {
			return zero, geom.ErrConvergence
		}
		allowedSingleton := localCells == 3 && len(edgeCrossings) == 1
		allowedSeparatedPair := separatedCandidate && len(edgeCrossings) == 2
		if (foreignMixed || canonicalOriginal || cellLocal || separatedCandidate) && len(e.joins) != 0 &&
			len(edgeCrossings) != expectedPerEdge && !allowedSingleton && !allowedSeparatedPair {
			return zero, fmt.Errorf("each original model edge requires a complete %d-cut repair: %w", expectedPerEdge, geom.ErrConvergence)
		}
		crossings = append(crossings, edgeCrossings...)
		chain = append(chain, row)
	}
	if cellLocal && mappedChainCrossingLocalCellCount(crossings) != localCells {
		return zero, fmt.Errorf("original edges require the complete local support event partition: %w", geom.ErrConvergence)
	}
	separated := separatedCandidate && mappedChainSeparatedSupportCells(chain, crossings)
	if separatedCandidate && !separated && !(allowThree && len(crossings) == 3 && allMappedChainCrossingsOnOneEdge(crossings)) {
		if len(crossings) < 3 {
			return zero, fmt.Errorf("required separated support event is missing: %w", geom.ErrConvergence)
		}
		return zero, fmt.Errorf("separated support events require opposite owners and an intervening original edge without a cut: %w", geom.ErrConvergence)
	}
	independent, err := mappedChainSTEPBasisModes(chain)
	if err != nil {
		return zero, err
	}
	if g.foreign {
		if independent != [2]bool{!foreignTrimmed[0], !foreignTrimmed[1]} {
			return zero, geom.ErrInvalid
		}
	} else if !o.allowsOwnerBases(independent) ||
		len(crossings) != 0 && independent[0] == independent[1] || canonicalReversed && len(crossings) == 0 {
		return zero, geom.ErrInvalid
	}
	end := stations[len(stations)-1]
	if stations[0].Sign() != 0 {
		return zero, geom.ErrInvalid
	}
	if foreignMixed || (!g.foreign && independent != [2]bool{}) {
		if !end.IsInt() || end.Sign() <= 0 || end.Num().BitLen() > 53 {
			return zero, geom.ErrInvalid
		}
		for i, station := range stations {
			if !station.IsInt() {
				return zero, geom.ErrInvalid
			}
			stations[i] = new(big.Rat).Quo(station, end)
		}
		for i := range crossings {
			crossing := &crossings[i]
			crossing.ModelStation.Quo(crossing.ModelStation, end)
		}
	} else if end.Cmp(big.NewRat(1, 1)) != 0 {
		return zero, geom.ErrInvalid
	}
	var seams [][2]trimmed.MappedChainClosedUse
	for _, id := range g.file.Order {
		owners, ok := g.uses[id]
		if !ok || slices.Contains(chainIDs, id) {
			continue
		}
		e, err := g.edge(ctx, id, o.Geometry, 1)
		if err != nil {
			return zero, fmt.Errorf("mapped STEP edge #%d: %w", id, err)
		}
		var pair [2]trimmed.MappedChainClosedUse
		for j, use := range owners {
			f := g.faces[use.face]
			var c trimmed.PCurve
			if g.foreign {
				c, err = g.proveForeignClosingUse(ctx, f, e, j, !use.sense, o.Geometry)
			} else {
				c, err = g.closingUse(ctx, f.surface, e, j, !use.sense, o.Geometry)
			}
			if err != nil {
				return zero, fmt.Errorf("mapped STEP closing edge #%d owner #%d: %w", id, f.id, err)
			}
			trim := use.trim
			if use.face < 2 {
				trim -= n
				closing[use.face][trim] = c
			} else {
				planar[use.face-2].Outer[trim] = c
			}
			pair[j] = trimmed.MappedChainClosedUse{Face: use.face, Trim: trim}
		}
		seams = append(seams, pair)
	}
	if len(g.pcurves) != len(g.uses)*2 {
		return zero, geom.ErrInvalid
	}
	if independent != [2]bool{} {
		for _, cell := range chain {
			for _, u := range cell.Owners {
				if u.TrimmedCurveID != 0 && g.curves[u.BasisID] {
					return zero, geom.ErrInvalid
				}
			}
		}
	}
	for _, face := range g.faces {
		for i, a := range face.uses {
			b := face.uses[(i+1)%len(face.uses)]
			end, start := 1, 0
			if !a.sense {
				end = 0
			}
			if !b.sense {
				start = 1
			}
			if g.edgeEnds[a.id][end] != g.edgeEnds[b.id][start] {
				return zero, fmt.Errorf("mapped STEP face #%d vertex loop: %w", face.id, geom.ErrInvalid)
			}
		}
	}
	var chains [2]trimmed.MappedSplineChain
	for i, source := range parts {
		makeChain := trimmed.NewMappedSplineChain
		if independent[i] {
			makeChain = trimmed.NewMappedSplineEdgeChain
		}
		q, err := makeChain(ctx, source, stations[1:len(stations)-1], o.Geometry)
		if err != nil {
			return zero, err
		}
		chains[i] = q
	}
	budget := edgecheck.Options{MaxWork: o.Geometry.MaxOperations, MaxBits: o.Geometry.MaxCoefficientBits}
	joined := joinedCandidate && (foreignMixed || canonicalOriginal) && mappedChainJoinedSupportCells(chain, crossings)
	if threeJoined && (foreignMixed || canonicalOriginal) && !joined {
		return zero, fmt.Errorf("three joined source cells require one owner-only interior event per original edge: %w", geom.ErrConvergence)
	}
	certify := trimmed.CertifyMappedSplineChainAgreement
	switch {
	case joined:
		certify = trimmed.CertifyMappedSplineChainJoinedSupportKnotsAgreement
		if asymmetricJoined {
			certify = trimmed.CertifyMappedSplineChainAsymmetricJoinedSupportKnotsAgreement
		}
	case len(crossings) == 1:
		if crossings[0].OwnersChangingCells != [2]bool{true, true} {
			return zero, fmt.Errorf("asynchronous repair requires a crossing on each original roof: %w", geom.ErrConvergence)
		}
		certify = trimmed.CertifyMappedSplineChainSupportKnotAgreement
	case len(crossings) == 2:
		a, b := crossings[0].OwnersChangingCells, crossings[1].OwnersChangingCells
		if crossings[0].EdgeID == crossings[1].EdgeID {
			shared := [2]bool{true, true}
			switch {
			case bezierRepairs&mappedChainBezierAsynchronous != 0 && mappedChainSameCellAsynchronousCrossings(crossings):
				certify = trimmed.CertifyMappedSplineChainSameCellAsynchronousSupportKnotsAgreement
			case a == shared && b == shared:
				certify = trimmed.CertifyMappedSplineChainTwoSupportKnotsAgreement
			case allowMixed && (a == shared && b[0] != b[1] || b == shared && a[0] != a[1]):
				certify = trimmed.CertifyMappedSplineChainMixedSupportKnotsAgreement
			default:
				return zero, geom.ErrConvergence
			}
		} else {
			if bezierRepairs != 0 && !allowAsync {
				return zero, fmt.Errorf("inferred support repair requires one original model Bézier: %w", geom.ErrConvergence)
			}
			if a[0] == a[1] || b[0] == b[1] || a == b {
				return zero, fmt.Errorf("asynchronous repair requires distinct edges and one crossing for each owner: %w", geom.ErrConvergence)
			}
			certify = trimmed.CertifyMappedSplineChainAsynchronousSupportKnotsAgreement
		}
	case len(crossings) == 3:
		if separated {
			certify = trimmed.CertifyMappedSplineChainCellLocalSupportKnotsAgreement
			break
		}
		if !allowThree || !allMappedChainCrossingsOnOneEdge(crossings) {
			return zero, fmt.Errorf("three support cuts must belong to one original foreign edge: %w", geom.ErrConvergence)
		}
		certify = trimmed.CertifyMappedSplineChainThreeSupportKnotsAgreement
	case len(crossings) >= 4 && len(crossings) <= 6:
		if cellLocal {
			certify = trimmed.CertifyMappedSplineChainCellLocalSupportKnotsAgreement
			break
		}
		if len(crossings) != v8Cuts || !allMappedChainCrossingsOnOneEdge(crossings) {
			return zero, fmt.Errorf("all support cuts must belong to one original edge: %w", geom.ErrConvergence)
		}
		certify = trimmed.CertifyMappedSplineChainMultipleSupportKnotsAgreement
	}
	agreement, err := certify(ctx, g.faces[0].surface, chains[0], g.faces[1].surface, chains[1], budget)
	if err != nil {
		return zero, fmt.Errorf("mapped source chain agreement: %w", err)
	}
	roofOrientation := [2]bool{g.faces[0].reversed, g.faces[1].reversed}
	shell, err := trimmed.CertifyMappedSplineChainShell(ctx, agreement, closing, roofOrientation, planar, seams, trimmed.MappedChainShellProofOptions{Closing: budget})
	if err != nil {
		return zero, err
	}
	native, err := shell.BuildRectangularSolid(ctx, o.Tolerance, o.Geometry)
	if err != nil {
		return zero, fmt.Errorf("mapped source solid: %w", err)
	}
	if len(native.ChainEdges) != len(chain)+len(crossings) {
		return zero, geom.ErrConvergence
	}
	for i := range crossings {
		crossing := &crossings[i]
		index := slices.IndexFunc(native.ChainStations, func(t *big.Rat) bool { return t.Cmp(crossing.ModelStation) == 0 })
		if index < 1 || index >= len(native.ChainEdges) {
			return zero, geom.ErrConvergence
		}
		crossing.NativeEdges = [2]int{native.ChainEdges[index-1], native.ChainEdges[index]}
	}
	out := MappedChainSTEPModel{Name: g.name, LengthUnit: g.unit, Shell: shell, Native: native, Chain: chain, Crossings: crossings}
	if len(crossings) == 1 {
		out.Crossing = &out.Crossings[0]
	}
	if joined {
		jointChanges := agreement.OriginalJointSupportChanges()
		for i := 1; i < len(chain); i++ {
			joint, err := g.sourceSupportJoint([2]mappedChainStepEdge(sourceEdges[i-1:i+1]), [2]MappedChainSTEPCell(chain[i-1:i+1]),
				[2][]trimmed.MappedSplineUV{parts[0][i-1 : i+1], parts[1][i-1 : i+1]}, stations[i],
				[2]int{crossings[i-1].NativeEdges[1], crossings[i].NativeEdges[0]})
			if err != nil {
				return zero, err
			}
			joint.OwnersChangingCells = jointChanges[i-1]
			out.SupportJoints = append(out.SupportJoints, *joint)
		}
		if len(out.SupportJoints) == 1 {
			out.SupportJoint = &out.SupportJoints[0]
		}
	}
	for _, f := range g.faces {
		out.FaceIDs = append(out.FaceIDs, f.id)
		if f.plane != nil {
			out.Planes = append(out.Planes, f.plane.evidence(f.id, f.supportID))
		}
	}
	return out, nil
}

func mappedChainJoinedSupportCells(chain []MappedChainSTEPCell, crossings []MappedChainSTEPSupportCrossing) bool {
	if len(chain) < 2 || len(chain) > 3 || len(crossings) != len(chain) {
		return false
	}
	var changed [2]bool
	for i, cut := range crossings {
		owners := cut.OwnersChangingCells
		if cut.EdgeID != chain[i].EdgeID || cut.StoredModelKnot || owners[0] == owners[1] {
			return false
		}
		changed[0], changed[1] = changed[0] || owners[0], changed[1] || owners[1]
	}
	return changed == [2]bool{true, true}
}

func (g *mappedChainStepSource) sourceSupportJoint(edges [2]mappedChainStepEdge, chain [2]MappedChainSTEPCell, parts [2][]trimmed.MappedSplineUV, station *big.Rat, native [2]int) (*MappedChainSTEPJointSupportCrossing, error) {
	ids := [2]int64{chain[0].EdgeID, chain[1].EdgeID}
	ends := [2]int{1, 0}
	for i, id := range ids {
		if !g.uses[id][0].sense {
			ends[i] = 1 - ends[i]
		}
	}
	vertex := g.edgeEnds[ids[0]][ends[0]]
	if vertex != g.edgeEnds[ids[1]][ends[1]] {
		return nil, geom.ErrConvergence
	}
	out := &MappedChainSTEPJointSupportCrossing{
		SourceVertexID: vertex, SourceEdges: ids, ModelStation: new(big.Rat).Set(station), NativeEdges: native,
	}
	for i, edge := range edges {
		out.SourceModelParameters[i] = new(big.Rat).Set([2]*big.Rat{edge.lo, edge.hi}[ends[i]])
	}
	for owner, row := range parts {
		for part := range 2 {
			raw, err := row[part].AuthoredParameter(big.NewRat(int64(1-part), 1))
			if err != nil {
				return nil, err
			}
			out.OwnerParameters[owner][part] = raw
		}
	}
	return out, nil
}

func allMappedChainCrossingsOnOneEdge(crossings []MappedChainSTEPSupportCrossing) bool {
	if len(crossings) == 0 {
		return false
	}
	for _, cut := range crossings[1:] {
		if cut.EdgeID != crossings[0].EdgeID {
			return false
		}
	}
	return true
}

func mappedChainInferredSharedCrossings(crossings []MappedChainSTEPSupportCrossing) bool {
	if len(crossings) == 0 || len(crossings) > 2 {
		return false
	}
	for _, cut := range crossings {
		if cut.EdgeID != crossings[0].EdgeID || cut.StoredModelKnot || cut.OwnersChangingCells != [2]bool{true, true} {
			return false
		}
	}
	return true
}

func mappedChainSameCellAsynchronousCrossings(crossings []MappedChainSTEPSupportCrossing) bool {
	if len(crossings) != 2 {
		return false
	}
	a, b := crossings[0], crossings[1]
	return a.EdgeID == b.EdgeID && !a.StoredModelKnot && !b.StoredModelKnot &&
		a.OwnersChangingCells[0] != a.OwnersChangingCells[1] &&
		b.OwnersChangingCells == [2]bool{!a.OwnersChangingCells[0], !a.OwnersChangingCells[1]}
}

func mappedChainSeparatedSupportCells(chain []MappedChainSTEPCell, crossings []MappedChainSTEPSupportCrossing) bool {
	if len(crossings) != 3 {
		return false
	}
	pair, single := crossings[:2], crossings[2]
	if pair[0].EdgeID != pair[1].EdgeID {
		pair, single = crossings[1:], crossings[0]
	}
	a, b := pair[0], pair[1]
	if a.EdgeID != b.EdgeID || a.EdgeID == single.EdgeID || single.StoredModelKnot || a.StoredModelKnot == b.StoredModelKnot {
		return false
	}
	if !a.StoredModelKnot {
		a, b = b, a
	}
	if a.OwnersChangingCells != [2]bool{true, true} || b.OwnersChangingCells[0] == b.OwnersChangingCells[1] ||
		single.OwnersChangingCells != [2]bool{!b.OwnersChangingCells[0], !b.OwnersChangingCells[1]} {
		return false
	}
	i := slices.IndexFunc(chain, func(e MappedChainSTEPCell) bool { return e.EdgeID == a.EdgeID })
	j := slices.IndexFunc(chain, func(e MappedChainSTEPCell) bool { return e.EdgeID == single.EdgeID })
	return i >= 0 && j >= 0 && max(i, j)-min(i, j) >= 2
}

func mappedChainCrossingLocalCellCount(crossings []MappedChainSTEPSupportCrossing) int {
	if len(crossings) != 4 && len(crossings) != 5 {
		return 0
	}
	seen := make(map[int64]bool, 3)
	var singleOwner [2]bool
	pairs, singles := 0, 0
	for i := 0; i < len(crossings); {
		edge := crossings[i].EdgeID
		if seen[edge] {
			return 0
		}
		seen[edge] = true
		j, stored := i, 0
		for j < len(crossings) && crossings[j].EdgeID == edge {
			cut := crossings[j]
			if cut.StoredModelKnot {
				if cut.OwnersChangingCells != [2]bool{true, true} {
					return 0
				}
				stored++
			} else {
				if cut.OwnersChangingCells[0] == cut.OwnersChangingCells[1] {
					return 0
				}
				for owner, changes := range cut.OwnersChangingCells {
					singleOwner[owner] = singleOwner[owner] || changes
				}
			}
			j++
		}
		if stored != 1 || j-i > 2 {
			return 0
		}
		if j-i == 2 {
			pairs++
		} else {
			singles++
		}
		i = j
	}
	if pairs != 2 || singles != len(crossings)-4 || singleOwner != [2]bool{true, true} {
		return 0
	}
	return pairs + singles
}

func mappedChainSTEPBasisModes(chain []MappedChainSTEPCell) ([2]bool, error) {
	var independent [2]bool
	seen := make(map[int64]bool, 2*len(chain))
	for owner := range independent {
		distinct := make(map[int64]bool, len(chain))
		for _, edge := range chain {
			id := edge.Owners[owner].BasisID
			if distinct[id] {
				continue
			}
			if seen[id] {
				return [2]bool{}, geom.ErrInvalid
			}
			seen[id], distinct[id] = true, true
		}
		if len(distinct) != 1 && len(distinct) != len(chain) {
			return [2]bool{}, geom.ErrInvalid
		}
		independent[owner] = len(distinct) == len(chain)
	}
	return independent, nil
}

func (g *mappedChainStepSource) ownerBasis(id int64) (*geom.BSpline, error) {
	if c := g.bases[id]; c != nil {
		return c, nil
	}
	s, err := readMappedChainStepSpline(g.file, id, 2, 4, 64, g.foreign)
	if err != nil || s.degree < 3 {
		return nil, geom.ErrInvalid
	}
	c, err := s.ownerBasis()
	if err != nil {
		return nil, err
	}
	g.bases[id] = c
	return c, nil
}

type mappedChainStepEdge struct {
	modelID int64
	model   *geom.ExactSplineCurve
	lo, hi  *big.Rat
	pcurves [2]struct{ id, curve int64 }
	points  [2][3]*big.Rat
	pieces  []*geom.ExactSplineCurve
	joins   []*big.Rat
}

func (g *mappedChainStepSource) edge(ctx context.Context, id int64, o geom.SurfacePatchOptions, maxModelPieces int) (mappedChainStepEdge, error) {
	var zero mappedChainStepEdge
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	owners := g.uses[id]
	if len(owners) != 2 || owners[0].face == owners[1].face ||
		(owners[0].sense == g.faces[owners[0].face].reversed) == (owners[1].sense == g.faces[owners[1].face].reversed) {
		return zero, geom.ErrInvalid
	}
	start, end, sid, err := analyticEdge(g.file, id)
	if err != nil || start == end {
		return zero, geom.ErrInvalid
	}
	g.edgeEnds[id] = [2]int64{start, end}
	sc, err := simple(g.file, sid, "SURFACE_CURVE", 4)
	if err != nil || sc.Args[0].Kind != String || sc.Args[3].Kind != Enumeration ||
		(sc.Args[3].Text != "CURVE_3D" && (!g.foreign || sc.Args[3].Text != "PCURVE_S1" && sc.Args[3].Text != "PCURVE_S2")) {
		return zero, geom.ErrInvalid
	}
	mid, err := ref(sc.Args[1])
	if err != nil || g.curves[mid] {
		return zero, geom.ErrInvalid
	}
	g.curves[mid] = true
	out := mappedChainStepEdge{modelID: mid}
	for i, v := range []int64{start, end} {
		x, err := mappedChainStepVertex(g.file, v)
		if err != nil {
			return zero, err
		}
		out.points[i] = x
		g.vertices[v] = true
	}
	if _, line := component(g.file.Entities[mid], "LINE"); g.foreign && line {
		out.model, err = g.proveForeignModelLine(ctx, mid, out.points, o)
		out.lo, out.hi = new(big.Rat), big.NewRat(1, 1)
	} else {
		var spline mappedChainStepSpline
		spline, err = readMappedChainStepSpline(g.file, mid, 3, 16, 16*maxModelPieces+1, g.foreign)
		if err != nil {
			return zero, err
		}
		out.lo, out.hi = spline.knots[0], spline.knots[len(spline.knots)-1]
		for i, x := range out.points {
			point := spline.poles[i*(len(spline.poles)-1)]
			for axis, q := range x {
				if q.Cmp(point[axis]) != 0 {
					return zero, geom.ErrConvergence
				}
			}
		}
		if maxModelPieces > 1 && len(spline.poles) != spline.degree+1 {
			out.pieces, out.joins, err = spline.exactBezierPieces(ctx, o, maxModelPieces)
		} else {
			out.model, err = spline.exactBezier(ctx, o)
		}
	}
	if err != nil {
		return zero, err
	}
	ids, err := listRefs(sc.Args[2], 2)
	if err != nil || len(ids) != 2 {
		return zero, geom.ErrInvalid
	}
	for _, pc := range ids {
		sid, curve, err := mappedChainStepPCurve(g.file, pc)
		if err != nil || g.pcurves[pc] || g.curves[curve] {
			return zero, geom.ErrInvalid
		}
		g.pcurves[pc], g.curves[curve] = true, true
		found := false
		for i, owner := range owners {
			if g.faces[owner.face].supportID == sid && out.pcurves[i].id == 0 {
				out.pcurves[i].id, out.pcurves[i].curve = pc, curve
				found = true
			}
		}
		if !found {
			return zero, geom.ErrInvalid
		}
	}
	return out, nil
}

func (g *mappedChainStepSource) closingUse(ctx context.Context, surface *geom.BSplineSurface, edge mappedChainStepEdge, i int, reverse bool, o geom.SurfacePatchOptions) (trimmed.PCurve, error) {
	input, err := readMappedChainStepSpline(g.file, edge.pcurves[i].curve, 2, 4, 5, g.foreign)
	if err != nil || edge.lo.Sign() != 0 || edge.hi.Cmp(big.NewRat(1, 1)) != 0 {
		return nil, geom.ErrInvalid
	}
	if input.knots[0].Sign() != 0 || input.knots[len(input.knots)-1].Cmp(big.NewRat(1, 1)) != 0 {
		return nil, geom.ErrInvalid
	}
	h, err := input.bezierControls()
	if err != nil {
		return nil, err
	}
	controls := make([][3]*big.Rat, len(h))
	for i, p := range h {
		controls[i] = [3]*big.Rat{p[0], p[1], p[3]}
	}
	if reverse {
		slices.Reverse(controls)
	}
	var curve trimmed.PCurve
	if len(controls) == 3 {
		net := [3][3]*big.Rat(controls)
		curve, err = trimmed.NewExactQuadraticUV(net)
		if errors.Is(err, geom.ErrUnrepresentable) {
			curve, err = trimmed.NewRationalEndpointQuadraticUV(net)
		}
	} else {
		curve, err = trimmed.NewExactBezierUV(controls)
	}
	if err != nil {
		return nil, err
	}
	model, err := trimmed.ExactTrimModelBezier(ctx, surface, curve, reverse, o)
	if err != nil {
		return nil, err
	}
	equal, err := geom.CompareExactCurvesContext(ctx, model, edge.model, geom.ExactCurveComparisonOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if err != nil {
		return nil, err
	}
	if !equal.Complete || !equal.Equal {
		return nil, geom.ErrConvergence
	}
	return curve, nil
}
