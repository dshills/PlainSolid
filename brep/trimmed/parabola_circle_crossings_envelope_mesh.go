package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CertifiedParabolaCircleCrossingsEnvelopeMesh displays a single outward
// Common or Fuse shell. JunctionVertices identifies the two exact original
// crossing nodes in History().Junctions order. Region is -1 because the
// shell covers the complete stock rather than an individual signed region.
type CertifiedParabolaCircleCrossingsEnvelopeMesh struct {
	CertifiedParabolaTwoCirclesComponentMesh
	JunctionVertices [2]int
}

// TessellateCertified displays the original identity-XY polynomial roof
// regions, original base, and planar stock walls. It certifies two-sided
// physical distance, positive signed volume, and optional angular bounds.
// NormalTurn and NormalAngle both apply separately to each original face.
// NormalAngle bounds each represented facet against its continuous source.
func (r *ParabolaCircleCrossingsEnvelope) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleCrossingsEnvelopeMesh, error) {
	var empty CertifiedParabolaCircleCrossingsEnvelopeMesh
	if ctx == nil || r == nil || r.sources[0] == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedParabolaCircleMeshOptions(o)
	if err != nil {
		return empty, err
	}
	if o.MaxTriangles < len(r.solid.faces) {
		return empty, parametric.ErrBudget
	}
	var height [3]parabolaCircleMeshHeight
	var patches [2]*geom.SurfacePatch
	for i, source := range r.sources {
		var patch *geom.SurfacePatch
		height[i], patch, err = parabolaCircleDisplayHeight(ctx, source)
		if err != nil {
			return empty, err
		}
		height[i].normalHessian = parabolaCircleMeshHessian(source)
		height[i].hessian = height[i].normalHessian
		center := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
		gradient := height[i].gradient(center)
		for axis := range gradient {
			w := new(big.Rat).Add(height[i].hessian[axis], height[i].hessian[axis+1])
			w.Mul(w, center[axis])
			w.Add(w, gradient[axis].Abs(gradient[axis]))
			if w.Cmp(height[i].slope[axis]) < 0 {
				height[i].slope[axis] = w
			}
		}
		if i > 0 {
			patches[i-1] = patch
		}
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patches[0], patches[1], r.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	volume := parabolaCircleCrossingsEnvelopeVolume(height, r.history.RoofOwners, proof)
	if volume[0].Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	plane, err := parabolaCircleMeshUVPlane(ctx)
	if err != nil {
		return empty, err
	}
	minimum := new(big.Rat).Mul(volume[1], new(big.Rat).Sub(big.NewRat(1, 1), ratInset(o.RelativeVolumeTolerance)))
	maximum := new(big.Rat).Mul(volume[0], new(big.Rat).Add(big.NewRat(1, 1), ratInset(o.RelativeVolumeTolerance)))
	for n := 1; n <= o.MaxSubdivisions; n *= 2 {
		out, err := r.displayEnvelope(ctx, plane, height, n, o)
		if errors.Is(err, errRefine) {
			continue
		}
		if err != nil {
			return empty, err
		}
		if err := auditParabolaTwoCirclesMesh(ctx, r.solid, out.CertifiedParabolaTwoCirclesComponentMesh); err != nil {
			return empty, fmt.Errorf("trimmed: crossing envelope display ownership: %w", err)
		}
		actual, report, err := checkClosedPeriodicMesh(ctx, out.Mesh, o.Collision)
		if err != nil {
			return empty, fmt.Errorf("trimmed: crossing envelope display embedding: %w", err)
		}
		if actual.Cmp(minimum) < 0 || actual.Cmp(maximum) > 0 {
			continue
		}
		for i, junction := range r.history.Junctions {
			id := out.OriginalVertices[junction.Vertex]
			if !exactPointCoordinatesEqual(exactBoundPoint(out.Mesh.Vertices[id]), proof.Nodes[i].Point) {
				return empty, geom.ErrConvergence
			}
			for _, native := range junction.Rays {
				edge := out.OriginalEdges[native].Vertices
				if edge[0] != id && edge[len(edge)-1] != id {
					return empty, geom.ErrConvergence
				}
			}
			out.JunctionVertices[i] = id
		}
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		out.SourceVolume, out.SignedDisplayVolume, out.Collision = volume, actual, report
		return out, nil
	}
	return empty, parametric.ErrBudget
}

func parabolaCircleCrossingsEnvelopeVolume(height [3]parabolaCircleMeshHeight, owners [4]int,
	proof *geom.SplineGraphParabolaCircleCrossings) [2]*big.Rat {
	// Start with roof A over the entire stock. Replace A with B only
	// where the native envelope selects B, using the signed A-B integrals.
	net := new(big.Rat)
	for i, h := range height[:2] {
		for u, row := range h.coefficients {
			for v, c := range row {
				if c != nil {
					moment := new(big.Rat).Quo(c, big.NewRat(int64((u+1)*(v+1)), 1))
					if i == 0 {
						moment.Neg(moment)
					}
					net.Add(net, moment)
				}
			}
		}
	}
	out := [2]*big.Rat{new(big.Rat).Set(net), new(big.Rat).Set(net)}
	signed := parabolaCircleCrossingMeshVolumes([2]parabolaCircleMeshHeight{height[1], height[2]}, proof)
	for i, owner := range owners {
		if owner == 1 {
			out[0].Sub(out[0], signed[i][1])
			out[1].Sub(out[1], signed[i][0])
		}
	}
	return out
}

type crossingEnvelopeDisplay struct {
	CertifiedParabolaCircleCrossingsEnvelopeMesh
	exact []exactBoundVector
	facet *parabolaCircleFacetCone
}

func (r *ParabolaCircleCrossingsEnvelope) displayEnvelope(ctx context.Context, plane *geom.SurfacePatch,
	height [3]parabolaCircleMeshHeight, n int, o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleCrossingsEnvelopeMesh, error) {
	var empty CertifiedParabolaCircleCrossingsEnvelopeMesh
	d := crossingEnvelopeDisplay{}
	d.Region, d.Subdivisions = -1, n
	d.Mesh.Vertices = slices.Clone(r.solid.vertices)
	d.OriginalVertices = make([]int, len(d.Mesh.Vertices))
	d.OriginalEdges = make([]ParabolaTwoCirclesMeshEdge, len(r.solid.edges))
	for i, p := range d.Mesh.Vertices {
		d.OriginalVertices[i] = i
		d.exact = append(d.exact, exactBoundPoint(p))
	}
	d.DistanceUpper = new(big.Rat)
	var turnLimit *big.Rat
	if o.NormalTurn > 0 {
		turnLimit = ratInset(o.NormalTurn)
		turnLimit.Mul(turnLimit, turnLimit)
		d.SquaredSourceNormalTurn = new(big.Rat)
	}
	if o.NormalAngle > 0 {
		d.facet = &parabolaCircleFacetCone{limit: mappedAngleTangentLimit(o.NormalAngle), maximum: new(big.Rat)}
	}
	cuts := make([]*big.Rat, n+1)
	for i := range cuts {
		cuts[i] = big.NewRat(int64(i), int64(n))
	}
	topArea := new(big.Rat)
	for i, id := range r.history.RoofFaces {
		area, err := d.addGraphFace(ctx, r.solid, id, plane, height[1+r.history.RoofOwners[i]], cuts, o)
		if err != nil {
			return empty, err
		}
		topArea.Add(topArea, area)
	}
	baseArea, err := d.addGraphFace(ctx, r.solid, r.history.Base, plane, height[0], cuts, o)
	if err != nil {
		return empty, err
	}
	// The top's internal coedges cancel precisely in XY and its six
	// stock sides tile the same unit rectangle as the oppositely oriented base.
	if topArea.Cmp(big.NewRat(2, 1)) != 0 || baseArea.Cmp(topArea) != 0 {
		return empty, geom.ErrConvergence
	}
	if d.DistanceUpper.Cmp(ratInset(o.Deflection)) > 0 ||
		turnLimit != nil && d.SquaredSourceNormalTurn.Cmp(turnLimit) > 0 {
		return empty, errRefine
	}
	for _, id := range r.history.WallFaces {
		if err := d.addWall(r.solid, id, o.MaxTriangles); err != nil {
			return empty, err
		}
	}
	if d.facet != nil {
		d.SquaredFacetNormalTangent = d.facet.maximum
	}
	return d.CertifiedParabolaCircleCrossingsEnvelopeMesh, nil
}

func (d *crossingEnvelopeDisplay) addGraphFace(ctx context.Context, s Solid, id int, plane *geom.SurfacePatch,
	h parabolaCircleMeshHeight, cuts []*big.Rat, o ParabolaTwoCirclesMeshOptions) (*big.Rat, error) {
	f := s.faces[id]
	if len(f.Holes) != 0 {
		return nil, geom.ErrConvergence
	}
	curves, fixed := make([]PCurve, len(f.Outer)), make([][]*big.Rat, len(f.Outer))
	for i, use := range f.Outer {
		var err error
		curves[i], err = mixedStoredUseCurveContext(ctx, use, geom.SurfacePatchOptions{})
		if err != nil {
			return nil, err
		}
		fixed[i] = cuts
	}
	remaining := o.MaxTriangles - len(d.Mesh.Triangles)
	if remaining < 1 {
		return nil, parametric.ErrBudget
	}
	foot, err := tessellateMixedExactAffineRegionCuts(ctx, plane, curves, nil, [][][]*big.Rat{fixed}, MixedExactAffineMeshOptions{
		MaxBoundaryVertices: 2048, MaxTriangulationWork: 10000000, MaxTriangles: remaining,
		Topology: trimvalidate.Options{MaxEdges: 2048, MaxWork: 10000000},
		Display:  geom.ComposedCollectionPolylineOptions{ComposedCurvePolylineOptions: geom.ComposedCurvePolylineOptions{Deflection: o.Deflection / 8}},
	})
	if errors.Is(err, geom.ErrConvergence) {
		return nil, errRefine
	}
	if err != nil {
		return nil, err
	}
	if len(foot.Samples) != len(f.Outer) {
		return nil, geom.ErrConvergence
	}
	boundary := make(map[[2]int]bool)
	for _, sample := range foot.Samples {
		if len(sample.Vertices) > o.MaxSubdivisions+1 {
			return nil, errRefine
		}
		for j := 1; j < len(sample.Vertices); j++ {
			boundary[parabolaCircleMeshEdgeKey(sample.Vertices[j-1], sample.Vertices[j])] = true
		}
	}
	uv, triangles := foot.ExactUV, foot.Mesh.Triangles
	if d.facet == nil {
		uv, triangles, err = refineParabolaCircleFootprint(ctx, uv, triangles, boundary, nil, len(cuts)-1, remaining, true)
	} else {
		uv, triangles, err = insertCrossingEnvelopeFacetGrid(ctx, uv, triangles, len(cuts)-1, remaining)
	}
	if err != nil {
		return nil, err
	}
	if err := improveParabolaCircleFacetGrid(ctx, uv, triangles, nil); err != nil {
		return nil, err
	}
	ids := make([]int, len(uv))
	points := make([]exactBoundVector, len(uv))
	for i, q := range uv {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ids[i] = -1
		points[i] = exactBoundVector{q[0], q[1], h.evaluate(q)}
	}
	for _, sample := range foot.Samples {
		if err := d.bindGraphEdge(s, f.Outer[sample.Trim], sample, ids, points); err != nil {
			return nil, err
		}
	}
	rounding := new(big.Rat)
	represented := make([][2]*big.Rat, len(uv))
	var facetPoints []exactBoundVector
	if d.facet != nil {
		facetPoints = make([]exactBoundVector, len(uv))
	}
	for i, p := range points {
		if ids[i] < 0 {
			ids[i], err = d.addVertex(p)
			if err != nil {
				return nil, err
			}
		}
		back := exactBoundPoint(d.Mesh.Vertices[ids[i]])
		represented[i] = [2]*big.Rat{back[0], back[1]}
		if facetPoints != nil {
			facetPoints[i] = back
		}
		errorSum := new(big.Rat)
		for axis := range p {
			delta := new(big.Rat).Sub(p[axis], back[axis])
			errorSum.Add(errorSum, delta.Abs(delta))
		}
		rounding.Set(maxRat(rounding, errorSum))
	}
	width := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	area := new(big.Rat)
	for _, t := range triangles {
		det := exactUVSignedArea(uv[t[0]], uv[t[1]], uv[t[2]])
		if det.Sign() <= 0 || exactUVSignedArea(represented[t[0]], represented[t[1]], represented[t[2]]).Sign() <= 0 {
			return nil, errRefine
		}
		area.Add(area, det)
		for j, a := range t {
			b := t[(j+1)%3]
			for axis := range width {
				delta := new(big.Rat).Sub(uv[a][axis], uv[b][axis])
				width[axis].Set(maxRat(width[axis], delta.Abs(delta)))
			}
		}
		d.emit(id, f.Reversed, [3]int{ids[t[0]], ids[t[1]], ids[t[2]]})
	}
	_, trimError, err := graphAreaSqrt(foot.Region.FillSquaredDistance, 64, 16384)
	if err != nil {
		return nil, err
	}
	// Transport each closed source fill across its proved two-sided UV
	// chord error. At the new UV, bound whole-triangle interpolation and
	// represented-vertex rounding. On a wall, use the same stock station
	// and vertical fraction for the exact and displayed height intervals.
	lipschitz := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Add(h.slope[0], h.slope[1]))
	distance := new(big.Rat).Mul(lipschitz, trimError)
	distance.Add(distance, h.residual(width[0], width[1]))
	distance.Add(distance, rounding)
	d.DistanceUpper.Set(maxRat(d.DistanceUpper, distance))
	pad := new(big.Rat).Add(trimError, rounding)
	if o.NormalTurn > 0 {
		turn := parabolaCircleMeshSourceTurnSquared([2]parabolaCircleMeshHeight{h, h}, width, pad)
		d.SquaredSourceNormalTurn.Set(maxRat(d.SquaredSourceNormalTurn, turn))
	}
	if d.facet != nil {
		for _, t := range triangles {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			center, radius := parabolaCircleFacetSourceBox(uv, t, pad)
			a, b, c := facetPoints[t[0]], facetPoints[t[1]], facetPoints[t[2]]
			if f.Reversed {
				b, c = c, b
			}
			facet := ddCross(boundSubtract(b, a), boundSubtract(c, a))
			if !d.facet.addGraph(facet, h, f.Reversed, center, radius) {
				return nil, errRefine
			}
		}
	}
	return area, nil
}

