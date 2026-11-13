package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// SplineGraphPlaneSection binds a certified section to an original
// rectangular spline face. The free coordinate increases on every piece.
// The other coordinate is a regular graph inside its original interval.
type SplineGraphPlaneSection struct {
	proof  *PiecewisePolynomialPlaneFaceSection
	pieces []PCurve
	plane  polynomialPlaneEquation
}

// CertifyRationalGraphPlaneFaceSection certifies one plane section without
// constructing a cutting body. The plane retains the exact orientation of
// (points[1]-points[0])×(points[2]-points[0]); freeAxis is original U (0) or V
// (1). Only free-axis knots are supported. Source seam continuity, strict
// interior crossing and exact endpoints are checked on every retained span.
// Each span adds one exact model composition and one raw-plane agreement
// check, using the corresponding Faces.Patches arithmetic limits.
func CertifyRationalGraphPlaneFaceSection(ctx context.Context, s Solid, face int, points [3]geom.Vec3, freeAxis int, o PolynomialPlaneSpanOptions) (*SplineGraphPlaneSection, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	for _, p := range points {
		if !p.IsFinite() {
			return nil, geom.ErrInvalid
		}
	}
	plane, e := polynomialPlaneFromPoints(points)
	if e != nil {
		return nil, e
	}
	return certifyRationalGraphPlaneFaceSection(ctx, s, face, plane, freeAxis, o)
}

func certifyRationalGraphPlaneFaceSection(ctx context.Context, s Solid, face int, plane polynomialPlaneEquation, freeAxis int, o PolynomialPlaneSpanOptions) (*SplineGraphPlaneSection, error) {
	proof, pieces, e := certifyPiecewiseGraphPlaneFaceSection(ctx, s, face, plane, freeAxis, o, true)
	if e != nil {
		return nil, e
	}
	// Complete exact composition proves coplanarity again on the retained
	// original charts, including the source knot boundaries.
	for _, piece := range pieces {
		curve, e := polynomialTrimDescriptor(ctx, piece, false, o.Faces.Patches)
		if e != nil {
			return nil, e
		}
		cells, e := proof.patch.BuildSurfaceCurveCompositionCells(ctx, curve, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: o.Faces.Patches})
		if e != nil || !cells.Complete || len(cells.Cells) != 1 {
			if e == nil {
				e = geom.ErrConvergence
			}
			return nil, e
		}
		ep, e := cells.Cells[0].IntersectRawPlane(ctx, [4]*big.Rat{plane.normal[0], plane.normal[1], plane.normal[2], new(big.Rat).Neg(plane.offset)}, geom.ExactCurvePlaneEventOptions{
			MaxOperations: o.Faces.Patches.MaxOperations, MaxCoefficientBits: o.Faces.Patches.MaxCoefficientBits,
		})
		if e != nil || !ep.Complete || !ep.Coincident {
			if e == nil {
				e = geom.ErrConvergence
			}
			return nil, e
		}
	}
	return &SplineGraphPlaneSection{proof: proof, pieces: pieces, plane: plane}, nil
}

func (p *SplineGraphPlaneSection) Face() int                    { return p.proof.Face() }
func (p *SplineGraphPlaneSection) FreeAxis() int                { return p.proof.FreeAxis() }
func (p *SplineGraphPlaneSection) Edges() [2]int                { return p.proof.Edges() }
func (p *SplineGraphPlaneSection) Parameters() [2]*big.Rat      { return p.proof.Parameters() }
func (p *SplineGraphPlaneSection) Knots() []PolynomialPlaneKnot { return p.proof.Knots() }
func (p *SplineGraphPlaneSection) Domains() []geom.ParameterRectangle {
	return p.proof.Domains()
}

// PlaneSideSign is the sign of the oriented raw plane residual when the
// original dependent coordinate increases across the section.
func (p *SplineGraphPlaneSection) PlaneSideSign() int { return p.proof.sign * p.plane.orientation }

// ProjectionAxes gives the two world coordinates independently proved
// affine on each original knot cell.
func (p *SplineGraphPlaneSection) ProjectionAxes() [2]int { return p.plane.axes }

// Pieces returns a caller-owned slice of immutable original-chart curves.
func (p *SplineGraphPlaneSection) Pieces() []PCurve { return append([]PCurve(nil), p.pieces...) }

// ModelCurveCells composes an original-chart section piece with the
// original support without exporting rounded spline controls.
func (p *SplineGraphPlaneSection) ModelCurveCells(ctx context.Context, piece int, o geom.SurfaceCurveAgreementOptions) (geom.SurfaceCurveCompositionCells, error) {
	if ctx == nil || p == nil || p.proof == nil || piece < 0 || piece >= len(p.pieces) {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	q, e := polynomialTrimDescriptor(ctx, p.pieces[piece], false, o.SurfacePatchOptions)
	if e != nil {
		return geom.SurfaceCurveCompositionCells{}, e
	}
	return p.proof.patch.BuildSurfaceCurveCompositionCells(ctx, q, o)
}
