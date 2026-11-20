package boolean

import (
	"context"
	"math"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
)

// Combine returns the regularized union, common or cut of two solids
// bounded by planes and cylinders (section 4). The operands are closed, outward
// oriented, embedded boundaries; the zero Solid is the empty solid. The
// result validates within the tolerances of o (section 8) or an error of
// section 9 is returned with the zero Result. A cancelled context returns
// the context's error unwrapped, in preference to any other error.
// Combine does not modify its operands and keeps no state between calls.
func Combine(ctx context.Context, a, b trimmed.Solid, op Operation, o Options) (Result, error) {
	if ctx == nil {
		return Result{}, fail(ErrInvalid, "intake", "nil context")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if op > Cut {
		return Result{}, fail(ErrInvalid, "intake", "unknown operation %d", op)
	}
	o, err := o.normalized()
	if err != nil {
		return Result{}, err
	}
	r, err := combine(ctx, a, b, op, o)
	if err != nil {
		if e := ctx.Err(); e != nil {
			return Result{}, e
		}
		return Result{}, err
	}
	return r, nil
}

func combine(ctx context.Context, a, b trimmed.Solid, op Operation, o Options) (Result, error) {
	// Section 10: the face limit is checked before a face or an edge is
	// examined. This is the one place where the number of faces of an
	// operand is read; it also tells an empty operand (S6).
	var empty [2]bool
	for i, n := range [2]int{a.Topology().Faces, b.Topology().Faces} {
		if n > o.MaxFaces {
			return Result{}, fail(ErrBudget, "intake", "operand %s has %d faces, above MaxFaces %d", operandName(i), n, o.MaxFaces)
		}
		empty[i] = n == 0
	}
	t, err := working(o, a, b, empty)
	if err != nil {
		return Result{}, err
	}
	if empty[0] && empty[1] {
		return Result{Tolerance: t.eps}, nil // T1, V8
	}
	var ops [2]*operand
	for i, s := range [2]trimmed.Solid{a, b} {
		// B is seated on A: A is taken in first, and as it is.
		var onto *operand
		if i == 1 && !empty[0] {
			onto = ops[0]
		}
		if ops[i], err = intake(i, s, t, onto); err != nil {
			return Result{}, err
		}
	}
	// I2: an empty operand, or bounds that stay apart when grown by the
	// filter margin, need no pairwise work: every face is one piece
	// outside the other operand. Otherwise the intersection stage builds
	// the arrangement, the faces are split along it and the pieces are
	// classified. Either way the same stages follow.
	var arr *arrangement
	var pieces []piece
	if empty[0] || empty[1] || ops[0].box.clear(ops[1].box, t.margin) {
		if arr, err = newArrangement(ctx, ops, t, o); err == nil {
			err = arr.seal()
		}
		if err != nil {
			return Result{}, err
		}
		if pieces = arr.whole(); len(pieces) > o.MaxPieces {
			return Result{}, fail(ErrBudget, "splitting", "more than MaxPieces %d pieces", o.MaxPieces)
		}
	} else {
		if arr, err = arrange(ctx, ops, t, o); err != nil {
			return Result{}, err
		}
		if pieces, err = arr.split(); err != nil {
			return Result{}, err
		}
		if err = arr.classify(pieces); err != nil {
			return Result{}, err
		}
	}
	keep := choose(pieces, op, false)
	if err := arr.manifold(pieces, keep); err != nil {
		return Result{}, err
	}
	edges, sheets, err := arr.coalesce(pieces, keep)
	if err != nil {
		return Result{}, err
	}
	built, err := assemble(ctx, arr.points, edges, sheets, t)
	if err != nil {
		return Result{}, err
	}
	r := Result{
		Solid: built.solid, Shells: built.shells, Voids: built.voids, Owners: built.owners, Solids: built.solids,
		Tolerance: t.eps, Nodes: len(arr.points), Spans: len(arr.spans), Bundles: len(arr.bundles), Traces: len(arr.traces), Pieces: len(pieces),
	}
	for _, sh := range built.sheets {
		r.Sources = append(r.Sources, sh.sources)
	}
	for _, p := range arr.points {
		r.NodeRadius = math.Max(r.NodeRadius, p.radius)
	}
	for _, c := range arr.contacts {
		at, _ := arr.locus(c)
		r.Touching = append(r.Touching, c.faces)
		r.TouchingAt = append(r.TouchingAt, [3]float64{at.X, at.Y, at.Z})
	}
	// Section 8. V1 and V2 were checked by assemble.
	if r.EdgeResidual, r.VertexResidual, err = residuals(built); err != nil {
		return Result{}, err
	}
	// V5 takes the measures of an operand from the caller when it gave
	// them, and otherwise from the closed form over its faces, as it
	// takes those of the result. Only a solid with a face that the
	// closed form does not cover is integrated numerically.
	var measures [3]parametric.Properties
	for i, known := range [2]*parametric.Properties{o.KnownA, o.KnownB} {
		faces := ops[i].faces
		closed, ok := measured(len(faces), func(f int) (surface, bool, [][]leg) { return faces[f].s, faces[f].reversed, arr.borders[i][f] })
		switch {
		case known != nil:
			measures[i] = *known
		case ok:
			measures[i] = closed
		default:
			if measures[i], err = measure(ctx, [2]trimmed.Solid{a, b}[i], empty[i], o.Integration); err != nil {
				return Result{}, err
			}
		}
	}
	pr, ok := measured(len(sheets), func(f int) (surface, bool, [][]leg) {
		return sheets[f].s, sheets[f].reversed, legsOf(edges, sheets[f].loops)
	})
	if !ok {
		if pr, err = measure(ctx, built.solid, built.faces == nil, o.Integration); err != nil {
			return Result{}, err
		}
	}
	pa, pb := measures[0], measures[1]
	r.VolumeA, r.VolumeB, r.Volume, r.Properties = pa.Volume, pb.Volume, pr.Volume, pr
	if err := identities(&arr.fabric, pieces, op, pa, pb, pr, o, t); err != nil {
		return Result{}, err
	}
	if err := membership(ops, empty, built, op, o, t); err != nil {
		return Result{}, err
	}
	return r, nil
}
