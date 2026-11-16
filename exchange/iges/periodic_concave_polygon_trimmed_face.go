package iges

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/geom"
)

// PeriodicConcavePolygonTrimmedFace retains one original concave affine
// outer loop. Patch evaluates its enclosing rectangle in the normalized
// unit square. The directed edges delimit material in the original chart.
type PeriodicConcavePolygonTrimmedFace struct {
	LengthUnit                      float64
	FaceID, SupportID, ContourID    int
	UVCompositeID, ModelCompositeID int
	Support                         *geom.ExactPeriodicSplineSurface
	Patch                           *geom.SurfacePatch
	Bounds                          [2]geom.ExactParameterInterval
	KnotOwners                      [2]geom.ExactSurfaceKnotSpan
	Edges                           []PeriodicTrimEdge
	ProjectionAxes                  [2]int
	ProjectionOrientation           int
	original                        string
}

// OriginalIGES returns the complete retained source file.
func (f PeriodicConcavePolygonTrimmedFace) OriginalIGES() string { return f.original }

// ReadPeriodicConcavePolygonTrimmedFace proves one counterclockwise,
// simple concave loop of at most eight affine edges strictly inside one
// original periodic knot cell. Every source model curve agrees exactly
// with its complete directed UV restriction.
func ReadPeriodicConcavePolygonTrimmedFace(ctx context.Context, r io.Reader, l Limits) (PeriodicConcavePolygonTrimmedFace, error) {
	source, err := readPeriodicTrimSourceForAxes(ctx, r, l, 0, periodicTrimAnyAxes)
	if err != nil {
		return PeriodicConcavePolygonTrimmedFace{}, err
	}
	return provePeriodicConcavePolygonTrimmedFace(ctx, source)
}

func provePeriodicConcavePolygonTrimmedFace(ctx context.Context, source periodicTrimSource) (PeriodicConcavePolygonTrimmedFace, error) {
	var zero PeriodicConcavePolygonTrimmedFace
	if len(source.holes) != 0 {
		return zero, fmt.Errorf("iges: concave periodic trim requires one outer loop: %w", geom.ErrUnrepresentable)
	}
	m, edges, err := provePeriodicAffineOuterGeometry(ctx, &source, "concave", func(uv []periodicTrimUVEdge) error {
		points, _, err := periodicAffineCycle(uv, "concave")
		if err != nil {
			return err
		}
		return provePeriodicUVConcavity(points)
	})
	if err != nil {
		return zero, err
	}
	if len(source.graph.claimed) != len(source.graph.records) {
		return zero, geom.ErrUnrepresentable
	}
	return PeriodicConcavePolygonTrimmedFace{
		LengthUnit: m.LengthUnit, FaceID: m.FaceID, SupportID: m.SupportID, ContourID: m.ContourID,
		UVCompositeID: m.UVCompositeID, ModelCompositeID: m.ModelCompositeID, Support: m.Support, Patch: m.Patch,
		Bounds: m.Bounds, KnotOwners: m.KnotOwners, Edges: edges, ProjectionAxes: m.ProjectionAxes,
		ProjectionOrientation: m.ProjectionOrientation, original: m.original,
	}, nil
}

func provePeriodicUVConcavity(points [][2]*big.Rat) error {
	n := len(points)
	reflex := false
	twiceArea := new(big.Rat)
	for i, a := range points {
		b := points[(i+1)%n]
		turn := periodicUVTurn(a, b, points[(i+2)%n])
		if turn == 0 {
			return fmt.Errorf("iges: concave trim has a zero turn between edges %d and %d: %w", i+1, (i+1)%n+1, geom.ErrUnrepresentable)
		}
		reflex = reflex || turn < 0
		forward := new(big.Rat).Mul(a[0], b[1])
		backward := new(big.Rat).Mul(a[1], b[0])
		twiceArea.Add(twiceArea, forward.Sub(forward, backward))

		// Nonzero turns already separate each adjacent pair away from
		// its shared endpoint. Every other closed segment must be disjoint.
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue
			}
			if periodicUVSegmentsMeet(a, b, points[j], points[(j+1)%n]) {
				return fmt.Errorf("iges: nonadjacent concave trim edges %d and %d meet: %w", i+1, j+1, geom.ErrUnrepresentable)
			}
		}
	}
	if twiceArea.Sign() <= 0 {
		return fmt.Errorf("iges: concave trim requires counterclockwise UV winding: %w", geom.ErrUnrepresentable)
	}
	if !reflex {
		return fmt.Errorf("iges: concave trim requires a reflex UV vertex: %w", geom.ErrUnrepresentable)
	}
	return nil
}

func periodicUVSegmentsMeet(a, b, c, d [2]*big.Rat) bool {
	abc, abd := periodicUVTurn(a, b, c), periodicUVTurn(a, b, d)
	cda, cdb := periodicUVTurn(c, d, a), periodicUVTurn(c, d, b)
	return abc*abd < 0 && cda*cdb < 0 ||
		abc == 0 && periodicUVInSegmentBox(a, b, c) || abd == 0 && periodicUVInSegmentBox(a, b, d) ||
		cda == 0 && periodicUVInSegmentBox(c, d, a) || cdb == 0 && periodicUVInSegmentBox(c, d, b)
}

func periodicUVInSegmentBox(a, b, point [2]*big.Rat) bool {
	for axis, q := range point {
		if q.Cmp(a[axis])*q.Cmp(b[axis]) > 0 {
			return false
		}
	}
	return true
}
