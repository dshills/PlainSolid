package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func (b *Bridge) extrudeArcLoop(p mixedProfileRequest) (Shape, error) {
	if p.Origin == nil || p.Normal == nil || p.XDirection == nil ||
		len(p.Segments) < 2 || len(p.Segments) > 64 || len(p.Circles) != 0 || len(p.Outer) != 0 ||
		len(p.Min) != 0 || len(p.Max) != 0 || len(p.Radii) != 0 || len(p.Center) != 0 ||
		len(p.Right) != 0 || p.InnerRadius != 0 || p.Radius != 0 || p.StartAngle != 0 || p.Sweep != 0 {
		return Shape{}, geom.ErrDomain
	}
	frame, err := geom.NewFrame(p.Origin.vec(), p.XDirection.vec(), p.Normal.vec())
	if err != nil {
		return Shape{}, err
	}
	if frame.XAxis() != p.XDirection.vec() || frame.ZAxis() != p.Normal.vec() {
		return Shape{}, unsupportedEdge("arc loop requires a represented orthonormal sketch frame")
	}
	segments := make([]trimmed.ExtrusionSegment, len(p.Segments))
	height := float64(p.Height)
	scale := max(1, math.Abs(height))
	for i, in := range p.Segments {
		s := &segments[i]
		s.Start, err = mixedPoint(in.Start)
		if err != nil {
			return Shape{}, err
		}
		s.End, err = mixedPoint(in.End)
		if err != nil {
			return Shape{}, err
		}
		if in.Radius != 0 {
			s.Center, err = mixedPoint(in.Center)
			if err != nil {
				return Shape{}, err
			}
		} else if len(in.Center) != 0 {
			return Shape{}, geom.ErrDomain
		}
		s.Radius, s.StartAngle, s.Sweep = float64(in.Radius), float64(in.StartAngle), float64(in.Sweep)
		scale = max(scale, s.Radius, math.Hypot(s.End.X-s.Start.X, s.End.Y-s.Start.Y))
	}
	result, err := trimmed.ExtrudeArcLoop(context.Background(), frame, segments, height, scale*1e-10)
	if err != nil {
		return Shape{}, err
	}
	return b.putArcLoop(result, frame, segments, height)
}

func (b *Bridge) putArcLoop(result trimmed.ArcLoopExtrusion, frame geom.Frame, segments []trimmed.ExtrusionSegment, height float64) (Shape, error) {
	body := &analyticBody{solid: result.Solid, volume: result.Area * math.Abs(height),
		area:    2*result.Area + result.Perimeter*math.Abs(height),
		arcLoop: &arcLoopSupport{frame: frame, segments: append([]trimmed.ExtrusionSegment(nil), segments...), height: height, bounds: result.Bounds}}
	return b.putAnalyticBody(body)
}

type arcLoopSupport struct {
	frame    geom.Frame
	segments []trimmed.ExtrusionSegment
	height   float64
	bounds   geom.Box
}
