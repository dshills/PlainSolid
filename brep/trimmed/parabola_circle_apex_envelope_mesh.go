package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// CertifiedParabolaCircleApexEnvelopeMesh keeps the original closed envelope,
// its four tangent rays and its original same-roof interior bridge.
type CertifiedParabolaCircleApexEnvelopeMesh struct {
	CertifiedParabolaTwoCirclesComponentMesh
	ApexVertex      int
	Rays            [2][2]int
	Bridge          int
	DependentSecond [2]*big.Rat
}

// TessellateCertified re-proves the source partition at the apex, independently
// verifies the rounded full shell and certifies two-sided physical distance,
// signed volume and the requested continuous and facet angular bounds.
func (r *ParabolaCircleApexTangencyEnvelope) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleApexEnvelopeMesh, error) {
	var empty CertifiedParabolaCircleApexEnvelopeMesh
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
	source, err := proveParabolaCircleApexPartition(ctx, r.sources, r.axis, r.tolerance, ParabolaCircleGraphOptions{})
	if err != nil {
		return empty, err
	}
	regions, bridge, err := parabolaCircleApexEnvelopeRegions(source)
	if err != nil {
		return empty, err
	}
	layout := parabolaCircleEnvelopeLayout{
		roofFaces: r.history.RoofFaces, roofOwners: r.history.RoofOwners,
		base: r.history.Base, walls: r.history.WallFaces[:],
	}
	for i, region := range regions {
		owner := 0
		if (region.sign > 0) != (r.op == parametric.Union) {
			owner = 1
		}
		face := r.solid.faces[layout.roofFaces[i]]
		if layout.roofOwners[i] != owner || face.Surface != r.sources[1+owner] || face.Reversed || len(face.Holes) != 0 {
			return empty, geom.ErrConvergence
		}
		// Only the two reconstructed cusp-ended Jordan regions are
		// authorized to use the source-specific proof. The base, disk
		// and opposite stock region use ordinary trim validation.
		if i == source.proof.DiskRegion || i == 3 {
			layout.apex[i] = authenticatedParabolaCircleApexLoop(region.loop)
		}
	}
	h := r.history
	model, ok := r.solid.edges[h.Bridge].Curve.(surfaceTrimCurve)
	if !ok || model.curve != nil || model.surface != r.sources[1+h.RoofOwners[source.proof.DiskRegion]] ||
		!(model.a == bridge.a && model.b == bridge.b || model.a == bridge.b && model.b == bridge.a) {
		return empty, geom.ErrConvergence
	}
	second := [2]*big.Rat{source.proof.ParabolaSecondDerivative, source.proof.CircleSecondDerivative}
	for i, rays := range h.Rays {
		for direction, id := range rays {
			sign, err := parabolaCircleApexRay(r.solid, id, h.Vertex, r.axis, second[i])
			if err != nil || sign != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
	}
	height, _, err := parabolaCircleEnvelopeHeights(ctx, r.sources)
	if err != nil {
		return empty, err
	}
	volume := parabolaCircleApexEnvelopeVolume(height, h.RoofOwners, source.proof)
	display, err := certifyParabolaCircleEnvelopeDisplay(ctx, r.solid, layout, height, volume, o)
	if err != nil {
		return empty, err
	}
	vertex := display.OriginalVertices[h.Vertex]
	if !exactPointCoordinatesEqual(exactBoundPoint(display.Mesh.Vertices[vertex]), source.proof.Point) {
		return empty, geom.ErrConvergence
	}
	for _, rays := range h.Rays {
		for direction, id := range rays {
			edge := display.OriginalEdges[id].Vertices
			next := edge[1]
			if edge[len(edge)-1] == vertex {
				next = edge[len(edge)-2]
			} else if edge[0] != vertex {
				return empty, geom.ErrConvergence
			}
			station := exactBoundPoint(display.Mesh.Vertices[next])[r.axis]
			if station.Cmp(source.proof.Parameters[r.axis]) != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
	}
	return CertifiedParabolaCircleApexEnvelopeMesh{
		CertifiedParabolaTwoCirclesComponentMesh: display,
		ApexVertex:                               vertex, Rays: h.Rays, Bridge: h.Bridge, DependentSecond: second,
	}, nil
}

// authenticatedParabolaCircleApexLoop carries the original coedges from the
// reconstructed apex partition. Those loops are simple and CCW by the
// complete circle–parabola factor contact proof and the outward bridge.
// Authenticate every sampled coefficient against that partition. The
// sampled chord polygon still receives its independent ordinary proof.
type authenticatedParabolaCircleApexLoop []openSectionPiece

func (a authenticatedParabolaCircleApexLoop) authenticate(ctx context.Context, domain geom.ParameterRectangle,
	input [][]PCurve, maximumEdges int) (trimvalidate.Report, error) {
	report := trimvalidate.Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}
	if domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || len(input) != 1 || len(input[0]) != len(a) {
		return report, geom.ErrConvergence
	}
	if maximumEdges > 0 && len(a) > maximumEdges {
		return report, trimvalidate.ErrLimit
	}
	for i, piece := range a {
		expected, err := mixedStoredUseCurveContext(ctx, Use{UVStart: piece.a, UVEnd: piece.b, Curve: piece.c}, geom.SurfacePatchOptions{})
		if err != nil {
			return report, err
		}
		if !sameParabolaCircleCrossingTrim(input[0][i], expected) {
			return report, geom.ErrConvergence
		}
	}
	report.Status = trimvalidate.Certified
	report.Reason = "exact parabola-circle apex sector and bridge"
	return report, nil
}

func parabolaCircleApexEnvelopeVolume(height [3]parabolaCircleMeshHeight, owners [4]int,
	p *geom.SplineGraphParabolaCircleApexTangency) [2]*big.Rat {
	net := new(big.Rat)
	for owner, h := range height[:2] {
		for u, row := range h.coefficients {
			for v, c := range row {
				if c != nil {
					m := new(big.Rat).Quo(c, big.NewRat(int64((u+1)*(v+1)), 1))
					if owner == 0 {
						m.Neg(m)
					}
					net.Add(net, m)
				}
			}
		}
	}
	// Start from A over the stock. Apply the signed A−B moments only
	// where the original envelope chooses B. The enclosing cusp faces
	// share one owner and together exclude the disk from their strip.
	strips, disk := parabolaCircleApexSignedMoments([2]parabolaCircleMeshHeight{height[1], height[2]}, p)
	for i, strip := range strips {
		if owners[i] == 1 {
			net.Sub(net, strip)
		}
	}
	factor := new(big.Rat).Mul(disk, big.NewRat(int64(owners[p.DiskRegion]-owners[2]), 1))
	pi := parabolaCircleMeshPi()
	if factor.Sign() < 0 {
		pi[0], pi[1] = pi[1], pi[0]
	}
	return [2]*big.Rat{
		new(big.Rat).Add(net, new(big.Rat).Mul(factor, pi[0])),
		new(big.Rat).Add(net, new(big.Rat).Mul(factor, pi[1])),
	}
}
