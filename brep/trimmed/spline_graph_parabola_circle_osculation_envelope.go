package trimmed

import (
	"context"
	"io"
	"math/big"
	"reflect"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCircleOsculationEnvelope is a source-verified Common or Fuse
// with one fourth-order circle-parabola junction and a single cyclic link.
type ParabolaCircleOsculationEnvelope struct {
	envelope *ParabolaCircleApexTangencyEnvelope
	jets     parabolaCircleOsculationRecipeJets
}

func (r *ParabolaCircleOsculationEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.envelope.Boundary()
}

// History returns the original roof and wall identifiers, both directed
// ray pairs, the osculation vertex and the same-roof bridge.
func (r *ParabolaCircleOsculationEnvelope) History() ParabolaCircleApexEnvelopeHistory {
	if r == nil {
		return ParabolaCircleApexEnvelopeHistory{}
	}
	return r.envelope.History()
}

// CombineParabolaCircleOsculationSplineGraphs builds Common or Fuse from
// the original roofs. The outward normal between the far circle extremum
// and stock side splits the enclosing roof into simple loops.
func CombineParabolaCircleOsculationSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	op parametric.BooleanOp, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleOsculationEnvelope, error) {
	if op != parametric.Intersection && op != parametric.Union {
		return nil, geom.ErrInvalid
	}
	src := [3]*geom.BSplineSurface{base, a, b}
	sec, p, err := proveParabolaCircleOsculationSection(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	node, err := exactGraphPartitionUV(p.Parameters)
	if err != nil {
		return nil, err
	}
	farUV := p.CircleCenter
	farUV[1-axis] = new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), farUV[1-axis]), p.Parameters[1-axis])
	far, err := exactGraphPartitionUV(farUV)
	if err != nil {
		return nil, err
	}
	wallUV := farUV
	wallUV[1-axis] = big.NewRat(int64(p.DiskRegion), 1)
	wall, err := exactGraphPartitionUV(wallUV)
	if err != nil {
		return nil, err
	}
	// The whole bridge stays at the stationary free coordinate and
	// strictly beyond the disk in the proved enclosing region.
	if err := subdivideOsculatingParabolaRays(&sec); err != nil {
		return nil, err
	}
	regions, bridge, err := splitParabolaCircleTangentEnvelopeRegions(sec.regions, axis, p.DiskRegion, node, far, wall)
	if err != nil {
		return nil, err
	}
	body, assembly, err := combineOpenGraphRegions(ctx, base, a, b, regions[:], op, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	h, err := bindParabolaCircleTangentEnvelope(ctx, body, assembly, sec, regions, bridge, src, op, o.Proof)
	if err != nil {
		return nil, err
	}
	for branch, rays := range h.Rays {
		for direction, id := range rays {
			if err := bindParabolaCircleOsculationRay(body, id, direction, branch, p); err != nil {
				return nil, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleOsculationEnvelope{
		envelope: &ParabolaCircleApexTangencyEnvelope{solid: body, history: h, sources: src, axis: axis, op: op, tolerance: tolerance},
		jets:     parabolaCircleOsculationJets(p),
	}, nil
}

// The osculating trim can have collinear endpoint chords at the
// junction. Exact de Casteljau subdivisions keep its full original
// trace and give the envelope two shorter chords on each parabolic ray.
func subdivideOsculatingParabolaRays(sec *parabolaCircleTangentSection) error {
	stations := [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)}
	for i := range 2 {
		var loop []openSectionPiece
		for _, piece := range sec.regions[i].loop {
			if piece.side >= 0 {
				loop = append(loop, piece)
				continue
			}
			original, ok := piece.c.(ExactQuadraticUV)
			if !ok {
				return geom.ErrConvergence
			}
			for j := range 2 {
				arc, err := original.Restrict(stations[j], stations[j+1])
				if err != nil {
					return err
				}
				loop = append(loop, openSectionPiece{a: arc.ends[0], b: arc.ends[1], c: arc, side: -1})
			}
		}
		sec.regions[i].loop = loop
	}
	return nil
}

const nativeParabolaCircleOsculationEnvelope = "trimmed-spline-graph-parabola-circle-osculation-envelope"

type parabolaCircleOsculationEnvelopeRecipe struct {
	parabolaCircleApexEnvelopeRecipe
	Osculation parabolaCircleOsculationRecipeJets `json:"osculation"`
}

// WriteJSON records the original roofs, native bridge and fourth-order jet.
func (r *ParabolaCircleOsculationEnvelope) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.envelope == nil {
		return geom.ErrInvalid
	}
	e := r.envelope
	sources, err := encodePolynomialGraphSupports(ctx, e.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleOsculationEnvelopeRecipe{
		parabolaCircleApexEnvelopeRecipe: parabolaCircleApexEnvelopeRecipe{
			Version: 1, Kind: nativeParabolaCircleOsculationEnvelope, Sources: sources, Axis: &e.axis,
			Operation: &e.op, Tolerance: e.tolerance, Ledger: parabolaCircleApexEnvelopeLedgerFor(e.history),
		},
		Osculation: r.jets,
	})
}

// ReadParabolaCircleOsculationEnvelopeJSON re-proves the original section,
// its fourth-order coedges and every bridge and wall incidence.
func ReadParabolaCircleOsculationEnvelopeJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleOsculationEnvelope, error) {
	var recipe parabolaCircleOsculationEnvelopeRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleOsculationEnvelope || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Operation == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CombineParabolaCircleOsculationSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, *recipe.Operation, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if recipe.Osculation != out.jets || !reflect.DeepEqual(recipe.Ledger, parabolaCircleApexEnvelopeLedgerFor(out.envelope.history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.envelope.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
