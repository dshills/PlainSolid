package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

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
// U-periodic type-128 support. Support uses physical file parameters; Patch
// maps the normalized unit square to the finite trim. All coordinates are in
// original file length units. Bounds and KnotOwners are in U,V order.
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
// original knot cell. Each UV/model correspondence is proved over the
// entire oriented edge under its normalized affine parameter.
func ReadPeriodicTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicTrimmedFace, error) {
	var zero PeriodicTrimmedFace
	source, err := readPeriodicTrimSource(ctx, r, l, 4)
	if err != nil {
		return zero, err
	}
	m := source.face
	cell, err := m.proveOriginalKnotCell(ctx)
	if err != nil {
		return zero, err
	}
	if err := m.proveEdges(ctx, source.graph, cell, source.uv, source.model, source.remaining); err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	m.Patch, err = cell.NormalizedPatch(ctx, geom.SurfacePatchOptions{})
	if err != nil {
		return zero, err
	}
	for _, axes := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		proof, err := geom.CertifySurfaceInjectivity(m.Patch, geom.SurfaceInjectivityOptions{
			ProjectionAxes: axes, MaxDepth: 6, MaxCells: 64, MaxOperations: 200000,
		})
		if err == nil && proof.ProvenInjective {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			m.ProjectionAxes, m.ProjectionOrientation = axes, proof.ProjectionOrientation
			return m, nil
		}
	}
	return zero, fmt.Errorf("iges: periodic trim projection is unproved: %w", geom.ErrUnrepresentable)
}

type periodicTrimSource struct {
	face      PeriodicTrimmedFace
	graph     periodicTrimGraph
	uv, model []int
	remaining int
}

func readPeriodicTrimSource(ctx context.Context, r io.Reader, l Limits, edgeCount int) (periodicTrimSource, error) {
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
	face, err := g.links(m.FaceID, 144, 5)
	if err != nil {
		return zero, err
	}
	if face[2] != 1 || face[3] != 0 {
		return zero, fmt.Errorf("iges: expected one explicit periodic trim with no inner curves: %w", geom.ErrUnrepresentable)
	}
	m.SupportID, m.ContourID = face[1], face[4]
	contour, err := g.links(m.ContourID, 142, 6)
	if err != nil {
		return zero, err
	}
	if contour[1] != 0 || contour[2] != m.SupportID || contour[5] != 3 {
		return zero, fmt.Errorf("iges: mismatched periodic trim contour support or mode: %w", geom.ErrUnrepresentable)
	}
	m.UVCompositeID, m.ModelCompositeID = contour[3], contour[4]
	uv, err := g.links(m.UVCompositeID, 102, 2+edgeCount)
	if err != nil {
		return zero, err
	}
	model, err := g.links(m.ModelCompositeID, 102, 2+edgeCount)
	if err != nil {
		return zero, err
	}
	if uv[1] != edgeCount || model[1] != edgeCount {
		return zero, geom.ErrUnrepresentable
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
	if !support.Surface.PeriodicU || support.Surface.PeriodicV || support.Surface.ClosedV {
		return zero, fmt.Errorf("iges: periodic trim requires one U-periodic support: %w", geom.ErrUnrepresentable)
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
	return periodicTrimSource{m, g, uv[2:], model[2:], remaining}, nil
}

func (m *PeriodicTrimmedFace) proveOriginalKnotCell(ctx context.Context) (geom.ExactPeriodicSurfaceBezierCell, error) {
	var zero geom.ExactPeriodicSurfaceBezierCell
	owners, err := periodicTrimKnotOwners(ctx, m.Support, m.Bounds)
	if err != nil {
		return zero, err
	}
	m.KnotOwners = owners
	if owners[0].Cycle.Sign() != 0 {
		return zero, fmt.Errorf("iges: periodic trim lies outside its original fundamental cycle: %w", geom.ErrUnrepresentable)
	}
	for i, owner := range m.KnotOwners {
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
	if len(r.values) < count {
		return nil, geom.ErrUnrepresentable
	}
	if err := noPointers(r.values[count:]); err != nil {
		return nil, err
	}
	out := make([]int, count)
	for i := range out {
		out[i], err = intValue(r.values[i])
		if err != nil {
			return nil, err
		}
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
