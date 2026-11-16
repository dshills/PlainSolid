package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// CuspidalCubicCommonOptions bounds the stock, affine chart, and section proofs.
type CuspidalCubicCommonOptions = NodalCubicCommonOptions

// CuspidalCubicCommonHistory relates the two exact signed regions and the
// singular section to the reconstructed native shell.
type CuspidalCubicCommonHistory struct {
	Base       int
	RoofFaces  [2]int // exterior, between the germs
	RoofOwners [2]int
	Vertices   [3]int // cusp, negative exit, positive exit
	Germs      [2]int // negative t, positive t
}

// CuspidalCubicCommon retains the exact intersection of the original stocks
// on a shared positive rectangular base.
type CuspidalCubicCommon struct {
	solid     Solid
	history   CuspidalCubicCommonHistory
	section   *geom.CuspidalCubicSection
	sources   [3]*geom.BSplineSurface
	freeAxis  int
	tolerance float64
}

func (r *CuspidalCubicCommon) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *CuspidalCubicCommon) History() CuspidalCubicCommonHistory {
	if r == nil {
		return CuspidalCubicCommonHistory{}
	}
	return r.history
}

// CommonCuspidalCubicSplineGraphs builds the lower envelope over a physical
// rectangle. The certified cusp opens along positive model X and its positive
// germ points towards positive model Y. All native vertices and endpoints
// must be exactly represented. The original cusp has zero first derivative.
func CommonCuspidalCubicSplineGraphs(ctx context.Context, base, a, c *geom.BSplineSurface, freeAxis int,
	tolerance float64, o CuspidalCubicCommonOptions) (*CuspidalCubicCommon, error) {
	boundary, err := buildCuspidalCubicEnvelope(ctx, base, a, c, freeAxis, tolerance, false, o)
	if err != nil {
		return nil, err
	}
	return &CuspidalCubicCommon{solid: boundary.solid, history: boundary.history, section: boundary.section,
		sources: [3]*geom.BSplineSurface{base, a, c}, freeAxis: freeAxis, tolerance: tolerance}, nil
}

type cuspidalCubicEnvelopeBoundary struct {
	solid   Solid
	history CuspidalCubicCommonHistory
	section *geom.CuspidalCubicSection
}

func buildCuspidalCubicEnvelope(ctx context.Context, base, a, c *geom.BSplineSurface, freeAxis int,
	tolerance float64, upper bool, o CuspidalCubicCommonOptions) (out cuspidalCubicEnvelopeBoundary, err error) {
	if ctx == nil || base == nil || a == nil || c == nil || freeAxis < 0 || freeAxis > 1 || !finite(tolerance) ||
		tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return out, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	stock, err := prepareRectangularPolynomialStock(ctx, base, a, c, o)
	if err != nil {
		return out, err
	}
	proof, err := geom.DiscoverProjectedGraphCuspidalCubic(ctx, stock.roofs[0], stock.roofs[1], [2]int{0, 1}, freeAxis, o.Proof)
	if err != nil {
		return out, err
	}
	d := stock.reference.Domain()
	pieces, xyz, err := cuspidalCubicEnvelopePieces(proof, d)
	if err != nil {
		return out, err
	}
	regions, err := cuspidalCubicEnvelopeRegions(proof, pieces, d)
	if err != nil {
		return out, err
	}
	if upper {
		for i := range regions {
			regions[i].first = !regions[i].first
		}
	}
	baseChart, err := polynomialEnvelopeBaseChart(stock.base)
	if err != nil {
		return out, err
	}
	var charts [2]graphEnvelopeChart
	for i, roof := range stock.roofs {
		aligned, err := geom.AlignAffineXYCharts(ctx, stock.reference, roof, o.Section.Patch)
		if err != nil {
			return out, err
		}
		charts[i], err = polynomialEnvelopeBaseChart(aligned)
		if err != nil {
			return out, err
		}
	}
	solid, envelope, err := assembleSplineEnvelopeOnCharts(ctx, base, a, c, regions[:], d, baseChart,
		charts, nodalCubicReferenceXY, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return out, err
	}
	if err := validateExactGraphPartitionEdges(ctx, solid, o.Proof); err != nil {
		return out, err
	}
	if err := bindPolynomialEnvelopeBase(solid, base, baseChart, envelope, nodalCubicReferenceXY); err != nil {
		return out, err
	}
	if err := affineMaterialVertexLinks(ctx, solid); err != nil {
		return out, err
	}
	history, err := bindCuspidalCubicEnvelope(solid, envelope, [2]*geom.BSplineSurface{a, c}, charts, proof, xyz, pieces, regions)
	if err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return cuspidalCubicEnvelopeBoundary{solid: solid, history: history, section: proof}, nil
}

