package trimmed

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type SplineShellTangencyPair struct {
	Faces, Projection [2]int
}

// SplineShellTangencySet records the distinct original chart pairs whose
// complete isolated contact proofs cover the two material boundaries.
type SplineShellTangencySet struct {
	Pairs                           []SplineShellTangency
	SeparatedPairs, ExactOperations int
}

// CombineExternallyTangentSplineShellPairs proves that two independently
// embedded rectangular spline shells meet only at finitely many explicitly
// selected interior graph contacts on two to eight original face pairs.
// All unlisted original pairs must independently prove strict separation.
// Both shells must be exterior to the other. Common is empty, cut keeps stock
// and point-contact union cannot be emitted as a native manifold.
func CombineExternallyTangentSplineShellPairs(ctx context.Context, a, b Solid, op parametric.BooleanOp, candidates []SplineShellTangencyPair,
	o SplineShellTangencyOptions) (Solid, SplineShellCombineHistory, SplineShellTangencySet, error) {
	var history SplineShellCombineHistory
	var empty SplineShellTangencySet
	if ctx == nil || op > parametric.Difference || len(candidates) < 2 || len(candidates) > 8 || o.Contact.MaxOperations < 0 ||
		o.Contact.MaxCoefficientBits != 0 && o.Contact.MaxCoefficientBits < 64 || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return Solid{}, history, empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, history, empty, e
	}
	if op == parametric.Union {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	ss := [2]Solid{a, b}
	var all [2][]splineEmbeddingFace
	for i, s := range ss {
		if _, e := s.validateSplineRectangleEmbedding(ctx, o.Shells.Embedding, &all[i]); e != nil {
			return Solid{}, history, empty, e
		}
	}
	return combineExternalSplineTangencySet(ctx, ss, all, op, candidates, o, false)
}

func combineExternalSplineTangencySet(ctx context.Context, solids [2]Solid, all [2][]splineEmbeddingFace, op parametric.BooleanOp,
	candidates []SplineShellTangencyPair, o SplineShellTangencyOptions, alreadySeparated bool) (Solid, SplineShellCombineHistory, SplineShellTangencySet, error) {
	var history SplineShellCombineHistory
	var empty SplineShellTangencySet
	if len(all[0])*len(all[1]) > 256 {
		return Solid{}, history, empty, geom.ErrConvergence
	}
	if o.Contact.MaxOperations == 0 {
		o.Contact.MaxOperations = 8000000
	}
	seen := map[[2]int]bool{}
	for _, pair := range candidates {
		id := pair.Faces
		if seen[id] || id[0] < 0 || id[0] >= len(all[0]) || id[1] < 0 || id[1] >= len(all[1]) {
			return Solid{}, history, empty, geom.ErrInvalid
		}
		seen[id] = true
	}
	var proof SplineShellTangencySet
	for i, a := range all[0] {
		for j, b := range all[1] {
			if e := ctx.Err(); e != nil {
				return Solid{}, history, empty, e
			}
			if seen[[2]int{i, j}] {
				continue
			}
			if !alreadySeparated {
				if e := proveSplinePatchPairDisjoint(ctx, a.patch, b.patch, o.Shells.Separation); e != nil {
					return Solid{}, history, empty, fmt.Errorf("trimmed: unqualified original face pair %d,%d: %w", i, j, e)
				}
			}
			proof.SeparatedPairs++
		}
	}
	for _, pair := range candidates {
		projections := [][2]int{pair.Projection}
		if alreadySeparated {
			projections = [][2]int{{0, 1}, {1, 2}, {0, 2}}
		}
		proven := false
		for _, axes := range projections {
			child := o
			child.Faces, child.Projection = pair.Faces, axes
			child.Contact.MaxOperations -= proof.ExactOperations
			if child.Contact.MaxOperations < 1 {
				return Solid{}, history, empty, geom.ErrConvergence
			}
			var used int
			_, _, p, e := combineTangentSplineShellPairWithWork(ctx, solids, all, op, child, true, true, &used)
			proof.ExactOperations += used
			if e != nil {
				if !alreadySeparated || !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDegenerate) &&
					!errors.Is(e, geom.ErrDomain) && !errors.Is(e, geom.ErrUnrepresentable) {
					return Solid{}, history, empty, e
				}
				continue
			}
			proof.Pairs = append(proof.Pairs, p)
			proven = true
			break
		}
		if !proven {
			return Solid{}, history, empty, geom.ErrConvergence
		}
	}
	if e := proveExternalSplineShellWitnesses(ctx, solids, all, o.Shells); e != nil {
		return Solid{}, history, empty, e
	}
	native, history := isolatedGraphTouchResult(solids[0], solids[1], op)
	return native, history, proof, nil
}

func proveExternalSplineShellWitnesses(ctx context.Context, solids [2]Solid, all [2][]splineEmbeddingFace, o SplineShellBooleanOptions) error {
	if o.MaxRays == 0 {
		o.MaxRays = 12
	}
	for i, s := range solids {
		j := i ^ 1
		inside, e := classifySeparatedSplineWitness(ctx, solids[j], splineShellPatchBounds(all[j]), s.vertices[0], o)
		if e != nil {
			return e
		}
		if inside {
			return geom.ErrConvergence
		}
	}
	return nil
}
