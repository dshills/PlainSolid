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

// MappedChainSTEPSupportCrossing relates one unsplit original STEP edge to
// its two certified native edges. RawParameters follow the original owners.
type MappedChainSTEPSupportCrossing struct {
	EdgeID               int64
	SourceModelParameter *big.Rat
	ModelStation         *big.Rat
	RawParameters        [2]*big.Rat
	NativeEdges          [2]int
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
	Crossing   *MappedChainSTEPSupportCrossing
	original   string
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
func ReadMappedChainRectangularShell(ctx context.Context, r io.Reader, l Limits, o MappedChainShellSTEPOptions) (MappedChainSTEPModel, error) {
	return readMappedChainRectangularShell(ctx, r, l, o, false)
}

// ReadOCCTMappedChainRectangularShell authenticates an independent seven-face
// AP242 shell with two original spline roof charts and five signed-axis
// analytic planes. It discovers the two roofs and the cyclic shared chain.
// Each direct original roof pcurve retains its individual source spline basis.
// The opt-in unsplit route also admits both roof chains as parameter-valued
// restrictions on their original complete bases.
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
	source, err := openMappedChainSTEPSource(f, l, foreign, maxRoofKnots)
	if err != nil {
		return zero, err
	}
	out, err := source.prove(ctx, o)
	if err != nil {
		return zero, err
	}
	sourceVertices := len(source.vertices)
	if out.Crossing != nil {
		sourceVertices++
	}
	if sourceVertices > l.MaxVertices || sourceVertices != len(out.Native.Solid.Vertices()) ||
		2*len(out.Native.Solid.Edges()) > l.MaxEdgeUses {
		return zero, geom.ErrConvergence
	}
	out.original = string(raw)
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
	var chainIDs []int64
	for i, use := range g.faces[0].uses {
		owners := g.uses[use.id]
		if len(owners) != 2 {
			return zero, geom.ErrInvalid
		}
		if owners[1].face != 1 {
			continue
		}
		if len(chainIDs) != i || owners[1].trim != i || use.sense != owners[1].sense || (!g.foreign && !use.sense) {
			return zero, geom.ErrInvalid
		}
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
	var basisIDs [2]int64
	var chain []MappedChainSTEPCell
	var stations []*big.Rat
	var crossing *MappedChainSTEPSupportCrossing
	var foreignTrimmed bool
	allowUnsplit := g.foreign && o.AllowUnsplitSupportKnot
	for _, id := range chainIDs {
		e, err := g.edge(ctx, id, o.Geometry, allowUnsplit)
		if err != nil {
			return zero, fmt.Errorf("mapped STEP edge #%d: %w", id, err)
		}
		if allowUnsplit && len(chain) == 0 {
			_, foreignTrimmed = component(g.file.Entities[e.pcurves[0].curve], "TRIMMED_CURVE")
		}
		if g.foreign && !foreignTrimmed {
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
		var localCrossing *big.Rat
		if e.join != nil {
			if crossing != nil {
				return zero, geom.ErrConvergence
			}
			localCrossing = e.join
			if !g.uses[id][0].sense {
				localCrossing = new(big.Rat).Sub(big.NewRat(1, 1), localCrossing)
			}
			a, b := stations[len(stations)-2], stations[len(stations)-1]
			global := new(big.Rat).Add(a, new(big.Rat).Mul(localCrossing, new(big.Rat).Sub(b, a)))
			original := new(big.Rat).Add(e.lo, new(big.Rat).Mul(e.join, new(big.Rat).Sub(e.hi, e.lo)))
			crossing = &MappedChainSTEPSupportCrossing{EdgeID: id, SourceModelParameter: original, ModelStation: global}
		}
		row := MappedChainSTEPCell{EdgeID: id, ModelCurveID: e.modelID}
		for owner, p := range e.pcurves {
			var q trimmed.MappedSplineUV
			_, trimmedPCurve := component(g.file.Entities[p.curve], "TRIMMED_CURVE")
			if allowUnsplit && trimmedPCurve != foreignTrimmed {
				return zero, geom.ErrConvergence
			}
			if g.foreign && !foreignTrimmed {
				q, err = g.proveForeignRoofUse(ctx, id, e, owner, o.Geometry)
				row.Owners[owner] = MappedChainSTEPCoedge{FaceID: g.faces[owner].id, PCurveID: p.id, BasisID: p.curve}
			} else {
				basis, span, reversed, err := mappedChainStepRawUse(g.file, p.curve)
				if err != nil || (basisIDs[owner] != 0 && basis != basisIDs[owner]) {
					return zero, geom.ErrInvalid
				}
				basisIDs[owner] = basis
				spline, err := g.ownerBasis(basis)
				if err != nil {
					return zero, err
				}
				reverseModel := !g.uses[id][owner].sense
				source := trimmed.SplineUV{Curve: spline, Range: span, Reversed: reversed != reverseModel}
				q, err = proveMappedChainSTEPTransfer(ctx, g.faces[owner].surface, source, e, reverseModel, o.Geometry)
				if err != nil {
					return zero, fmt.Errorf("mapped STEP edge #%d owner #%d: %w", id, g.faces[owner].id, err)
				}
				row.Owners[owner] = MappedChainSTEPCoedge{g.faces[owner].id, p.id, p.curve, basis}
			}
			if err != nil {
				return zero, fmt.Errorf("mapped STEP edge #%d owner #%d: %w", id, g.faces[owner].id, err)
			}
			if localCrossing != nil {
				crossing.RawParameters[owner], err = q.AuthoredParameter(localCrossing)
				if err != nil {
					return zero, err
				}
			}
			parts[owner] = append(parts[owner], q)
		}
		chain = append(chain, row)
	}
	if stations[0].Sign() != 0 || stations[len(stations)-1].Cmp(big.NewRat(1, 1)) != 0 ||
		((!g.foreign || foreignTrimmed) && basisIDs[0] == basisIDs[1]) {
		return zero, geom.ErrInvalid
	}
	var seams [][2]trimmed.MappedChainClosedUse
	for _, id := range g.file.Order {
		owners, ok := g.uses[id]
		if !ok || slices.Contains(chainIDs, id) {
			continue
		}
		e, err := g.edge(ctx, id, o.Geometry, false)
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
	makeChain := trimmed.NewMappedSplineChain
	if g.foreign && !foreignTrimmed {
		makeChain = trimmed.NewMappedSplineEdgeChain
	}
	for i, source := range parts {
		q, err := makeChain(ctx, source, stations[1:len(stations)-1], o.Geometry)
		if err != nil {
			return zero, err
		}
		chains[i] = q
	}
	budget := edgecheck.Options{MaxWork: o.Geometry.MaxOperations, MaxBits: o.Geometry.MaxCoefficientBits}
	certify := trimmed.CertifyMappedSplineChainAgreement
	if crossing != nil {
		certify = trimmed.CertifyMappedSplineChainSupportKnotAgreement
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
	if crossing != nil {
		i := slices.IndexFunc(native.ChainStations, func(t *big.Rat) bool { return t.Cmp(crossing.ModelStation) == 0 })
		if i < 1 || i >= len(native.ChainEdges) || len(native.ChainEdges) != len(chain)+1 {
			return zero, geom.ErrConvergence
		}
		crossing.NativeEdges = [2]int{native.ChainEdges[i-1], native.ChainEdges[i]}
	}
	out := MappedChainSTEPModel{Name: g.name, LengthUnit: g.unit, Shell: shell, Native: native, Chain: chain, Crossing: crossing}
	for _, f := range g.faces {
		out.FaceIDs = append(out.FaceIDs, f.id)
		if f.plane != nil {
			out.Planes = append(out.Planes, f.plane.evidence(f.id, f.supportID))
		}
	}
	return out, nil
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
	pieces  [2]*geom.ExactSplineCurve
	join    *big.Rat
}

func (g *mappedChainStepSource) edge(ctx context.Context, id int64, o geom.SurfacePatchOptions, allowUnsplit bool) (mappedChainStepEdge, error) {
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
		maxPoles := 17
		if allowUnsplit {
			maxPoles = 33
		}
		spline, err = readMappedChainStepSpline(g.file, mid, 3, 16, maxPoles, g.foreign)
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
		if allowUnsplit && len(spline.poles) != spline.degree+1 {
			out.pieces, out.join, err = spline.exactTwoBezier(ctx, o)
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
