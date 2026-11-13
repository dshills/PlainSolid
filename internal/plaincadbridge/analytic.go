package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// analyticBody owns an analytic boundary. Some revolutions also retain a
// rectangular patch representation for shared-grid display at polar caps.
type analyticBody struct {
	solid        trimmed.Solid
	patches      *parametric.Solid
	sector       *radialSector
	shaft        *trimmed.SteppedShaft
	segment      *circularSegmentSweep
	volume, area float64
}

func (b *Bridge) putAnalytic(s trimmed.Solid, volume, area float64) (Shape, error) {
	return b.putAnalyticBody(&analyticBody{solid: s, volume: volume, area: area})
}

func (b *Bridge) putAnalyticBody(body *analyticBody) (Shape, error) {
	if !finite(body.volume) || !finite(body.area) || body.volume <= 0 || body.area <= 0 {
		return Shape{}, errors.New("invalid analytic solid measures")
	}
	if len(body.solid.Topology().Shells) != 1 {
		return Shape{}, errors.New("expected one connected analytic shell")
	}
	var box geom.Box
	var err error
	if body.segment != nil {
		box, err = body.segment.bounds()
	} else {
		box, err = body.solid.Bounds()
	}
	if err != nil {
		return Shape{}, err
	}
	return b.publish(stored{analytic: body, result: Shape{
		GeometryAssertions: Assertions{true, body.volume, body.area, 1},
		Bounds:             Bounds{coordinates(box.Min), coordinates(box.Max)},
	}})
}

func (a *analyticBody) tessellate(o parametric.TessellationOptions) (mesh.Mesh, error) {
	if a.patches != nil {
		// Rectangular-patch tessellation compares normals at centroids.
		// Use a finer bound for the curved preview than the generic centroid test.
		o.Angle /= 4
		if o.Angle == 0 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		return a.patches.Tessellate(context.Background(), o)
	}
	return a.solid.Tessellate(context.Background(), o)
}

func (a *analyticBody) transformed(m geom.Affine) (*analyticBody, error) {
	if err := rigid(m); err != nil {
		return nil, err
	}
	out := *a
	if a.segment != nil {
		s := *a.segment
		s.origin = m.Apply(s.origin)
		s.x, s.y, s.z = m.ApplyVector(s.x), m.ApplyVector(s.y), m.ApplyVector(s.z)
		out.segment = &s
	}
	// Preserve the analytic boundary for every rigid map. Requalify an axis
	// recipe only for coordinate-aligned proper rotations.
	out.sector = nil
	out.shaft = nil
	if a.shaft != nil {
		p := *a.shaft
		f, e := geom.NewFrame(m.Apply(p.Frame.Origin()), m.ApplyVector(p.Frame.XAxis()), m.ApplyVector(p.Frame.ZAxis()))
		if e == nil && cardinalFrame(f) && f.YAxis() == m.ApplyVector(p.Frame.YAxis()) {
			p.Frame = f
			out.shaft = &p
		}
	}
	if a.sector != nil {
		r := *a.sector
		f, e := geom.NewFrame(m.Apply(r.frame.Origin()), m.ApplyVector(r.frame.XAxis()), m.ApplyVector(r.frame.ZAxis()))
		if e == nil && cardinalFrame(f) && f.YAxis() == m.ApplyVector(r.frame.YAxis()) {
			r.frame = f
			out.sector = &r
		}
	}
	var err error
	out.solid, err = a.solid.Transformed(m)
	if err != nil {
		return nil, err
	}
	if a.patches != nil {
		p, e := a.patches.Transformed(m)
		if e != nil {
			return nil, e
		}
		out.patches = &p
	}
	return &out, nil
}

func (a *analyticBody) planarFace(origin, normal geom.Vec3) (PlanarFace, error) {
	n, err := normal.Unit()
	if err != nil || !origin.IsFinite() {
		return PlanarFace{}, errors.New("invalid planar reference")
	}
	found := -1
	var result PlanarFace
	for i, f := range a.solid.Faces() {
		surface, m := parametric.SurfaceMapping(f.Surface)
		plane, ok := surface.(geom.PlaneSurface)
		if !ok {
			continue
		}
		o := m.Apply(plane.Frame().Origin())
		x := m.ApplyVector(plane.Frame().XAxis())
		y := m.ApplyVector(plane.Frame().YAxis())
		faceNormal, err := x.Cross(y).Unit()
		if err != nil {
			return PlanarFace{}, err
		}
		if math.Abs(math.Abs(faceNormal.Dot(n))-1) > 1e-9 || math.Abs(o.Sub(origin).Dot(faceNormal)) > 1e-7 {
			continue
		}
		if found >= 0 {
			return PlanarFace{}, errors.New("planar face reference is ambiguous")
		}
		found = i
		if f.Reversed {
			faceNormal = faceNormal.Scale(-1)
		}
		result = PlanarFace{Face: i, Origin: coordinates(o), Normal: coordinates(faceNormal)}
	}
	if found < 0 {
		return PlanarFace{}, errors.New("planar face reference no longer exists")
	}
	result.Area, err = a.solid.PlanarFaceArea(found)
	return result, err
}
