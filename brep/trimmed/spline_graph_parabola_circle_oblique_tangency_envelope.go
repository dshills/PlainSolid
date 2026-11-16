package trimmed

import (
	"context"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCircleObliqueEnvelopeHistory binds the four roof regions,
// original tangent rays, shared vertex and same-roof bridge to native IDs.
type ParabolaCircleObliqueEnvelopeHistory ParabolaCircleApexEnvelopeHistory

// ParabolaCircleObliqueTangencyEnvelope is the original closed Common or
// Fuse shell with a source-proved cyclic link at the nonapex contact.
type ParabolaCircleObliqueTangencyEnvelope struct {
	envelope *ParabolaCircleApexTangencyEnvelope
}

func (r *ParabolaCircleObliqueTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.envelope.Boundary()
}

func (r *ParabolaCircleObliqueTangencyEnvelope) History() ParabolaCircleObliqueEnvelopeHistory {
	if r == nil {
		return ParabolaCircleObliqueEnvelopeHistory{}
	}
	return ParabolaCircleObliqueEnvelopeHistory(r.envelope.History())
}

// CombineParabolaCircleObliqueTangencySplineGraphs assembles Common or Fuse
// only when the original circle normal admits a proved bridge to the
// interior of the dependent stock side. The bridge splits the enclosing
// roof while keeping the original source support all the way to the wall.
func CombineParabolaCircleObliqueTangencySplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleObliqueTangencyEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	section, err := proveParabolaCircleObliqueSection(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	regions, bridge, err := parabolaCircleObliqueEnvelopeRegions(section)
	if err != nil {
		return nil, err
	}
	s, assembly, err := combineOpenGraphRegions(ctx, base, a, b, regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	h, err := bindParabolaCircleTangentEnvelope(ctx, s, assembly, section, regions, bridge, src, op, o.Proof)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleObliqueTangencyEnvelope{
		envelope: &ParabolaCircleApexTangencyEnvelope{solid: s, history: h, sources: src, axis: axis, op: op, tolerance: tolerance},
	}, nil
}

func parabolaCircleObliqueEnvelopeRegions(sec parabolaCircleTangentSection) (out [4]openGraphRegion, bridge openSectionPiece, err error) {
	free, dependent := sec.axis, 1-sec.axis
	wall := big.NewRat(int64(sec.diskRegion), 1)
	advance := new(big.Rat).Sub(wall, sec.parameters[dependent])
	// At original contact (x0,y0), the normal is (−m,1). At
	// (x0−m*t,y0+t) the exact difference to p is
	// t*(1+m²−k*m²*t). The affine factor is positive at zero
	// and the proposed wall station, giving strict order along
	// the complete normal segment. The dependent side's endpoints
	// also bound the free coordinate throughout that segment.
	m2 := new(big.Rat).Mul(sec.slope, sec.slope)
	k := new(big.Rat).Quo(sec.second[0], big.NewRat(2, 1))
	drop := new(big.Rat).Mul(k, new(big.Rat).Mul(m2, advance))
	positiveAtWall := new(big.Rat).Add(big.NewRat(1, 1), m2)
	positiveAtWall.Sub(positiveAtWall, drop)
	if positiveAtWall.Sign() <= 0 {
		return out, bridge, geom.ErrConvergence
	}
	stockUV := [2]*big.Rat{}
	stockUV[free] = new(big.Rat).Sub(sec.parameters[free], new(big.Rat).Mul(sec.slope, advance))
	stockUV[dependent] = wall
	if stockUV[free].Sign() <= 0 || stockUV[free].Cmp(big.NewRat(1, 1)) >= 0 {
		return out, bridge, geom.ErrDomain
	}
	farUV := [2]*big.Rat{}
	for i, center := range sec.circleCenter {
		farUV[i] = new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), center), sec.parameters[i])
	}
	apex, err := exactGraphPartitionUV(sec.parameters)
	if err != nil {
		return out, bridge, err
	}
	far, err := exactGraphPartitionUV(farUV)
	if err != nil {
		return out, bridge, err
	}
	stock, err := exactGraphPartitionUV(stockUV)
	if err != nil {
		return out, bridge, err
	}
	return splitParabolaCircleTangentEnvelopeRegions(sec.regions, free, sec.diskRegion, apex, far, stock)
}

const nativeParabolaCircleObliqueEnvelope = "trimmed-spline-graph-parabola-circle-oblique-tangency-envelope"

// WriteJSON records the original supports and exact oblique bridge and contact.
func (r *ParabolaCircleObliqueTangencyEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.envelope == nil {
		return geom.ErrInvalid
	}
	e := r.envelope
	src, err := encodePolynomialGraphSupports(ctx, e.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleApexEnvelopeRecipe{
		Version: 1, Kind: nativeParabolaCircleObliqueEnvelope, Sources: src, Axis: &e.axis, Operation: &e.op,
		Tolerance: e.tolerance, Ledger: parabolaCircleApexEnvelopeLedgerFor(e.history),
	})
}

// ReadParabolaCircleObliqueTangencyEnvelopeJSON re-proves the finite outward
// bridge, tangent jets and all native face and coedge identities.
func ReadParabolaCircleObliqueTangencyEnvelopeJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleObliqueTangencyEnvelope, error) {
	var recipe parabolaCircleApexEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleObliqueEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CombineParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, parabolaCircleApexEnvelopeLedgerFor(out.envelope.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.envelope.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
