package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// PeriodicTrimEdge retains one directed source UV line and its matching model
// spline. The exact model curve is on [0,1] in the original edge direction.
// ModelParameter retains the edge's original full type-126 parameter interval.
type PeriodicTrimEdge struct {
	UVSourceID, ModelSourceID int
	UVStart, UVEnd            [2]*big.Rat
	ModelParameter            geom.ExactParameterInterval
	ModelCurve                *geom.ExactSplineCurve
}

// PeriodicTrimmedFace is one oriented, injective, exact rectangular face on a
// type-128 support periodic in U, V, or both. Support uses physical
// file parameters. Patch maps the normalized unit square to the finite
// trim. All coordinates are in original file length units. Bounds and
// KnotOwners are in U,V order.
type PeriodicTrimmedFace struct {
	LengthUnit                      float64
	FaceID, SupportID, ContourID    int
	UVCompositeID, ModelCompositeID int
	Support                         *geom.ExactPeriodicSplineSurface
	Patch                           *geom.SurfacePatch
	Bounds                          [2]geom.ExactParameterInterval
	KnotOwners                      [2]geom.ExactSurfaceKnotSpan
	ProjectionAxes                  [2]int
	ProjectionOrientation           int
	Edges                           [4]PeriodicTrimEdge
	original                        string
}

// OriginalIGES returns the complete file before any geometric conversion.
func (f PeriodicTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicTrimmedFace proves one standalone unplaced type-144 face with
// an explicit outer type-142 contour, four ordered type-110 UV edges and
// four type-126 model edges. The CCW rectangle must be strictly inside one
// original knot cell of the cyclic support. Each UV/model correspondence
// is proved over the entire oriented edge under its normalized affine
// parameter in the original source orientation.
func ReadPeriodicTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicTrimmedFace, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 4, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicTrimmedFace{}, err
	}
	return provePeriodicTrimmedFace(ctx, source)
}

func provePeriodicTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicTrimmedFace, error) {
	var zero PeriodicTrimmedFace
	m := source.face
	cell, err := m.proveOriginalKnotCell(ctx)
	if err != nil {
		return zero, err
	}
	if _, err := m.proveEdges(ctx, source.graph, cell, source.uv, source.model, source.remaining); err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	if err := m.proveInjectivePatch(ctx, cell); err != nil {
		return zero, err
	}
	return m, nil
}

func (m *PeriodicTrimmedFace) proveInjectivePatch(ctx context.Context, cell geom.ExactPeriodicSurfaceBezierCell) error {
	var err error
	m.Patch, err = cell.NormalizedPatch(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		return err
	}
	for _, axes := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		if err := ctx.Err(); err != nil {
			return err
		}
		proof, err := geom.CertifySurfaceInjectivity(m.Patch, geom.SurfaceInjectivityOptions{
			ProjectionAxes: axes, MaxDepth: 6, MaxCells: 64, MaxOperations: 200000,
		})
		if err == nil && proof.ProvenInjective {
			if err := ctx.Err(); err != nil {
				return err
			}
			m.ProjectionAxes, m.ProjectionOrientation = axes, proof.ProjectionOrientation
			return nil
		}
	}
	return fmt.Errorf("iges: periodic trim projection is unproved: %w", geom.ErrUnrepresentable)
}

type periodicTrimSource struct {
	face      PeriodicTrimmedFace
	support   *Surface
	graph     periodicTrimGraph
	uv, model []int
	holes     []periodicTrimLoopSource
	remaining int
}

type periodicTrimAxisPolicy uint8

const (
	periodicTrimUOnly periodicTrimAxisPolicy = iota
	periodicTrimAnyAxes
)

func readPeriodicTrimSource(ctx context.Context, r io.Reader, l Limits, edgeCount int, holeLayouts ...[]int) (periodicTrimSource, error) {
	return readPeriodicTrimSourceForAxes(ctx, r, l, edgeCount, periodicTrimUOnly, holeLayouts...)
}

