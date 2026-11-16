package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicSeamTrimCell retains one exact single-cell patch of the lifted
// face. Bounds contain the clipped file parameters. KnotOwners describe
// the original physical knot spans and the original cycle numbers.
type PeriodicSeamTrimCell struct {
	Bounds     [2]geom.ExactParameterInterval
	KnotOwners [2]geom.ExactSurfaceKnotSpan
	Patch      *geom.SurfacePatch
}

// PeriodicSeamTrimmedFace retains the periodic carrier and one lifted
// type-144 rectangular face crossing its U seam. The two normalized
// patches are in ascending lifted U order. The six paired edges retain
// the original outer-wire order and each original model parameter range.
type PeriodicSeamTrimmedFace struct {
	LengthUnit                      float64
	FaceID, SupportID, ContourID    int
	UVCompositeID, ModelCompositeID int
	Support                         *geom.ExactPeriodicSplineSurface
	CarrierBounds, Bounds           [2]geom.ExactParameterInterval
	SeamU                           *big.Rat
	Cells                           [2]PeriodicSeamTrimCell
	Edges                           [6]PeriodicTrimEdge
	ProjectionAxes                  [2]int
	ProjectionOrientation           int
	original                        string
}

// OriginalIGES returns the complete original face and dependency records.
func (f PeriodicSeamTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicSeamTrimmedFace proves one rectangular type-144 face whose
// directed UV and model composites both split at the upper U seam of the
// original full type-128 carrier. The lift must meet exactly the two
// adjacent U knot cells and one ordinary V cell. Both sides must share
// the entire seam, and one fixed projection must prove injectivity over
// the complete lifted rectangle.
func ReadPeriodicSeamTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicSeamTrimmedFace, error) {
	var zero PeriodicSeamTrimmedFace
	source, err := readPeriodicTrimSource(ctx, r, l, 6)
	if err != nil {
		return zero, err
	}
	base := source.face
	m := PeriodicSeamTrimmedFace{
		LengthUnit: base.LengthUnit, FaceID: base.FaceID, SupportID: base.SupportID, ContourID: base.ContourID,
		UVCompositeID: base.UVCompositeID, ModelCompositeID: base.ModelCompositeID,
		Support: base.Support, CarrierBounds: base.Bounds, original: base.original,
	}
	u, v, _ := m.Support.Data()
	period := new(big.Rat).Sub(u.Knots[u.Count], u.Knots[0])
	for i, expected := range [2][2]*big.Rat{{u.Knots[0], u.Knots[u.Count]}, {v.Knots[v.Degree], v.Knots[v.Count]}} {
		if m.CarrierBounds[i].Lower.Cmp(expected[0]) != 0 || m.CarrierBounds[i].Upper.Cmp(expected[1]) != 0 {
			return zero, fmt.Errorf("iges: seam trim requires the full original carrier intervals: %w", geom.ErrUnrepresentable)
		}
	}
	m.SeamU = u.Knots[u.Count]
	uv, err := m.readLiftedUV(source.graph, source.uv)
	if err != nil {
		return zero, err
	}
	if m.Bounds[0].Lower.Cmp(u.Knots[0]) <= 0 || m.Bounds[0].Lower.Cmp(m.SeamU) >= 0 ||
		m.Bounds[0].Upper.Cmp(m.SeamU) <= 0 || new(big.Rat).Sub(m.Bounds[0].Upper, m.Bounds[0].Lower).Cmp(period) >= 0 {
		return zero, fmt.Errorf("iges: expected one lift crossing the upper original U seam: %w", geom.ErrUnrepresentable)
	}
	cells, err := m.proveSeamCells(ctx)
	if err != nil {
		return zero, err
	}
	if err := m.proveLiftedEdges(ctx, source, uv, cells); err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	if err := m.proveLiftInjectivity(ctx, period); err != nil {
		return zero, err
	}
	return m, nil
}

func (m *PeriodicSeamTrimmedFace) proveSeamCells(ctx context.Context) ([]geom.ExactPeriodicSurfaceBezierCell, error) {
	cells, _, err := m.Support.ExactBezierCells(ctx, m.Bounds[0], m.Bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 2})
	if err != nil {
		return nil, fmt.Errorf("iges: split lifted trim at two original knot cells: %w", err)
	}
	if len(cells) != 2 || cells[0].U.Domain.Upper.Cmp(m.SeamU) != 0 || cells[1].U.Domain.Lower.Cmp(m.SeamU) != 0 {
		return nil, fmt.Errorf("iges: lifted rectangle must occupy the two seam-adjacent cells: %w", geom.ErrUnrepresentable)
	}
	for i, cell := range cells {
		box := [2]geom.ExactParameterInterval{cell.U.Domain, cell.V.Domain}
		owners, err := periodicTrimKnotOwners(ctx, m.Support, box)
		if err != nil {
			return nil, err
		}
		if owners[0].Cycle.Cmp(big.NewInt(int64(i))) != 0 ||
			i == 0 && box[0].Lower.Cmp(owners[0].Domain.Lower) <= 0 ||
			i == 1 && box[0].Upper.Cmp(owners[0].Domain.Upper) >= 0 ||
			box[1].Lower.Cmp(owners[1].Domain.Lower) <= 0 || box[1].Upper.Cmp(owners[1].Domain.Upper) >= 0 {
			return nil, fmt.Errorf("iges: lifted rectangle touches an additional original knot station: %w", geom.ErrUnrepresentable)
		}
		patch, err := cell.NormalizedPatch(ctx, geom.SurfacePatchOptions{})
		if err != nil {
			return nil, err
		}
		m.Cells[i] = PeriodicSeamTrimCell{box, owners, patch}
	}
	left, err := periodicTrimCellEdge(ctx, cells[0], 1)
	if err != nil {
		return nil, err
	}
	right, err := periodicTrimCellEdge(ctx, cells[1], 3)
	if err != nil {
		return nil, err
	}
	right, err = right.Reparameterize(0, 1, true, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, err
	}
	pair, err := geom.CompareExactCurvesContext(ctx, left, right, geom.ExactCurveComparisonOptions{MaxSpans: 1})
	if err != nil {
		return nil, err
	}
	if !pair.Complete || !pair.Equal {
		return nil, fmt.Errorf("iges: extracted original knot cells disagree along the seam: %w", geom.ErrUnrepresentable)
	}
	return cells, nil
}

