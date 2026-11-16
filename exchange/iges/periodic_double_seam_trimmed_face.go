package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicDoubleSeamTrimmedFace retains an original periodic type-144
// rectangle crossing both upper seams. Cells are indexed by U and V
// cycles [0,1] respectively. The source's eight directed pairs retain
// their original owners and model parameter intervals. The proof covers
// both physical first partials on every shared seam segment and one
// injective projection of the entire lifted rectangle.
type PeriodicDoubleSeamTrimmedFace struct {
	LengthUnit                      float64
	FaceID, SupportID, ContourID    int
	UVCompositeID, ModelCompositeID int
	Support                         *geom.ExactPeriodicSplineSurface
	CarrierBounds, Bounds           [2]geom.ExactParameterInterval
	SeamU, SeamV                    *big.Rat
	Cells                           [2][2]PeriodicSeamTrimCell
	Edges                           [8]PeriodicTrimEdge
	ProjectionAxes                  [2]int
	ProjectionOrientation           int
	original                        string
}

// OriginalIGES returns the complete retained source file.
func (f PeriodicDoubleSeamTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicDoubleSeamTrimmedFace allows one unplaced outer-only face
// with both type-102 composites split at both original upper seams.
// The lifted rectangle must lie in the four seam-adjacent cells with
// exact complete UV/model pairing and first partial continuity.
func ReadPeriodicDoubleSeamTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicDoubleSeamTrimmedFace, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 8, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicDoubleSeamTrimmedFace{}, err
	}
	return provePeriodicDoubleSeamTrimmedFace(ctx, source)
}

func provePeriodicDoubleSeamTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicDoubleSeamTrimmedFace, error) {
	var zero PeriodicDoubleSeamTrimmedFace
	base := source.face
	m := PeriodicDoubleSeamTrimmedFace{
		LengthUnit: base.LengthUnit, FaceID: base.FaceID, SupportID: base.SupportID, ContourID: base.ContourID,
		UVCompositeID: base.UVCompositeID, ModelCompositeID: base.ModelCompositeID, Support: base.Support,
		CarrierBounds: base.Bounds, original: base.original,
	}
	u, v, _ := m.Support.Data()
	if !u.Periodic || !v.Periodic {
		return zero, fmt.Errorf("iges: double-seam trim requires both original periodic axes: %w", geom.ErrUnrepresentable)
	}
	var uv [8]periodicTrimUVEdge
	bounds, err := readPeriodicTrimUVEdges(source.graph, source.uv, uv[:])
	if err != nil {
		return zero, err
	}
	m.Bounds = bounds
	var seams, periods [2]*big.Rat
	for i, b := range [2]geom.ExactTensorSplineAxis{u, v} {
		carrier := m.CarrierBounds[i]
		if carrier.Lower.Cmp(b.Knots[0]) != 0 || carrier.Upper.Cmp(b.Knots[b.Count]) != 0 {
			return zero, fmt.Errorf("iges: double-seam trim requires the full original carrier intervals: %w", geom.ErrUnrepresentable)
		}
		seams[i] = new(big.Rat).Set(carrier.Upper)
		periods[i] = new(big.Rat).Sub(carrier.Upper, carrier.Lower)
		if bounds[i].Lower.Cmp(carrier.Lower) <= 0 || !crossesOriginalUpperSeam(bounds[i], seams[i]) ||
			new(big.Rat).Sub(bounds[i].Upper, bounds[i].Lower).Cmp(periods[i]) >= 0 {
			return zero, fmt.Errorf("iges: double-seam trim must cross both original upper seams in cycles 0 and 1: %w", geom.ErrUnrepresentable)
		}
	}
	m.SeamU, m.SeamV = seams[0], seams[1]
	cells, err := m.proveDoubleSeamCells(ctx)
	if err != nil {
		return zero, err
	}
	m.Edges, err = proveDoubleSeamTrimEdges(ctx, source, m.Bounds, seams, uv, cells)
	if err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	m.ProjectionAxes, m.ProjectionOrientation, err = provePeriodicTrimLiftProjection(ctx, m.Support, bounds, periods, 4)
	if err != nil {
		return zero, err
	}
	return m, nil
}

