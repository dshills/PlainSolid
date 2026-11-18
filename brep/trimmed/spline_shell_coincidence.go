//go:build research

package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SplineShellCoincidenceOptions bounds a sufficient equality shortcut. A must
// have an independently proved outward rectangular spline embedding. B may
// reorder faces and refine uses. Limits on Face and Embedding proofs apply
// independently; MaxPairs (default256,maximum4096) bounds total face attempts.
type SplineShellCoincidenceOptions struct {
	Embedding SplineRectangleEmbeddingOptions
	Face      SplineFaceCoincidenceOptions
	MaxPairs  int
}
type SplineShellCoincidenceHistory struct {
	Faces          []SplineFaceCoincidence
	FacesA, FacesB [][]int
	// B's edges/vertices retain their exact subdivision provenance on A.
	EdgesB    []CoincidentShellEdge
	VerticesB []CoincidentShellVertex
	Complete  bool
}
type CoincidentShellEdge struct {
	EdgeA  int
	RangeA [2]*big.Rat
}
type CoincidentShellVertex struct {
	VertexA, EdgeA int
	Parameter      *big.Rat
}

// CombineCoincidentSplineShells accepts only complete equality of one oriented
// material shell, never partial overlap. Common/Fuse retain A's native topology
// verbatim; Difference is empty. Every face and source subdivision is retained
// in the history, including both parents of retained output faces. Failure is
// atomic. No input is altered and no sampled geometry establishes equality.
func CombineCoincidentSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o SplineShellCoincidenceOptions) (Solid, SplineShellCoincidenceHistory, error) {
	fail := func(e error) (Solid, SplineShellCoincidenceHistory, error) {
		return Solid{}, SplineShellCoincidenceHistory{}, e
	}
	if ctx == nil || op > parametric.Difference || len(a.faces) == 0 || len(a.faces) != len(b.faces) {
		return fail(geom.ErrInvalid)
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 256
	}
	if o.MaxPairs < 1 || o.MaxPairs > 4096 {
		return fail(geom.ErrInvalid)
	}
	maxFaces := o.Embedding.MaxFaces
	if maxFaces == 0 {
		maxFaces = 64
	}
	if maxFaces < 1 || maxFaces > 128 {
		return fail(geom.ErrInvalid)
	}
	if len(a.faces) > maxFaces {
		return fail(parametric.ErrBudget)
	}
	limit := o.Face.MaxUses
	if limit == 0 {
		limit = 64
	}
	if limit < 1 || limit > 256 {
		return fail(geom.ErrInvalid)
	}
	uses := 0
	for _, f := range b.faces {
		n := len(f.Outer)
		for _, h := range f.Holes {
			n += len(h)
		}
		if n > limit {
			return fail(parametric.ErrBudget)
		}
		uses += n
	}
	if len(b.edges) > uses || len(b.vertices) > 2*uses {
		return fail(parametric.ErrBudget)
	}
	proof, e := a.ValidateSplineRectangleEmbedding(ctx, o.Embedding)
	if e != nil {
		return fail(e)
	}
	if !proof.Complete {
		return fail(geom.ErrConvergence)
	}
	// Reconstruct to authenticate closed incidence, all used entities and edge
	// directions, rather than trusting an unchecked collection of Face values.
	checked, e := NewSolidContext(ctx, b.vertices, b.edges, b.faces, b.tolerance)
	if e != nil {
		return fail(e)
	}
	if len(checked.Topology().Shells) != 1 {
		return fail(geom.ErrConvergence)
	}
	agreement, e := checked.ValidateEdgeAgreement(ctx, o.Embedding.EdgeAgreement)
	if e != nil {
		return fail(e)
	}
	if !agreement.Complete {
		return fail(geom.ErrConvergence)
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return fail(geom.ErrConvergence)
		}
	}
	if e = coincidentShellExactVertices(ctx, checked, o.Face); e != nil {
		return fail(e)
	}
	h := SplineShellCoincidenceHistory{FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)), EdgesB: make([]CoincidentShellEdge, len(b.edges)), VerticesB: make([]CoincidentShellVertex, len(b.vertices))}
	matched := make([]bool, len(b.faces))
	pairs := 0
	for i := range a.faces {
		found := false
		for j := range b.faces {
			if matched[j] {
				continue
			}
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if pairs == o.MaxPairs {
				return fail(parametric.ErrBudget)
			}
			pairs++
			q, err := a.CertifySplineFaceCoincidence(ctx, i, b, j, o.Face)
			if ctx.Err() != nil {
				return fail(ctx.Err())
			}
			if err != nil || !q.Complete || q.OppositeOrientation {
				continue
			}
			matched[j] = true
			found = true
			h.Faces = append(h.Faces, q)
			if op != parametric.Difference {
				h.FacesA[i] = []int{i}
				h.FacesB[j] = []int{i}
			}
			break
		}
		if !found {
			return fail(geom.ErrConvergence)
		}
	}
	edgeSeen := make([]bool, len(b.edges))
	vertexSeen := make([]bool, len(b.vertices))
	edgeKeys := map[string]int{}
	vertexKeys := map[string]int{}
	for _, face := range h.Faces {
		for _, pair := range face.Uses {
			ua := append([][]Use{a.faces[face.Faces[0]].Outer}, a.faces[face.Faces[0]].Holes...)[pair.LoopA][pair.UseA]
			ub := append([][]Use{b.faces[face.Faces[1]].Outer}, b.faces[face.Faces[1]].Holes...)[pair.LoopB][pair.UseB]
			ends := [2]*big.Rat{new(big.Rat).Set(pair.RangeA[0]), new(big.Rat).Set(pair.RangeA[1])}
			if ua.Reversed {
				for k := range ends {
					ends[k].Sub(big.NewRat(1, 1), ends[k])
				}
			}
			lo, hi := ends[0], ends[1]
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			key := fmt.Sprintf("%d:%s:%s", ua.Edge, lo.RatString(), hi.RatString())
			if prior, ok := edgeKeys[key]; ok && prior != ub.Edge {
				return fail(geom.ErrConvergence)
			}
			edgeKeys[key] = ub.Edge
			if edgeSeen[ub.Edge] {
				p := h.EdgesB[ub.Edge]
				if p.EdgeA != ua.Edge || p.RangeA[0].Cmp(lo) != 0 || p.RangeA[1].Cmp(hi) != 0 {
					return fail(geom.ErrConvergence)
				}
			} else {
				edgeSeen[ub.Edge] = true
				h.EdgesB[ub.Edge] = CoincidentShellEdge{ua.Edge, [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
			}
			if pair.RangeB[0].Sign() != 0 {
				ends[0], ends[1] = ends[1], ends[0]
			}
			ids := [2]int{b.edges[ub.Edge].Start, b.edges[ub.Edge].End}
			if ub.Reversed {
				ids[0], ids[1] = ids[1], ids[0]
			}
			for k, id := range ids {
				v := CoincidentShellVertex{VertexA: -1, EdgeA: ua.Edge, Parameter: new(big.Rat).Set(ends[k])}
				if ends[k].Sign() == 0 {
					v.VertexA = a.edges[ua.Edge].Start
				} else if ends[k].Cmp(big.NewRat(1, 1)) == 0 {
					v.VertexA = a.edges[ua.Edge].End
				}
				key := fmt.Sprintf("e%d:%s", v.EdgeA, v.Parameter.RatString())
				if v.VertexA >= 0 {
					key = fmt.Sprintf("v%d", v.VertexA)
					v.EdgeA = -1
					v.Parameter = nil
				}
				if prior, ok := vertexKeys[key]; ok && prior != id {
					return fail(geom.ErrConvergence)
				}
				vertexKeys[key] = id
				if vertexSeen[id] {
					old := h.VerticesB[id]
					if old.VertexA != v.VertexA || old.EdgeA != v.EdgeA || old.Parameter != nil && (v.Parameter == nil || old.Parameter.Cmp(v.Parameter) != 0) {
						return fail(geom.ErrConvergence)
					}
				} else {
					vertexSeen[id] = true
					h.VerticesB[id] = v
				}
			}
		}
	}
	for _, seen := range edgeSeen {
		if !seen {
			return fail(geom.ErrConvergence)
		}
	}
	for _, seen := range vertexSeen {
		if !seen {
			return fail(geom.ErrConvergence)
		}
	}
	h.Complete = true
	if op == parametric.Difference {
		return Solid{}, h, nil
	}
	return a, h, nil
}

func coincidentShellExactVertices(ctx context.Context, s Solid, o SplineFaceCoincidenceOptions) error {
	for _, f := range s.faces {
		p, e := exactFaceSupportPatch(f, o.Patches)
		if e != nil {
			return e
		}
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				c, reverse, e := coincidentUseCurve(ctx, u, o.Patches)
				if e != nil {
					return e
				}
				net, _, e := c.BezierControlNet(ctx, o.Patches)
				if e != nil {
					return e
				}
				ids := [2]int{s.edges[u.Edge].Start, s.edges[u.Edge].End}
				if u.Reversed {
					ids[0], ids[1] = ids[1], ids[0]
				}
				for side, id := range ids {
					index := 0
					if (side == 1) != reverse {
						index = len(net.Control) - 1
					}
					h := net.Control[index]
					point, e := p.EvaluateRational(ctx, new(big.Rat).Quo(h[0], h[3]), new(big.Rat).Quo(h[1], h[3]), geom.SurfacePatchEvaluationOptions{MaxOperations: o.Patches.MaxOperations, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
					if e != nil {
						return e
					}
					actual := s.vertices[id]
					for axis, value := range []float64{actual.X, actual.Y, actual.Z} {
						if point.Point[axis].Cmp(ratInset(value)) != 0 {
							return geom.ErrUnrepresentable
						}
					}
				}
			}
		}
	}
	return nil
}
