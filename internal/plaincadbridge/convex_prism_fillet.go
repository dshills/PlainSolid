package plaincadbridge

import (
	"context"
	"errors"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) filletConvexPrismAxialEdges(stock brep.Solid, requestedAxis, normal geom.Vec3, resolved [][2]geom.Vec3, radius float64) (Shape, bool, error) {
	axis, ids, ok := nativeAxialEdgeSelection(stock, requestedAxis, normal, resolved)
	if !ok {
		return Shape{}, false, nil
	}
	requests := make([]trimmed.ConvexPrismEdgeRadius, len(ids))
	for i, id := range ids {
		requests[i] = trimmed.ConvexPrismEdgeRadius{Edge: id, Radius: radius}
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