func (d *crossingEnvelopeDisplay) bindGraphEdge(s Solid, use Use, sample MixedExactTrimSamples,
	ids []int, points []exactBoundVector) error {
	e := s.edges[use.Edge]
	low, high := ratInset(e.Range.Min), ratInset(e.Range.Max)
	span := new(big.Rat).Sub(high, low)
	ledger := d.OriginalEdges[use.Edge]
	seen := len(ledger.Vertices) != 0
	m := len(sample.Vertices)
	if m < 2 || m != len(sample.Parameters) || seen && len(ledger.Vertices) != m {
		return geom.ErrConvergence
	}
	if !seen {
		ledger = ParabolaTwoCirclesMeshEdge{Edge: use.Edge, Vertices: make([]int, m), Parameters: make([]*big.Rat, m)}
		ledger.Vertices[0], ledger.Vertices[m-1] = e.Start, e.End
	}
	for j := range m {
		k := j
		if use.Reversed {
			k = m - 1 - j
		}
		v, fraction := sample.Vertices[k], sample.Parameters[k]
		t := new(big.Rat).Mul(span, fraction)
		if use.Reversed {
			t.Sub(high, t)
		} else {
			t.Add(low, t)
		}
		id := ledger.Vertices[j]
		if !seen {
			if j != 0 && j != m-1 {
				var err error
				id, err = d.addVertex(points[v])
				if err != nil {
					return err
				}
			}
			ledger.Vertices[j], ledger.Parameters[j] = id, t
		}
		if t.Cmp(ledger.Parameters[j]) != 0 || !exactPointCoordinatesEqual(points[v], d.exact[id]) || ids[v] >= 0 && ids[v] != id {
			return geom.ErrConvergence
		}
		ids[v] = id
	}
	d.OriginalEdges[use.Edge] = ledger
	return nil
}

