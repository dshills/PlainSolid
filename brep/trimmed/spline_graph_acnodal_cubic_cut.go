package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// AcnodalCubicCutOptions bounds the original chart, stock and section proofs.
type AcnodalCubicCutOptions = CuspidalCubicCommonOptions

// AcnodalCubicCutHistory gives the original roof and wall occurrences of
// the retained regular cap. The isolated point has no boundary occurrence.
type AcnodalCubicCutHistory struct {
	RoofFaces   [2]int // minuend, subtrahend
	ArcVertices [3]int // regular turn, negative exit, positive exit
	ArcEdges    [2]int // negative, then positive restriction
	WallFaces   [2]int // two faces on one original stock wall
}

// AcnodalCubicCut retains one positive regular cap between the two original
// polynomial roofs, separated from their isolated zero-thickness contact.
type AcnodalCubicCut struct {
	solid     Solid
	history   AcnodalCubicCutHistory
	section   *geom.AcnodalCubicSection
	volume    *big.Rat
	sources   [3]*geom.BSplineSurface
	freeAxis  int
	tolerance float64
}

func (r *AcnodalCubicCut) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *AcnodalCubicCut) History() AcnodalCubicCutHistory {
	if r == nil {
		return AcnodalCubicCutHistory{}
	}
	return r.history
}

// ExactVolume returns the independently integrated original roof difference
// over the positive wall-bounded region.
func (r *AcnodalCubicCut) ExactVolume() *big.Rat {
	if r == nil || r.volume == nil {
		return nil
	}
	return new(big.Rat).Set(r.volume)
}

// IsolatedContact returns the exact original model point outside the retained
// cap. The returned rationals are independent of the proof.
func (r *AcnodalCubicCut) IsolatedContact() (point [3]*big.Rat) {
	if r != nil && r.section != nil {
		for i, q := range r.section.Isolated.Point {
			point[i] = new(big.Rat).Set(q)
		}
	}
	return point
}

