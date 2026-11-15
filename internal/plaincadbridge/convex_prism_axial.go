package plaincadbridge

import (
	"context"
	"math"

	"plainsolid/brep"
	"plainsolid/csg/draft"
	"plainsolid/geom"
)

func nativeAxialEdgeSelection(stock brep.Solid, requestedAxis, normal geom.Vec3, resolved [][2]geom.Vec3) (geom.Frame, []int, bool) {
	axis, err := geom.FrameFromNormal(geom.Vec3{}, requestedAxis)
	if err != nil || !cardinalFrame(axis) || normal != axis.ZAxis() && normal != axis.ZAxis().Scale(-1) {
		return geom.Frame{}, nil, false
	}
	vertices, edges := stock.Vertices(), stock.Edges()
	selected := make(map[int]bool, len(resolved))
	ids := make([]int, 0, len(resolved))
	for _, pair := range resolved {
		a, b := axis.ToLocal(pair[0]), axis.ToLocal(pair[1])
		if a.X != b.X || a.Y != b.Y || a.Z == b.Z {
			return geom.Frame{}, nil, false
		}
		match := -1
		for i, edge := range edges {
			start, end := vertices[edge.Start], vertices[edge.End]
			if start == pair[0] && end == pair[1] || start == pair[1] && end == pair[0] {
				match = i
				break
			}
		}
		if match < 0 {
			return geom.Frame{}, nil, false
		}
		if !selected[match] {
			selected[match] = true
			ids = append(ids, match)
		}
	}
	return axis, ids, true
}

func (b *Bridge) chamferConvexPrismAxialEdges(stock brep.Solid, requestedAxis, normal geom.Vec3, resolved [][2]geom.Vec3, setback float64) (Shape, bool, error) {
	axis, ids, ok := nativeAxialEdgeSelection(stock, requestedAxis, normal, resolved)
	if !ok || !pairedPrismLayers(stock, axis) {
		return Shape{}, false, nil
	}
	faces := stock.Faces()
	selections := make([]draft.EdgeChamfer, 0, len(ids))
	for _, id := range ids {
		first := -1
		for i, face := range faces {
			for _, edge := range face.Boundary {
				if edge.Edge == id {
					first = i
					break
				}
			}
			if first >= 0 {
				break
			}
		}
		if first < 0 {
			return Shape{}, false, nil
		}
		selections = append(selections, draft.EdgeChamfer{Edge: id, Face: first, Setback: setback, OtherSetback: setback})
	}
	result, err := draft.Chamfer(context.Background(), stock, selections, false, draft.Options{})
	if err != nil {
		return Shape{}, true, unsupportedEdge("convex prism chamfer: " + err.Error())
	}
	if result.ExactVolume.Sign() <= 0 || result.MaxSquaredDisplacement.Sign() != 0 {
		return Shape{}, true, unsupportedEdge("convex prism chamfer needs positive material and exactly representable bevel vertices")
	}
	for _, face := range result.OldToNew {
		if face < 0 {
			return Shape{}, true, unsupportedEdge("convex prism chamfer consumes an original support face")
		}
	}
	for _, face := range result.EdgeFaces {
		if face < 0 {
			return Shape{}, true, unsupportedEdge("convex prism chamfer consumes a selected bevel face")
		}
	}
	out, err := b.put(result.Solid)
	return out, true, err
}

// pairedPrismLayers checks the original vertices before convex reconstruction.
// The chamfer proof still validates the stock's original support and topology.
func pairedPrismLayers(stock brep.Solid, axis geom.Frame) bool {
	vertices := stock.Vertices()
	if len(vertices) < 6 || len(vertices)%2 != 0 {
		return false
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, p := range vertices {
		z := axis.ToLocal(p).Z
		lo, hi = min(lo, z), max(hi, z)
	}
	if lo == hi {
		return false
	}
	layers := make(map[geom.Vec2]uint8, len(vertices)/2)
	for _, vertex := range vertices {
		p := axis.ToLocal(vertex)
		key := geom.Vec2{X: p.X, Y: p.Y}
		var layer uint8
		switch p.Z {
		case lo:
			layer = 1
		case hi:
			layer = 2
		default:
			return false
		}
		if layers[key]&layer != 0 {
			return false
		}
		layers[key] |= layer
	}
	return len(layers) == len(vertices)/2
}