type periodicSeamUVEdge struct {
	start, end [2]*big.Rat
}

func (m *PeriodicSeamTrimmedFace) readLiftedUV(graph periodicTrimGraph, ids []int) ([6]periodicSeamUVEdge, error) {
	var uv [6]periodicSeamUVEdge
	for i, id := range ids {
		edge, err := graph.take(id, 110)
		if err != nil {
			return uv, err
		}
		a, b, err := periodicTrimUVLine(edge)
		if err != nil {
			return uv, fmt.Errorf("iges: periodic UV line #%d: %w", id, err)
		}
		uv[i] = periodicSeamUVEdge{a, b}
		for _, p := range [2][2]*big.Rat{a, b} {
			for axis, q := range p {
				bound := &m.Bounds[axis]
				if bound.Lower == nil || q.Cmp(bound.Lower) < 0 {
					bound.Lower = q
				}
				if bound.Upper == nil || q.Cmp(bound.Upper) > 0 {
					bound.Upper = q
				}
			}
		}
	}
	return uv, nil
}

func (m *PeriodicSeamTrimmedFace) proveLiftedEdges(ctx context.Context, s periodicTrimSource, uv [6]periodicSeamUVEdge,
	cells []geom.ExactPeriodicSurfaceBezierCell) error {
	u, v := m.Bounds[0], m.Bounds[1]
	stations := [6][2]*big.Rat{{u.Lower, v.Lower}, {m.SeamU, v.Lower}, {u.Upper, v.Lower},
		{u.Upper, v.Upper}, {m.SeamU, v.Upper}, {u.Lower, v.Upper}}
	station := func(q [2]*big.Rat) int {
		for i, point := range stations {
			if q[0].Cmp(point[0]) == 0 && q[1].Cmp(point[1]) == 0 {
				return i
			}
		}
		return -1
	}
	// Every directed rectangle segment owns one side of one exact cell.
	cellSides := [6][2]int{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}, {0, 3}}
	first := station(uv[0].start)
	for i, edge := range uv {
		start := (first + i) % 6
		if first < 0 || station(edge.start) != start || station(edge.end) != (start+1)%6 {
			return fmt.Errorf("iges: original periodic UV line #%d does not follow the directed split rectangle: %w", s.uv[i], geom.ErrUnrepresentable)
		}
		owner := cellSides[start]
		expected, err := periodicTrimCellEdge(ctx, cells[owner[0]], owner[1])
		if err != nil {
			return err
		}
		pair, used, err := provePeriodicTrimEdge(ctx, s.graph, s.uv[i], s.model[i], edge.start, edge.end, expected, s.remaining)
		if err != nil {
			return err
		}
		s.remaining -= used
		m.Edges[i] = pair
	}
	return nil
}

func (m *PeriodicSeamTrimmedFace) proveLiftInjectivity(ctx context.Context, period *big.Rat) error {
	fundamental, err := m.Support.ExactFundamentalPatch(ctx, geom.PeriodicSurfaceBezierOptions{MaxCells: 64})
	if err != nil {
		return err
	}
	source := [4]*big.Rat{m.Bounds[0].Lower, new(big.Rat).Sub(m.Bounds[0].Upper, period), m.Bounds[1].Lower, m.Bounds[1].Upper}
	var mapped [4]float64
	_, periodicSpans := fundamental.Coordinates[0].Breaks()
	for i, q := range source {
		t, err := fundamental.Coordinates[i/2].ToWorking(q)
		if err != nil {
			return err
		}
		if i == 1 {
			t.Add(t, big.NewRat(int64(len(periodicSpans)), 1))
		}
		mapped[i], err = geom.RepresentedExactScalar(t)
		if err != nil {
			return err
		}
	}
	domain := geom.ParameterRectangle{UMin: mapped[0], UMax: mapped[1], VMin: mapped[2], VMax: mapped[3]}
	for _, axes := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		p, err := fundamental.Wrapped.CertifyLiftedRectangleInjectivity(ctx, domain, geom.SurfaceInjectivityOptions{
			ProjectionAxes: axes, MaxDepth: 6, MaxCells: 64, MaxOperations: 200000,
		})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil && p.ProvenInjective && p.FundamentalPieces == 2 {
			m.ProjectionAxes, m.ProjectionOrientation = axes, p.ProjectionOrientation
			return nil
		}
	}
	return fmt.Errorf("iges: complete lifted periodic trim projection is unproved: %w", geom.ErrUnrepresentable)
}