// CutAcnodalCubicSplineGraphs subtracts the second stock from the first on
// their shared positive rectangular base. The retained cap must lie between
// the regular arc and the single original stock side containing its exits.
// An exterior layer collapses at the isolated contact and returns
// geom.ErrDegenerate. A subtraction with no positive region returns
// geom.ErrDomain.
func CutAcnodalCubicSplineGraphs(ctx context.Context, base, a, c *geom.BSplineSurface, freeAxis int,
	tolerance float64, o AcnodalCubicCutOptions) (*AcnodalCubicCut, error) {
	if ctx == nil || base == nil || a == nil || c == nil || freeAxis < 0 || freeAxis > 1 || !finite(tolerance) ||
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
	section, err := geom.DiscoverProjectedGraphAcnodalCubic(ctx, stock.roofs[0], stock.roofs[1], [2]int{0, 1}, freeAxis, o.Proof)
	if err != nil {
		return nil, err
	}
	if section.Regions[0].HeightSide == 1 {
		// The two cap sheets would touch at an interior point with a
		// punctured link on each sheet, in the same positive component.
		return nil, fmt.Errorf("trimmed: acnodal exterior Cut pinches at its isolated contact: %w", geom.ErrDegenerate)
	}
	if section.Arc == nil {
		return nil, fmt.Errorf("trimmed: isolated acnodal contact leaves no positive Cut: %w", geom.ErrDomain)
	}
	if len(section.Regions) != 2 || section.Regions[0].HeightSide != -1 || section.Regions[1].HeightSide != 1 {
		return nil, geom.ErrConvergence
	}

	d := stock.reference.Domain()
	pieces, points, err := acnodalCubicEnvelopeArc(section, d)
	if err != nil {
		return nil, err
	}
	if section.ProjectionOrientation < 0 {
		pieces[0], pieces[1] = pieces[1], pieces[0]
		for i := range pieces {
			p := &pieces[i]
			p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
		}
	}
	regions, err := cubicOpenEnvelopeRegions(pieces, d, [2]bool{false, true})
	if err != nil {
		return nil, err
	}
	cap := regions[1].loop
	if len(cap) != 3 || cap[1].side < 0 {
		return nil, geom.ErrConvergence
	}
	// The arc and its closing wall enclose a disk with positive interior
	// height gap. The gap vanishes only on the arc. The isolated point
	// lies behind the regular turn in the normalized longitudinal chart.
	walls, err := splitGraphSideAtMidpoint(cap[1])
	if err != nil {
		return nil, err
	}
	loop := []openSectionPiece{cap[0], walls[0], walls[1], cap[2]}

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
	roofs := [2]*geom.BSplineSurface{a, c}
	volume, err := acnodalCubicCutVolume(ctx, roofs, stock.roofs, section, o.Proof)
	if err != nil {
		return nil, err
	}
	solid, layer, err := assembleOpenSplineGraphPairOnCharts(ctx, a, c, 0, true, tolerance, loop, charts,
		nodalCubicReferenceXY, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	expected, err := nodalCubicCutOwnerPieces(charts, loop)
	if err != nil {
		return nil, err
	}
	if err := bindAlignedPolynomialCutLayer(ctx, solid, layer, roofs, charts, loop, expected, o.Proof); err != nil {
		return nil, err
	}
	history, err := bindAcnodalCubicCutArc(ctx, solid, layer, section, points)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &AcnodalCubicCut{solid: solid, history: history, section: section, volume: volume,
		sources: [3]*geom.BSplineSurface{base, a, c}, freeAxis: freeAxis, tolerance: tolerance}, nil
}

func bindAcnodalCubicCutArc(ctx context.Context, s Solid, layer OpenSectionHistory, section *geom.AcnodalCubicSection,
	points [3]geom.Vec3) (AcnodalCubicCutHistory, error) {
	var out AcnodalCubicCutHistory
	if len(layer.ContourEdges) != 2 || len(layer.WallFaces) != 2 {
		return out, geom.ErrConvergence
	}
	turn, err := polynomialCutContactVertex(s, layer, points[0])
	if err != nil {
		return out, err
	}
	out.RoofFaces = [2]int{layer.SplineFace, layer.PlaneFace}
	out.WallFaces = [2]int(layer.WallFaces)
	out.ArcVertices = [3]int{turn, -1, -1}
	out.ArcEdges = [2]int{-1, -1}
	for _, id := range layer.ContourEdges {
		e := s.edges[id]
		exit := e.End
		if e.End == turn {
			exit = e.Start
		} else if e.Start != turn {
			return out, geom.ErrConvergence
		}
		part := 0
		if s.vertices[exit] == points[2] {
			part = 1
		}
		if s.vertices[exit] != points[part+1] || out.ArcEdges[part] >= 0 ||
			!slices.Equal(nodalCubicIncidentFaces(s, id), out.RoofFaces[:]) {
			return out, geom.ErrConvergence
		}
		out.ArcEdges[part], out.ArcVertices[part+1] = id, exit
	}
	for owner, id := range out.RoofFaces {
		face := s.faces[id]
		for region, want := range []trimvalidate.Location{trimvalidate.Outside, trimvalidate.Inside} {
			uv := section.Isolated.UV[owner]
			if region == 1 {
				uv = section.Regions[1].UV[owner]
			}
			where, proof, err := LocateRationalInFaceTrim(ctx, face, uv, trimvalidate.Options{})
			if err != nil {
				return out, err
			}
			if proof.Status != trimvalidate.Certified || where != want {
				return out, geom.ErrConvergence
			}
		}
		for _, use := range face.Outer {
			for i, id := range out.ArcEdges {
				if use.Edge != id {
					continue
				}
				edge := s.edges[id]
				start := edge.Start
				if use.Reversed {
					start = edge.End
				}
				source := section.Arc.Edges[i]
				q, ok := use.Curve.(ExactBezierUV)
				if !ok || !acnodalCubicCutEndpointJets(q, section.Arc, i, owner, start != out.ArcVertices[source.Start]) {
					return out, geom.ErrConvergence
				}
			}
		}
	}
	return out, nil
}

func acnodalCubicCutEndpointJets(q ExactBezierUV, arc *geom.AcnodalCubicArc, id, owner int, reversed bool) bool {
	control := q.HomogeneousControls()
	if len(control) != 4 {
		return false
	}
	if reversed {
		slices.Reverse(control)
	}
	one := big.NewRat(1, 1)
	for _, p := range control {
		if p[2].Cmp(one) != 0 {
			return false
		}
	}
	e := arc.Edges[id]
	width := new(big.Rat).Sub(e.Range[1], e.Range[0])
	for end, v := range [2]int{e.Start, e.End} {
		j := 3 * end
		for axis := range 2 {
			if control[j][axis].Cmp(arc.Vertices[v].UV[owner][axis]) != 0 {
				return false
			}
			base := 2 * end
			derivative := new(big.Rat).Sub(control[base+1][axis], control[base][axis])
			derivative.Mul(derivative, big.NewRat(3, 1))
			if derivative.Cmp(new(big.Rat).Mul(width, arc.UVJet[v][owner][axis])) != 0 {
				return false
			}
		}
	}
	return true
}
