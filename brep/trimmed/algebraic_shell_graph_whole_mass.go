//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphWholeMassOptions adds opt-in exact retained-cell integration
// to the shared whole-object area/volume partition. MomentOrder0 (default)
// performs no polynomial or moment work;1 includes centroid/first moments;
// 2 additionally includes second moments and inertia. Constant-weight original
// spans must have degree<=6 for order1 and<=3 for order2. Unresolved rim cells
// remain interval contributions. Zero limits select existing cell defaults.
// Boundary.Refinement alone controls classification; it is never repeated.
type AlgebraicGraphWholeMassOptions struct {
	Boundary                                                                                AlgebraicGraphWindowAreaOptions
	MomentOrder                                                                             int
	MaxPolynomialCells, PolynomialCellOperations, FirstCellOperations, SecondCellOperations int
}
type AlgebraicGraphWholeMass struct {
	Whole   AlgebraicGraphWholeMeasures
	Moments *AlgebraicGraphWindowMoments
	Inertia *AlgebraicGraphWindowInertia
}

// BoundWholeMassProperties derives the entire support window and classifies
// once. Optional moment computations consume only private same-call partition
// evidence; no mutable public partition is accepted as a proof. Whole.Volume
// and Boundary.Partition.Volume are tightened consistently on those same cells.
// A positive volume lower bound is required for centroid/centroidal inertia;
// the presence of moment results does not mean all rim cells were resolved.
func (q *AlgebraicGraphMaterialQuery) BoundWholeMassProperties(ctx context.Context, o AlgebraicGraphWholeMassOptions) (out AlgebraicGraphWholeMass, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphWholeMass{}
		}
	}()
	if o.MomentOrder < 0 || o.MomentOrder > 2 {
		return out, geom.ErrInvalid
	}
	out.Whole, err = q.BoundWholeMeasures(ctx, o.Boundary)
	if err != nil {
		return out, err
	}
	if o.MomentOrder == 0 {
		return out, nil
	}
	polynomialOptions := AlgebraicGraphPolynomialWindowOptions{Refinement: o.Boundary.Refinement, MaxPolynomialCells: o.MaxPolynomialCells, CellOperations: o.PolynomialCellOperations}
	polynomial, e := q.integratePolynomialWindowPrepared(ctx, out.Whole.SupportWindow, polynomialOptions, &out.Whole.Boundary.Partition)
	if e != nil {
		return out, e
	}
	momentOptions := AlgebraicGraphWindowMomentOptions{Polynomial: polynomialOptions, MomentCellOperations: o.FirstCellOperations}
	moments, e := q.integrateWindowMomentsPrepared(ctx, out.Whole.SupportWindow, momentOptions, &polynomial)
	if e != nil {
		return out, e
	}
	out.Moments = &moments
	if o.MomentOrder == 2 {
		inertia, e := q.integrateWindowInertiaPrepared(ctx, out.Whole.SupportWindow, AlgebraicGraphWindowInertiaOptions{Moments: momentOptions, SecondCellOperations: o.SecondCellOperations}, &moments)
		if e != nil {
			return out, e
		}
		out.Inertia = &inertia
	}
	for i := 0; i < 2; i++ {
		out.Whole.Volume[i] = new(big.Rat).Set(polynomial.Bounds.Volume[i])
		out.Whole.Boundary.Partition.Volume[i] = new(big.Rat).Set(polynomial.Bounds.Volume[i])
	}
	return out, nil
}
