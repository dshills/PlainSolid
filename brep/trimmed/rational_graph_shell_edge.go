package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func verifyRationalGraphModelTerminal(ctx context.Context, model parametric.Edge, terminal RationalGraphShellTerminal, plane [4]*big.Rat, limits geom.SurfacePatchOptions, contacts geom.ComposedPlaneContactsOptions) error {
	curve, reverse, e := exactEdgeSplineWithOptions(model, limits)
	if e != nil {
		return e
	}
	if curve == nil {
		return geom.ErrConvergence
	}
	lo, hi := ratInset(model.Range.Min), ratInset(model.Range.Max)
	x := terminal.Parameter
	if x.Cmp(lo) <= 0 || x.Cmp(hi) >= 0 {
		return geom.ErrConvergence
	}
	// A model descriptor extracted from an isocurve retains the host
	// spline's original units, while the edge stores its actual range.
	t := new(big.Rat).Quo(new(big.Rat).Sub(x, lo), new(big.Rat).Sub(hi, lo))
	if reverse {
		t.Sub(big.NewRat(1, 1), t)
	}
	a, b := curve.Domain()
	start, end := ratInset(a), ratInset(b)
	p := new(big.Rat).Add(start, new(big.Rat).Mul(t, new(big.Rat).Sub(end, start)))
	r, e := curve.DiscoverPlaneContacts(ctx, plane, contacts)
	if e != nil {
		return e
	}
	if !r.Complete || len(r.Events) != 1 {
		return geom.ErrConvergence
	}
	h := r.Events[0]
	if h.Junction != nil {
		if !h.Junction.Crossing || !h.Junction.Before.Transverse || !h.Junction.After.Transverse {
			return geom.ErrConvergence
		}
	} else if len(h.Local) != 1 || !h.Local[0].Transverse || !h.Local[0].Crossing {
		return geom.ErrConvergence
	}
	w, e := h.Event.EvaluateRationalWitness(ctx, p, contacts.EventOptions)
	if e != nil {
		return e
	}
	for i, v := range w.Point {
		if v.Cmp(terminal.Point[i]) != 0 {
			return geom.ErrConvergence
		}
	}
	return nil
}
