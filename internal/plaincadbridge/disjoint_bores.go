package plaincadbridge

import (
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

var disjointBoreAxes = [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}

func disjointBoresFromSTEP(p step.DisjointBoresBody, unit float64) (brep.Solid, []*cylinderBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return brep.Solid{}, nil, geom.ErrInvalid
	}
	if !cardinalFrame(p.Frame) {
		return brep.Solid{}, nil, geom.ErrDomain
	}
	origin, end := p.Frame.Origin().Scale(scale), p.Frame.ToWorld(p.Dimensions).Scale(scale)
	bounds, err := geom.Bounds([]geom.Vec3{origin, end})
	if err != nil {
		return brep.Solid{}, nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return brep.Solid{}, nil, err
	}
	axes := [3]geom.Vec3{p.Frame.XAxis(), p.Frame.YAxis(), p.Frame.ZAxis()}
	sizes := coordinates(p.Dimensions)
	tools := make([]*cylinderBody, 0, len(p.Bores))
	sites := make([]step.DisjointBlindBore, 0, len(p.Bores)+len(p.Blind))
	for _, site := range p.Bores {
		sites = append(sites, step.DisjointBlindBore{Bore: site})
	}
	sites = append(sites, p.Blind...)
	for _, item := range sites {
		site := item.Bore
		if site.Axis < 0 || site.Axis >= len(axes) {
			return brep.Solid{}, nil, geom.ErrInvalid
		}
		start := p.Frame.ToWorld(site.Center).Scale(scale)
		end := p.Frame.ToWorld(site.Center.Add(disjointBoreAxes[site.Axis].Scale(sizes[site.Axis]))).Scale(scale)
		direction := axes[site.Axis]
		if item.Depth != 0 {
			if item.OppositeCap {
				start = end
				direction = direction.Scale(-1)
			}
			end = start.Add(direction.Scale(item.Depth * scale))
		}
		length := end.Sub(start).Dot(direction)
		if start.Add(direction.Scale(length)) != end {
			return brep.Solid{}, nil, geom.ErrUnrepresentable
		}
		frame, err := geom.FrameFromNormal(start, direction)
		if err != nil {
			return brep.Solid{}, nil, err
		}
		tool, err := newCylinder(frame, site.Radius*scale, length)
		if err != nil {
			return brep.Solid{}, nil, err
		}
		tools = append(tools, tool)
	}
	return base, tools, nil
}
