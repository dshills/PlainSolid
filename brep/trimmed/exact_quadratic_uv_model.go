package trimmed

import (
	"context"

	"plainsolid/geom"
)

// ModelCurve composes this exact UV trim with a single Bézier support. It
// preserves rational homogeneous coefficients even if ordinary spline
// export cannot represent them. The model domain is the normalized [0,1].
func (q ExactQuadraticUV) ModelCurve(ctx context.Context, p *geom.SurfacePatch, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if ctx == nil || p == nil || q.control[0][0] == nil {
		return nil, geom.ErrInvalid
	}
	return composeSplineTrimControls(ctx, p, q.control, o)
}

// ModelCurvePieces composes the trim across every exact source knot event.
// Nonrational crossings remain unresolved and do not round into the result.
func (q ExactQuadraticUV) ModelCurvePieces(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveCompositionOptions) (geom.SurfaceCurveCompositionPieces, error) {
	if p == nil {
		return geom.SurfaceCurveCompositionPieces{}, geom.ErrInvalid
	}
	uv, e := q.modelParameterCurve(ctx, o.SurfacePatchOptions)
	if e != nil {
		return geom.SurfaceCurveCompositionPieces{}, e
	}
	return p.ComposeParameterCurvePieces(ctx, uv, o)
}

func (q ExactQuadraticUV) modelParameterCurve(ctx context.Context, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if ctx == nil || q.control[0][0] == nil {
		return nil, geom.ErrInvalid
	}
	return exactQuadraticTrimParameterCurve(ctx, q.control, o)
}

// ModelCurveCells retains an exact model composition across algebraic
// source knot parameters. Each immutable cell supports exact evaluation
// at rational parameters proved to belong to its true interval.
func (q ExactQuadraticUV) ModelCurveCells(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveAgreementOptions) (geom.SurfaceCurveCompositionCells, error) {
	if p == nil {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	uv, e := q.modelParameterCurve(ctx, o.SurfacePatchOptions)
	if e != nil {
		return geom.SurfaceCurveCompositionCells{}, e
	}
	return p.BuildSurfaceCurveCompositionCells(ctx, uv, o)
}

// ExactQuadraticModelJunction records the common represented model point
// and independently certified enclosures from both incident knot cells.
type ExactQuadraticModelJunction struct {
	Left, Right geom.AlgebraicCurveEndpointBounds
	Point       geom.Vec3
	ErrorUpper  float64
}

// EncloseModelKnotJunctions bounds the model image at every interior
// algebraic knot event. The original C0 composition proves both endpoint
// limits identify the same point. One representative is used for both.
func (q ExactQuadraticUV) EncloseModelKnotJunctions(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveAgreementOptions,
	e geom.ComposedCurveEndpointOptions) ([]ExactQuadraticModelJunction, error) {
	cells, err := q.ModelCurveCells(ctx, p, o)
	if err != nil {
		return nil, err
	}
	if !cells.Complete || len(cells.Cells) == 0 {
		return nil, geom.ErrConvergence
	}
	leftWork, leftSub := e.MaxOperations, e.MaxSubdivisions
	if leftWork == 0 {
		leftWork = 8000000
	}
	if leftSub == 0 {
		leftSub = 8192
	}
	var result []ExactQuadraticModelJunction
	for i := 1; i < len(cells.Cells); i++ {
		var sides [2]geom.AlgebraicCurveEndpointBounds
		for j, c := range [2]*geom.ExactComposedCurveCell{cells.Cells[i-1], cells.Cells[i]} {
			if leftWork < 1 || leftSub < 1 {
				return nil, geom.ErrConvergence
			}
			options := e
			options.MaxOperations, options.MaxSubdivisions = leftWork, leftSub
			r, err := c.EncloseEndpoint(ctx, 1-j, options)
			if err != nil {
				return nil, err
			}
			if !r.Converged || !r.HasRepresentative {
				return nil, geom.ErrConvergence
			}
			leftWork -= r.ExactOperations
			leftSub -= r.Subdivisions
			sides[j] = r
		}
		best := sides[0]
		if sides[1].ErrorUpper < best.ErrorUpper {
			best = sides[1]
		}
		result = append(result, ExactQuadraticModelJunction{
			Left: sides[0], Right: sides[1], Point: best.Point, ErrorUpper: best.ErrorUpper,
		})
	}
	return result, nil
}
