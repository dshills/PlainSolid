package fillet

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// prepareTangentPocket qualifies only a rectangular through pocket whose one
// edge coincides with an upper fillet tangent. Other boundaries remain strict.
func prepareTangentPocket(ctx context.Context, stock LongEdgeTreatment, pocket *trimmed.ExtrusionRectangle, tol float64) (LongEdgeTreatment, float64, bool, error) {
	if pocket == nil {
		return stock, 0, false, nil
	}
	low, high := pocket.Min, pocket.Max
	if !finite(low.X) || !finite(low.Y) || !finite(high.X) || !finite(high.Y) {
		return stock, 0, false, ErrInvalid
	}
	side := -1
	y := 0.
	if low.Y == stock.TopMinY && stock.CornerFilletFaces[3] >= 0 {
		side = 3
		y = low.Y
	}
	if high.Y == stock.TopMaxY && stock.CornerFilletFaces[2] >= 0 {
		if side >= 0 {
			return stock, 0, false, ErrRadius
		}
		side = 2
		y = high.Y
	}
	if side < 0 {
		return stock, 0, false, nil
	}
	pad := qm(num(8), num(tol))
	if num(low.X).Cmp(pad) <= 0 || qs(num(stock.Length), num(high.X)).Cmp(pad) <= 0 || qs(num(high.X), num(low.X)).Cmp(pad) <= 0 || qs(num(high.Y), num(low.Y)).Cmp(pad) <= 0 || qs(num(low.Y), num(stock.BottomMinY)).Cmp(pad) <= 0 || qs(num(stock.BottomMaxY), num(high.Y)).Cmp(pad) <= 0 {
		return stock, 0, false, ErrRadius
	}
	if side == 3 && qs(num(stock.TopMaxY), num(high.Y)).Cmp(pad) <= 0 || side == 2 && qs(num(low.Y), num(stock.TopMinY)).Cmp(pad) <= 0 {
		return stock, 0, false, ErrRadius
	}
	s := stock.Solid
	for _, x := range []float64{low.X, high.X} {
		found := false
		for id, f := range s.Faces() {
			surface, _ := parametric.SurfaceMapping(f.Surface)
			if _, ok := surface.(geom.CylinderSurface); !ok {
				continue
			}
			v, edges := s.Vertices(), s.Edges()
			var anchors []trimmed.EdgeAnchor
			touches := false
			for _, u := range f.Outer {
				edge := edges[u.Edge]
				a, b := v[edge.Start], v[edge.End]
				if a.X == b.X || x <= math.Min(a.X, b.X) || x >= math.Max(a.X, b.X) {
					continue
				}
				if math.Abs(a.Y-y) <= tol && math.Abs(a.Z-stock.Height) <= tol && math.Abs(b.Y-y) <= tol && math.Abs(b.Z-stock.Height) <= tol {
					touches = true
				}
				base, _ := parametric.CurveMapping(edge.Curve)
				if _, ok := base.(geom.Line); !ok {
					return stock, 0, false, ErrInvalid
				}
				t := edge.Range.Min + (x-a.X)/(b.X-a.X)*(edge.Range.Max-edge.Range.Min)
				anchors = append(anchors, trimmed.EdgeAnchor{Edge: u.Edge, Parameter: t})
			}
			if !touches {
				continue
			}
			if len(anchors) != 2 || found {
				return stock, 0, false, ErrInvalid
			}
			var e error
			s, _, e = s.SplitFaceAcrossEdges(ctx, id, anchors[0], anchors[1], func(a, b geom.Vec2) ([]trimmed.CutSegment, error) {
				return []trimmed.CutSegment{{Start: a, End: b}}, nil
			})
			if e != nil {
				return stock, 0, false, e
			}
			found = true
			break
		}
		if !found {
			return stock, 0, false, ErrInvalid
		}
	}
	stock.Solid = s
	return stock, y, true, nil
}

func mergeTangentPocketLoop(outer, hole []trimmed.Use, shared int) ([]trimmed.Use, error) {
	oi, hi := -1, -1
	for i, u := range outer {
		if u.Edge == shared {
			oi = i
		}
	}
	for i, u := range hole {
		if u.Edge == shared {
			hi = i
		}
	}
	if oi < 0 || hi < 0 || outer[oi].Reversed == hole[hi].Reversed {
		return nil, ErrInvalid
	}
	result := append([]trimmed.Use(nil), outer[:oi]...)
	for k := 1; k < len(hole); k++ {
		result = append(result, hole[(hi+k)%len(hole)])
	}
	result = append(result, outer[oi+1:]...)
	return result, nil
}
