package poly

import (
	"context"
	"plainsolid/brep"
	"plainsolid/geom"
)

// Operand identifies an input to CombineWithHistory.
type Operand uint8

const (
	OperandA Operand = iota
	OperandB
)

// Source identifies the oriented input tessellation triangle supporting an
// output face. Reversed means the output normal opposes that input triangle.
// Face indexes the original polygon face, including faces with inner wires.
type Source struct {
	Operand        Operand
	Face, Triangle int
	Reversed       bool
}

// State distinguishes a missing boundary, an unchanged triangle, and a triangle
// whose retained boundary has been cut or subdivided. Retained may be reversed.
type State uint8

const (
	Deleted State = iota
	Retained
	Fragmented
)

// TriangleHistory records every output face descended from one input triangle.
// Fragmented includes pure conforming subdivision as well as partial removal.
type TriangleHistory struct {
	State       State
	OutputFaces []int
}

// History is complete only on success. Faces aligns with result.Faces(); A and B
// align with the respective input Tessellate().Triangles. Coplanar overlap has
// one owning source, not an exhaustive list of geometrically coincident inputs.
// Identity is local to this call; it is not persistent CAD naming.
type History struct {
	Faces []Source
	A, B  []TriangleHistory
}

// ValidateEmbeddedBoundary proves exact geometric embedding and material
// orientation of a planar boundary. Disconnected shells and nested cavities
// are supported. Each planar face is represented by its exact triangles;
// their contacts and containment are checked without tolerance welding.
func ValidateEmbeddedBoundary(ctx context.Context, s brep.Solid, o Options) error {
	if ctx == nil {
		return ErrInvalid
	}
	o, e := defaults(o)
	if e != nil {
		return e
	}
	b := &budget{ctx: ctx, o: o}
	if !b.tick(1) {
		return b.err
	}
	_, e = input(s, b)
	return e
}

// Combine applies a regularized Boolean to the exact oriented triangle surfaces
// represented by the inputs. Inputs must be embedded manifolds with outward
// material boundaries (cavity shells inward). Edge/vertex-only contacts that
// cannot form an embedded manifold are rejected. No convex decomposition is used.
func Combine(ctx context.Context, a, b brep.Solid, op Operation, o Options) (brep.Solid, error) {
	var sources []Source
	return combine(ctx, a, b, op, o, &sources)
}

// CombineWithHistory performs the same operation as Combine and returns boundary
// ancestry. Deleted means no boundary survives under that input triangle's
// ownership; it does not imply its enclosed material was removed.
func CombineWithHistory(ctx context.Context, a, b brep.Solid, op Operation, o Options) (brep.Solid, History, error) {
	var sources []Source
	s, err := combine(ctx, a, b, op, o, &sources)
	if err != nil {
		return brep.Solid{}, History{}, err
	}
	return s, faceHistory(a, b, s, sources), nil
}
func faceHistory(a, b, s brep.Solid, sources []Source) History {
	am, bm, rm := a.Tessellate(), b.Tessellate(), s.Tessellate()
	h := History{Faces: sources, A: make([]TriangleHistory, len(am.Triangles)), B: make([]TriangleHistory, len(bm.Triangles))}
	// Parent BRep API supplies original-face correspondence in tessellation order.
	af, bf := a.TriangleFaces(), b.TriangleFaces()
	for i, src := range sources {
		history := h.A
		im := am
		faceMap := af
		if src.Operand == OperandB {
			history = h.B
			im = bm
			faceMap = bf
		}
		h.Faces[i].Face = faceMap[src.Triangle]
		entry := &history[src.Triangle]
		entry.OutputFaces = append(entry.OutputFaces, i)
		entry.State = Fragmented
		if len(entry.OutputFaces) == 1 {
			t, u := im.Triangles[src.Triangle], rm.Triangles[i]
			iv := [3]geom.Vec3{im.Vertices[t[0]], im.Vertices[t[1]], im.Vertices[t[2]]}
			rv := [3]geom.Vec3{rm.Vertices[u[0]], rm.Vertices[u[1]], rm.Vertices[u[2]]}
			same := true
			for _, v := range rv {
				same = same && (v == iv[0] || v == iv[1] || v == iv[2])
			}
			if same {
				entry.State = Retained
			}
		}
	}
	return h
}
