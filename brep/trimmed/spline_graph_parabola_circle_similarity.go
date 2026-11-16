package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type parabolaCircleSimilarity struct {
	world          geom.Affine
	forward        [3][4]*big.Rat
	inverse        [3][4]*big.Rat
	scale, volume  *big.Rat
	signedAxisUnit bool
	fastInverse    geom.Affine
}

func parabolaCircleChartFromBase(base *geom.BSplineSurface, limits geom.SurfacePatchOptions) (parabolaCircleSimilarity, error) {
	var c parabolaCircleSimilarity
	if _, err := geom.NewSurfacePatch(base, limits); err != nil {
		return c, err
	}
	data := base.Data()
	origin := exactBoundPoint(data.Poles[0])
	u := boundSubtract(exactBoundPoint(data.Poles[(data.CountU-1)*data.CountV]), origin)
	v := boundSubtract(exactBoundPoint(data.Poles[data.CountV-1]), origin)
	squared := ddDot(u, u)
	if squared.Sign() <= 0 || ddDot(v, v).Cmp(squared) != 0 || ddDot(u, v).Sign() != 0 {
		return c, geom.ErrConvergence
	}
	scale, ok := ddSqrt(squared)
	if !ok {
		return c, geom.ErrConvergence
	}
	vertical := ddCross(u, v)
	for _, x := range vertical {
		x.Quo(x, scale)
	}
	c.scale = scale
	c.volume = new(big.Rat).Mul(squared, scale)
	c.signedAxisUnit = scale.Cmp(big.NewRat(1, 1)) == 0
	axes := [3]exactBoundVector{u, v, vertical}
	for i, axis := range axes {
		for j, coefficient := range axis {
			world, err := geom.RepresentedExactScalar(coefficient)
			if err != nil {
				return parabolaCircleSimilarity{}, err
			}
			if world != -1 && world != 0 && world != 1 {
				c.signedAxisUnit = false
			}
			c.world[j][i], c.forward[j][i] = world, coefficient
			c.inverse[i][j] = new(big.Rat).Quo(coefficient, squared)
		}
		c.world[i][3], _ = origin[i].Float64()
		c.forward[i][3] = origin[i]
		c.inverse[i][3] = new(big.Rat).Neg(new(big.Rat).Quo(ddDot(axis, origin), squared))
	}
	if c.signedAxisUnit {
		var err error
		c.fastInverse, err = c.world.Inverse()
		if err != nil {
			return parabolaCircleSimilarity{}, err
		}
	}
	return c, nil
}

func parabolaCircleApplyRational(m [3][4]*big.Rat, p exactBoundVector) (out exactBoundVector) {
	for axis, row := range m {
		out[axis] = new(big.Rat).Set(row[3])
		var term big.Rat
		for i, x := range row[:3] {
			if x.Sign() != 0 {
				out[axis].Add(out[axis], term.Mul(p[i], x))
			}
		}
	}
	return out
}

func parabolaCircleExactFloatPoint(p exactBoundVector) (geom.Vec3, error) {
	var v [3]float64
	for i, q := range p {
		var err error
		v[i], err = geom.RepresentedExactScalar(q)
		if err != nil {
			return geom.Vec3{}, err
		}
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}

func (c parabolaCircleSimilarity) exactWorldPoint(p geom.Vec3) (geom.Vec3, error) {
	return parabolaCircleExactFloatPoint(parabolaCircleApplyRational(c.forward, exactBoundPoint(p)))
}

func (c parabolaCircleSimilarity) normalizeSurface(s *geom.BSplineSurface, limits geom.SurfacePatchOptions) (*geom.BSplineSurface, error) {
	p, err := geom.NewSurfacePatch(s, limits)
	if err != nil {
		return nil, err
	}
	if p.Domain() != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return nil, geom.ErrDomain
	}
	if c.signedAxisUnit {
		p, err = p.Transformed(c.fastInverse)
		if err != nil {
			return nil, err
		}
		return p.ToBSplineSurface()
	}
	d := s.Data()
	for i, world := range d.Poles {
		d.Poles[i], err = parabolaCircleExactFloatPoint(parabolaCircleApplyRational(c.inverse, exactBoundPoint(world)))
		if err != nil {
			return nil, err
		}
	}
	return geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
}

func (c parabolaCircleSimilarity) localDistance(world float64) (float64, error) {
	if !finite(world) || world <= 0 {
		return 0, geom.ErrInvalid
	}
	q := new(big.Rat).Quo(ratInset(world), c.scale)
	local, exact := q.Float64()
	if !finite(local) || local <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	if !exact && ratInset(local).Cmp(q) > 0 {
		local = math.Nextafter(local, 0)
	}
	if local <= 0 {
		return 0, geom.ErrUnrepresentable
	}
	return local, nil
}

func (c parabolaCircleSimilarity) evaluatorLocalHeight(p geom.Vec3, point bool) float64 {
	if c.signedAxisUnit {
		if point {
			return c.fastInverse.Apply(p).Z
		}
		return c.fastInverse.ApplyVector(p).Z
	}
	if !p.IsFinite() {
		return math.NaN()
	}
	row := c.inverse[2]
	var z, term big.Rat
	if point {
		z.Set(row[3])
	}
	for i, x := range exactBoundPoint(p) {
		z.Add(&z, term.Mul(x, row[i]))
	}
	v, _ := z.Float64()
	return v
}

func (c parabolaCircleSimilarity) plane(p geom.PlaneSurface) (geom.Surface, error) {
	if !c.signedAxisUnit {
		return parabolaCircleSimilarityWall{local: p, world: c.world}, nil
	}
	fr := p.Frame()
	origin, err := c.exactWorldPoint(fr.Origin())
	if err != nil {
		return nil, err
	}
	world, err := geom.NewFrame(origin, c.world.ApplyVector(fr.XAxis()), c.world.ApplyVector(fr.ZAxis()))
	if err != nil {
		return nil, err
	}
	return geom.NewPlaneSurface(world)
}

// A scaled world wall retains the exact affine image of its original planar
// chart. The tangent vectors carry the scale and UV stations stay local.
type parabolaCircleSimilarityWall struct {
	local geom.PlaneSurface
	world geom.Affine
}

func (w parabolaCircleSimilarityWall) Eval(u, v float64) (geom.Vec3, error) {
	return parametric.MapSurface(w.local, w.world).Eval(u, v)
}

func (w parabolaCircleSimilarityWall) Partials(u, v float64) (geom.Vec3, geom.Vec3, error) {
	return parametric.MapSurface(w.local, w.world).Partials(u, v)
}

func (w parabolaCircleSimilarityWall) agreesLine(ctx context.Context, model parametric.Edge, use Use, o edgecheck.Options) (bool, error) {
	base, placement := parametric.CurveMapping(model.Curve)
	line, ok := base.(geom.Line)
	chart, planar := edgeChart(w.local)
	trace, straight := edgeBezier(use)
	if !ok || !planar || !straight || placement != w.world {
		return false, nil
	}
	trace.Reversed = use.Reversed
	report, err := edgecheck.CheckPlanarLine(ctx, trace, chart, line, model.Range.Min, model.Range.Max, 1, o)
	return report.Status == edgecheck.Certified && report.SquaredUpperBound != nil && report.SquaredUpperBound.Sign() == 0, err
}
