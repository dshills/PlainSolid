//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// AlgebraicGraphStockPoint is an exact point on an original stock cap trim.
// Every coordinate is a rational image of the same source parameter and
// its interval has the requested absolute width or less.
type AlgebraicGraphStockPoint struct {
	Coordinates                   [3]*geom.ExactAlgebraicRoot
	Bounds                        [3]geom.ExactParameterInterval
	ExactOperations, Subdivisions int
}

// ResolveDirectionalWitness evaluates the selected original trim point,
// using its exact parameter. A witness from independently prepared stock
// is rejected, even when it happens to use the same numeric owner indices.
func (m AlgebraicGraphVerticalMaterial) ResolveDirectionalWitness(ctx context.Context,
	w AlgebraicGraphDirectionalWitness, width *big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockPoint, error) {
	return m.resolveDirectionalWitness(ctx, w, width, [3][4]*big.Rat{
		{big.NewRat(1, 1)}, {nil, big.NewRat(1, 1)}, {nil, nil, big.NewRat(1, 1)}}, opts)
}

// ResolveDirectionalWitness returns the selected point in world axes
// by evaluating exact affine coordinate forms of the original trim.
func (o AlgebraicGraphStockOccurrence) ResolveDirectionalWitness(ctx context.Context,
	w AlgebraicGraphDirectionalWitness, width *big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockPoint, error) {
	m, err := o.pose.Matrix()
	if err != nil {
		return AlgebraicGraphStockPoint{}, err
	}
	return o.model.resolveDirectionalWitness(ctx, w, width, m, opts)
}

func (m AlgebraicGraphVerticalMaterial) resolveDirectionalWitness(ctx context.Context, w AlgebraicGraphDirectionalWitness,
	width *big.Rat, forms [3][4]*big.Rat, opts geom.ExactCurvePlaneEventOptions) (AlgebraicGraphStockPoint, error) {
	var zero AlgebraicGraphStockPoint
	if w.Cap < 0 || w.Cap > 1 || w.Wire < 0 || w.Use < 0 ||
		w.CurveParameter == nil || w.source == nil || width == nil || width.Sign() <= 0 {
		return zero, geom.ErrInvalid
	}
	cap := m.caps[w.Cap]
	if cap.recipe != w.source.stock || w.source.index != [3]int{w.Cap, w.Wire, w.Use} || w.CurveParameter != w.source.root ||
		w.Wire >= len(cap.wires) || w.Use >= len(cap.wires[w.Wire]) {
		return zero, geom.ErrInvalid
	}
	b, err := m.extremaBudget(ctx, opts)
	if err != nil {
		return zero, err
	}
	work, splits := b.work, b.subdiv
	cell := cap.wires[w.Wire][w.Use].Cell
	var out AlgebraicGraphStockPoint
	for axis, form := range forms {
		root, n, steps, e := cell.LinearImageRootWithWork(ctx, w.CurveParameter, form, b.limits())
		if e != nil {
			return zero, e
		}
		if e = b.charge(n, steps); e != nil {
			return zero, e
		}
		out.Coordinates[axis] = root
		h, e := root.Enclose(ctx, width, b.limits())
		if e != nil {
			return zero, e
		}
		if e = b.charge(h.Operations, h.Subdivisions); e != nil {
			return zero, e
		}
		if !h.Converged || !h.HasBounds {
			return zero, geom.ErrConvergence
		}
		out.Bounds[axis] = h.Interval
	}
	out.ExactOperations, out.Subdivisions = work-b.work, splits-b.subdiv
	return out, nil
}
