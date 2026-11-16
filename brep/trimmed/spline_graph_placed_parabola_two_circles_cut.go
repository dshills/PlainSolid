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
	sources [3]*geom.BSplineSurface
	chart   parabolaCircleRigidChart
	local   *ParabolaTwoCirclesGraphCut
	world   []Solid
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
// chart on all three supports, placed by a proper signed-axis rigid motion.
// The original base must be a planar unit square. The frame is recovered
// exactly from its original corners. Each roof restriction is reconstructed
// on its authored world surface and every continuous coedge is rechecked.
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
		p, err := geom.NewSurfacePatch(s, o.Stock.Patch)
		if err != nil {
			return nil, err
		}
		if p.Domain() != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
			return nil, geom.ErrDomain
		}
		p, err = p.Transformed(chart.inverse)
		if err != nil {
			return nil, err
		}
		aligned[i], err = p.ToBSplineSurface()
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
	local, err := CutParabolaTwoCirclesSplineGraphs(ctx, aligned[0], aligned[1], aligned[2], freeAxis, tolerance, o)
	if err != nil {
		return nil, err
	}
	world := make([]Solid, len(local.components))
	originals := map[*geom.BSplineSurface]*geom.BSplineSurface{aligned[1]: a, aligned[2]: b}
	for i, part := range local.components {
		world[i], err = placeParabolaCircleComponent(ctx, part, originals, chart, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: placed parabola and circles region %d: %w", local.history[i].Region, err)
		}
	}
	return &PlacedParabolaTwoCirclesGraphCut{sources: sources, chart: chart, local: local, world: world}, nil
}

type parabolaCircleRigidChart struct{ world, inverse geom.Affine }

func parabolaCircleChartFromBase(base *geom.BSplineSurface, limits geom.SurfacePatchOptions) (parabolaCircleRigidChart, error) {
	var empty parabolaCircleRigidChart
	if _, err := geom.NewSurfacePatch(base, limits); err != nil {
		return empty, err
	}
	d := base.Data()
	origin, endU, endV := d.Poles[0], d.Poles[(d.CountU-1)*d.CountV], d.Poles[d.CountV-1]
	exactOrigin := exactBoundPoint(origin)
	var axes [2]geom.Vec3
	for i, end := range [2]geom.Vec3{endU, endV} {
		difference := [3]float64{}
		nonzero := 0
		for j, q := range exactBoundPoint(end) {
			q.Sub(q, exactOrigin[j])
			x, ok := q.Float64()
			if !ok || x != 0 && x != 1 && x != -1 {
				return empty, geom.ErrConvergence
			}
			difference[j] = x
			if x != 0 {
				nonzero++
			}
		}
		if nonzero != 1 {
			return empty, geom.ErrConvergence
		}
		axes[i] = geom.Vec3{X: difference[0], Y: difference[1], Z: difference[2]}
	}
	if axes[0].Dot(axes[1]) != 0 {
		return empty, geom.ErrConvergence
	}
	u, v, z := axes[0], axes[1], axes[0].Cross(axes[1])
	world := geom.Affine{{u.X, v.X, z.X, origin.X}, {u.Y, v.Y, z.Y, origin.Y}, {u.Z, v.Z, z.Z, origin.Z}}
	inverse, err := world.Inverse()
	if err != nil {
		return empty, err
	}
	return parabolaCircleRigidChart{world, inverse}, nil
}

func parabolaCircleRigidPoint(m geom.Affine, p exactBoundVector) (out exactBoundVector) {
	for row, r := range m {
		out[row] = ratInset(r[3])
		for k, x := range r[:3] {
			switch x {
			case 1:
				out[row].Add(out[row], p[k])
			case -1:
				out[row].Sub(out[row], p[k])
			}
		}
	}
	return out
}

func parabolaCircleExactPlacedPoint(m geom.Affine, p geom.Vec3) (geom.Vec3, error) {
	var v [3]float64
	for i, q := range parabolaCircleRigidPoint(m, exactBoundPoint(p)) {
		var err error
		v[i], err = geom.RepresentedExactScalar(q)
		if err != nil {
			return geom.Vec3{}, err
		}
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}

func (c parabolaCircleRigidChart) plane(p geom.PlaneSurface) (geom.PlaneSurface, error) {
	fr := p.Frame()
	origin, err := parabolaCircleExactPlacedPoint(c.world, fr.Origin())
	if err != nil {
		return geom.PlaneSurface{}, err
	}
	world, err := geom.NewFrame(origin, c.world.ApplyVector(fr.XAxis()), c.world.ApplyVector(fr.ZAxis()))
	if err != nil {
		return geom.PlaneSurface{}, err
	}
	return geom.NewPlaneSurface(world)
}

func placeParabolaCircleComponent(ctx context.Context, s Solid, original map[*geom.BSplineSurface]*geom.BSplineSurface,
	chart parabolaCircleRigidChart, proof geom.GraphPointContactOptions) (Solid, error) {
	v, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	for i, p := range v {
		var err error
		v[i], err = parabolaCircleExactPlacedPoint(chart.world, p)
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
			o, err := parabolaCircleExactPlacedPoint(chart.world, c.Origin())
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
	world, err := NewSolidContext(ctx, v, edges, faces, s.tolerance)
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
	chart    parabolaCircleRigidChart
	plane    geom.PlaneSurface
}

func (w placedParabolaCircleWallTrim) EvalUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t <= 0 || t >= 1 {
		return w.wallTrim.EvalUV(t)
	}
	uv := mix(w.a, w.b, t)
	p, err := w.original.Eval(uv.X, uv.Y)
	return geom.Vec2{X: w.length * t, Y: w.chart.inverse.Apply(p).Z - w.height}, err
}

func (w placedParabolaCircleWallTrim) DerivativeUV(t float64) (geom.Vec2, error) {
	if !finite(t) || t < 0 || t > 1 {
		return geom.Vec2{}, geom.ErrDomain
	}
	uv := mix(w.a, w.b, t)
	du, dv, err := w.original.Partials(uv.X, uv.Y)
	d := sub2(w.b, w.a)
	p := du.Scale(d.X).Add(dv.Scale(d.Y))
	return geom.Vec2{X: w.length, Y: w.chart.inverse.ApplyVector(p).Z}, err
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
	restriction.surface = parametric.MapSurface(w.original, w.chart.inverse)
	model.Curve, use.Curve = restriction, w.wallTrim
	if reversed {
		use.Curve = reverseTrim{w.wallTrim}
	}
	return compareSplineWallUse(ctx, model, w.plane, use, o)
}
