package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// NodalCubicCommonOptions bounds the original chart, clearance and section proofs.
type NodalCubicCommonOptions = PolynomialTangencyEnvelopeOptions

// NodalCubicCommonHistory retains the original nodal section and the
// independent seam in the exterior region. The four roof faces are upper
// exterior, lower exterior, loop interior, and between the open arms.
type NodalCubicCommonHistory struct {
	Base        int
	RoofFaces   [4]int
	RoofOwners  [4]int
	Vertices    [4]int    // node, loop turn, negative exit, positive exit
	Chains      [4][2]int // second slot is -1 for either unsplit arm
	Seam        int
	NodeRays    [4]int
	NodeSectors [4]int // face between each ray and the next cyclic ray
}

// NodalCubicCommon retains the exact intersection of the original shared-base
// stocks. All original roof coedges and the exterior seam are re-proved.
type NodalCubicCommon struct {
	solid     Solid
	history   NodalCubicCommonHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	tolerance float64
}

func (r *NodalCubicCommon) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *NodalCubicCommon) History() NodalCubicCommonHistory {
	if r == nil {
		return NodalCubicCommonHistory{}
	}
	return r.history
}

// CommonNodalCubicSplineGraphs builds a single exact Common on one physical
// rectangular footprint. It uses the certified nodal cubic's canonical axes
// pointing along positive model X and Y, and requires represented native
// vertices. The base and both roof charts retain their original support laws.
func CommonNodalCubicSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o NodalCubicCommonOptions) (*NodalCubicCommon, error) {
	if ctx == nil || base == nil || a == nil || b == nil || freeAxis < 0 || freeAxis > 1 || !finite(tolerance) ||
		tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stock, err := prepareRectangularPolynomialStock(ctx, base, a, b, o)
	if err != nil {
		return nil, err
	}
	proof, err := geom.DiscoverProjectedGraphNodalCubic(ctx, stock.roofs[0], stock.roofs[1], [2]int{0, 1}, freeAxis, o.Proof)
	if err != nil {
		return nil, err
	}
	domain := stock.reference.Domain()
	pieces, xyz, err := nodalCubicCommonPieces(proof, domain)
	if err != nil {
		return nil, err
	}
	regions, seam, err := nodalCubicCommonRegions(proof, pieces, xyz, domain)
	if err != nil {
		return nil, err
	}
	baseChart, err := polynomialEnvelopeBaseChart(stock.base)
	if err != nil {
		return nil, err
	}
	var charts [2]graphEnvelopeChart
	for i, roof := range stock.roofs {
		alignment, err := geom.AlignAffineXYCharts(ctx, stock.reference, roof, o.Section.Patch)
		if err != nil {
			return nil, err
		}
		charts[i], err = polynomialEnvelopeBaseChart(alignment)
		if err != nil {
			return nil, err
		}
	}
	reference := func(p geom.Vec3) (geom.Vec2, error) { return geom.Vec2{X: p.X, Y: p.Y}, nil }
	solid, envelope, err := assembleSplineEnvelopeOnCharts(ctx, base, a, b, regions[:], domain, baseChart,
		charts, reference, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	if err := validateExactGraphPartitionEdges(ctx, solid, o.Proof); err != nil {
		return nil, err
	}
	if err := bindPolynomialEnvelopeBase(solid, base, baseChart, envelope, reference); err != nil {
		return nil, err
	}
	if err := affineMaterialVertexLinks(ctx, solid); err != nil {
		return nil, err
	}
	history, err := bindNodalCubicCommon(solid, envelope, [2]*geom.BSplineSurface{a, b}, charts, proof, xyz, pieces, regions, seam)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &NodalCubicCommon{solid: solid, history: history, sources: [3]*geom.BSplineSurface{base, a, b},
		axis: freeAxis, tolerance: tolerance}, nil
}

func nodalCubicCommonPieces(p *geom.NodalCubicSection, domain geom.ParameterRectangle) (pieces [4][]openSectionPiece,
	xyz [4]geom.Vec3, err error) {
	for i, vertex := range p.Vertices {
		xyz[i], err = geom.RepresentedExactPoint(vertex.Point)
		if err != nil {
			return pieces, xyz, err
		}
	}
	// The transverse geometry proof gives rays (1,alpha) and (-1,alpha).
	// Recover the exact canonical-to-physical derivatives from that basis.
	negativeNodeParameter := p.Edges[0].Range[1]
	alpha := new(big.Rat).Neg(negativeNodeParameter)
	scaleX := new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Sub(p.Rays[0].Model[0], p.Rays[1].Model[0]))
	scaleY := new(big.Rat).Quo(new(big.Rat).Add(p.Rays[0].Model[1], p.Rays[1].Model[1]), new(big.Rat).Mul(big.NewRat(2, 1), alpha))
	if scaleX.Sign() <= 0 || scaleY.Sign() <= 0 ||
		new(big.Rat).Add(p.Rays[0].Model[0], p.Rays[1].Model[0]).Sign() != 0 ||
		p.Rays[0].Model[1].Cmp(p.Rays[1].Model[1]) != 0 ||
		xyz[2].X != domain.UMax || xyz[3].X != domain.UMax {
		return pieces, xyz, geom.ErrDomain
	}
	for i, edge := range p.Edges {
		control := nodalCubicBezierControls(p, scaleX, scaleY, edge.Range)
		curve, err := NewExactBezierUV(control)
		if err != nil {
			return pieces, xyz, err
		}
		for j, id := range [2]int{edge.Start, edge.End} {
			point := geom.Vec2{X: xyz[id].X, Y: xyz[id].Y}
			if curve.ends[j] != point {
				return pieces, xyz, geom.ErrConvergence
			}
		}
		children := []ExactBezierUV{curve}
		if i == 1 || i == 2 {
			// The two half-loops have the same pair of endpoints. Use
			// both exact midpoints to keep each native end pair distinct.
			first, second, err := curve.Split(big.NewRat(1, 2))
			if err != nil {
				return pieces, xyz, err
			}
			children = []ExactBezierUV{first, second}
		}
		for _, child := range children {
			pieces[i] = append(pieces[i], openSectionPiece{a: child.ends[0], b: child.ends[1], c: child, side: -1})
		}
	}
	return pieces, xyz, nil
}

