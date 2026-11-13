package plaincadbridge

import (
	"context"
	"errors"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// analyticBody owns an analytic boundary. Some revolutions also retain a
// rectangular patch representation for shared-grid display at polar caps.
type analyticBody struct {
	planarBores *planarBoredStock
	// Endpoints of the proved outer polygon; strictly interior holes cannot change extrema.
	polygonExtrema []geom.Vec3
	solid          trimmed.Solid
	patches        *parametric.Solid
	rimPatches     bool
	sector         *radialSector
	shaft          *trimmed.SteppedShaft
	segment        *circularSegmentSweep
	arcLoop        *arcLoopSupport
	helical        bool
	lead           *leadPart
	helicalMesh    *mesh.Mesh
	stepRounded    *step.RoundedRectangleExtrusionBody
	pockets        *pocketStock
	coaxial        *coaxialBody
	rounded        *roundedStock
	annular        *annularStock
	threadStock    *planarThreadStock
	islandPocket   *trimmed.RectangularIslandPocket
	filletedCap    *filletedCapStock
	volume, area   float64
}

func (b *Bridge) putAnalytic(s trimmed.Solid, volume, area float64) (Shape, error) {
	return b.putAnalyticBody(&analyticBody{solid: s, volume: volume, area: area})
}

func (b *Bridge) putAnalyticBody(body *analyticBody) (Shape, error) {
	if !finite(body.volume) || !finite(body.area) || body.volume <= 0 || body.area <= 0 {
		return Shape{}, errors.New("invalid analytic solid measures")
	}
	if len(body.solid.Topology().Shells) != 1 && (body.planarBores == nil || !body.planarBores.connected) {
		return Shape{}, errors.New("expected one connected analytic shell")
	}
	var box geom.Box
	var err error
	if len(body.polygonExtrema) != 0 {
		box, err = geom.Bounds(body.polygonExtrema)
	} else if body.arcLoop != nil {
		box = body.arcLoop.bounds
	} else if body.segment != nil {
		box, err = body.segment.bounds()
	} else if body.coaxial != nil {
		box, err = body.coaxial.bounds()
	} else if body.islandPocket != nil {
		box, err = islandPocketBounds(*body.islandPocket)
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
	if a.coaxial != nil {
		return a.coaxial.tessellate(o)
	}
	if a.annular != nil && !a.annular.cut {
		return a.annular.Tessellate(context.Background(), o)
	}
	if a.helicalMesh != nil && o.Deflection >= .5 && o.Angle >= .1 && len(a.helicalMesh.Triangles) <= o.MaxTriangles {
		return *a.helicalMesh, nil
	}
	if a.helical {
		o.MaxTriangles = min(o.MaxTriangles, 100000)
		return a.solid.TessellateHelical(context.Background(), o)
	}
	if a.segment != nil || a.arcLoop != nil {
		// Segment rims retain the same facet-angle budget as full circles.
		o.Angle = min(o.Angle, .025)
	}
	if a.patches != nil {
		// Rectangular-patch tessellation compares normals at centroids.
		// Use a finer bound for the curved preview than the generic centroid test.
		o.Angle /= 4
		if o.Angle == 0 {
			return mesh.Mesh{}, parametric.ErrBudget
		}
		if a.rimPatches {
			return a.patches.TessellateRevolutionRims(context.Background(), o)
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
	if len(a.polygonExtrema) != 0 {
		out.polygonExtrema = make([]geom.Vec3, len(a.polygonExtrema))
		for i, p := range a.polygonExtrema {
			out.polygonExtrema[i] = m.Apply(p)
		}
	}
	if a.arcLoop != nil {
		p := *a.arcLoop
		// Reflection changes the sign of the frame's Y axis. Reflect the
		// profile in the rebuilt right-handed chart as well.
		if m.Determinant() < 0 {
			p.segments = append([]trimmed.ExtrusionSegment(nil), p.segments...)
			for i := range p.segments {
				s := &p.segments[i]
				s.Start.Y, s.End.Y, s.Center.Y = -s.Start.Y, -s.End.Y, -s.Center.Y
				s.StartAngle, s.Sweep = -s.StartAngle, -s.Sweep
			}
		}
		f, err := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
		if err != nil {
			return nil, err
		}
		p.frame = f
		p.bounds, err = trimmed.ArcLoopBounds(f, p.segments, p.height)
		if err != nil {
			return nil, err
		}
		out.arcLoop = &p
	}
	if a.segment != nil {
		s := *a.segment
		s.origin = m.Apply(s.origin)
		s.x, s.y, s.z = m.ApplyVector(s.x), m.ApplyVector(s.y), m.ApplyVector(s.z)
		out.segment = &s
	}
	// Preserve the analytic boundary for every rigid map. Requalify an axis
	// recipe only for coordinate-aligned proper rotations.
	out.sector = nil
	out.stepRounded = nil
	out.shaft = nil
	out.pockets = nil
	out.planarBores = nil
	out.coaxial = nil
	out.rounded = nil
	out.annular = nil
	out.threadStock = nil
	out.islandPocket = nil
	out.filletedCap = nil
	if a.coaxial != nil {
		out.coaxial, _ = a.coaxial.transformed(m)
	}
	if a.planarBores != nil {
		p := *a.planarBores
		f, e := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
		if e == nil && cardinalFrame(f) && f.YAxis() == m.ApplyVector(p.frame.YAxis()) {
			p.frame = f
			p.base, e = p.base.Transformed(m)
			if e != nil {
				return nil, e
			}
			out.planarBores = &p
		}
	}
	if a.stepRounded != nil {
		p := *a.stepRounded
		f, e := geom.NewFrame(m.Apply(p.Frame.Origin()), m.ApplyVector(p.Frame.XAxis()), m.ApplyVector(p.Frame.ZAxis()))
		if e == nil && cardinalFrame(f) && f.YAxis() == m.ApplyVector(p.Frame.YAxis()) {
			p.Frame = f
			out.stepRounded = &p
		}
	}
	if a.pockets != nil {
		p := *a.pockets
		f, e := geom.NewFrame(m.Apply(p.frame.Origin()), m.ApplyVector(p.frame.XAxis()), m.ApplyVector(p.frame.ZAxis()))
		if e == nil && cardinalFrame(f) && f.YAxis() == m.ApplyVector(p.frame.YAxis()) {
			p.frame = f
			out.pockets = &p
		}
	}
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
	if a.helical {
		var display mesh.Mesh
		options := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 100000}
		out.solid, display, err = a.solid.TransformedHelical(context.Background(), m, options)
		out.helicalMesh = &display
	} else {
		out.solid, err = a.solid.Transformed(m)
	}
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
