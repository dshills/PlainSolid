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

func (r *NodalCubicCommon) recipeLedger() *nodalCubicCommonLedger {
	h := r.history
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
	sources, err := encodePolynomialGraphSupports(ctx, r.sources)
	if err != nil {
		return err
	}
	return writePolynomialGraphRecipe(ctx, w, nodalCubicCommonRecipe{
		Version: 1, Kind: nativeNodalCubicCommon, Sources: sources, Axis: &r.axis, Tolerance: r.tolerance, Ledger: r.recipeLedger(),
	})
}

// ReadNodalCubicCommonJSON reconstructs the original graph law and proves
// the same embedded native boundary before checking the recorded ledger.
func ReadNodalCubicCommonJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o NodalCubicCommonOptions) (*NodalCubicCommon, error) {
	var recipe nodalCubicCommonRecipe
	if err := readPolynomialGraphRecipe(ctx, r, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeNodalCubicCommon || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || recipe.Ledger == nil {
		return nil, geom.ErrInvalid
	}
	source, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CommonNodalCubicSplineGraphs(ctx, source[0], source[1], source[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(recipe.Ledger, out.recipeLedger()) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount([]Solid{out.solid}) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
