package circularsweep

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type Insets struct {
	Native   trimmed.CircularSweepInsets
	Supports []Support
	Rims     []trimmed.ExactCircularLoopSegment
}

func BuildInsets(ctx context.Context, host trimmed.ExactCircularSweep, guests []trimmed.ExactCircularSweep) (Insets, error) {
	var zero Insets
	p, err := trimmed.JoinExactCircularSweepInsets(ctx, host, guests, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return zero, err
	}
	out := Insets{Native: p, Supports: make([]Support, len(p.Solid.Faces())), Rims: make([]trimmed.ExactCircularLoopSegment, len(p.Solid.Edges()))}
	seen := make([]bool, len(out.Supports))
	for i, original := range append([]trimmed.ExactCircularSweep{host}, guests...) {
		var faceMap [][]int
		var edgeOffset int
		if i == 0 {
			faceMap = p.HostFaces
		} else {
			g := p.Guests[i-1]
			faceMap = make([][]int, len(g.Faces))
			for j, k := range g.Faces {
				if k >= 0 {
					faceMap[j] = []int{k}
				}
			}
			edgeOffset = g.EdgeOffset
		}
		segments := append([]trimmed.ExactCircularLoopSegment(nil), original.Outer...)
		for _, hole := range original.Holes {
			segments = append(segments, hole...)
		}
		n := len(segments)
		for j, curve := range segments {
			out.Rims[edgeOffset+j], out.Rims[edgeOffset+n+j] = curve, curve
		}
		for j, fragments := range faceMap {
			s := Support{Z: original.Bottom}
			if j == 1 {
				s.Z = original.Top
			} else if j >= 2 {
				s.Wall, s.Segment = true, segments[j-2]
			}
			for _, output := range fragments {
				if seen[output] {
					return zero, geom.ErrInvalid
				}
				seen[output], out.Supports[output] = true, s
			}
		}
	}
	for _, ok := range seen {
		if !ok {
			return zero, geom.ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
