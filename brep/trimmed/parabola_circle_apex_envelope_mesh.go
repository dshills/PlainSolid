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
	section := source.tangentSection()
	height, _, err := parabolaCircleEnvelopeHeights(ctx, r.sources)
	if err != nil {
		return empty, err
	}
	volume := parabolaCircleApexEnvelopeVolume(height, r.history.RoofOwners, source.proof)
	out, err := displayParabolaCircleTangentEnvelope(ctx, r, section, regions, bridge, height, volume, o)
	if err != nil {
		return empty, err
	}
	return CertifiedParabolaCircleApexEnvelopeMesh{
		CertifiedParabolaTwoCirclesComponentMesh: out.component,
		ApexVertex:                               out.vertex, Rays: r.history.Rays, Bridge: r.history.Bridge, DependentSecond: section.second,
	}, nil
}

type parabolaCircleTangentEnvelopeDisplay struct {
	component CertifiedParabolaTwoCirclesComponentMesh
	vertex    int
	bridgeUV  [2]geom.Vec2
}

func displayParabolaCircleTangentEnvelope(ctx context.Context, r *ParabolaCircleApexTangencyEnvelope,
	section parabolaCircleTangentSection, regions [4]openGraphRegion, bridge openSectionPiece, height [3]parabolaCircleMeshHeight,
	volume [2]*big.Rat, o ParabolaTwoCirclesMeshOptions) (parabolaCircleTangentEnvelopeDisplay, error) {
	var empty parabolaCircleTangentEnvelopeDisplay
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
		if i == section.diskRegion || i == 3 {
			layout.tangent[i] = authenticatedParabolaCircleTangentLoop(region.loop)
		}
	}
	h := r.history
	if h.Bridge < 0 || h.Bridge >= len(r.solid.edges) || h.Vertex < 0 || h.Vertex >= len(r.solid.vertices) {
		return empty, geom.ErrConvergence
	}
	model, ok := r.solid.edges[h.Bridge].Curve.(surfaceTrimCurve)
	if !ok || model.curve != nil || model.surface != r.sources[1+h.RoofOwners[section.diskRegion]] ||
		!(model.a == bridge.a && model.b == bridge.b || model.a == bridge.b && model.b == bridge.a) {
		return empty, geom.ErrConvergence
	}
	a, b := [2]float64{model.a.X, model.a.Y}, [2]float64{model.b.X, model.b.Y}
	free, dependent := section.axis, 1-section.axis
	du := new(big.Rat).Sub(ratInset(b[free]), ratInset(a[free]))
	dv := new(big.Rat).Sub(ratInset(b[dependent]), ratInset(a[dependent]))
	if dv.Sign() == 0 || du.Add(du, new(big.Rat).Mul(section.slope, dv)).Sign() != 0 {
		return empty, geom.ErrConvergence
	}
	for i, rays := range h.Rays {
		for direction, id := range rays {
			if id < 0 || id >= len(r.solid.edges) {
				return empty, geom.ErrConvergence
			}
			sign, err := parabolaCircleTangentRay(r.solid, id, h.Vertex, section.axis, section.slope, section.second[i])
			if err != nil || sign != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
	}
	display, err := certifyParabolaCircleEnvelopeDisplay(ctx, r.solid, layout, height, volume, o)
	if err != nil {
		return empty, err
	}
	vertex := display.OriginalVertices[h.Vertex]
	if !exactPointCoordinatesEqual(exactBoundPoint(display.Mesh.Vertices[vertex]), section.point) {
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
			station := exactBoundPoint(display.Mesh.Vertices[next])[section.axis]
			if station.Cmp(section.parameters[section.axis]) != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
	}
	return parabolaCircleTangentEnvelopeDisplay{component: display, vertex: vertex, bridgeUV: [2]geom.Vec2{model.a, model.b}}, nil
}

// authenticatedParabolaCircleTangentLoop carries the original coedges from the
// reconstructed tangent partition. Those loops are simple and CCW by the
// complete circle–parabola factor contact proof and the outward bridge.
// Authenticate every sampled coefficient against that partition. The
// sampled chord polygon still receives its independent ordinary proof.
type authenticatedParabolaCircleTangentLoop []openSectionPiece

func (a authenticatedParabolaCircleTangentLoop) authenticate(ctx context.Context, domain geom.ParameterRectangle,
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
	report.Reason = "exact parabola-circle tangent sector and bridge"
	return report, nil
}

func parabolaCircleApexEnvelopeVolume(height [3]parabolaCircleMeshHeight, owners [4]int,
	p *geom.SplineGraphParabolaCircleApexTangency) [2]*big.Rat {
	strips, disk := parabolaCircleApexSignedMoments([2]parabolaCircleMeshHeight{height[1], height[2]}, p)
	return parabolaCircleTangentEnvelopeVolume(height, owners, p.DiskRegion, strips, disk)
}

func parabolaCircleTangentEnvelopeVolume(height [3]parabolaCircleMeshHeight, owners [4]int, diskRegion int,
	strips [2]*big.Rat, disk *big.Rat) [2]*big.Rat {
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
	for i, strip := range strips {
		if owners[i] == 1 {
			net.Sub(net, strip)
		}
	}
	factor := new(big.Rat).Mul(disk, big.NewRat(int64(owners[diskRegion]-owners[2]), 1))
	pi := parabolaCircleMeshPi()
	if factor.Sign() < 0 {
		pi[0], pi[1] = pi[1], pi[0]
	}
	return [2]*big.Rat{
		new(big.Rat).Add(net, new(big.Rat).Mul(factor, pi[0])),
		new(big.Rat).Add(net, new(big.Rat).Mul(factor, pi[1])),
	}
}
