package boolean

import (
	"context"
	"math"

	"plainsolid/brep/trimmed"
)

// Combine returns the regularized union, common or cut of two solids
// bounded by planes (section 4). The operands are closed, outward
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
		if ops[i], err = intake(i, s, t); err != nil {
			return Result{}, err
		}
	}
	// I2: an empty operand, or boxes that stay apart when grown by the
	// filter margin, need no pairwise work. Every face is one piece
	// outside the other operand.
	if !empty[0] && !empty[1] && !ops[0].box.clear(ops[1].box, t.margin) {
		return Result{}, fail(ErrUnsupported, "arrangement", "operands whose boxes overlap need the arrangement of section 7.2, which is not built yet")
	}
	f := unsplit(ops, t)
	pieces := f.whole()
	built, err := assemble(ctx, f.points, f.elems, sheets(f, pieces, choose(pieces, op, false)), t)
	if err != nil {
		return Result{}, err
	}
	r := Result{
		Solid: built.solid, Shells: built.shells, Voids: built.voids, Owners: built.owners, Solids: built.solids,
		Tolerance: t.eps, Nodes: len(f.points), Spans: len(f.elems), Pieces: len(pieces),
	}
	for _, sh := range built.sheets {
		r.Sources = append(r.Sources, sh.sources)
	}
	for _, p := range f.points {
		r.NodeRadius = math.Max(r.NodeRadius, p.radius)
	}
	// Section 8. V1 and V2 were checked by assemble.
	if r.EdgeResidual, r.VertexResidual, err = residuals(built); err != nil {
		return Result{}, err
	}
	pa, err := measure(ctx, a, empty[0], o.Integration)
	if err != nil {
		return Result{}, err
	}
	pb, err := measure(ctx, b, empty[1], o.Integration)
	if err != nil {
		return Result{}, err
	}
	pr, err := measure(ctx, built.solid, built.faces == nil, o.Integration)
	if err != nil {
		return Result{}, err
	}
	r.VolumeA, r.VolumeB, r.Volume = pa.Volume, pb.Volume, pr.Volume
	if err := identities(f, pieces, op, pa, pb, pr, o, t); err != nil {
		return Result{}, err
	}
	if err := membership(ops, empty, built, op, o, t); err != nil {
		return Result{}, err
	}
	return r, nil
}