func readPeriodicTrimSourceForAxes(ctx context.Context, r io.Reader, l Limits, edgeCount int, axes periodicTrimAxisPolicy, holeLayouts ...[]int) (periodicTrimSource, error) {
	var zero periodicTrimSource
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	l, err := l.defaults()
	if err != nil {
		return zero, err
	}
	doc, err := readTopologyRecordPolicyWithDirectory(r, l, false, false, false, false, false, false, false,
		false, false, periodicTrimmedFaceDirectory)
	if err != nil {
		return zero, err
	}
	owners := 5 + 2*edgeCount
	var layout []int
	if len(holeLayouts) != 0 {
		for _, candidate := range holeLayouts {
			n := owners
			for _, edges := range candidate {
				n += 3 + 2*edges
			}
			if n == len(doc.entities) {
				layout = slices.Clone(candidate)
				owners = n
				break
			}
		}
		if layout == nil {
			return zero, fmt.Errorf("iges: invalid explicit periodic trim hole count or layout: %w", geom.ErrUnrepresentable)
		}
	}
	holeCount := len(layout)
	if len(doc.entities) != owners {
		return zero, fmt.Errorf("iges: expected one periodic trim with %d source owners: %w", owners, geom.ErrUnrepresentable)
	}
	g := periodicTrimGraph{records: make(map[int]topologyRecord, owners), claimed: make(map[int]bool, owners)}
	var m PeriodicTrimmedFace
	for _, q := range doc.entities {
		g.records[q.id] = q
		if q.kind == 144 {
			if m.FaceID != 0 {
				return zero, geom.ErrUnrepresentable
			}
			m.FaceID = q.id
		}
	}
	face, err := g.links(m.FaceID, 144, 5+holeCount)
	if err != nil {
		return zero, err
	}
	if face[2] != 1 || face[3] != holeCount {
		return zero, fmt.Errorf("iges: inconsistent explicit periodic trim loop count: %w", geom.ErrUnrepresentable)
	}
	m.SupportID, m.ContourID = face[1], face[4]
	outer, err := readPeriodicTrimLoop(g, m.SupportID, m.ContourID, edgeCount)
	if err != nil {
		return zero, err
	}
	m.UVCompositeID, m.ModelCompositeID = outer.uvID, outer.modelID
	holes := make([]periodicTrimLoopSource, holeCount)
	for i, id := range face[5:] {
		holes[i], err = readPeriodicTrimLoop(g, m.SupportID, id, 0)
		if err != nil {
			return zero, err
		}
		owner := slices.Index(layout, len(holes[i].uv))
		if owner < 0 {
			return zero, fmt.Errorf("iges: wrong periodic trim edge count on hole #%d: %w", id, geom.ErrUnrepresentable)
		}
		layout = slices.Delete(layout, owner, owner+1)
	}

	rec, err := g.take(m.SupportID, 128)
	if err != nil {
		return zero, err
	}
	var bounds [4]*big.Rat
	support, err := decodeSurfaceMode(rec.values, l.MaxControlPoints, true, &bounds)
	if err != nil {
		return zero, err
	}
	s := support.Surface
	uOnly := s.PeriodicU && !s.PeriodicV && !s.ClosedV
	vOnly := s.PeriodicV && !s.PeriodicU && !s.ClosedU
	both := s.PeriodicU && s.PeriodicV
	if !uOnly && !(axes == periodicTrimAnyAxes && (vOnly || both)) {
		expected := "one U-periodic support with an open opposite axis"
		if axes == periodicTrimAnyAxes {
			expected = "one U-periodic support or one V-periodic support with an open opposite axis, or a doubly periodic support"
		}
		return zero, fmt.Errorf("iges: periodic trim requires %s: %w", expected, geom.ErrUnrepresentable)
	}
	m.Support, err = support.Surface.ExactPeriodicGeometry()
	if err != nil {
		return zero, err
	}
	d := support.Surface.Geometry.Data()
	remaining := l.MaxControlPoints - d.CountU*d.CountV
	for i := range m.Bounds {
		m.Bounds[i] = geom.ExactParameterInterval{Lower: bounds[2*i], Upper: bounds[2*i+1]}
	}
	m.LengthUnit, m.original = doc.unit, string(doc.bytes)
	return periodicTrimSource{face: m, support: support.Surface, graph: g, uv: outer.uv, model: outer.model, holes: holes, remaining: remaining}, nil
}

type periodicTrimLoopSource struct {
	contourID, uvID, modelID int
	uv, model                []int
}

func readPeriodicTrimLoop(g periodicTrimGraph, supportID, contourID, edgeCount int) (periodicTrimLoopSource, error) {
	var zero periodicTrimLoopSource
	contour, err := g.links(contourID, 142, 6)
	if err != nil {
		return zero, err
	}
	if contour[1] != 0 || contour[2] != supportID || contour[5] != 3 {
		return zero, fmt.Errorf("iges: mismatched periodic trim contour support or mode: %w", geom.ErrUnrepresentable)
	}
	uv, err := readPeriodicTrimComposite(g, contour[3], edgeCount)
	if err != nil {
		return zero, err
	}
	model, err := readPeriodicTrimComposite(g, contour[4], len(uv))
	if err != nil {
		return zero, err
	}
	return periodicTrimLoopSource{contourID: contourID, uvID: contour[3], modelID: contour[4], uv: uv, model: model}, nil
}