func cuspidalCubicEnvelopePieces(p *geom.CuspidalCubicSection, d geom.ParameterRectangle) (pieces [2]openSectionPiece, xyz [3]geom.Vec3, err error) {
	for i, v := range p.Vertices {
		xyz[i], err = geom.RepresentedExactPoint(v.Point)
		if err != nil {
			return pieces, xyz, err
		}
	}
	second, third := p.Germs[1].ModelSecond, p.Germs[1].ModelThird
	if second[0].Sign() <= 0 || second[1].Sign() != 0 || third[0].Sign() != 0 || third[1].Sign() <= 0 ||
		xyz[1].X != d.UMax || xyz[2].X != d.UMax {
		return pieces, xyz, geom.ErrDomain
	}
	xScale := new(big.Rat).Mul(big.NewRat(1, 2), second[0])
	yScale := new(big.Rat).Mul(big.NewRat(1, 6), third[1])
	endpoints := [2][2]int{{1, 0}, {0, 2}}
	for i, germ := range p.Germs {
		control := cuspidalCubicBezierControls(p.Vertices[0].Point, xScale, yScale, germ.Range)
		curve, err := NewExactBezierUV(control)
		if err != nil {
			return pieces, xyz, err
		}
		for j, id := range endpoints[i] {
			if curve.ends[j] != (geom.Vec2{X: xyz[id].X, Y: xyz[id].Y}) {
				return pieces, xyz, geom.ErrConvergence
			}
		}
		pieces[i] = openSectionPiece{a: curve.ends[0], b: curve.ends[1], c: curve, side: -1}
	}
	return pieces, xyz, nil
}

func cuspidalCubicBezierControls(point [3]*big.Rat, xScale, yScale *big.Rat, span [2]*big.Rat) [][3]*big.Rat {
	width := new(big.Rat).Sub(span[1], span[0])
	control := make([][3]*big.Rat, 4)
	for i, t := range span {
		t2 := new(big.Rat).Mul(t, t)
		t3 := new(big.Rat).Mul(t2, t)
		x := new(big.Rat).Add(point[0], new(big.Rat).Mul(xScale, t2))
		y := new(big.Rat).Add(point[1], new(big.Rat).Mul(yScale, t3))
		control[3*i] = [3]*big.Rat{x, y, big.NewRat(1, 1)}
		h := new(big.Rat).Mul(width, big.NewRat(int64(1-2*i), 3))
		dx := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(t, xScale))
		dy := new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(t2, yScale))
		control[i+1] = [3]*big.Rat{
			new(big.Rat).Add(x, new(big.Rat).Mul(h, dx)),
			new(big.Rat).Add(y, new(big.Rat).Mul(h, dy)),
			big.NewRat(1, 1),
		}
	}
	return control
}

func cuspidalCubicEnvelopeRegions(p *geom.CuspidalCubicSection, pieces [2]openSectionPiece, d geom.ParameterRectangle) (out [2]graphEnvelopeRegion, err error) {
	for i, exit := range [2][2]geom.Vec2{{pieces[1].b, pieces[0].a}, {pieces[0].a, pieces[1].b}} {
		perimeter, err := saddleRectangleArc(exit[0], exit[1], d)
		if err != nil {
			return out, fmt.Errorf("trimmed: cuspidal cubic perimeter: %w", err)
		}
		first, last := pieces[1], pieces[0]
		if i == 1 {
			first.a, first.b, first.c = last.b, last.a, reversePCurve(last.c)
			last.a, last.b, last.c = pieces[1].b, pieces[1].a, reversePCurve(pieces[1].c)
		}
		out[i] = graphEnvelopeRegion{loop: append(append([]openSectionPiece{first}, perimeter...), last),
			first: p.Regions[i].HeightSide < 0}
	}
	return out, nil
}
