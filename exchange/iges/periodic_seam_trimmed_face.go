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
// type-144 rectangle crossing its U or V seam. Only the occupied seam
// coordinate is set. The normalized patches follow increasing lifted
// parameter on that axis. The six pairs retain the original outer-wire
// order and each original model parameter range.
type PeriodicSeamTrimmedFace struct {
	LengthUnit                      float64
	FaceID, SupportID, ContourID    int
	UVCompositeID, ModelCompositeID int
	Support                         *geom.ExactPeriodicSplineSurface
	CarrierBounds, Bounds           [2]geom.ExactParameterInterval
	SeamU, SeamV                    *big.Rat
	Cells                           [2]PeriodicSeamTrimCell
	Edges                           [6]PeriodicTrimEdge
	ProjectionAxes                  [2]int
	ProjectionOrientation           int
	original                        string
}

// OriginalIGES returns the complete original face and dependency records.
func (f PeriodicSeamTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicSeamTrimmedFace proves one rectangular type-144 face whose
// directed UV and model composites both split at the upper seam of the
// original full type-128 carrier. Exactly one axis may cross a seam.
// The lift must meet the two neighboring cells on that axis and lie
// strictly inside one original knot span along the opposite axis. Both
// sides must share the entire seam, and one fixed projection must prove
// injectivity over the lifted rectangle.
func ReadPeriodicSeamTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicSeamTrimmedFace, error) {
	var zero PeriodicSeamTrimmedFace
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 6, periodicTrimAnyAxes)
	if err != nil {
		return zero, err
	}
	return provePeriodicSeamTrimmedFace(ctx, source)
}

func provePeriodicSeamTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicSeamTrimmedFace, error) {
	m, err := provePeriodicSeamTrimmedOuter(ctx, &source)
	if err != nil {
		return PeriodicSeamTrimmedFace{}, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return PeriodicSeamTrimmedFace{}, geom.ErrUnrepresentable
	}
	return m, nil
}

func provePeriodicSeamTrimmedOuter(ctx context.Context, source *periodicTrimSource) (PeriodicSeamTrimmedFace, error) {
	var zero PeriodicSeamTrimmedFace
	base := source.face
	m := PeriodicSeamTrimmedFace{
		LengthUnit: base.LengthUnit, FaceID: base.FaceID, SupportID: base.SupportID, ContourID: base.ContourID,
		UVCompositeID: base.UVCompositeID, ModelCompositeID: base.ModelCompositeID,
		Support: base.Support, CarrierBounds: base.Bounds, original: base.original,
	}
	u, v, _ := m.Support.Data()
	basis := [2]geom.ExactTensorSplineAxis{u, v}
	for i, b := range basis {
		first := b.Degree
		if b.Periodic {
			first = 0
		}
		if m.CarrierBounds[i].Lower.Cmp(b.Knots[first]) != 0 || m.CarrierBounds[i].Upper.Cmp(b.Knots[b.Count]) != 0 {
			return zero, fmt.Errorf("iges: seam trim requires the full original carrier intervals: %w", geom.ErrUnrepresentable)
		}
	}
	var uv [6]periodicTrimUVEdge
	var err error
	m.Bounds, err = readPeriodicTrimUVEdges(source.graph, source.uv, uv[:])
	if err != nil {
		return zero, err
	}
	axis := 0
	if u.Periodic && v.Periodic {
		uCrosses := crossesOriginalUpperSeam(m.Bounds[0], m.CarrierBounds[0].Upper)
		vCrosses := crossesOriginalUpperSeam(m.Bounds[1], m.CarrierBounds[1].Upper)
		if uCrosses == vCrosses {
			return zero, fmt.Errorf("iges: doubly periodic trim requires exactly one original upper seam crossing: %w", geom.ErrUnrepresentable)
		}
		if vCrosses {
			axis = 1
		}
	} else if v.Periodic {
		axis = 1
	}
	carrier := m.CarrierBounds[axis]
	seam := new(big.Rat).Set(carrier.Upper)
	period := new(big.Rat).Sub(seam, carrier.Lower)
	if axis == 0 {
		m.SeamU = seam
	} else {
		m.SeamV = seam
	}
	lift := m.Bounds[axis]
	if lift.Lower.Cmp(carrier.Lower) <= 0 || !crossesOriginalUpperSeam(lift, seam) ||
		new(big.Rat).Sub(lift.Upper, lift.Lower).Cmp(period) >= 0 {
		return zero, fmt.Errorf("iges: expected one lift crossing the upper original %s seam: %w", [2]string{"U", "V"}[axis], geom.ErrUnrepresentable)
	}
	cells, err := m.proveSeamCells(ctx)
	if err != nil {
		return zero, err
	}
	m.Edges, source.remaining, err = proveLiftedTrimEdges(ctx, source.graph, m.Bounds, axis, seam, uv, source.model, cells, 1, source.remaining)
	if err != nil {
		return zero, err
	}
	if err := m.proveLiftInjectivity(ctx, axis, period); err != nil {
		return zero, err
	}
	return m, nil
}

