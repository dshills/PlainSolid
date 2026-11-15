package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolygonMultiSpanMeshOptions enables exact original knot clipping for every
// display triangle. TotalCells/TotalPieces include failed refinement attempts,
// default100000 each,max1M. Triangle retains its independent arithmetic limits.
// The existing MaxTriangleChecks also remains cumulative. There is no fallback
// to one-sided bounds if this explicit two-sided proof fails.
type PolygonMultiSpanMeshOptions struct {
	Triangle                      MultiSpanTriangleBoundOptions
	MaxTotalCells, MaxTotalPieces int
}
type polygonKnotBudget struct {
	o             PolygonMultiSpanMeshOptions
	cells, pieces int
}

func newPolygonKnotBudget(o PolygonMultiSpanMeshOptions) (*polygonKnotBudget, error) {
	if o.Triangle.MaxCells < 0 || o.Triangle.MaxCells > 65536 || o.Triangle.MaxPieces < 0 || o.Triangle.MaxPieces > 1000000 {
		return nil, geom.ErrInvalid
	}
	if o.MaxTotalCells == 0 {
		o.MaxTotalCells = 100000
	}
	if o.MaxTotalPieces == 0 {
		o.MaxTotalPieces = 100000
	}
	if o.MaxTotalCells < 1 || o.MaxTotalCells > 1000000 || o.MaxTotalPieces < 1 || o.MaxTotalPieces > 1000000 {
		return nil, geom.ErrInvalid
	}
	return &polygonKnotBudget{o: o}, nil
}
func polygonTriangleDeviation(ctx context.Context, p *geom.SurfacePatch, uv [3]geom.Vec2, model [3]geom.Vec3, b *polygonKnotBudget) (SplineTriangleDeviation, error) {
	if b == nil {
		return BoundSplineTriangleDeviation(ctx, p, uv, model)
	}
	remainingCells, remainingPieces := b.o.MaxTotalCells-b.cells, b.o.MaxTotalPieces-b.pieces
	if remainingCells <= 0 || remainingPieces <= 0 {
		return SplineTriangleDeviation{}, parametric.ErrBudget
	}
	o := b.o.Triangle
	if o.MaxCells == 0 {
		o.MaxCells = 1024
	}
	if o.MaxPieces == 0 {
		o.MaxPieces = 8192
	}
	o.MaxCells = min(o.MaxCells, remainingCells)
	o.MaxPieces = min(o.MaxPieces, remainingPieces)
	r, e := BoundMultiSpanSplineTriangleDeviation(ctx, p, uv, model, o)
	if e != nil {
		return SplineTriangleDeviation{}, e
	}
	if !r.BothDirections {
		return SplineTriangleDeviation{}, geom.ErrConvergence
	}
	b.cells += r.Cells
	b.pieces += len(r.Pieces)
	return SplineTriangleDeviation{SquaredDistance: r.SquaredDistance, BothDirections: true}, nil
}
