// Package circularsweep adapts proved native circular sweeps to the analytic
// supports needed by STEP and IGES writers.
package circularsweep

import (
	"context"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type Support struct {
	Z       float64
	Wall    bool
	Segment trimmed.ExactCircularLoopSegment
}

type Overlay struct {
	Native   trimmed.CircularSweepCapOverlay
	Supports []Support
	Rims     []trimmed.ExactCircularLoopSegment
}

func BuildCapOverlay(ctx context.Context, a, b trimmed.ExactCircularSweep) (Overlay, error) {
	var zero Overlay
	p, err := trimmed.JoinExactCircularSweepCapOverlay(ctx, a, b, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		return zero, err
	}
	fs, es, vs := p.Solid.Faces(), p.Solid.Edges(), p.Solid.Vertices()
	out := Overlay{Native: p, Supports: make([]Support, len(fs)), Rims: make([]trimmed.ExactCircularLoopSegment, len(es))}
	writtenFaces, writtenRims := make([]bool, len(fs)), make([]bool, len(es))
	for k, src := range [2]trimmed.ExactCircularSweep{a, b} {
		for cap, h := range [2]float64{src.Bottom, src.Top} {
			for _, face := range p.Faces[k][cap] {
				out.Supports[face], writtenFaces[face] = Support{Z: h}, true
			}
		}
		n := len(p.Faces[k]) - 2
		i := 0
		for _, ring := range append([][]trimmed.ExactCircularLoopSegment{src.Outer}, src.Holes...) {
			for _, original := range ring {
				walls, lower, upper := p.Faces[k][i+2], p.Edges[k][i], p.Edges[k][n+i]
				if len(walls) == 0 || len(lower) != len(walls) || len(upper) != len(walls) {
					return zero, geom.ErrInvalid
				}
				for j, face := range walls {
					e0, e1 := lower[j].Edge, upper[j].Edge
					e := es[e0]
					a, b := vs[e.Start], vs[e.End]
					span := original
					span.Start, span.End = geom.Vec2{X: a.X, Y: a.Y}, geom.Vec2{X: b.X, Y: b.Y}
					if span.Radius == 0 {
						span.Center = geom.Vec2{}
					}
					for _, id := range []int{e0, e1} {
						if writtenRims[id] && out.Rims[id] != span {
							return zero, geom.ErrInvalid
						}
						out.Rims[id], writtenRims[id] = span, true
					}
					if writtenFaces[face] {
						return zero, geom.ErrInvalid
					}
					out.Supports[face], writtenFaces[face] = Support{Z: src.Bottom, Wall: true, Segment: span}, true
				}
				i++
			}
		}
	}
	for _, ok := range writtenFaces {
		if !ok {
			return zero, geom.ErrInvalid
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