func crossesOriginalUpperSeam(interval geom.ExactParameterInterval, seam *big.Rat) bool {
	return interval.Lower.Cmp(seam) < 0 && interval.Upper.Cmp(seam) > 0
}

func (m *PeriodicSeamTrimmedFace) proveSeamCells(ctx context.Context) ([]geom.ExactPeriodicSurfaceBezierCell, error) {
	cells, _, err := m.Support.ExactBezierCells(ctx, m.Bounds[0], m.Bounds[1], geom.PeriodicSurfaceBezierOptions{MaxCells: 2})
	if err != nil {
		return nil, fmt.Errorf("iges: split lifted trim at two original knot cells: %w", err)
	}
	if len(cells) != 2 {
		return nil, fmt.Errorf("iges: lifted rectangle must occupy the two seam-adjacent cells: %w", geom.ErrUnrepresentable)
	}
	axis, seam := 0, m.SeamU
	if m.SeamV != nil {
		axis, seam = 1, m.SeamV
	}
	opposite := 1 - axis
	for i, cell := range cells {
		box := [2]geom.ExactParameterInterval{cell.U.Domain, cell.V.Domain}
		if (i == 0 && box[axis].Upper.Cmp(seam) != 0) || (i == 1 && box[axis].Lower.Cmp(seam) != 0) {
			return nil, fmt.Errorf("iges: lifted rectangle must occupy the two seam-adjacent cells: %w", geom.ErrUnrepresentable)
		}
		owners, err := periodicTrimKnotOwners(ctx, m.Support, box)
		if err != nil {
			return nil, err
		}
		if cycle := owners[opposite].Cycle; cycle != nil && cycle.Sign() != 0 {
			return nil, fmt.Errorf("iges: opposite axis is outside its original fundamental cycle: %w", geom.ErrUnrepresentable)
		}
		if owners[axis].Cycle.Cmp(big.NewInt(int64(i))) != 0 ||
			i == 0 && box[axis].Lower.Cmp(owners[axis].Domain.Lower) <= 0 ||
			i == 1 && box[axis].Upper.Cmp(owners[axis].Domain.Upper) >= 0 ||
			box[opposite].Lower.Cmp(owners[opposite].Domain.Lower) <= 0 ||
			box[opposite].Upper.Cmp(owners[opposite].Domain.Upper) >= 0 {
			return nil, fmt.Errorf("iges: lifted rectangle touches an additional original knot station: %w", geom.ErrUnrepresentable)
		}
		patch, err := cell.NormalizedPatch(ctx, geom.SurfacePatchOptions{})
		if err != nil {
			return nil, err
		}
		m.Cells[i] = PeriodicSeamTrimCell{box, owners, patch}
	}
	seamSides := [2][2]int{{1, 3}, {2, 0}}[axis]
	lower, err := periodicTrimCellEdge(ctx, cells[0], seamSides[0])
	if err != nil {
		return nil, err
	}
	upper, err := periodicTrimCellEdge(ctx, cells[1], seamSides[1])
	if err != nil {
		return nil, err
	}
	upper, err = upper.Reparameterize(0, 1, true, geom.SurfacePatchOptions{})
	if err != nil {
		return nil, err
	}
	pair, err := geom.CompareExactCurvesContext(ctx, lower, upper, geom.ExactCurveComparisonOptions{MaxSpans: 1})
	if err != nil {
		return nil, err
	}
	if !pair.Complete || !pair.Equal {
		return nil, fmt.Errorf("iges: extracted original knot cells disagree along the seam: %w", geom.ErrUnrepresentable)
	}
	return cells, nil
}

