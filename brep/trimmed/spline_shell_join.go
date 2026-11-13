package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineShellJoinOptions controls the proofs and sewing for a shared planar
// interface between two rectangular spline shells.
type SplineShellJoinOptions struct {
	Embedding   SplineRectangleEmbeddingOptions
	Coincidence intersect.SplineCoincidenceOptions
	Sew         SewOptions
}

// SplineShellJoinHistory identifies the input interface and maps each input
// face to its output face, with -1 for removed faces.
type SplineShellJoinHistory struct {
	InterfaceA, InterfaceB int
	Map                    intersect.SplineChartMap
	FacesA, FacesB         []int
}

// CombineFaceJoinedSplineShells recognizes a complete shared planar spline
// face between two certified single-shell bodies. The shells must occupy
// opposite halfspaces and all other contact with the interface plane must be
// confined to its own shared edges and vertices. Union removes the interface
// and sews the remaining exact supports, intersection is regularized empty,
// and difference leaves A intact. Partial, tangent and interior intersections
// are not supported by this path.
func CombineFaceJoinedSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellJoinOptions) (Solid, SplineShellJoinHistory, error) {
	h := SplineShellJoinHistory{InterfaceA: -1, InterfaceB: -1}
	if ctx == nil || op > parametric.Difference || len(a.faces) == 0 || len(b.faces) == 0 {
		return Solid{}, h, geom.ErrInvalid
	}
	var ap, bp []splineEmbeddingFace
	if _, e := a.validateSplineRectangleEmbedding(ctx, o.Embedding, &ap); e != nil {
		return Solid{}, h, e
	}
	if _, e := b.validateSplineRectangleEmbedding(ctx, o.Embedding, &bp); e != nil {
		return Solid{}, h, e
	}
	var as, bs int
	for i, x := range ap {
		if x.planeSign == 0 {
			continue
		}
		for j, y := range bp {
			if e := ctx.Err(); e != nil {
				return Solid{}, h, e
			}
			if y.planeSign == 0 || !sameSplineFaceCorners(a, b, x, y) {
				continue
			}
			q, e := intersect.FindExactSplineCoincidence(ctx, x.patch, y.patch, o.Coincidence)
			if e != nil && !errors.Is(e, geom.ErrConvergence) {
				return Solid{}, h, e
			}
			if !q.Found {
				continue
			}
			as, e = proveSplineShellOnInterfaceSide(ctx, ap, i, x.plane, o.Embedding.Plane)
			if e != nil {
				return Solid{}, h, e
			}
			if as == 0 {
				continue
			}
			bs, e = proveSplineShellOnInterfaceSide(ctx, bp, j, x.plane, o.Embedding.Plane)
			if e != nil {
				return Solid{}, h, e
			}
			if bs != -as {
				continue
			}
			h.InterfaceA, h.InterfaceB, h.Map = i, j, q.Map
			break
		}
		if h.InterfaceA >= 0 {
			break
		}
	}
	if h.InterfaceA < 0 {
		return Solid{}, h, fmt.Errorf("%w: no complete planar interface between the spline shells", geom.ErrConvergence)
	}
	return joinProvedSplineShells(ctx, a, b, op, h, nil, [4]geom.Vec3{}, o.Sew)
}

func joinProvedSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, h SplineShellJoinHistory,
	inset *geom.ParameterRectangle, corners [4]geom.Vec3, o SewOptions) (Solid, SplineShellJoinHistory, error) {
	h.FacesA, h.FacesB = make([]int, len(a.faces)), make([]int, len(b.faces))
	for _, m := range [][]int{h.FacesA, h.FacesB} {
		for i := range m {
			m[i] = -1
		}
	}
	if op == parametric.Intersection {
		return Solid{}, h, nil
	}
	if op == parametric.Difference {
		for i := range h.FacesA {
			h.FacesA[i] = i
		}
		return a, h, nil
	}
	parts := make([]SewFace, 0, len(a.faces)+len(b.faces)-2)
	for _, x := range []struct {
		s              Solid
		mapping        []int
		interfaceIndex int
		a              bool
	}{{a, h.FacesA, h.InterfaceA, true}, {b, h.FacesB, h.InterfaceB, false}} {
		for i, f := range x.s.faces {
			if i == x.interfaceIndex && (!x.a || inset == nil) {
				continue
			}
			x.mapping[i] = len(parts)
			p := x.s.splineSewFace(f)
			if x.a && i == x.interfaceIndex {
				addSplineSewHole(&p, *inset, corners)
			}
			parts = append(parts, p)
		}
	}
	if o.Tolerance == 0 {
		o.Tolerance = max(a.tolerance, b.tolerance)
	}
	s, _, e := Sew(ctx, parts, o)
	if e != nil {
		return Solid{}, SplineShellJoinHistory{}, e
	}
	return s, h, nil
}

func sameSplineFaceCorners(a, b Solid, x, y splineEmbeddingFace) bool {
	for _, i := range x.vertex {
		match := false
		for _, j := range y.vertex {
			match = match || a.vertices[i] == b.vertices[j]
		}
		if !match {
			return false
		}
	}
	return true
}

func proveSplineShellOnInterfaceSide(ctx context.Context, faces []splineEmbeddingFace, candidate int, plane [3]geom.Vec3, o geom.SurfaceSupportingPlaneOptions) (int, error) {
	side := 0
	for i, f := range faces {
		q, e := geom.CertifySurfaceSupportingPlane(ctx, f.patch, plane[0], plane[1], plane[2], o)
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) {
				return 0, nil
			}
			return 0, e
		}
		if !q.Complete {
			return 0, nil
		}
		if i == candidate {
			if !q.Coincident {
				return 0, nil
			}
			continue
		}
		if q.Coincident || q.HalfspaceSign == 0 || side != 0 && side != q.HalfspaceSign || !rectangleContactsMatch(faces[candidate], f, q) {
			return 0, nil
		}
		side = q.HalfspaceSign
	}
	return side, nil
}

func (s Solid) splineSewFace(f Face) SewFace {
	p, _ := s.splineSewFaceMapped(f, nil)
	return p
}

func (s Solid) splineSewFaceMapped(f Face, points map[int]geom.Vec3) (SewFace, []int) {
	p := SewFace{Face: cloneFace(f)}
	vertices := map[int]int{}
	var source []int
	for _, loop := range append([][]Use{p.Face.Outer}, p.Face.Holes...) {
		for i, u := range loop {
			e := s.edges[u.Edge]
			for _, id := range []*int{&e.Start, &e.End} {
				j, ok := vertices[*id]
				if !ok {
					j = len(p.Vertices)
					vertices[*id] = j
					q, ok := points[*id]
					if !ok {
						q = s.vertices[*id]
					}
					p.Vertices = append(p.Vertices, q)
				}
				*id = j
			}
			loop[i].Edge = len(p.Edges)
			source = append(source, u.Edge)
			p.Edges = append(p.Edges, e)
		}
	}
	return p, source
}
