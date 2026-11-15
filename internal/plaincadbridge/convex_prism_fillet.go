package plaincadbridge

import (
	"context"
	"errors"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) filletConvexPrismAxialEdges(stock brep.Solid, requestedAxis, normal geom.Vec3, resolved [][2]geom.Vec3, radius float64) (Shape, bool, error) {
	axis, e := geom.FrameFromNormal(geom.Vec3{}, requestedAxis)
	if e != nil || !cardinalFrame(axis) || normal != axis.ZAxis() && normal != axis.ZAxis().Scale(-1) {
		return Shape{}, false, nil
	}
	for _, pair := range resolved {
		a, c := axis.ToLocal(pair[0]), axis.ToLocal(pair[1])
		if a.X != c.X || a.Y != c.Y || a.Z == c.Z {
			return Shape{}, false, nil
		}
	}
	vertices, native := stock.Vertices(), stock.Edges()
	selected := make(map[int]bool, len(resolved))
	requests := make([]trimmed.ConvexPrismEdgeRadius, 0, len(resolved))
	for _, pair := range resolved {
		match := -1
		for i, edge := range native {
			a, c := vertices[edge.Start], vertices[edge.End]
			if a == pair[0] && c == pair[1] || a == pair[1] && c == pair[0] {
				match = i
				break
			}
		}
		if match < 0 {
			return Shape{}, false, nil
		}
		if !selected[match] {
			selected[match] = true
			requests = append(requests, trimmed.ConvexPrismEdgeRadius{Edge: match, Radius: radius})
		}
	}
	x, y, z := axis.XAxis(), axis.YAxis(), axis.ZAxis()
	toLocal := geom.Affine{{x.X, x.Y, x.Z, 0}, {y.X, y.Y, y.Z, 0}, {z.X, z.Y, z.Z, 0}}
	toWorld := geom.Affine{{x.X, y.X, z.X, 0}, {x.Y, y.Y, z.Y, 0}, {x.Z, y.Z, z.Z, 0}}
	local, e := stock.Transformed(toLocal)
	if e != nil {
		return Shape{}, false, nil
	}
	ctx := context.Background()
	s, h, e := trimmed.FilletConvexPrismEdges(ctx, local, requests, max(1, radius)*1e-10)
	if e != nil {
		if errors.Is(e, geom.ErrDomain) || errors.Is(e, geom.ErrUnrepresentable) || errors.Is(e, geom.ErrConvergence) {
			return Shape{}, true, unsupportedEdge("convex prism fillet: " + e.Error())
		}
		return Shape{}, true, e
	}
	profile := h.Section
	worldFrame, e := geom.NewFrame(toWorld.Apply(profile.Frame.Origin()),
		toWorld.ApplyVector(profile.Frame.XAxis()), toWorld.ApplyVector(profile.Frame.ZAxis()))
	if e != nil {
		return Shape{}, true, e
	}
	if toWorld != geom.Identity() {
		s, e = s.TransformedContext(ctx, toWorld)
		if e != nil {
			return Shape{}, true, e
		}
	}
	box, e := trimmed.ArcLoopBounds(worldFrame, profile.Segments, profile.Height)
	if e != nil {
		return Shape{}, true, e
	}
	out, e := b.putArcLoop(trimmed.ArcLoopExtrusion{Solid: s, Area: profile.Area, Perimeter: profile.Perimeter, Bounds: box},
		worldFrame, profile.Segments, profile.Height)
	return out, true, e
}
