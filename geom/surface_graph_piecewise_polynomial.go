package geom

import (
	"context"
	"math/big"
)

// PolynomialGraphMode selects one complete proof family, with no fallback.
type PolynomialGraphMode uint8

const (
	ConstantPolynomialGraph PolynomialGraphMode = iota
	FreeFactoredPolynomialGraph
	BivariatePolynomialGraph
)

// PiecewisePolynomialOptions preserves the bounded original slab and ownership
// contract of PiecewiseParabolaOptions while explicitly selecting each slab's
// proof family. Actual degrees may differ between cubic and quartic slabs.
type PiecewisePolynomialOptions struct {
	PiecewiseParabolaOptions
	Mode PolynomialGraphMode
}

// PolynomialGraphPiece retains exactly one authentic slab proof. Returned
// certificates are reports; a caller-modified report is not native authorization.
type PolynomialGraphPiece struct {
	Constant   *SplineGraphPolynomialContact
	FreeFactor *SplineGraphFactoredPolynomialContact
	Bivariate  *SplineGraphBivariatePolynomialContact
}

// Controls returns caller-owned original UV Bernstein data for the proved slab.
func (p PolynomialGraphPiece) Controls() [][2]*big.Rat {
	d := p.description()
	out := make([][2]*big.Rat, len(d.controls))
	for i, c := range d.controls {
		out[i] = [2]*big.Rat{new(big.Rat).Set(c[0]), new(big.Rat).Set(c[1])}
	}
	return out
}
func (p PolynomialGraphPiece) description() graphPieceDescription {
	if p.Constant != nil {
		x := p.Constant
		return graphPieceDescription{x.Power, x.Controls, x.PositiveSide, len(x.CertifiedCells) + x.Graph.VisitedCells, x.ExactOperations}
	}
	if p.FreeFactor != nil {
		x := p.FreeFactor
		cells := len(x.CertifiedCells) + x.Graph.VisitedCells
		for _, f := range x.Factors {
			cells += f.VisitedCells
		}
		return graphPieceDescription{x.Power, x.Controls, x.PositiveSide, cells, x.ExactOperations}
	}
	if p.Bivariate != nil {
		x := p.Bivariate
		cells := len(x.CertifiedCells) + x.Graph.VisitedCells
		for _, f := range x.Factors {
			cells += f.VisitedCells
		}
		return graphPieceDescription{x.Power, x.Controls, x.PositiveSide, cells, x.ExactOperations}
	}
	return graphPieceDescription{}
}

// PiecewiseSplineGraphPolynomial is a complete source-owned graph contact.
// Joins preserve distinct one-sided physical tangents, never inferred smoothness.
type PiecewiseSplineGraphPolynomial struct {
	Proven                 bool
	Domain                 ParameterRectangle
	FreeAxis, PositiveSide int
	Mode                   PolynomialGraphMode
	Pieces                 []PolynomialGraphPiece
	Joins                  []ParabolaKnotJoin
	ExactOperations        int
}

func DiscoverPiecewiseSplineGraphPolynomial(ctx context.Context, a, b *SurfacePatch, o PiecewisePolynomialOptions) (PiecewiseSplineGraphPolynomial, error) {
	if o.Mode > BivariatePolynomialGraph {
		return PiecewiseSplineGraphPolynomial{}, ErrInvalid
	}
	discover := func(ctx context.Context, a, b *SurfacePatch, axis int, p GraphPointContactOptions) (PolynomialGraphPiece, error) {
		switch o.Mode {
		case ConstantPolynomialGraph:
			r, e := DiscoverSplineGraphPolynomialContact(ctx, a, b, axis, p)
			return PolynomialGraphPiece{Constant: &r}, e
		case FreeFactoredPolynomialGraph:
			r, e := DiscoverSplineGraphFactoredPolynomialContact(ctx, a, b, axis, p)
			return PolynomialGraphPiece{FreeFactor: &r}, e
		default:
			r, e := DiscoverSplineGraphBivariatePolynomialContact(ctx, a, b, axis, p)
			return PolynomialGraphPiece{Bivariate: &r}, e
		}
	}
	r, e := discoverPiecewiseGraph(ctx, a, b, o.PiecewiseParabolaOptions, discover, func(p PolynomialGraphPiece) graphPieceDescription { return p.description() })
	out := PiecewiseSplineGraphPolynomial{Proven: r.Proven, Domain: r.Domain, FreeAxis: r.FreeAxis, PositiveSide: r.PositiveSide, Mode: o.Mode, Pieces: r.Pieces, Joins: r.Joins, ExactOperations: r.ExactOperations}
	if e != nil {
		return PiecewiseSplineGraphPolynomial{ExactOperations: r.ExactOperations}, e
	}
	return out, nil
}
