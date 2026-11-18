//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRectangleFaceOverlapOptions bounds all source evaluations, chart
// comparison and embedding work together. Zero selects 8M operations/16384 bits.
// Only identical original parameter domains are compared; phase is an integer
// lifted occurrence, not an inferred rotation of the model.
type WrappedRectangleFaceOverlapOptions struct {
	// AllowSeamCrossing uses one global lifted injectivity proof across original
	// fundamental restrictions. False preserves the single fundamental hull gate.
	AllowSeamCrossing                 bool
	MaxOperations, MaxCoefficientBits int
	ProjectionAxes                    [2]int
	MaxCells, MaxDepth                int
}

// WrappedFaceFragmentOwner refers to an original SewFace boundary use and model
// edge. Parameters are in that edge's original Range, in fragment direction.
type WrappedFaceFragmentOwner struct {
	Face, Use, Edge int
	Parameters      [2]*big.Rat
	UV              [2][2]*big.Rat
}

type WrappedFaceRegionFragment struct {
	Start, End [3]*big.Rat
	Owners     []WrappedFaceFragmentOwner
}

// WrappedFaceRegionOverlap is a physical face-region query, not a sewed solid
// or Boolean result. Cycles are inside-left in A's parameter chart. Reversed
// face flags are retained separately, and do not reverse these query cycles.
// Model vertices and continuous original model-edge mates are independently
// checked. All returned exact numbers are caller-owned.
type WrappedFaceRegionOverlap struct {
	Complete     bool
	FaceReversed [2]bool
	Embedding    geom.EmbeddedWrappedRectangleArrangement
	// LiftedEmbedding is populated instead of Embedding when AllowSeamCrossing
	// is selected. It retains each exact original fundamental restriction.
	LiftedEmbedding     *geom.LiftedWrappedRectangleArrangement
	Union, Intersection []WrappedFaceRegionFragment
	ExactOperations     int
}