func (m *PeriodicDoubleSeamTrimmedFace) proveDoubleSeamCells(ctx context.Context) ([]geom.ExactPeriodicSurfaceBezierCell, error) {
	cells, _, err := m.Support.ExactBezierCells(ctx, m.Bounds[0], m.Bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 4})
	if err != nil {
		return nil, fmt.Errorf("iges: split lifted trim at four original knot cells: %w", err)
	}
	if len(cells) != 4 {
		return nil, fmt.Errorf("iges: lifted trim must occupy the four seam-adjacent knot cells: %w", geom.ErrUnrepresentable)
	}
	stations := [2][3]*big.Rat{{m.Bounds[0].Lower, m.SeamU, m.Bounds[0].Upper}, {m.Bounds[1].Lower, m.SeamV, m.Bounds[1].Upper}}
	// ExactBezierCells returns U-major order.
	for i, c := range cells {
		cycles := [2]int{i / 2, i % 2}
		box := [2]geom.ExactParameterInterval{c.U.Domain, c.V.Domain}
		owners, err := periodicTrimKnotOwners(ctx, m.Support, box)
		if err != nil {
			return nil, err
		}
		for axis, cycle := range cycles {
			d, owner := box[axis], owners[axis]
			if d.Lower.Cmp(stations[axis][cycle]) != 0 || d.Upper.Cmp(stations[axis][cycle+1]) != 0 ||
				owner.Cycle.Cmp(big.NewInt(int64(cycle))) != 0 ||
				cycle == 0 && d.Lower.Cmp(owner.Domain.Lower) <= 0 ||
				cycle == 1 && d.Upper.Cmp(owner.Domain.Upper) >= 0 {
				return nil, fmt.Errorf("iges: double-seam trim meets another original knot or cycle: %w", geom.ErrUnrepresentable)
			}
		}
		patch, err := c.NormalizedPatch(ctx, geom.SurfacePatchOptions{})
		if err != nil {
			return nil, err
		}
		m.Cells[cycles[0]][cycles[1]] = PeriodicSeamTrimCell{Bounds: box, KnotOwners: owners, Patch: patch}
	}
	for tangentialCycle := range 2 {
		if err := provePeriodicTrimPhysicalSeam(ctx, cells[tangentialCycle], cells[tangentialCycle+2], 0); err != nil {
			return nil, fmt.Errorf("iges: U seam on V cycle %d: %w", tangentialCycle, err)
		}
		if err := provePeriodicTrimPhysicalSeam(ctx, cells[2*tangentialCycle], cells[2*tangentialCycle+1], 1); err != nil {
			return nil, fmt.Errorf("iges: V seam on U cycle %d: %w", tangentialCycle, err)
		}
	}
	return cells, nil
}

func proveDoubleSeamTrimEdges(ctx context.Context, source periodicTrimSource, bounds [2]geom.ExactParameterInterval,
	seams [2]*big.Rat, uv [8]periodicTrimUVEdge, cells []geom.ExactPeriodicSurfaceBezierCell) ([8]PeriodicTrimEdge, error) {
	var zero, pairs [8]PeriodicTrimEdge
	u, v := bounds[0], bounds[1]
	stations := [8][2]*big.Rat{
		{u.Lower, v.Lower}, {seams[0], v.Lower}, {u.Upper, v.Lower}, {u.Upper, seams[1]},
		{u.Upper, v.Upper}, {seams[0], v.Upper}, {u.Lower, v.Upper}, {u.Lower, seams[1]},
	}
	owners := [8][2]int{{0, 0}, {2, 0}, {2, 1}, {3, 1}, {3, 2}, {1, 2}, {1, 3}, {0, 3}}
	same := func(a, b [2]*big.Rat) bool { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 }
	first := -1
	for i, station := range stations {
		if same(station, uv[0].start) {
			first = i
			break
		}
	}
	for i, edge := range uv {
		start := (first + i + 8) % 8
		if first < 0 || !same(edge.start, stations[start]) || !same(edge.end, stations[(start+1)%8]) {
			return zero, fmt.Errorf("iges: periodic UV line #%d does not follow the directed eight-edge split rectangle: %w", edge.id, geom.ErrUnrepresentable)
		}
		owner := owners[start]
		expected, err := periodicTrimCellEdge(ctx, cells[owner[0]], owner[1])
		if err != nil {
			return zero, err
		}
		pair, used, err := provePeriodicTrimEdge(ctx, source.graph, edge.id, source.model[i], edge.start, edge.end, expected, source.remaining)
		if err != nil {
			return zero, err
		}
		source.remaining -= used
		pairs[i] = pair
	}
	return pairs, nil
}