func nodalCubicBezierControls(p *geom.NodalCubicSection, sx, sy *big.Rat, t [2]*big.Rat) [][3]*big.Rat {
	widthOverThree := new(big.Rat).Mul(big.NewRat(1, 3), new(big.Rat).Sub(t[1], t[0]))
	h := make([][3]*big.Rat, 4)
	for i, value := range t {
		squared := new(big.Rat).Quo(new(big.Rat).Mul(value, value), p.Scale)
		x := new(big.Rat).Sub(squared, p.LoopDepth)
		y := new(big.Rat).Mul(value, x)
		point := [2]*big.Rat{
			new(big.Rat).Add(p.Vertices[0].Point[0], new(big.Rat).Mul(sx, x)),
			new(big.Rat).Add(p.Vertices[0].Point[1], new(big.Rat).Mul(sy, y)),
		}
		derivative := [2]*big.Rat{
			new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), value), p.Scale),
			new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(3, 1), squared), p.LoopDepth),
		}
		factor := new(big.Rat).Mul(big.NewRat(int64(1-2*i), 1), widthOverThree)
		h[3*i] = [3]*big.Rat{point[0], point[1], big.NewRat(1, 1)}
		h[1+i] = [3]*big.Rat{
			new(big.Rat).Add(point[0], new(big.Rat).Mul(sx, new(big.Rat).Mul(factor, derivative[0]))),
			new(big.Rat).Add(point[1], new(big.Rat).Mul(sy, new(big.Rat).Mul(factor, derivative[1]))),
			big.NewRat(1, 1),
		}
	}
	return h
}

func nodalCubicCommonRegions(p *geom.NodalCubicSection, branches [4][]openSectionPiece, xyz [4]geom.Vec3,
	d geom.ParameterRectangle) (regions [4]graphEnvelopeRegion, seam openSectionPiece, err error) {
	toXY := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
	// In the open seam interval x<-LoopDepth, the exact residual
	// y²-Scale*x²*(x+LoopDepth) is strictly positive. Both sides use the
	// exterior roof and the seam can meet the section only at the turn.
	seam = openSectionPiece{a: geom.Vec2{X: d.UMin, Y: xyz[0].Y}, b: toXY(xyz[1]), side: -1}
	if seam.a.X >= seam.b.X || seam.a.Y != seam.b.Y {
		return regions, seam, geom.ErrConvergence
	}
	reverse := func(path []openSectionPiece) []openSectionPiece {
		out := make([]openSectionPiece, len(path))
		for i, p := range path {
			p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
			out[len(path)-1-i] = p
		}
		return out
	}
	routes := [3][2]geom.Vec2{{toXY(xyz[3]), seam.a}, {seam.a, toXY(xyz[2])}, {toXY(xyz[2]), toXY(xyz[3])}}
	var perimeter [3][]openSectionPiece
	for i, route := range routes {
		perimeter[i], err = saddleRectangleArc(route[0], route[1], d)
		if err != nil {
			return regions, seam, fmt.Errorf("trimmed: nodal cubic stock perimeter: %w", err)
		}
	}
	top := append(append([]openSectionPiece{}, branches[3]...), perimeter[0]...)
	top = append(append(top, seam), reverse(branches[1])...)
	bottom := append(reverse(branches[2]), reverse([]openSectionPiece{seam})...)
	bottom = append(append(bottom, perimeter[1]...), branches[0]...)
	loop := append(append([]openSectionPiece{}, branches[1]...), branches[2]...)
	arms := append(append(reverse(branches[0]), perimeter[2]...), reverse(branches[3])...)
	for i, face := range [][]openSectionPiece{top, bottom, loop, arms} {
		region := [4]int{0, 0, 1, 2}[i]
		regions[i] = graphEnvelopeRegion{loop: face, first: p.Regions[region].HeightSide < 0}
	}
	return regions, seam, nil
}
