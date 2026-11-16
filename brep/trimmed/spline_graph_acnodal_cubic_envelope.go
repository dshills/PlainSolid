package trimmed

import (
	"context"
	"math/big"
	"slices"

	"plainsolid/geom"
)

type AcnodalCubicEnvelopeOptions = CuspidalCubicCommonOptions

// AcnodalCubicEnvelopeHistory separates the regular material seam from the
// isolated contact, which lies in the interior of roof face 0. Absent arc
// entries and an absent second roof have ID -1.
type AcnodalCubicEnvelopeHistory struct {
	Base, IsolatedFace int
	RoofFaces          [2]int // exterior, interior cap
	RoofOwners         [2]int // original input, 0 or 1
	ArcVertices        [3]int // regular turn, negative exit, positive exit
	ArcEdges           [2]int // negative, then positive restriction
}

// AcnodalCubicEnvelope retains the exact common or fused material on two
// original roof supports. The isolated contact does not split a face.
type AcnodalCubicEnvelope struct {
	solid   Solid
	history AcnodalCubicEnvelopeHistory
	section *geom.AcnodalCubicSection
	sources [3]*geom.BSplineSurface
	axis    int
	tol     float64
	upper   bool
}

func (r *AcnodalCubicEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *AcnodalCubicEnvelope) History() AcnodalCubicEnvelopeHistory {
	if r == nil {
		return AcnodalCubicEnvelopeHistory{}
	}
	return r.history
}

// IsolatedContact returns the exact model point in the selected roof
// interior, without turning it into a boundary vertex.
func (r *AcnodalCubicEnvelope) IsolatedContact() [3]*big.Rat {
	var result [3]*big.Rat
	if r != nil && r.section != nil {
		for i, x := range r.section.Isolated.Point {
			result[i] = new(big.Rat).Set(x)
		}
	}
	return result
}

// CommonAcnodalCubicSplineGraphs constructs the lower material envelope on
// the shared positive rectangular stock.
func CommonAcnodalCubicSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o AcnodalCubicEnvelopeOptions) (*AcnodalCubicEnvelope, error) {
	return buildAcnodalCubicEnvelope(ctx, base, a, b, freeAxis, tolerance, false, o)
}

// FuseAcnodalCubicSplineGraphs constructs the upper material envelope on
// the shared positive rectangular stock.
func FuseAcnodalCubicSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o AcnodalCubicEnvelopeOptions) (*AcnodalCubicEnvelope, error) {
	return buildAcnodalCubicEnvelope(ctx, base, a, b, freeAxis, tolerance, true, o)
}

