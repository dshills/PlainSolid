package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type closedSplineRing struct {
	faces [2]int
	ring  transverseRing
}

func (w *openSplineWire) addClosedRings(ctx context.Context, sections []SplineShellSection, curves [][]intersect.TubeSegment,
	ap, bp []splineEmbeddingFace) error {
	count := 0
	for _, b := range w.branches {
		count += len(b.segments)
	}
	for i, s := range sections {
		if !s.Closed {
			continue
		}
		r, e := transverseRings(ctx, [][]intersect.TubeSegment{curves[i]}, ap[s.FaceA].patch.Domain(), bp[s.FaceB].patch.Domain())
		if e != nil {
			return e
		}
		if count+len(r[0].vertices) > 256 || len(w.rings) == 16 {
			return parametric.ErrBudget
		}
		r[0].base = count
		count += len(r[0].vertices)
		w.rings = append(w.rings, closedSplineRing{[2]int{s.FaceA, s.FaceB}, r[0]})
	}
	return nil
}

// A closed ring is either inside a fragment produced by open paths, or
// inside another closed ring. Distinct branches cannot intersect on the
// embedded source chart. Exact winding at one enclosed point on a ring
// therefore proves which fragment contains its entire boundary.
func (w openSplineWire) insertClosedSplineRings(ctx context.Context, chart int, cycles map[int][]openWireFacePath, model [][2]geom.Vec3, v []geom.Vec3, edges []parametric.Edge,
	faces []Face, original []int, section map[int]int) ([]geom.Vec3, []parametric.Edge, []Face, []int, error) {
	byFace := make([][]transverseRing, len(faces))
	for _, r := range w.rings {
		f := r.faces[chart]
		byFace[f] = append(byFace[f], r.ring)
	}
	for i := range byFace {
		for _, cycle := range cycles[i] {
			r, e := closedOpenWireRing(ctx, faces[i], chart, cycle, model)
			if e != nil {
				return nil, nil, nil, nil, fmt.Errorf("trimmed: closed wire inside face %d: %w", i, e)
			}
			byFace[i] = append(byFace[i], r)
		}
	}
	for face, rings := range byFace {
		if len(rings) == 0 {
			continue
		}
		parent, _, e := transverseRingNesting(ctx, rings, chart)
		if e != nil {
			return nil, nil, nil, nil, e
		}
		regions := make([]int, len(rings))
		for i, r := range rings {
			if parent[i] != -1 {
				continue
			}
			region := -1
			for j, f := range faces {
				if original[j] != face {
					continue
				}
				inside, e := splineRingInsideFragment(ctx, r.parts[chart], f)
				if e != nil {
					return nil, nil, nil, nil, e
				}
				if inside {
					if region != -1 {
						return nil, nil, nil, nil, geom.ErrConvergence
					}
					region = j
				}
			}
			if region == -1 {
				return nil, nil, nil, nil, fmt.Errorf("trimmed: cannot place closed ring in an open fragment: %w", geom.ErrConvergence)
			}
			regions[i] = region
		}
		first := len(faces)
		holes := make([][]Use, len(rings))
		for i, r := range rings {
			if e := ctx.Err(); e != nil {
				return nil, nil, nil, nil, e
			}
			f := faces[face]
			base := len(v)
			v = append(v, r.vertices...)
			loop, hole := make([]Use, len(r.vertices)), make([]Use, len(r.vertices))
			for j, x := range r.parts[chart] {
				start, end := x.segment, (x.segment+1)%len(r.vertices)
				if x.reversed {
					start, end = end, start
				}
				id := len(edges)
				edges = append(edges, parametric.Edge{Start: base + start, End: base + end,
					Curve: surfaceTrimCurve{f.Surface, x.from, x.to, x.curve}, Range: parametric.Interval{Max: 1}})
				sectionID := r.base + x.segment
				if r.sections != nil {
					sectionID = r.sections[x.segment]
				}
				section[id] = sectionID
				loop[j] = Use{Edge: id, UVStart: x.from, UVEnd: x.to, Curve: x.curve}
				hole[len(loop)-1-j] = Use{Edge: id, Reversed: true, UVStart: x.to, UVEnd: x.from, Curve: reversePCurve(x.curve)}
			}
			if parent[i] >= 0 {
				regions[i] = first + parent[i]
			}
			faces = append(faces, Face{Surface: f.Surface, Reversed: f.Reversed, Outer: loop})
			original = append(original, face)
			// The parent can be later in input order. Defer adding the hole
			// until the destination fragment exists.
			holes[i] = hole
		}
		for i, hole := range holes {
			p := &faces[regions[i]]
			p.Holes = append(p.Holes, hole)
		}
	}
	return v, edges, faces, original, nil
}

func closedOpenWireRing(ctx context.Context, f Face, chart int, p openWireFacePath, model [][2]geom.Vec3) (transverseRing, error) {
	var r transverseRing
	d, e := faceUVBounds(f)
	if e != nil {
		return r, e
	}
	for i, x := range p.pieces {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		trim := [2]intersect.TubeTrim{x.segment.TrimA(), x.segment.TrimB()}[chart]
		box, e := trim.BoundsUV()
		if e != nil {
			return r, e
		}
		if box.Min.X <= d.UMin || box.Max.X >= d.UMax || box.Min.Y <= d.VMin || box.Max.Y >= d.VMax {
			return r, geom.ErrConvergence
		}
		var c PCurve = trim
		end := 0
		if x.reverse {
			c = reversePCurve(c)
			end = 1
		}
		a, e := c.EvalUV(0)
		if e != nil {
			return r, e
		}
		b, e := c.EvalUV(1)
		if e != nil {
			return r, e
		}
		r.vertices = append(r.vertices, model[x.id][end])
		r.sections = append(r.sections, x.id)
		r.parts[chart] = append(r.parts[chart], transverseRingUse{i, c, a, b, false})
		if i == 0 {
			r.boxes[chart] = box
		} else {
			q := &r.boxes[chart]
			q.Min.X, q.Min.Y = min(q.Min.X, box.Min.X), min(q.Min.Y, box.Min.Y)
			q.Max.X, q.Max.Y = max(q.Max.X, box.Max.X), max(q.Max.Y, box.Max.Y)
		}
	}
	r.parts[chart], e = orientTransverseRing(ctx, r.parts[chart])
	return r, e
}

func splineRingInsideFragment(ctx context.Context, loop []transverseRingUse, f Face) (bool, error) {
	for _, sample := range loop[:min(8, len(loop))] {
		trim, reversed, ok := trimTube(sample.curve)
		if !ok {
			return false, geom.ErrInvalid
		}
		t := 0.
		if reversed {
			t = 1
		}
		q, e := trim.EvaluateBounds(t)
		if e != nil {
			return false, e
		}
		loc, known, e := classifyTrimBox(ctx, f, q, 128)
		if e != nil {
			return false, e
		}
		if known {
			return loc == parametric.Inside, nil
		}
	}
	return false, geom.ErrConvergence
}
