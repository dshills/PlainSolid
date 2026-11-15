// Package heal repairs limited planar topology defects. It does not repair
// arbitrary geometry or close gaps with new faces.
package heal

import (
	"context"
	"errors"
	"fmt"
	"math"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

const MaxInputVertices = 8192
const MaxFaces = 2048

var ErrLimit = errors.New("heal: work limit exceeded")
var ErrInvalid = errors.New("heal: invalid geometry")
var ErrOpenShell = errors.New("heal: open shell")
var ErrNonManifold = errors.New("heal: nonmanifold edge")
var ErrNonOrientable = errors.New("heal: inconsistent orientation constraints")

// Face is an independent planar polygon, with optional oppositely wound holes.
// Loops omit a repeated closing vertex. Input winding may differ between faces.
type Face struct {
	Outer []geom.Vec3
	Holes [][]geom.Vec3
}

// Options controls positional welding and face orientation. Tolerance is an
// absolute distance; zero requests exact coordinate welding. Welding uses an
// existing input position, never a moving average or transitive proximity chain.
type Options struct {
	Tolerance float64
	// RepairOrientation makes neighboring faces consistently oriented, preserving
	// the first input face's orientation in each connected shell.
	RepairOrientation bool
	// OrientOutward implies RepairOrientation and makes each shell's volume
	// positive. It must not be used for cavity shells that need inward orientation;
	// no geometric containment or cavity nesting is inferred.
	OrientOutward bool
	// OrientByNesting implies RepairOrientation and uses exact tests to verify
	// embedding and orient cavities and material shells by containment depth.
	// Nil disables this check. It is mutually exclusive with OrientOutward.
	OrientByNesting *collision.LocationOptions
}

// FaceMap maps each original input vertex occurrence to the output vertex index.
// Its order stays identical to the input even when the output face is reversed.
type FaceMap struct {
	Outer []int
	Holes [][]int
}

// EdgeDiagnostic identifies an edge by welded vertex indices and input faces.
type EdgeDiagnostic struct {
	Start, End int
	Faces      []int
}

// Report remains available on errors. Vertices are the deterministic weld targets;
// on success their indices match the returned Solid. Positions in the input move
// by at most Tolerance. WeldedVertices counts eliminated vertex occurrences.
type Report struct {
	InputVertices    int
	WeldedVertices   int
	MaxDisplacement  float64
	Vertices         []geom.Vec3
	VertexMap        []FaceMap
	FlippedFaces     []int
	Shells           []ShellOrientation // populated on success with OrientByNesting
	OpenEdges        []EdgeDiagnostic
	NonManifoldEdges []EdgeDiagnostic
}

type occurrence struct {
	point              geom.Vec3
	face, loop, vertex int
}
type edgeUse struct{ face, sign int }

// Sew welds independent face vertices, shares equal endpoint edges, validates
// closed manifold incidence, and optionally repairs whole-face orientation.
// It does not split T-junction edges, remove collinear vertices, project warped
// faces, or add missing faces. Such unresolved defects return an error and report.
// Face geometry is checked by brep.NewSolid after welding, so small changes that
// break planarity are rejected rather than silently changing the face surface.
func Sew(input []Face, options Options) (brep.Solid, Report, error) {
	return SewContext(context.Background(), input, options)
}

// SewContext is Sew with cancellation during welding and nesting validation.
func SewContext(ctx context.Context, input []Face, options Options) (brep.Solid, Report, error) {
	var report Report
	if ctx == nil || (options.OrientOutward && options.OrientByNesting != nil) {
		return brep.Solid{}, report, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return brep.Solid{}, report, err
	}
	if options.Tolerance < 0 || math.IsNaN(options.Tolerance) || math.IsInf(options.Tolerance, 0) {
		return brep.Solid{}, report, fmt.Errorf("%w: tolerance must be finite and nonnegative", ErrInvalid)
	}
	if len(input) > MaxFaces {
		return brep.Solid{}, report, ErrLimit
	}
	report.VertexMap = make([]FaceMap, len(input))
	var occurrences []occurrence
	for f, face := range input {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, report, err
		}
		report.VertexMap[f].Holes = make([][]int, len(face.Holes))
		for loop, ring := range append([][]geom.Vec3{face.Outer}, face.Holes...) {
			if len(ring) < 3 {
				return brep.Solid{}, report, fmt.Errorf("%w: face %d wire %d has fewer than three vertices", ErrInvalid, f, loop)
			}
			if len(occurrences)+len(ring) > MaxInputVertices {
				return brep.Solid{}, report, ErrLimit
			}
			ids := make([]int, len(ring))
			if loop == 0 {
				report.VertexMap[f].Outer = ids
			} else {
				report.VertexMap[f].Holes[loop-1] = ids
			}
			for i, p := range ring {
				if !p.IsFinite() {
					return brep.Solid{}, report, fmt.Errorf("%w: nonfinite input coordinate", ErrInvalid)
				}
				occurrences = append(occurrences, occurrence{p, f, loop, i})
			}
		}
	}
	report.InputVertices = len(occurrences)
	sort.SliceStable(occurrences, func(i, j int) bool {
		a, b := occurrences[i].point, occurrences[j].point
		if a.X != b.X {
			return a.X < b.X
		}
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.Z < b.Z
	})
	for _, o := range occurrences {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, report, err
		}
		best := -1
		distance := math.Inf(1)
		// Representatives are lexicographically sorted. Search only the nearby X
		// window, but test full Euclidean distance to prevent box-tolerance drift.
		first := sort.Search(len(report.Vertices), func(i int) bool { return report.Vertices[i].X >= o.point.X-options.Tolerance })
		for i := first; i < len(report.Vertices); i++ {
			p := report.Vertices[i]
			if p.X > o.point.X+options.Tolerance {
				break
			}
			d := o.point.Sub(p).Length()
			if d <= options.Tolerance && d < distance {
				best, distance = i, d
			}
		}
		if best < 0 {
			best = len(report.Vertices)
			report.Vertices = append(report.Vertices, o.point)
			distance = 0
		}
		report.MaxDisplacement = math.Max(report.MaxDisplacement, distance)
		mapping := report.VertexMap[o.face].Outer
		if o.loop > 0 {
			mapping = report.VertexMap[o.face].Holes[o.loop-1]
		}
		mapping[o.vertex] = best
	}
	report.WeldedVertices = report.InputVertices - len(report.Vertices)
	var edges []brep.Edge
	var incidence [][]edgeUse
	faces := make([]brep.Face, len(input))
	lookup := map[[2]int]int{}
	for f, mapping := range report.VertexMap {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, report, err
		}
		seen := map[int]bool{}
		for loop, ids := range append([][]int{mapping.Outer}, mapping.Holes...) {
			uses := make([]brep.EdgeUse, len(ids))
			for i, a := range ids {
				if seen[a] {
					return brep.Solid{}, report, fmt.Errorf("%w: welding collapses or joins wires of face %d", ErrInvalid, f)
				}
				seen[a] = true
				b := ids[(i+1)%len(ids)]
				lo, hi := a, b
				if lo > hi {
					lo, hi = hi, lo
				}
				key := [2]int{lo, hi}
				edge, ok := lookup[key]
				if !ok {
					edge = len(edges)
					lookup[key] = edge
					edges = append(edges, brep.Edge{Start: lo, End: hi})
					incidence = append(incidence, nil)
				}
				sign := 1
				if a > b {
					sign = -1
				}
				incidence[edge] = append(incidence[edge], edgeUse{f, sign})
				uses[i] = brep.EdgeUse{Edge: edge, Reversed: a > b}
			}
			if loop == 0 {
				faces[f].Boundary = uses
			} else {
				faces[f].Holes = append(faces[f].Holes, uses)
			}
		}
	}
	for i, uses := range incidence {
		if len(uses) == 2 {
			continue
		}
		d := EdgeDiagnostic{Start: edges[i].Start, End: edges[i].End}
		for _, u := range uses {
			d.Faces = append(d.Faces, u.face)
		}
		if len(uses) == 1 {
			report.OpenEdges = append(report.OpenEdges, d)
		} else {
			report.NonManifoldEdges = append(report.NonManifoldEdges, d)
		}
	}
	if len(report.NonManifoldEdges) > 0 {
		return brep.Solid{}, report, ErrNonManifold
	}
	if len(report.OpenEdges) > 0 {
		return brep.Solid{}, report, ErrOpenShell
	}
	flips := make([]int, len(faces))
	var shells [][]int
	if options.RepairOrientation || options.OrientOutward || options.OrientByNesting != nil {
		type neighbor struct{ face, relation int }
		adjacent := make([][]neighbor, len(faces))
		for _, uses := range incidence {
			a, b := uses[0], uses[1]
			relation := -a.sign * b.sign
			adjacent[a.face] = append(adjacent[a.face], neighbor{b.face, relation})
			adjacent[b.face] = append(adjacent[b.face], neighbor{a.face, relation})
		}
		for start := range faces {
			if flips[start] != 0 {
				continue
			}
			flips[start] = 1
			queue := []int{start}
			var shell []int
			for len(queue) > 0 {
				f := queue[len(queue)-1]
				queue = queue[:len(queue)-1]
				shell = append(shell, f)
				for _, n := range adjacent[f] {
					needed := flips[f] * n.relation
					if flips[n.face] == 0 {
						flips[n.face] = needed
						queue = append(queue, n.face)
					} else if flips[n.face] != needed {
						return brep.Solid{}, report, ErrNonOrientable
					}
				}
			}
			shells = append(shells, shell)
		}
		for i, flip := range flips {
			if flip < 0 {
				reverseFace(&faces[i])
			}
		}
	}
	solid, err := brep.NewSolid(report.Vertices, edges, faces)
	if err != nil {
		return brep.Solid{}, report, fmt.Errorf("%w: welded boundary: %v", ErrInvalid, err)
	}
	if options.OrientByNesting != nil {
		solid, report.Shells, err = orientNestedShells(ctx, solid, report.Vertices, edges, faces, flips, *options.OrientByNesting)
		if err != nil {
			return brep.Solid{}, report, err
		}
	}
	if options.OrientOutward {
		for _, shell := range shells {
			volume := shellVolume(shell, faces, edges, report.Vertices)
			if volume == 0 || math.IsNaN(volume) || math.IsInf(volume, 0) {
				return brep.Solid{}, report, fmt.Errorf("%w: shell volume is zero or nonfinite", ErrInvalid)
			}
			if volume < 0 {
				for _, f := range shell {
					reverseFace(&faces[f])
					flips[f] = -flips[f]
				}
			}
		}
		solid, err = brep.NewSolid(report.Vertices, edges, faces)
		if err != nil {
			return brep.Solid{}, report, fmt.Errorf("%w: oriented boundary: %v", ErrInvalid, err)
		}
	}
	for i, flip := range flips {
		if flip < 0 {
			report.FlippedFaces = append(report.FlippedFaces, i)
		}
	}
	return solid, report, nil
}
func reverseFace(face *brep.Face) {
	for _, loop := range append([][]brep.EdgeUse{face.Boundary}, face.Holes...) {
		for i, j := 0, len(loop)-1; i < j; i, j = i+1, j-1 {
			loop[i], loop[j] = loop[j], loop[i]
		}
		for i := range loop {
			loop[i].Reversed = !loop[i].Reversed
		}
	}
}
func shellVolume(shell []int, faces []brep.Face, edges []brep.Edge, vertices []geom.Vec3) float64 {
	first := faces[shell[0]].Boundary[0]
	origin := vertices[edges[first.Edge].Start]
	sum, correction := 0.0, 0.0
	for _, f := range shell {
		for _, loop := range append([][]brep.EdgeUse{faces[f].Boundary}, faces[f].Holes...) {
			p := make([]geom.Vec3, len(loop))
			for i, u := range loop {
				edge := edges[u.Edge]
				id := edge.Start
				if u.Reversed {
					id = edge.End
				}
				p[i] = vertices[id].Sub(origin)
			}
			for i := 1; i+1 < len(p); i++ {
				v := p[0].Dot(p[i].Cross(p[i+1]))/6 - correction
				next := sum + v
				correction = (next - sum) - v
				sum = next
			}
		}
	}
	return sum
}