func buildAcnodalCubicEnvelope(ctx context.Context, base, a, c *geom.BSplineSurface, axis int,
	tolerance float64, upper bool, o AcnodalCubicEnvelopeOptions) (*AcnodalCubicEnvelope, error) {
	if ctx == nil || base == nil || a == nil || c == nil || axis < 0 || axis > 1 || !finite(tolerance) ||
		tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stock, err := prepareRectangularPolynomialStock(ctx, base, a, c, o)
	if err != nil {
		return nil, err
	}
	proof, err := geom.DiscoverProjectedGraphAcnodalCubic(ctx, stock.roofs[0], stock.roofs[1], [2]int{0, 1}, axis, o.Proof)
	if err != nil {
		return nil, err
	}
	d := stock.reference.Domain()
	baseChart, err := polynomialEnvelopeBaseChart(stock.base)
	if err != nil {
		return nil, err
	}
	var charts [2]graphEnvelopeChart
	for i, roof := range stock.roofs {
		aligned, err := geom.AlignAffineXYCharts(ctx, stock.reference, roof, o.Section.Patch)
		if err != nil {
			return nil, err
		}
		charts[i], err = polynomialEnvelopeBaseChart(aligned)
		if err != nil {
			return nil, err
		}
	}
	exteriorFirst := (proof.Regions[0].HeightSide < 0) != upper
	var regions []graphEnvelopeRegion
	var arcPoints [3]geom.Vec3
	if proof.Arc == nil {
		corners := insetCorners(d)
		loop := make([]openSectionPiece, len(corners))
		for i, v := range corners {
			loop[i] = openSectionPiece{a: v, b: corners[(i+1)%len(corners)], side: i}
		}
		regions = []graphEnvelopeRegion{{loop: loop, first: exteriorFirst}}
	} else {
		pieces, points, err := acnodalCubicEnvelopeArc(proof, d)
		if err != nil {
			return nil, err
		}
		if proof.ProjectionOrientation < 0 {
			// The two region wires traverse the physical outer perimeter
			// counterclockwise, even when the original UV law reflects XY.
			pieces[0], pieces[1] = pieces[1], pieces[0]
			for i := range pieces {
				p := &pieces[i]
				p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
			}
		}
		built, err := cubicOpenEnvelopeRegions(pieces, d, [2]bool{exteriorFirst, !exteriorFirst})
		if err != nil {
			return nil, err
		}
		regions, arcPoints = built[:], points
	}
	result, env, err := assembleSplineEnvelopeOnCharts(ctx, base, a, c, regions, d, baseChart, charts,
		nodalCubicReferenceXY, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	if err := validateExactGraphPartitionEdges(ctx, result, o.Proof); err != nil {
		return nil, err
	}
	if err := bindPolynomialEnvelopeBase(result, base, baseChart, env, nodalCubicReferenceXY); err != nil {
		return nil, err
	}
	if err := affineMaterialVertexLinks(ctx, result); err != nil {
		return nil, err
	}
	h := AcnodalCubicEnvelopeHistory{Base: env.Base, IsolatedFace: 0, RoofFaces: [2]int{0, -1}, RoofOwners: [2]int{0, -1},
		ArcVertices: [3]int{-1, -1, -1}, ArcEdges: [2]int{-1, -1}}
	if !exteriorFirst {
		h.RoofOwners[0] = 1
	}
	if proof.Arc != nil {
		h.RoofFaces[1], h.RoofOwners[1] = 1, h.RoofOwners[0]^1
		if err := bindAcnodalCubicArc(result, env, arcPoints, &h); err != nil {
			return nil, err
		}
	} else if len(env.ContourEdges) != 0 {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &AcnodalCubicEnvelope{solid: result, history: h, section: proof, sources: [3]*geom.BSplineSurface{base, a, c},
		axis: axis, tol: tolerance, upper: upper}, nil
}

func acnodalCubicEnvelopeArc(p *geom.AcnodalCubicSection, d geom.ParameterRectangle) (pieces [2]openSectionPiece, points [3]geom.Vec3, err error) {
	for i, vertex := range p.Arc.Vertices {
		points[i], err = geom.RepresentedExactPoint(vertex.Point)
		if err != nil {
			return pieces, points, err
		}
	}
	f, g := p.IsolatedTangent[0], p.IsolatedTangent[1]
	longitudinal := 0
	if f[0].Sign() == 0 {
		longitudinal = 1
	}
	if f[longitudinal].Sign() == 0 || f[longitudinal^1].Sign() != 0 ||
		g[longitudinal].Sign() != 0 || g[longitudinal^1].Sign() == 0 {
		return pieces, points, geom.ErrDomain
	}
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	side := 0
	if f[longitudinal].Sign() > 0 {
		side = 1
	}
	for _, point := range points[1:] {
		if ([2]float64{point.X, point.Y})[longitudinal] != limits[longitudinal][side] {
			return pieces, points, geom.ErrDomain
		}
	}
	var coefficients [2][4]*big.Rat
	for axis := range coefficients {
		coefficients[axis] = [4]*big.Rat{p.Arc.Vertices[0].Point[axis],
			new(big.Rat).Mul(g[axis], p.Gap), new(big.Rat).Quo(f[axis], p.Scale), new(big.Rat).Quo(g[axis], p.Scale)}
	}
	for i, edge := range p.Arc.Edges {
		width := new(big.Rat).Sub(edge.Range[1], edge.Range[0])
		control := make([][3]*big.Rat, 4)
		for j, t := range edge.Range {
			h := new(big.Rat).Mul(width, big.NewRat(int64(1-2*j), 3))
			control[3*j][2], control[j+1][2] = big.NewRat(1, 1), big.NewRat(1, 1)
			for axis, c := range coefficients {
				value, tangent := new(big.Rat).Set(c[3]), new(big.Rat)
				for k := 2; k >= 0; k-- {
					tangent.Add(tangent.Mul(tangent, t), value)
					value.Add(value.Mul(value, t), c[k])
				}
				control[3*j][axis] = value
				control[j+1][axis] = new(big.Rat).Add(value, new(big.Rat).Mul(h, tangent))
			}
		}
		q, err := NewExactBezierUV(control)
		if err != nil {
			return pieces, points, err
		}
		for j, idx := range [2]int{edge.Start, edge.End} {
			if q.ends[j] != (geom.Vec2{X: points[idx].X, Y: points[idx].Y}) {
				return pieces, points, geom.ErrConvergence
			}
		}
		pieces[i] = openSectionPiece{a: q.ends[0], b: q.ends[1], c: q, side: -1}
	}
	return pieces, points, nil
}

func bindAcnodalCubicArc(s Solid, env SplineGraphBooleanHistory, points [3]geom.Vec3, h *AcnodalCubicEnvelopeHistory) error {
	if len(env.ContourEdges) != 2 {
		return geom.ErrConvergence
	}
	for i, point := range points {
		for id, v := range s.vertices {
			if v == point {
				if h.ArcVertices[i] >= 0 {
					return geom.ErrConvergence
				}
				h.ArcVertices[i] = id
			}
		}
		if h.ArcVertices[i] < 0 {
			return geom.ErrConvergence
		}
	}
	for _, id := range env.ContourEdges {
		e := s.edges[id]
		if !slices.Equal(nodalCubicIncidentFaces(s, id), h.RoofFaces[:]) {
			return geom.ErrConvergence
		}
		turn, exit := h.ArcVertices[0], e.End
		if e.End == turn {
			exit = e.Start
		} else if e.Start != turn {
			return geom.ErrConvergence
		}
		part := 0
		if exit == h.ArcVertices[2] {
			part = 1
		}
		if exit != h.ArcVertices[part+1] || h.ArcEdges[part] >= 0 {
			return geom.ErrConvergence
		}
		h.ArcEdges[part] = id
	}
	return nil
}
