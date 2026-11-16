package trimmed

import (
	"context"
	"io"
	"reflect"

	"plainsolid/geom"
)

// ParabolaCircleObliqueCutHistory identifies the original region, caps and
// coedges of one native component, with component-local IDs.
type ParabolaCircleObliqueCutHistory ParabolaCircleApexCutHistory

// ParabolaCircleObliqueCutContact identifies the exact tangent point and
// distinct native vertex IDs. Rays are ordered by negative and positive
// free-coordinate direction within each component.
type ParabolaCircleObliqueCutContact ParabolaCircleApexCutContact

// ParabolaCircleObliqueTangencyCut has one closed wall layer and one closed
// circular lens. Their closures share the single proved tangent point.
type ParabolaCircleObliqueTangencyCut struct {
	cut *ParabolaCircleApexTangencyCut
}

func (r *ParabolaCircleObliqueTangencyCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.cut.Components()
}

func (r *ParabolaCircleObliqueTangencyCut) History() [2]ParabolaCircleObliqueCutHistory {
	if r == nil {
		return [2]ParabolaCircleObliqueCutHistory{}
	}
	h := r.cut.History()
	return [2]ParabolaCircleObliqueCutHistory{ParabolaCircleObliqueCutHistory(h[0]), ParabolaCircleObliqueCutHistory(h[1])}
}

func (r *ParabolaCircleObliqueTangencyCut) Contact() ParabolaCircleObliqueCutContact {
	if r == nil {
		return ParabolaCircleObliqueCutContact{}
	}
	return ParabolaCircleObliqueCutContact(r.cut.Contact())
}

// CutParabolaCircleObliqueTangencySplineGraphs admits the original disk
// and opposite open region when both have positive thickness. The other
// operand order has a same-shell pinch and returns ErrDegenerate.
func CutParabolaCircleObliqueTangencySplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface,
	axis int, tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleObliqueTangencyCut, error) {
	src := [3]*geom.BSplineSurface{base, a, b}
	section, err := proveParabolaCircleObliqueSection(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	cut, err := assembleParabolaCircleTangentCut(ctx, src, section, tolerance, o.Proof)
	if err != nil {
		return nil, err
	}
	return &ParabolaCircleObliqueTangencyCut{cut: cut}, nil
}

const nativeParabolaCircleObliqueCut = "trimmed-spline-graph-parabola-circle-oblique-tangency-cut"

// WriteJSON records both original roofs and the exact indexed contact.
func (r *ParabolaCircleObliqueTangencyCut) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.cut == nil {
		return geom.ErrInvalid
	}
	src, err := encodePolynomialGraphSupports(ctx, r.cut.sources)
	if err != nil {
		return err
	}
	layers, contact := parabolaCircleApexCutLedger(r.cut)
	return writePolynomialGraphRecipe(ctx, w, parabolaCircleApexCutRecipe{Version: 1, Kind: nativeParabolaCircleObliqueCut,
		Sources: src, Axis: &r.cut.axis, Tolerance: r.cut.tolerance, Layers: layers, Contact: contact})
}

// ReadParabolaCircleObliqueTangencyCutJSON re-proves the original tangent
// jets and both component-local indices of the sole shared point.
func ReadParabolaCircleObliqueTangencyCutJSON(ctx context.Context, input io.Reader, limits JSONLimits,
	o ParabolaCircleGraphOptions) (*ParabolaCircleObliqueTangencyCut, error) {
	var recipe parabolaCircleApexCutRecipe
	if err := readPolynomialGraphRecipe(ctx, input, limits, &recipe); err != nil {
		return nil, err
	}
	if recipe.Version != 1 || recipe.Kind != nativeParabolaCircleObliqueCut || len(recipe.Sources) != 3 ||
		recipe.Axis == nil || len(recipe.Layers) != 2 || recipe.Contact == nil {
		return nil, geom.ErrInvalid
	}
	src, err := decodePolynomialGraphSupports(ctx, recipe.Sources, limits.MaxControlPoints)
	if err != nil {
		return nil, err
	}
	out, err := CutParabolaCircleObliqueTangencySplineGraphs(ctx, src[0], src[1], src[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	layers, contact := parabolaCircleApexCutLedger(out.cut)
	if !reflect.DeepEqual(recipe.Layers, layers) || !reflect.DeepEqual(recipe.Contact, contact) ||
		(limits.MaxEntities > 0 && polynomialGraphEntityCount(out.cut.components[:]) > limits.MaxEntities) {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
