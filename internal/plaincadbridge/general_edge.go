package plaincadbridge

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/blend"
	"plainsolid/geom"
)

// This file is the adapter of the edge treatment of a general Boolean
// result (specs/general-edge-treatment). Only a general result takes
// this path; every other stock is treated as before.

// generalEdgeTolerance holds the tolerances of a reference: the bridge's
// point tolerance (nearPoint) and the angular tolerance of the general
// Boolean.
var generalEdgeTolerance = blend.Tolerance{Point: 1e-7, Angular: generalOptions.Angular}

// generalEdgeAvailable is edgeAvailable on a general result (R11): the
// reference resolves to one chain of straight edges between two planes
// that are not parallel.
func generalEdgeAvailable(s stored, start, end geom.Vec3) bool {
	solid, err := generalOperand(s)
	if err != nil {
		return false
	}
	_, err = blend.Locate(solid, start, end, generalEdgeTolerance)
	return err == nil
}

// generalEdgeKinds maps the operations to the treatments of brep/blend.
var generalEdgeKinds = map[string]blend.Kind{"chamfer": blend.Chamfer}

// aligned reports whether two unit vectors are parallel or antiparallel.
func aligned(a, b geom.Vec3) bool { return a.Cross(b).Length() <= generalEdgeTolerance.Angular }

// generalEdges resolves the references of a request in a general result,
// in their order, and holds each edge to the normal and the direction of
// the request (R1 to R10, N1, N2). References to one chain are one edge.
// lost is the refusal for a reference that does not resolve.
func generalEdges(s stored, q request, refs []edgeReference, normal geom.Vec3, lost func(error) error) ([]blend.Edge, error) {
	solid, err := generalOperand(s)
	if err != nil {
		return nil, err
	}
	var edges []blend.Edge
	for _, ref := range refs {
		if ref.Start == nil || ref.End == nil {
			return nil, errors.New("edge reference requires both endpoints")
		}
		e, err := blend.Locate(solid, ref.Start.vec(), ref.End.vec(), generalEdgeTolerance)
		if err != nil {
			return nil, lost(err)
		}
		known := false
		for _, other := range edges {
			known = known || e.Same(other)
		}
		if !known {
			edges = append(edges, e)
		}
	}
	var direction *geom.Vec3
	if q.Direction != nil {
		d, err := q.Direction.vec().Unit()
		if err != nil {
			return nil, unsupportedEdge(q.Op + " failed: direction does not match the selected edge")
		}
		direction = &d
	}
	for _, e := range edges {
		start, end := e.Ends()
		axis, err := end.Sub(start).Unit()
		if err != nil {
			return nil, err
		}
		if n := e.Normals(); !aligned(normal, n[0]) && !aligned(normal, n[1]) && !aligned(normal, axis) {
			return nil, unsupportedEdge(q.Op + " failed: extrusion normal does not match the selected edge")
		}
		if direction != nil && !aligned(*direction, axis) {
			return nil, unsupportedEdge(q.Op + " failed: direction does not match the selected edge")
		}
	}
	return edges, nil
}

// generalTreatEdge is fillet and chamfer on a general result. The
// request has passed the argument checks of treatEdge.
func (b *Bridge) generalTreatEdge(s stored, q request, refs []edgeReference, normal geom.Vec3, size float64) (Shape, error) {
	edges, err := generalEdges(s, q, refs, normal, func(err error) error {
		if errors.Is(err, blend.ErrCurved) {
			return unsupportedEdge(q.Op + " failed: this stock takes a treatment only on a straight edge between two planar faces")
		}
		return unsupportedEdge(q.Op + " failed: edge reference was lost or is ambiguous; no complete sharp edge resolves")
	})
	if err != nil {
		return Shape{}, err
	}
	for i, e := range edges {
		for _, other := range edges[:i] {
			if e.Meets(other) {
				return Shape{}, unsupportedEdge(q.Op + " failed: grouped edges must not share an end point")
			}
		}
	}
	return b.generalTreat(s, q.Op, edges, generalEdgeKinds[q.Op], size)
}

// generalTreat applies a treatment to resolved edges of a general result
// and publishes the result, which is a general result again (U1). It
// publishes nothing when the treatment is refused.
func (b *Bridge) generalTreat(s stored, op string, edges []blend.Edge, kind blend.Kind, size float64) (Shape, error) {
	solid, err := generalOperand(s)
	if err != nil {
		return Shape{}, err
	}
	body := blend.Body{Solid: solid, Known: generalKnown(s), Solids: s.result.GeometryAssertions.SolidCount}
	r, err := blend.Treat(context.Background(), body, edges, kind, size, blend.Options{Boolean: generalOptions, Tolerance: generalEdgeTolerance})
	switch {
	case err == nil:
		return b.putGeneral(r)
	case errors.Is(err, blend.ErrSize):
		return Shape{}, unsupportedEdge(op + " failed: the treatment size does not fit the selected edge")
	case errors.Is(err, blend.ErrFit):
		return Shape{}, unsupportedEdge(op + " failed: the treatment does not fit; a void, another feature or the far side of the stock lies within its size")
	case errors.Is(err, blend.ErrEnds):
		return Shape{}, unsupportedEdge(op + " failed: the treatment does not end on faces of the stock, or reaches under a neighbouring feature")
	case errors.Is(err, blend.ErrDivides):
		return Shape{}, unsupportedEdge(op + " failed: the treatment would divide the stock or leaves the selected edge in place")
	}
	// An error of the general Boolean, with its code (L2).
	var refusal *failureError
	if mapped := generalFailure(err); errors.As(mapped, &refusal) {
		return Shape{}, &failureError{refusal.code, op + " failed: " + refusal.message}
	}
	return Shape{}, fmt.Errorf("%s failed: %w", op, err)
}
