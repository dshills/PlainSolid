package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type pocketStock struct {
	frame   geom.Frame
	size    geom.Vec3
	pockets []trimmed.BoxBlindPocket
	bores   []trimmed.ExtrusionCircle
}

func (b *Bridge) putPocketStock(p pocketStock) (Shape, error) {
	s, err := trimmed.ExtrudeBoxWithPocketsAndBores(context.Background(), p.frame, p.size, p.pockets, p.bores)
	if err != nil {
		return Shape{}, err
	}
	x, y, z := p.size.X, p.size.Y, p.size.Z
	volume, area := x*y*z, 2*(x*y+x*z+y*z)
	top := z
	for _, cut := range p.pockets {
		w := cut.Footprint.Max.X - cut.Footprint.Min.X
		h := cut.Footprint.Max.Y - cut.Footprint.Min.Y
		depth := top - cut.Floor
		volume -= w * h * depth
		area += 2 * (w + h) * depth
		top = cut.Floor
	}
	for _, bore := range p.bores {
		r := bore.Radius
		volume -= math.Pi * r * r * z
		area += 2 * math.Pi * r * (z - r)
	}
	return b.putAnalyticBody(&analyticBody{solid: s, volume: volume, area: area, pockets: &p})
}

func orthogonalPrism(s brep.Solid, f geom.Frame) (geom.Box, bool) {
	vertices := s.Vertices()
	if len(vertices) != 8 || len(s.Faces()) != 6 || len(s.Edges()) != 12 {
		return geom.Box{}, false
	}
	local := make([]geom.Vec3, len(vertices))
	for i, v := range vertices {
		local[i] = f.ToLocal(v)
		if f.ToWorld(local[i]) != v {
			return geom.Box{}, false
		}
	}
	bounds, err := geom.Bounds(local)
	return bounds, err == nil && rectangularFaces(s, f, bounds.Min, bounds.Max)
}

func (b *Bridge) rectangularPocket(a, tool stored) (Shape, bool, error) {
	var p pocketStock
	var extent geom.Box
	if a.analytic != nil && a.analytic.pockets != nil {
		p = *a.analytic.pockets
		bounds, ok := orthogonalPrism(tool.solid, p.frame)
		if !ok {
			return Shape{}, false, nil
		}
		extent = bounds
	} else if a.planar() {
		// Infer the entrance from whichever cardinal stock cap the tool spans.
		// Its other axial end must be strictly interior.
		found := false
		source := a.solid
		for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
			f, err := geom.FrameFromNormal(geom.Vec3{}, axis)
			if err != nil {
				continue
			}
			stock, ok := orthogonalPrism(source, f)
			if !ok {
				continue
			}
			frame, err := geom.NewFrame(f.ToWorld(stock.Min), f.XAxis(), f.ZAxis())
			if err != nil {
				continue
			}
			d := stock.Max.Sub(stock.Min)
			bounds, ok := orthogonalPrism(tool.solid, frame)
			if !ok || bounds.Min.Z <= 0 || bounds.Min.Z >= d.Z || bounds.Max.Z < d.Z ||
				bounds.Min.X <= 0 || bounds.Min.Y <= 0 || bounds.Max.X >= d.X || bounds.Max.Y >= d.Y {
				continue
			}
			if found {
				return Shape{}, false, nil
			}
			p, extent, found = pocketStock{frame: frame, size: d}, bounds, true
		}
		if !found {
			return Shape{}, false, nil
		}
	} else {
		return Shape{}, false, nil
	}
	top := p.size.Z
	if len(p.pockets) > 0 {
		top = p.pockets[len(p.pockets)-1].Floor
	}
	if extent.Min.Z <= 0 || extent.Min.Z >= top || extent.Max.Z != top && len(p.pockets) > 0 || extent.Max.Z < top {
		return Shape{}, false, nil
	}
	p.pockets = append(append([]trimmed.BoxBlindPocket(nil), p.pockets...), trimmed.BoxBlindPocket{
		Footprint: trimmed.ExtrusionRectangle{Min: geom.Vec2{X: extent.Min.X, Y: extent.Min.Y}, Max: geom.Vec2{X: extent.Max.X, Y: extent.Max.Y}}, Floor: extent.Min.Z})
	result, err := b.putPocketStock(p)
	return result, true, err
}

// A pocket-only box has a straight shared-edge graph. Preserve that graph
// when a later planar cut cannot use the nested rectangular recipe.
func (s stored) planarCutBoundary() (brep.Solid, bool, error) {
	if s.planar() {
		return s.solid, true, nil
	}
	if s.analytic == nil || s.analytic.pockets == nil || len(s.analytic.pockets.bores) != 0 {
		return brep.Solid{}, false, nil
	}
	native := s.analytic.solid
	modelEdges := native.Edges()
	edges := make([]brep.Edge, len(modelEdges))
	for i, edge := range modelEdges {
		edges[i] = brep.Edge{Start: edge.Start, End: edge.End}
	}
	convert := func(loop []trimmed.Use, reverse bool) []brep.EdgeUse {
		uses := make([]brep.EdgeUse, len(loop))
		for i, use := range loop {
			j := i
			if reverse {
				j = len(loop) - 1 - i
			}
			uses[j] = brep.EdgeUse{Edge: use.Edge, Reversed: use.Reversed != reverse}
		}
		return uses
	}
	var faces []brep.Face
	for _, face := range native.Faces() {
		polygon := brep.Face{Boundary: convert(face.Outer, face.Reversed)}
		for _, hole := range face.Holes {
			polygon.Holes = append(polygon.Holes, convert(hole, face.Reversed))
		}
		faces = append(faces, polygon)
	}
	solid, err := brep.NewSolid(native.Vertices(), edges, faces)
	return solid, true, err
}
