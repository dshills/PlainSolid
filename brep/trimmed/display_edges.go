package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ErrDisplaySegmentBudget signals that sampling the requested native edge
// lines would exceed the caller's output segment budget.
var ErrDisplaySegmentBudget = fmt.Errorf("native edges exceed the display segment budget: %w", parametric.ErrBudget)

// DisplayEdges samples sharp model edges. Face normals suppress regular
// tangent connections, and two uses by the same face identify a seam.
// maxUses and maxSegments bound traversal and output independently.
func (s Solid) DisplayEdges(ctx context.Context, maxUses, maxSegments int) ([]geom.Vec3, int, error) {
	if ctx == nil || maxUses < 1 || maxSegments < 1 {
		return nil, 0, geom.ErrInvalid
	}
	type owner struct {
		face int
		use  Use
	}
	owners := make([][2]owner, len(s.edges))
	counts := make([]int, len(s.edges))
	uses := 0
	for i, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				if e := ctx.Err(); e != nil {
					return nil, uses, e
				}
				if uses >= maxUses {
					return nil, uses, fmt.Errorf("display edge use budget exhausted: %w", parametric.ErrBudget)
				}
				uses++
				owners[u.Edge][counts[u.Edge]] = owner{i, u}
				counts[u.Edge]++
			}
		}
	}
	normalAt := func(o owner, t float64) (geom.Vec3, error) {
		p, err := (trimSide{use: o.use}).point(t)
		if err != nil {
			return geom.Vec3{}, err
		}
		f := s.faces[o.face]
		u, v, err := f.Surface.Partials(p.X, p.Y)
		if err != nil {
			return geom.Vec3{}, err
		}
		n, err := normal(u, v)
		if f.Reversed {
			n = n.Scale(-1)
		}
		return n, err
	}
	lines := []geom.Vec3{}
	for i, edge := range s.edges {
		if e := ctx.Err(); e != nil {
			return nil, uses, e
		}
		if edge.Curve == nil || counts[i] != 2 || owners[i][0].face == owners[i][1].face {
			continue
		}
		smooth := true
		for _, t := range []float64{.25, .5, .75} {
			x, ex := normalAt(owners[i][0], t)
			y, ey := normalAt(owners[i][1], t)
			if ex != nil || ey != nil || x.Dot(y) < .99999 {
				smooth = false
				break
			}
		}
		if smooth {
			continue
		}
		remaining := maxSegments - len(lines)/2
		if remaining <= 0 {
			return nil, uses, ErrDisplaySegmentBudget
		}
		if compactStraightEdge(edge.Curve) {
			lines = append(lines, s.vertices[edge.Start], s.vertices[edge.End])
			continue
		}
		options := geom.CurveTessellationOptions{
			ChordTolerance: .02, InitialSegments: min(4, remaining), MaxDepth: 12,
			MaxEvaluations: min(4096, remaining*12+64),
		}
		chords, err := geom.TessellateCurve(edge.Curve, edge.Range.Min, edge.Range.Max, options)
		if errors.Is(err, geom.ErrConvergence) {
			return nil, uses, ErrDisplaySegmentBudget
		}
		if err != nil {
			return nil, uses, fmt.Errorf("invalid display curve at edge %d: %w", i, err)
		}
		chords[0].Start = s.vertices[edge.Start]
		chords[len(chords)-1].End = s.vertices[edge.End]
		for _, chord := range chords {
			if span := chord.Start.Sub(chord.End).Length(); finite(span) && span > 1e-9 {
				if len(lines)/2 >= maxSegments {
					return nil, uses, ErrDisplaySegmentBudget
				}
				lines = append(lines, chord.Start, chord.End)
			}
		}
	}
	return lines, uses, nil
}