func readPeriodicTrimComposite(g periodicTrimGraph, id, edges int) ([]int, error) {
	r, err := g.take(id, 102)
	if err != nil {
		return nil, err
	}
	if len(r.values) < 2 {
		return nil, geom.ErrUnrepresentable
	}
	n, err := intValue(r.values[1])
	if err != nil || n != 4 && n != 6 || edges != 0 && n != edges {
		return nil, fmt.Errorf("iges: wrong periodic trim edge count on composite #%d: %w", id, geom.ErrUnrepresentable)
	}
	values, err := periodicTrimLinkValues(r, 2+n)
	if err != nil {
		return nil, err
	}
	return values[2:], nil
}

func (m *PeriodicTrimmedFace) proveOriginalKnotCell(ctx context.Context) (geom.ExactPeriodicSurfaceBezierCell, error) {
	var zero geom.ExactPeriodicSurfaceBezierCell
	owners, err := periodicTrimKnotOwners(ctx, m.Support, m.Bounds)
	if err != nil {
		return zero, err
	}
	m.KnotOwners = owners
	for i, owner := range m.KnotOwners {
		if owner.Cycle != nil && owner.Cycle.Sign() != 0 {
			return zero, fmt.Errorf("iges: periodic trim lies outside its original fundamental cycle: %w", geom.ErrUnrepresentable)
		}
		r := m.Bounds[i]
		if r.Lower.Cmp(owner.Domain.Lower) <= 0 || r.Upper.Cmp(owner.Domain.Upper) >= 0 {
			return zero, fmt.Errorf("iges: periodic trim meets or crosses an original knot station: %w", geom.ErrUnrepresentable)
		}
	}
	cells, _, err := m.Support.ExactBezierCells(ctx, m.Bounds[0], m.Bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 1})
	if err != nil {
		return zero, err
	}
	if len(cells) != 1 {
		return zero, geom.ErrUnrepresentable
	}
	return cells[0], nil
}

func periodicTrimKnotOwners(ctx context.Context, support *geom.ExactPeriodicSplineSurface,
	bounds [2]geom.ExactParameterInterval) ([2]geom.ExactSurfaceKnotSpan, error) {
	var zero [2]geom.ExactSurfaceKnotSpan
	var middle [2]*big.Rat
	for i, r := range bounds {
		if r.Lower.Cmp(r.Upper) >= 0 {
			return zero, geom.ErrUnrepresentable
		}
		middle[i] = new(big.Rat).Mul(new(big.Rat).Add(r.Lower, r.Upper), big.NewRat(1, 2))
	}
	jet, err := support.EvaluateRational(ctx, middle[0], middle[1], geom.SurfacePatchOptions{})
	if err != nil {
		return zero, err
	}
	return [2]geom.ExactSurfaceKnotSpan{jet.U, jet.V}, nil
}

type periodicTrimGraph struct {
	records map[int]topologyRecord
	claimed map[int]bool
}

func (g periodicTrimGraph) take(id, kind int) (topologyRecord, error) {
	r, ok := g.records[id]
	if ok && !g.claimed[id] && r.kind == kind && len(r.values) > 0 {
		if tag, err := intValue(r.values[0]); err == nil && tag == kind {
			g.claimed[id] = true
			return r, nil
		}
	}
	return topologyRecord{}, fmt.Errorf("iges: missing, duplicated or wrong periodic trim owner #%d: %w", id, geom.ErrUnrepresentable)
}

func (g periodicTrimGraph) links(id, kind, count int) ([]int, error) {
	r, err := g.take(id, kind)
	if err != nil {
		return nil, err
	}
	return periodicTrimLinkValues(r, count)
}

func periodicTrimLinkValues(r topologyRecord, count int) ([]int, error) {
	if len(r.values) < count {
		return nil, geom.ErrUnrepresentable
	}
	if err := noPointers(r.values[count:]); err != nil {
		return nil, err
	}
	out := make([]int, count)
	for i := range out {
		n, err := intValue(r.values[i])
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func periodicTrimmedFaceDirectory(first, second [9]int) bool {
	if first[3] != 0 || first[4] != 0 || first[6] != 0 ||
		second[1] != 0 || second[2] != 0 || second[4] != 0 || second[8] < 0 || second[8] > 99999999 {
		return false
	}
	switch first[0] {
	case 144:
		return first[8] == 0
	case 142:
		return first[8] == 10500
	case 102, 110, 126, 128:
		return first[8] == 10000
	}
	return false
}