// CertifyWrappedRectangleFaceOverlap accepts two direct immutable wrapped
// supports with a single CCW four-sided rectangular loop, no holes, and exact
// represented vertices. Each edge must be an affine parameter line on that
// same support with exactly the original use parameters (including reversal).
// Arbitrary mapped supports, other model-curve representations, approximate
// vertices remain unsupported. Seam-straddling hulls require the explicit
// AllowSeamCrossing proof. The result certifies finite region ancestry, not
// the topology of either surrounding shell.
func CertifyWrappedRectangleFaceOverlap(ctx context.Context, a, b SewFace, o WrappedRectangleFaceOverlapOptions) (WrappedFaceRegionOverlap, error) {
	var zero WrappedFaceRegionOverlap
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 1 {
		return zero, geom.ErrInvalid
	}
	used := 0
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q == nil || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	eval := func(w *geom.WrappedSurfacePatch, uv [2]*big.Rat) ([3]*big.Rat, error) {
		if used >= o.MaxOperations {
			return [3]*big.Rat{}, geom.ErrConvergence
		}
		j, e := w.EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return [3]*big.Rat{}, e
		}
		used += j.ExactOperations
		return j.Point, nil
	}
	inputs := [2]SewFace{a, b}
	var supports [2]*geom.WrappedSurfacePatch
	var rects [2][2][2]*big.Rat
	for owner, f := range inputs {
		w, ok := f.Face.Surface.(*geom.WrappedSurfacePatch)
		if !ok || w == nil || len(f.Face.Outer) != 4 || len(f.Face.Holes) != 0 || len(f.Edges) != 4 || len(f.Vertices) != 4 {
			return zero, geom.ErrConvergence
		}
		supports[owner] = w
		var lo, hi geom.Vec2
		for i, u := range f.Face.Outer {
			if !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) || u.Curve != nil || u.Edge < 0 || u.Edge >= 4 || u.UVEnd != f.Face.Outer[(i+1)%4].UVStart {
				return zero, geom.ErrConvergence
			}
			if i == 0 {
				lo = u.UVStart
				hi = lo
			}
			if u.UVStart.X < lo.X {
				lo.X = u.UVStart.X
			}
			if u.UVStart.Y < lo.Y {
				lo.Y = u.UVStart.Y
			}
			if u.UVStart.X > hi.X {
				hi.X = u.UVStart.X
			}
			if u.UVStart.Y > hi.Y {
				hi.Y = u.UVStart.Y
			}
		}
		if lo.X >= hi.X || lo.Y >= hi.Y {
			return zero, geom.ErrConvergence
		}
		corners := insetCorners(geom.ParameterRectangle{UMin: lo.X, UMax: hi.X, VMin: lo.Y, VMax: hi.Y})
		seen := map[int]bool{}
		seenVertices := map[int]bool{}
		for i, u := range f.Face.Outer {
			// Permit any cyclic starting corner, but never opposite winding or diagonal.
			ci := -1
			for j, p := range corners {
				if p == u.UVStart {
					ci = j
				}
			}
			if ci < 0 || u.UVEnd != corners[(ci+1)%4] || seen[u.Edge] {
				return zero, geom.ErrConvergence
			}
			seen[u.Edge] = true
			edge := f.Edges[u.Edge]
			model, ok := analyticModelLine(edge)
			use, valid := analyticUseLine(w, u)
			if !ok || !valid || model.surface != w {
				return zero, geom.ErrConvergence
			}
			for end := 0; end < 2; end++ {
				for axis := 0; axis < 2; axis++ {
					if e := check(model.uv[end][axis]); e != nil {
						return zero, e
					}
					if model.uv[end][axis].Cmp(use.uv[end][axis]) != 0 {
						return zero, geom.ErrConvergence
					}
				}
			}
			start, end := edge.Start, edge.End
			if u.Reversed {
				start, end = end, start
			}
			next := f.Face.Outer[(i+1)%4]
			nextEdge := f.Edges[next.Edge]
			nextStart := nextEdge.Start
			if next.Reversed {
				nextStart = nextEdge.End
			}
			if start < 0 || start >= 4 || end < 0 || end >= 4 || start == end || end != nextStart || seenVertices[start] {
				return zero, geom.ErrConvergence
			}
			seenVertices[start] = true
			point, e := eval(w, [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)})
			if e != nil {
				return zero, e
			}
			vertex := f.Vertices[start]
			if !vertex.IsFinite() {
				return zero, geom.ErrInvalid
			}
			for axis, v := range [3]float64{vertex.X, vertex.Y, vertex.Z} {
				if point[axis].Cmp(ratInset(v)) != 0 {
					return zero, geom.ErrConvergence
				}
			}
		}
		rects[owner] = [2][2]*big.Rat{{ratInset(lo.X), ratInset(hi.X)}, {ratInset(lo.Y), ratInset(hi.Y)}}
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	relation, proof, e := geom.CompareWrappedSurfaceCharts(ctx, supports[0], supports[1], geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: o.MaxCells})
	if e != nil {
		return zero, e
	}
	used += proof.ExactOperations
	if relation == nil || !proof.Complete || !proof.Equal || used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	options := geom.WrappedRectangleEmbeddingOptions{Arrangement: geom.WrappedRectangleOverlapOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits}, ProjectionAxes: o.ProjectionAxes, MaxCells: o.MaxCells, MaxDepth: o.MaxDepth}
	var embedding geom.EmbeddedWrappedRectangleArrangement
	var lifted *geom.LiftedWrappedRectangleArrangement
	var arrangement geom.WrappedRectangleArrangement
	if o.AllowSeamCrossing {
		proof, err := relation.CertifyLiftedRectangleArrangement(ctx, rects[0], rects[1], options)
		if err != nil {
			return zero, err
		}
		lifted = &proof
		arrangement = proof.Arrangement
		used += proof.ExactOperations
	} else {
		embedding, e = relation.CertifyEmbeddedRectangleArrangement(ctx, rects[0], rects[1], options)
		if e != nil {
			return zero, e
		}
		arrangement = embedding.Arrangement
		used += embedding.ExactOperations
	}
	build := func(edges []geom.WrappedRectangleEdge) ([]WrappedFaceRegionFragment, error) {
		out := make([]WrappedFaceRegionFragment, 0, len(edges))
		for _, edge := range edges {
			p, e := eval(supports[0], edge.Start)
			if e != nil {
				return nil, e
			}
			q, e := eval(supports[0], edge.End)
			if e != nil {
				return nil, e
			}
			fragment := WrappedFaceRegionFragment{Start: p, End: q}
			for owner, f := range inputs {
				if edge.Owners&(1<<owner) == 0 {
					continue
				}
				uv := edge.AUV
				if owner == 1 {
					uv = edge.BUV
				}
				found := false
				for ui, u := range f.Face.Outer {
					s := [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
					t := [2]*big.Rat{ratInset(u.UVEnd.X), ratInset(u.UVEnd.Y)}
					axis := 0
					if s[0].Cmp(t[0]) == 0 {
						axis = 1
					}
					fixed := axis ^ 1
					if uv[0][fixed].Cmp(s[fixed]) != 0 || uv[1][fixed].Cmp(s[fixed]) != 0 {
						continue
					}
					var parameters [2]*big.Rat
					inside := true
					for end := 0; end < 2; end++ {
						fraction := new(big.Rat).Quo(new(big.Rat).Sub(uv[end][axis], s[axis]), new(big.Rat).Sub(t[axis], s[axis]))
						if fraction.Sign() < 0 || fraction.Cmp(big.NewRat(1, 1)) > 0 {
							inside = false
							break
						}
						if u.Reversed {
							fraction.Sub(big.NewRat(1, 1), fraction)
						}
						parameters[end] = fraction
					}
					if !inside {
						continue
					}
					if found {
						return nil, geom.ErrConvergence
					}
					found = true
					for end := 0; end < 2; end++ {
						interval := f.Edges[u.Edge].Range
						parameters[end].Mul(parameters[end], new(big.Rat).Sub(ratInset(interval.Max), ratInset(interval.Min)))
						parameters[end].Add(parameters[end], ratInset(interval.Min))
						if e := check(parameters[end]); e != nil {
							return nil, e
						}
					}
					fragment.Owners = append(fragment.Owners, WrappedFaceFragmentOwner{Face: owner, Use: ui, Edge: u.Edge, Parameters: parameters, UV: uv})
				}
				if !found {
					return nil, geom.ErrConvergence
				}
			}
			out = append(out, fragment)
		}
		return out, nil
	}
	union, e := build(arrangement.Union)
	if e != nil {
		return zero, e
	}
	intersection, e := build(arrangement.Intersection)
	if e != nil {
		return zero, e
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return WrappedFaceRegionOverlap{Complete: true, FaceReversed: [2]bool{a.Face.Reversed, b.Face.Reversed}, Embedding: embedding, LiftedEmbedding: lifted, Union: union, Intersection: intersection, ExactOperations: used}, nil
}
