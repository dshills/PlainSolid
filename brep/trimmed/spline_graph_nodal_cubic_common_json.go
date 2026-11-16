package trimmed

import (
	"context"
	"encoding/json"
	"io"
	"reflect"

	"plainsolid/geom"
)

const nativeNodalCubicCommon = "trimmed-nodal-cubic-common"

type nodalCubicCommonRecipe struct {
	Version   int                     `json:"version"`
	Kind      string                  `json:"kind"`
	Sources   []json.RawMessage       `json:"sources"`
	Axis      *int                    `json:"freeAxis"`
	Tolerance float64                 `json:"tolerance"`
	Ledger    *nodalCubicCommonLedger `json:"ledger"`
}

type nodalCubicCommonLedger struct {
	Base        int     `json:"base"`
	RoofFaces   []int   `json:"roofFaces"`
	RoofOwners  []int   `json:"roofOwners"`
	Vertices    []int   `json:"vertices"`
	Chains      [][]int `json:"chains"`
	Seam        int     `json:"seam"`
	NodeRays    []int   `json:"nodeRays"`
	NodeSectors []int   `json:"nodeSectors"`
}

func makeNodalCubicEnvelopeLedger(h NodalCubicCommonHistory) *nodalCubicCommonLedger {
	chains := make([][]int, len(h.Chains))
	for i, edge := range h.Chains {
		chains[i] = h.Chains[i][:]
		if edge[1] < 0 {
			chains[i] = chains[i][:1]
		}
	}
	return &nodalCubicCommonLedger{
		Base: h.Base, RoofFaces: h.RoofFaces[:], RoofOwners: h.RoofOwners[:], Vertices: h.Vertices[:], Chains: chains,
		Seam: h.Seam, NodeRays: h.NodeRays[:], NodeSectors: h.NodeSectors[:],
	}
}

// WriteJSON records the original stock supports and the complete native
// seam, sector, and curve incidence. Output contains the source recipe.
func (r *NodalCubicCommon) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writeNodalCubicEnvelopeRecipe(ctx, w, nativeNodalCubicCommon, r.sources, r.axis, r.tolerance, r.history)
}

func writeNodalCubicEnvelopeRecipe(ctx context.Context, w io.Writer, kind string, stock [3]*geom.BSplineSurface, axis int,
	tolerance float64, h NodalCubicCommonHistory) error {
	sources, err := encodePolynomialGraphSupports(ctx, stock)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, nodalCubicCommonRecipe{
		Version: 1, Kind: kind, Sources: sources, Axis: &axis, Tolerance: tolerance, Ledger: makeNodalCubicEnvelopeLedger(h),
	})
}

// ReadNodalCubicCommonJSON reconstructs the original graph law and proves
// the same embedded native boundary before checking the recorded ledger.
func ReadNodalCubicCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o NodalCubicCommonOptions) (*NodalCubicCommon, error) {
	recipe, source, err := readNodalCubicEnvelopeRecipe(ctx, r, limits, nativeNodalCubicCommon)
	if err != nil {
		return nil, err
	}
	out, err := CommonNodalCubicSplineGraphs(ctx, source[0], source[1], source[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if err := recipe.verify(ctx, out.solid, out.history, limits); err != nil {
		return nil, err
	}
	return out, nil
}

func readNodalCubicEnvelopeRecipe(ctx context.Context, r io.Reader, limits JSONLimits, kind string) (nodalCubicCommonRecipe, [3]*geom.BSplineSurface, error) {
	var recipe nodalCubicCommonRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return recipe, [3]*geom.BSplineSurface{}, err
	}
	if recipe.Version != 1 || recipe.Kind != kind || len(recipe.Sources) != 3 || recipe.Axis == nil || recipe.Ledger == nil {
		return recipe, [3]*geom.BSplineSurface{}, geom.ErrInvalid
	}
	source, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	return recipe, source, err
}

func (r nodalCubicCommonRecipe) verify(ctx context.Context, solid Solid, history NodalCubicCommonHistory, limits JSONLimits) error {
	if !reflect.DeepEqual(r.Ledger, makeNodalCubicEnvelopeLedger(history)) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{solid}) > limits.MaxEntities) {
		return geom.ErrConvergence
	}
	return ctx.Err()
}
