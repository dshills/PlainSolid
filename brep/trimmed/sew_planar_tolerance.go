//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// PlanarSewFace is an original plane and its independently authored straight
// trim loops in the plane's UV coordinates. Reversed changes its outward sense.
type PlanarSewFace struct {
	Surface  geom.PlaneSurface
	Outer    []geom.Vec2
	Holes    [][]geom.Vec2
	Reversed bool
}

// PlanarSewBorder identifies an original directed loop edge by the index of its
// start vertex. Loop zero is the outer boundary, and holes follow in order.
type PlanarSewBorder struct{ Face, Loop, Edge int }

// PlanarSewSeam declares which original borders to join. Reversed maps the
// first start to the second end, and the first end to the second start.
type PlanarSewSeam struct {
	First, Second PlanarSewBorder
	Reversed      bool
}

// PlanarToleranceSewOptions bounds the maximum Euclidean displacement of any
// original wire point. The caller explicitly supplies every seam. Display
// tolerance affects ordinary native construction checks and defaults to 1e-8.
type PlanarToleranceSewOptions struct {
	Tolerance, DisplayTolerance float64
	MaxFaces, MaxUses           int
	RepairOrientation           bool
	Trims                       trimvalidate.Options
	Collision                   collision.Options
}

// PlanarSewnVertex reports the repaired native vertex and its exact squared
// displacement from the represented source plane chart at that original UV.
type PlanarSewnVertex struct {
	Vertex              int
	SquaredDisplacement *big.Rat
}

// PlanarToleranceSewReport maps [face][loop][vertex or edge] to native topology.
// Faces keep their original order and support. The maximum displacement also
// bounds every straight source coedge under the affine parameter correspondence.
type PlanarToleranceSewReport struct {
	Vertices               [][][]PlanarSewnVertex
	Edges                  [][][]ExactSewnEdge
	MovedVertices          int
	FlippedFaces           []int
	MaxSquaredDisplacement *big.Rat
	ExactVolume            *big.Rat
	Collision              collision.Report
}

// SewPlanarWithinTolerance repairs one explicitly paired closed planar shell.
// Each sewn vertex is the exact projection of its original samples' mean onto
// all its incident original planes. A corner with three independent planes
// uses their unique intersection. The displacement budget covers all original
// coedges. The repair rechecks original and repaired trim topology, continuous
// model agreement, orientation, closed links and embedded planar material.
// Inconsistent supports, ambiguous topology, or unrepresentable repaired points
// return no geometry. No other nearby vertices are merged.
func SewPlanarWithinTolerance(ctx context.Context, input []PlanarSewFace, seams []PlanarSewSeam, o PlanarToleranceSewOptions) (Solid, PlanarToleranceSewReport, error) {
	var empty PlanarToleranceSewReport
	o, err := planarToleranceSewOptions(ctx, len(input), o)
	if err != nil {
		return Solid{}, empty, err
	}
	if len(seams) > o.MaxUses/2 {
		return Solid{}, empty, geom.ErrConvergence
	}
	charts, rings, vertices, err := planarSewOriginal(ctx, input, o)
	if err != nil {
		return Solid{}, empty, err
	}
	return sewPreparedPlanarFaces(ctx, input, seams, charts, rings, vertices, o)
}