func proveLiftedTrimEdges(ctx context.Context, graph periodicTrimGraph, bounds [2]geom.ExactParameterInterval,
	axis int, seam *big.Rat, uv [6]periodicTrimUVEdge, model []int, cells []geom.ExactPeriodicSurfaceBezierCell,
	winding, remaining int) ([6]PeriodicTrimEdge, int, error) {
	var zero, pairs [6]PeriodicTrimEdge
	u, v := bounds[0], bounds[1]
	stations := [6][2]*big.Rat{{u.Lower, v.Lower}, {seam, v.Lower}, {u.Upper, v.Lower},
		{u.Upper, v.Upper}, {seam, v.Upper}, {u.Lower, v.Upper}}
	// Each CCW segment owns one side of one exact cell.
	cellSides := [6][2]int{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {0, 2}, {0, 3}}
	if axis == 1 {
		stations = [6][2]*big.Rat{{u.Lower, v.Lower}, {u.Upper, v.Lower}, {u.Upper, seam},
			{u.Upper, v.Upper}, {u.Lower, v.Upper}, {u.Lower, seam}}
		cellSides = [6][2]int{{0, 0}, {0, 1}, {1, 1}, {1, 2}, {1, 3}, {0, 3}}
	}
	station := func(q [2]*big.Rat) int {
		for i, point := range stations {
			if q[0].Cmp(point[0]) == 0 && q[1].Cmp(point[1]) == 0 {
				return i
			}
		}
		return -1
	}
	first := station(uv[0].start)
	for i, edge := range uv {
		start := (first + i*winding + 6) % 6
		end := (start + winding + 6) % 6
		if first < 0 || station(edge.start) != start || station(edge.end) != end {
			return zero, 0, fmt.Errorf("iges: original periodic UV line #%d does not follow the directed split rectangle: %w", edge.id, geom.ErrUnrepresentable)
		}
		owner := cellSides[start]
		if winding < 0 {
			owner = cellSides[end]
		}
		expected, err := periodicTrimCellEdge(ctx, cells[owner[0]], owner[1])
		if err != nil {
			return zero, 0, err
		}
		if winding < 0 {
			expected, err = expected.Reparameterize(0, 1, true, geom.SurfacePatchOptions{})
			if err != nil {
				return zero, 0, err
			}
		}
		pair, used, err := provePeriodicTrimEdge(ctx, graph, edge.id, model[i], edge.start, edge.end, expected, remaining)
		if err != nil {
			return zero, 0, err
		}
		remaining -= used
		pairs[i] = pair
	}
	return pairs, remaining, nil
}

func (m *PeriodicSeamTrimmedFace) proveLiftInjectivity(ctx context.Context, axis int, period *big.Rat) error {
	var periods [2]*big.Rat
	periods[axis] = period
	var err error
	m.ProjectionAxes, m.ProjectionOrientation, err = provePeriodicTrimLiftProjection(ctx, m.Support, m.Bounds, periods, 2)
	return err
}

func provePeriodicTrimLiftProjection(ctx context.Context, support *geom.ExactPeriodicSplineSurface,
	bounds [2]geom.ExactParameterInterval, periods [2]*big.Rat, expectedPieces int) ([2]int, int, error) {
	var zero [2]int
	fundamental, err := support.ExactFundamentalPatch(ctx, geom.PeriodicSurfaceBezierOptions{MaxCells: 64})
	if err != nil {
		return zero, 0, err
	}
	var mapped [4]float64
	for axis, b := range bounds {
		coords := fundamental.Coordinates[axis]
		upper, shift := b.Upper, int64(0)
		if period := periods[axis]; period != nil {
			upper = new(big.Rat).Sub(upper, period)
			_, spans := coords.Breaks()
			shift = int64(len(spans))
		}
		for i, q := range [2]*big.Rat{b.Lower, upper} {
			t, err := coords.ToWorking(q)
			if err != nil {
				return zero, 0, err
			}
			if i == 1 && shift != 0 {
				t.Add(t, big.NewRat(shift, 1))
			}
			mapped[2*axis+i], err = geom.RepresentedExactScalar(t)
			if err != nil {
				return zero, 0, err
			}
		}
	}
	domain := geom.ParameterRectangle{UMin: mapped[0], UMax: mapped[1], VMin: mapped[2], VMax: mapped[3]}
	for _, axes := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		p, err := fundamental.Wrapped.CertifyLiftedRectangleInjectivity(ctx, domain, geom.SurfaceInjectivityOptions{
			ProjectionAxes: axes, MaxDepth: 6, MaxCells: 64, MaxOperations: 200000,
		})
		if ctx.Err() != nil {
			return zero, 0, ctx.Err()
		}
		if err == nil && p.ProvenInjective && p.FundamentalPieces == expectedPieces {
			return axes, p.ProjectionOrientation, nil
		}
	}
	return zero, 0, fmt.Errorf("iges: complete lifted periodic trim projection is unproved: %w", geom.ErrUnrepresentable)
}