func (d *crossingEnvelopeDisplay) addVertex(point exactBoundVector) (int, error) {
	p, _, err := roundParabolaCircleMeshPoint(point, nil)
	if err != nil {
		return 0, err
	}
	id := len(d.Mesh.Vertices)
	d.Mesh.Vertices = append(d.Mesh.Vertices, p)
	d.exact = append(d.exact, point)
	return id, nil
}

func (d *crossingEnvelopeDisplay) addWall(s Solid, id, budget int) error {
	face := s.faces[id]
	plane, ok := face.Surface.(geom.PlaneSurface)
	if !ok || len(face.Outer) != 4 || len(face.Holes) != 0 {
		return geom.ErrConvergence
	}
	normal := plane.Frame().ZAxis()
	origin := exactBoundPoint(plane.Frame().Origin())
	axis := 0
	if normal.X == 0 {
		axis = 1
	}
	if normal.Z != 0 || !(axis == 0 && (normal.X == -1 || normal.X == 1) && normal.Y == 0 ||
		axis == 1 && (normal.Y == -1 || normal.Y == 1)) {
		return geom.ErrConvergence
	}
	bottom, top := face.Outer[0], face.Outer[2]
	lower := slices.Clone(d.OriginalEdges[bottom.Edge].Vertices)
	upper := slices.Clone(d.OriginalEdges[top.Edge].Vertices)
	if bottom.Reversed {
		slices.Reverse(lower)
	}
	if !top.Reversed {
		slices.Reverse(upper)
	}
	if len(lower) < 2 || len(lower) != len(upper) {
		return geom.ErrConvergence
	}
	if len(d.Mesh.Triangles)+2*(len(lower)-1) > budget {
		return parametric.ErrBudget
	}
	firstTriangle := len(d.Mesh.Triangles)
	for i, a := range lower {
		b := upper[i]
		p, q := d.exact[a], d.exact[b]
		if p[axis].Cmp(origin[axis]) != 0 || p[0].Cmp(q[0]) != 0 || p[1].Cmp(q[1]) != 0 ||
			d.Mesh.Vertices[a].Z >= d.Mesh.Vertices[b].Z {
			return geom.ErrConvergence
		}
		if i > 0 {
			d.emit(id, face.Reversed, [3]int{lower[i-1], a, b})
			d.emit(id, face.Reversed, [3]int{lower[i-1], b, upper[i-1]})
		}
	}
	if d.facet != nil {
		if face.Reversed {
			normal = normal.Scale(-1)
		}
		source := exactBoundPoint(normal)
		for _, t := range d.Mesh.Triangles[firstTriangle:] {
			a, b, c := exactBoundPoint(d.Mesh.Vertices[t[0]]), exactBoundPoint(d.Mesh.Vertices[t[1]]), exactBoundPoint(d.Mesh.Vertices[t[2]])
			facet := ddCross(boundSubtract(b, a), boundSubtract(c, a))
			if !d.facet.add(facet, source) {
				return errRefine
			}
		}
	}
	for _, j := range [...]int{1, 3} {
		use := face.Outer[j]
		e := s.edges[use.Edge]
		start, end := s.vertices[e.Start], s.vertices[e.End]
		if start.X != end.X || start.Y != end.Y || start.Z == end.Z {
			return geom.ErrConvergence
		}
		d.OriginalEdges[use.Edge] = ParabolaTwoCirclesMeshEdge{Edge: use.Edge, Vertices: []int{e.Start, e.End},
			Parameters: []*big.Rat{ratInset(e.Range.Min), ratInset(e.Range.Max)}}
	}
	return nil
}

func (d *crossingEnvelopeDisplay) emit(face int, reverse bool, t [3]int) {
	if reverse {
		t[1], t[2] = t[2], t[1]
	}
	d.Mesh.Triangles = append(d.Mesh.Triangles, t)
	d.OriginalFace = append(d.OriginalFace, face)
}