func planarToleranceSewOptions(ctx context.Context, count int, o PlanarToleranceSewOptions) (PlanarToleranceSewOptions, error) {
	if o.MaxFaces == 0 {
		o.MaxFaces = 256
	}
	if o.MaxUses == 0 {
		o.MaxUses = 4096
	}
	if o.DisplayTolerance == 0 {
		o.DisplayTolerance = 1e-8
	}
	if ctx == nil || !finite(o.Tolerance) || o.Tolerance <= 0 || !finite(o.DisplayTolerance) || o.DisplayTolerance <= 0 ||
		o.MaxFaces < 4 || o.MaxFaces > 2048 || o.MaxUses < 12 || o.MaxUses > 16384 || count < 4 ||
		o.Collision.MaxPairs < 0 || o.Collision.MaxContacts < 0 {
		return o, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	if count > o.MaxFaces {
		return o, geom.ErrConvergence
	}
	return o, nil
}

func sewPreparedPlanarFaces(ctx context.Context, input []PlanarSewFace, seams []PlanarSewSeam, charts []planarSewChart, rings [][][]int, vertices []planarSewSample, o PlanarToleranceSewOptions) (Solid, PlanarToleranceSewReport, error) {
	var empty PlanarToleranceSewReport
	weld, pairs, mapping, err := planarSewPairBorders(ctx, rings, len(vertices), seams)
	if err != nil {
		return Solid{}, empty, err
	}
	tolerance := ratInset(o.Tolerance)
	points, vertexMap, moved, maximum, err := planarSewReconcile(ctx, weld, charts, vertices, tolerance.Mul(tolerance, tolerance))
	if err != nil {
		return Solid{}, empty, err
	}
	var edges []parametric.Edge
	for _, p := range pairs {
		a, b := vertexMap[p[0]].Vertex, vertexMap[p[1]].Vertex
		if a == b || points[a] == points[b] {
			return Solid{}, empty, geom.ErrDegenerate
		}
		curve, err := straightSewCurve([2]geom.Vec3{points[a], points[b]})
		if err != nil {
			return Solid{}, empty, err
		}
		edges = append(edges, parametric.Edge{Start: a, End: b, Curve: curve, Range: parametric.Interval{Max: 1}})
	}
	faces, report, err := planarSewRebuildFaces(ctx, input, charts, rings, points, vertexMap, mapping, o.Trims)
	if err != nil {
		return Solid{}, empty, err
	}
	faces, flipped, err := sewExactOrientation(faces, mapping, len(edges), o.RepairOrientation)
	if err != nil {
		return Solid{}, empty, err
	}
	result, err := NewSolidContext(ctx, points, edges, faces, o.DisplayTolerance)
	if err != nil {
		return Solid{}, empty, err
	}
	if err = validateExactSewSource(ctx, Solid{vertices: points, edges: edges, faces: faces}, ExactSewOptions{Trims: o.Trims}); err != nil {
		return Solid{}, empty, err
	}
	if len(result.Topology().Shells) != 1 {
		return Solid{}, empty, fmt.Errorf("trimmed: planar repair needs one connected shell: %w", geom.ErrConvergence)
	}
	if err = validateExactSewnVertexLinks(ctx, result); err != nil {
		return Solid{}, empty, err
	}
	volume, contacts, err := validatePlanarSewMaterial(ctx, result, o.Collision)
	if err != nil {
		return Solid{}, empty, err
	}
	report.MovedVertices, report.MaxSquaredDisplacement = moved, maximum
	report.FlippedFaces, report.ExactVolume, report.Collision = flipped, volume, contacts
	return result, report, nil
}

type planarSewSample struct {
	face  int
	point [3]*big.Rat
}

func planarSewOriginal(ctx context.Context, input []PlanarSewFace, o PlanarToleranceSewOptions) ([]planarSewChart, [][][]int, []planarSewSample, error) {
	charts := make([]planarSewChart, len(input))
	rings := make([][][]int, len(input))
	var vertices []planarSewSample
	for i, item := range input {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		chart, err := newPlanarSewChart(item.Surface)
		if err != nil {
			return nil, nil, nil, err
		}
		charts[i] = chart
		var wires [][]Use
		if len(item.Holes) > o.MaxUses/3 {
			return nil, nil, nil, geom.ErrConvergence
		}
		for _, polygon := range append([][]geom.Vec2{item.Outer}, item.Holes...) {
			if len(polygon) < 3 {
				return nil, nil, nil, geom.ErrInvalid
			}
			if len(polygon) > o.MaxUses-len(vertices) {
				return nil, nil, nil, geom.ErrConvergence
			}
			ring := make([]int, len(polygon))
			wire := make([]Use, len(polygon))
			for j, uv := range polygon {
				if !uvFinite(uv) {
					return nil, nil, nil, geom.ErrInvalid
				}
				id := len(vertices)
				ring[j] = id
				wire[j] = Use{Edge: id, UVStart: uv, UVEnd: polygon[(j+1)%len(polygon)]}
				vertices = append(vertices, planarSewSample{i, chart.point(uv)})
			}
			rings[i] = append(rings[i], ring)
			wires = append(wires, wire)
		}
		f := Face{Surface: item.Surface, Outer: wires[0], Holes: wires[1:], Reversed: item.Reversed}
		if err := planarSewCheckLoops(ctx, f, o.Trims); err != nil {
			return nil, nil, nil, fmt.Errorf("trimmed: original planar face %d: %w", i, err)
		}
	}
	return charts, rings, vertices, nil
}

func planarSewPairBorders(ctx context.Context, rings [][][]int, uses int, seams []PlanarSewSeam) (exactVertexWeld, [][2]int, []ExactSewnEdge, error) {
	if len(seams)*2 != uses {
		return nil, nil, nil, geom.ErrInvalid
	}
	lookup := func(b PlanarSewBorder) ([2]int, bool) {
		if b.Face < 0 || b.Face >= len(rings) || b.Loop < 0 || b.Loop >= len(rings[b.Face]) {
			return [2]int{}, false
		}
		ring := rings[b.Face][b.Loop]
		if b.Edge < 0 || b.Edge >= len(ring) {
			return [2]int{}, false
		}
		return [2]int{ring[b.Edge], ring[(b.Edge+1)%len(ring)]}, true
	}
	weld := newExactVertexWeld(uses)
	pairs, mapping := make([][2]int, len(seams)), make([]ExactSewnEdge, uses)
	used := make([]bool, uses)
	for i, seam := range seams {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		a, aok := lookup(seam.First)
		b, bok := lookup(seam.Second)
		if !aok || !bok || seam.First.Face == seam.Second.Face || used[a[0]] || used[b[0]] {
			return nil, nil, nil, fmt.Errorf("trimmed: invalid planar seam %d: %w", i, geom.ErrInvalid)
		}
		used[a[0]], used[b[0]] = true, true
		pairs[i] = a
		mapping[a[0]], mapping[b[0]] = ExactSewnEdge{Edge: i}, ExactSewnEdge{Edge: i, Reversed: seam.Reversed}
		if seam.Reversed {
			b[0], b[1] = b[1], b[0]
		}
		weld.join(a[0], b[0])
		weld.join(a[1], b[1])
	}
	return weld, pairs, mapping, nil
}

func planarSewRebuildFaces(ctx context.Context, input []PlanarSewFace, charts []planarSewChart, rings [][][]int, points []geom.Vec3, vertexMap []PlanarSewnVertex, mapping []ExactSewnEdge, options trimvalidate.Options) ([]Face, PlanarToleranceSewReport, error) {
	report := PlanarToleranceSewReport{Vertices: make([][][]PlanarSewnVertex, len(input)), Edges: make([][][]ExactSewnEdge, len(input))}
	faces := make([]Face, len(input))
	for i, f := range input {
		if err := ctx.Err(); err != nil {
			return nil, PlanarToleranceSewReport{}, err
		}
		wires := make([][]Use, len(rings[i]))
		for j, ring := range rings[i] {
			uv := make([]geom.Vec2, len(ring))
			for k, id := range ring {
				v := vertexMap[id]
				p, err := charts[i].parameters(exactBoundPoint(points[v.Vertex]))
				if err != nil {
					return nil, PlanarToleranceSewReport{}, err
				}
				uv[k] = p
			}
			wires[j] = make([]Use, len(ring))
			report.Vertices[i] = append(report.Vertices[i], make([]PlanarSewnVertex, len(ring)))
			report.Edges[i] = append(report.Edges[i], make([]ExactSewnEdge, len(ring)))
			for k, id := range ring {
				// The orientation solver consumes original directed source uses.
				wires[j][k] = Use{Edge: id, UVStart: uv[k], UVEnd: uv[(k+1)%len(uv)]}
				report.Vertices[i][j][k], report.Edges[i][j][k] = vertexMap[id], mapping[id]
			}
		}
		faces[i] = Face{Surface: f.Surface, Outer: wires[0], Holes: wires[1:], Reversed: f.Reversed}
		if err := planarSewCheckLoops(ctx, faces[i], options); err != nil {
			return nil, PlanarToleranceSewReport{}, fmt.Errorf("trimmed: repaired planar face %d: %w", i, err)
		}
	}
	return faces, report, nil
}

func planarSewCheckLoops(ctx context.Context, f Face, o trimvalidate.Options) error {
	r, err := ValidateFaceTrimLoops(ctx, f, o)
	if err != nil {
		return err
	}
	if r.Status != trimvalidate.Certified {
		return geom.ErrConvergence
	}
	return nil
}

func validatePlanarSewMaterial(ctx context.Context, s Solid, o collision.Options) (*big.Rat, collision.Report, error) {
	m := mesh.Mesh{Vertices: s.vertices}
	for _, f := range s.faces {
		var rings [][]geom.Vec3
		ids := make(map[geom.Vec2]int)
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			var ring []geom.Vec3
			for _, u := range loop {
				e := s.edges[u.Edge]
				id := e.Start
				if u.Reversed {
					id = e.End
				}
				ids[u.UVStart] = id
				ring = append(ring, geom.Vec3{X: u.UVStart.X, Y: u.UVStart.Y})
			}
			rings = append(rings, ring)
		}
		triangles, err := triangulateUVBounded(ctx, rings, brep.MaxHoledFaceVertices)
		if err != nil {
			return nil, collision.Report{}, err
		}
		for _, tri := range triangles {
			var indexed [3]int
			for i, uv := range tri {
				id, present := ids[uv]
				if !present {
					return nil, collision.Report{}, geom.ErrConvergence
				}
				indexed[i] = id
			}
			if f.Reversed {
				indexed[1], indexed[2] = indexed[2], indexed[1]
			}
			m.Triangles = append(m.Triangles, indexed)
		}
	}
	return checkClosedPeriodicMesh(ctx, m, o)
}
