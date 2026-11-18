//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedParabolaCircleObliqueEnvelopeMesh retains the original closed
// shell, contact vertex, four tangent rays and the discrete image of the
// exact original same-roof bridge. BridgeUV gives that native model
// coedge's endpoints in model parameter order.
type CertifiedParabolaCircleObliqueEnvelopeMesh struct {
	CertifiedParabolaTwoCirclesComponentMesh
	ContactVertex   int
	Rays            [2][2]int
	Bridge          int
	BridgeUV        [2]geom.Vec2
	Slope           *big.Rat
	Owners          [2]geom.ParabolaCircleObliqueOwner
	DependentSecond [2]*big.Rat
	ModelSecond     [2][2][3]*big.Rat
}

// TessellateCertified verifies the rounded closed envelope on its original
// graph charts. The two tangent-ended roof loops require the reconstructed
// source proof; independently checked polygon cuts and the signed embedding
// proof apply to the entire shell. NormalTurn and NormalAngle are optional.
func (r *ParabolaCircleObliqueTangencyEnvelope) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleObliqueEnvelopeMesh, error) {
	var empty CertifiedParabolaCircleObliqueEnvelopeMesh
	if ctx == nil || r == nil || r.envelope == nil || r.envelope.sources[0] == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedParabolaCircleMeshOptions(o)
	if err != nil {
		return empty, err
	}
	env := r.envelope
	if o.MaxTriangles < len(env.solid.faces) {
		return empty, parametric.ErrBudget
	}
	section, err := proveParabolaCircleObliqueSection(ctx, env.sources, env.axis, env.tolerance, ParabolaCircleGraphOptions{})
	if err != nil {
		return empty, err
	}
	regions, bridge, err := parabolaCircleObliqueEnvelopeRegions(section)
	if err != nil {
		return empty, err
	}
	height, patches, err := parabolaCircleEnvelopeHeights(ctx, env.sources)
	if err != nil {
		return empty, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, patches[0], patches[1], section.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	lifts, err := parabolaCircleObliqueModelSecond(ctx, patches, proof, section.second)
	if err != nil {
		return empty, err
	}
	strips, disk := parabolaCircleTangentSignedMoments([2]parabolaCircleMeshHeight{height[1], height[2]}, proof.ParabolaControls,
		section.axis, geom.GraphContactCircle{Center: proof.CircleCenter, RadiusSquared: proof.RadiusSquared})
	volume := parabolaCircleTangentEnvelopeVolume(height, env.history.RoofOwners, section.diskRegion, strips, disk)
	out, err := displayParabolaCircleTangentEnvelope(ctx, env, section, regions, bridge, height, volume, o)
	if err != nil {
		return empty, err
	}
	return CertifiedParabolaCircleObliqueEnvelopeMesh{
		CertifiedParabolaTwoCirclesComponentMesh: out.component,
		ContactVertex:                            out.vertex, Rays: env.history.Rays, Bridge: env.history.Bridge, BridgeUV: out.bridgeUV,
		Slope: section.slope, Owners: proof.Owners, DependentSecond: section.second, ModelSecond: lifts,
	}, nil
}
