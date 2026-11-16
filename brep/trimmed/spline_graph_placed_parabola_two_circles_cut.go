package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PlacedParabolaTwoCirclesGraphCut retains original world roof surfaces,
// original UV charts and model ranges, and every component's indexed boundary.
type PlacedParabolaTwoCirclesGraphCut struct {
	sources   [3]*geom.BSplineSurface
	chart     parabolaCircleSimilarity
	local     *ParabolaTwoCirclesGraphCut
	world     []Solid
	tolerance float64
}

func (r *PlacedParabolaTwoCirclesGraphCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.world)
}

func (r *PlacedParabolaTwoCirclesGraphCut) History() []ParabolaCircleCutComponentHistory {
	if r == nil {
		return nil
	}
	return r.local.History()
}

// CutPlacedParabolaTwoCirclesSplineGraphs accepts the same original unit
// chart on all three supports, placed by a proper similarity with an exact
// rational scale and representable world basis. The original base must be
// the corresponding planar square. Its corners determine the exact frame.
// Every original world roof and its continuous coedges are retained.
func CutPlacedParabolaTwoCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*PlacedParabolaTwoCirclesGraphCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	chart, err := parabolaCircleChartFromBase(base, o.Stock.Patch)
	if err != nil {
		return nil, err
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	var aligned [3]*geom.BSplineSurface
	for i, s := range sources {
		aligned[i], err = chart.normalizeSurface(s, o.Stock.Patch)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			for _, point := range aligned[i].Data().Poles {
				if point.Z != 0 {
					return nil, geom.ErrConvergence
				}
			}
		}
	}
	localTolerance, err := chart.localDistance(tolerance)
	if err != nil {
		return nil, err
	}
	local, err := CutParabolaTwoCirclesSplineGraphs(ctx, aligned[0], aligned[1], aligned[2], freeAxis, localTolerance, o)
	if err != nil {
		return nil, err
	}
	world := make([]Solid, len(local.components))
	originals := map[*geom.BSplineSurface]*geom.BSplineSurface{aligned[1]: a, aligned[2]: b}
	for i, part := range local.components {
		world[i], err = placeParabolaCircleComponent(ctx, part, originals, chart, tolerance, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: placed parabola and circles region %d: %w", local.history[i].Region, err)
		}
	}
	return &PlacedParabolaTwoCirclesGraphCut{sources: sources, chart: chart, local: local, world: world, tolerance: tolerance}, nil
}

func placeParabolaCircleComponent(ctx context.Context, s Solid, original map[*geom.BSplineSurface]*geom.BSplineSurface,
	chart parabolaCircleSimilarity, tolerance float64, proof geom.GraphPointContactOptions) (Solid, error) {
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for i, p := range v {
		var err error
		v[i], err = chart.exactWorldPoint(p)
		if err != nil {
			return Solid{}, err
		}
	}
	for i, e := range edges {
		switch c := e.Curve.(type) {
		case surfaceTrimCurve:
			local, ok := c.surface.(*geom.BSplineSurface)
			if !ok || original[local] == nil {
				return Solid{}, geom.ErrConvergence
			}
			c.surface = original[local]
			edges[i].Curve = c
		case geom.Line:
			if !chart.signedAxisUnit {
				edges[i].Curve = parametric.MapCurve(c, chart.world)
				continue
			}
			o, err := chart.exactWorldPoint(c.Origin())
			if err != nil {
				return Solid{}, err
			}
			edges[i].Curve, err = geom.NewLine(o, chart.world.ApplyVector(c.Direction()))
			if err != nil {
				return Solid{}, err
			}
		default:
			return Solid{}, geom.ErrConvergence
		}
	}
	for i, f := range faces {
		switch support := f.Surface.(type) {
		case *geom.BSplineSurface:
			raw := original[support]
			if raw == nil {
				return Solid{}, geom.ErrConvergence
			}
			faces[i].Surface = raw
		case geom.PlaneSurface:
			physical, err := chart.plane(support)
			if err != nil {
				return Solid{}, err
			}
			faces[i].Surface = physical
			for j, use := range f.Outer {
				var w wallTrim
				reversed := false
				switch c := use.Curve.(type) {
				case nil:
					continue
				case wallTrim:
					w = c
				case reverseTrim:
					var ok bool
					w, ok = c.differentiableTrim.(wallTrim)
					if !ok {
						return Solid{}, geom.ErrConvergence
					}
					reversed = true
				default:
					return Solid{}, geom.ErrConvergence
				}
				raw := original[w.surface]
				if raw == nil {
					return Solid{}, geom.ErrConvergence
				}
				p := placedParabolaCircleWallTrim{wallTrim: w, original: raw, chart: chart, plane: support}
				faces[i].Outer[j].Curve = p
				if reversed {
					faces[i].Outer[j].Curve = reverseTrim{p}
				}
			}
		default:
			return Solid{}, geom.ErrConvergence
		}
	}
	world, err := NewSolidContext(ctx, v, edges, faces, tolerance)
	if err != nil {
		return Solid{}, err
	}
	if err := validateExactGraphPartitionEdges(ctx, world, proof); err != nil {
		return Solid{}, err
	}
	return world, nil
}

// The world wall still uses the original roof restriction. Its UV height is
// the signed chart-height coordinate of that exact world owner.
type placedParabolaCircleWallTrim struct {
	wallTrim
	original *geom.BSplineSurface
	chart    parabolaCircleSimilarity
	plane    geom.PlaneSurface
}

func (w placedParabolaCircleWallTrim) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t <= 0 || t >= 1 {
		return w.wallTrim.EvalUV(t)
	}
	uv := mix(w.a, w.b, t)
	p, err := w.original.Eval(uv.X, uv.Y)
	return geom.Vec2{X: w.length * t, Y: w.chart.evaluatorLocalHeight(p, true) - w.height}, err
}

func (w placedParabolaCircleWallTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	uv := mix(w.a, w.b, t)
	du, dv, err := w.original.Partials(uv.X, uv.Y)
	d := sub2(w.b, w.a)
	p := du.Scale(d.X).Add(dv.Scale(d.Y))
	return geom.Vec2{X: w.length, Y: w.chart.evaluatorLocalHeight(p, false)}, err
}

func (w placedParabolaCircleWallTrim) agrees(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, reversed bool,
	o edgecheck.Options) (bool, error) {
	want, err := w.chart.plane(w.plane)
	if err != nil || surface != want {
		return false, err
	}
	restriction, ok := model.Curve.(surfaceTrimCurve)
	if !ok || restriction.surface != w.original {
		return false, nil
	}
	if w.chart.signedAxisUnit {
		restriction.surface = parametric.MapSurface(w.original, w.chart.fastInverse)
	} else {
		normalized, err := w.chart.normalizeSurface(w.original, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
		if err != nil || !geom.SameSplineSurface(normalized, w.surface) {
			return false, err
		}
		restriction.surface = normalized
	}
	model.Curve, use.Curve = restriction, w.wallTrim
	if reversed {
		use.Curve = reverseTrim{w.wallTrim}
	}
	return compareSplineWallUse(ctx, model, w.plane, use, o)
}
