package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

// ParabolaCircleOsculationCut contains separate closed positive components
// whose closures meet at the sole original fourth-order contact point.
type ParabolaCircleOsculationCut struct {
	cut  *ParabolaCircleApexTangencyCut
	jets parabolaCircleOsculationRecipeJets
}

func (r *ParabolaCircleOsculationCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.cut.Components()
}

// History returns the original section coedges and roof and wall owners.
func (r *ParabolaCircleOsculationCut) History() [2]ParabolaCircleApexCutHistory {
	if r == nil {
		return [2]ParabolaCircleApexCutHistory{}
	}
	return r.cut.History()
}

// Contact returns the sole proved coordinate with both component-local
// vertices and respective negative and positive free rays.
func (r *ParabolaCircleOsculationCut) Contact() ParabolaCircleApexCutContact {
	if r == nil {
		return ParabolaCircleApexCutContact{}
	}
	return r.cut.Contact()
}

// CutParabolaCircleOsculationSplineGraphs admits the positive opposite
// open layer and circular lens. An enclosing layer with two incident
// sectors at the contact is a same-component pinch and returns ErrDegenerate.
func CutParabolaCircleOsculationSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleOsculationCut, error) {
	src := [3]*geom.BSplineSurface{base, a, b}
	sec, p, err := proveParabolaCircleOsculationSection(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	cut, err := assembleParabolaCircleTangentCut(ctx, src, sec, tolerance, o.Proof)
	if err != nil {
		return nil, err
	}
	for branch, rays := range cut.contact.Rays {
		for direction, id := range rays {
			if err := bindParabolaCircleOsculationRay(cut.components[branch], id, direction, branch, p); err != nil {
				return nil, err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ParabolaCircleOsculationCut{cut: cut, jets: parabolaCircleOsculationJets(p)}, nil
}

const nativeParabolaCircleOsculationCut = "trimmed-spline-graph-parabola-circle-osculation-cut"

type parabolaCircleOsculationCutRecipe struct {
	parabolaCircleApexCutRecipe
	Osculation parabolaCircleOsculationRecipeJets `json:"osculation"`
}

// WriteJSON records all original supports, both component-local contact
// identities and the exact fourth-order source jet.
func (r *ParabolaCircleOsculationCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.cut == nil {
		return geom.ErrInvalid
	}
	c := r.cut
	src, err := encodePolynomialGraphSupports(ctx, c.sources)
	if err != nil {
		return err
	}
	layers, contact := parabolaCircleApexCutLedger(c)
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleOsculationCutRecipe{
		parabolaCircleApexCutRecipe: parabolaCircleApexCutRecipe{
			Version: 1, Kind: nativeParabolaCircleOsculationCut, Sources: src,
			Axis: &c.axis, Tolerance: c.tolerance, Layers: layers, Contact: contact,
		},
		Osculation: r.jets,
	})
}

// ReadParabolaCircleOsculationCutJSON re-proves all original osculating
// coedges, both closed components and their sole permitted common point.
func ReadParabolaCircleOsculationCutJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleOsculationCut, error) {
	var recipe parabolaCircleOsculationCutRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleOsculationCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Layers) != 2 || recipe.Contact == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutParabolaCircleOsculationSplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	layers, contact := parabolaCircleApexCutLedger(out.cut)
	if recipe.Osculation != out.jets || !reflect.DeepEqual(recipe.Layers, layers) || !reflect.DeepEqual(recipe.Contact, contact) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.cut.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
