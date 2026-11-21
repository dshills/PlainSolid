package plaincadbridge

import (
	"plainsolid/brep"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

var disjointBoreAxes = [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}

func disjointBoreSTEPRecipe(body *boreBody) (step.DisjointBoresBody, bool) {
	var noRecipe step.DisjointBoresBody
	vertices := body.base.Vertices()
	if body.union != nil || body.layered != nil ||
		len(body.cylinders) < 2 || len(vertices) != 8 ||
		len(body.base.Edges()) != 12 || len(body.base.Faces()) != 6 {
		return noRecipe, false
	}
	bounds, err := geom.Bounds(vertices)
	if err != nil {
		return noRecipe, false
	}
	frame, err := geom.NewFrame(bounds.Min, disjointBoreAxes[0], disjointBoreAxes[2])
	size := bounds.Max.Sub(bounds.Min)
	if err != nil || frame.ToWorld(size) != bounds.Max ||
		!rectangularFaces(body.base, frame, geom.Vec3{}, size) {
		return noRecipe, false
	}
	p := step.DisjointBoresBody{Name: "PlainSolid", Frame: frame, Dimensions: size}
	var axes [3]bool
	for i, tool := range body.cylinders {
		if !cardinalFrame(tool.frame) {
			return noRecipe, false
		}
		direction := tool.frame.ZAxis()
		axis := 0
		for direction.Dot(disjointBoreAxes[axis]) == 0 {
			axis++
		}
		world := coordinates(tool.frame.Origin())
		local := coordinates(frame.ToLocal(tool.frame.Origin()))
		// The point's axial coordinate does not affect the line, including
		// when a finite tool starts outside the stock cap.
		local[axis] = 0
		center := geom.Vec3{X: local[0], Y: local[1], Z: local[2]}
		recovered := coordinates(frame.ToWorld(center))
		recovered[axis] = world[axis]
		if recovered != world {
			return noRecipe, false
		}
		axes[axis] = true
		site := step.DisjointBore{Axis: axis, Center: center, Radius: tool.radius}
		if body.blind[i] {
			p.Blind = append(p.Blind, step.DisjointBlindBore{Bore: site, Depth: body.spans[i], OppositeCap: direction.Dot(disjointBoreAxes[axis]) < 0})
		} else {
			p.Bores = append(p.Bores, site)
		}
	}
	return p, axes[0] && axes[1] || axes[0] && axes[2] || axes[1] && axes[2]
}

func disjointBoresFromSTEP(p step.DisjointBoresBody, unit float64) (*boreBody, error) {
	scale := unit / .001
	if !finite(scale) || scale <= 0 {
		return nil, geom.ErrInvalid
	}
	if !cardinalFrame(p.Frame) {
		return nil, geom.ErrDomain
	}
	origin, end := p.Frame.Origin().Scale(scale), p.Frame.ToWorld(p.Dimensions).Scale(scale)
	bounds, err := geom.Bounds([]geom.Vec3{origin, end})
	if err != nil {
		return nil, err
	}
	base, err := brep.Box(bounds.Min, bounds.Max)
	if err != nil {
		return nil, err
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
			return nil, geom.ErrInvalid
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
			return nil, geom.ErrUnrepresentable
		}
		frame, err := geom.FrameFromNormal(start, direction)
		if err != nil {
			return nil, err
		}
		tool, err := newCylinder(frame, site.Radius*scale, length)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return newBore(base, tools)
}
