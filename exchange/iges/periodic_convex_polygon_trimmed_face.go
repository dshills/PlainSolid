package iges

import (
	"context"
	"fmt"
	"io"

	"plainsolid/geom"
)

const (
	maxPeriodicConvexPolygonEdges = 8
	maxPeriodicConvexPolygonHoles = 2
)

// PeriodicConvexPolygonTrimLoop retains one source contour, its exact
// UV bounds and every original directed model/parameter edge pair.
type PeriodicConvexPolygonTrimLoop struct {
	ContourID, UVCompositeID, ModelCompositeID int
	Bounds                                     [2]geom.ExactParameterInterval
	Edges                                      []PeriodicTrimEdge
}

// PeriodicConvexPolygonTrimmedFace retains one strictly convex outer
// affine loop and up to two strictly nested, disjoint affine holes in
// original contour order. Patch evaluates their enclosing rectangle
// in the normalized unit square.
type PeriodicConvexPolygonTrimmedFace struct {
	LengthUnit            float64
	FaceID, SupportID     int
	Support               *geom.ExactPeriodicSplineSurface
	Patch                 *geom.SurfacePatch
	KnotOwners            [2]geom.ExactSurfaceKnotSpan
	Outer                 PeriodicConvexPolygonTrimLoop
	Holes                 []PeriodicConvexPolygonTrimLoop
	ProjectionAxes        [2]int
	ProjectionOrientation int
	original              string
}

// OriginalIGES returns the complete retained source file.
func (f PeriodicConvexPolygonTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicConvexPolygonTrimmedFace proves loops of three to eight
// affine UV edges strictly within one original periodic knot cell.
// The outer contour is counterclockwise, holes are clockwise, and
// every source model curve matches the exact affine composition.
func ReadPeriodicConvexPolygonTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicConvexPolygonTrimmedFace, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 0, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicConvexPolygonTrimmedFace{}, err
	}
	return provePeriodicConvexPolygonTrimmedFace(ctx, source)
}

func provePeriodicConvexPolygonTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicConvexPolygonTrimmedFace, error) {
	var zero PeriodicConvexPolygonTrimmedFace
	m, outer, err := provePeriodicConvexOuterGeometry(ctx, &source)
	if err != nil {
		return zero, err
	}
	holes := make([]PeriodicConvexPolygonTrimLoop, len(source.holes))
	for i, owner := range source.holes {
		holes[i], source.remaining, err = provePeriodicConvexInnerGeometry(ctx, source.graph, m.Patch, m.Bounds, outer, owner, source.remaining)
		if err != nil {
			return zero, err
		}
	}
	if len(holes) == 2 && !periodicConvexWiresStrictlyDisjoint(periodicAffineWireVertices(holes[0].Edges), periodicAffineWireVertices(holes[1].Edges)) {
		return zero, fmt.Errorf("iges: convex trim holes #%d and #%d overlap or meet: %w", holes[0].ContourID, holes[1].ContourID, geom.ErrUnrepresentable)
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConvexPolygonTrimmedFace{
		LengthUnit: m.LengthUnit, FaceID: m.FaceID, SupportID: m.SupportID, Support: m.Support, Patch: m.Patch, KnotOwners: m.KnotOwners,
		Outer: PeriodicConvexPolygonTrimLoop{ContourID: m.ContourID, UVCompositeID: m.UVCompositeID, ModelCompositeID: m.ModelCompositeID,
			Bounds: m.Bounds, Edges: outer},
		Holes: holes, ProjectionAxes: m.ProjectionAxes, ProjectionOrientation: m.ProjectionOrientation, original: m.original,
	}, nil
}
