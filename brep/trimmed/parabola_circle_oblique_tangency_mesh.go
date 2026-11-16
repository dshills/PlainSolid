package trimmed

import (
	"context"
	"math/big"

	"plainsolid/collision"
	"plainsolid/geom"
)

// ParabolaCircleObliqueMeshContact preserves the sole original contact as
// two mesh vertex IDs and records each original roof's oblique tangent.
// ModelSecond orders the exact second derivatives by component (parabola
// then circle) and original roof. All derivatives use the free source clock.
type ParabolaCircleObliqueMeshContact struct {
	Source          ParabolaCircleObliqueCutContact
	Display         collision.AssemblyVertexContact
	Owners          [2]geom.ParabolaCircleObliqueOwner
	Slope           *big.Rat
	DependentSecond [2]*big.Rat
	ModelSecond     [2][2][3]*big.Rat
}

// CertifiedParabolaCircleObliqueTangencyMesh retains two positive, separately
// indexed shells whose closures meet at the sole authenticated source point.
type CertifiedParabolaCircleObliqueTangencyMesh struct {
	Components [2]CertifiedParabolaTwoCirclesComponentMesh
	Contact    ParabolaCircleObliqueMeshContact
	Assembly   collision.VertexContactAssemblyReport
}

// TessellateCertified verifies both closed source shells and their single
// oblique contact. It certifies signed volume, two-sided distance and the
// optional source-turn and facet-normal bounds on the original supports.
func (r *ParabolaCircleObliqueTangencyCut) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleObliqueTangencyMesh, error) {
	var empty CertifiedParabolaCircleObliqueTangencyMesh
	if ctx == nil || r == nil || r.cut == nil || r.cut.sources[0] == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedParabolaCircleMeshOptions(o)
	if err != nil {
		return empty, err
	}
	height, patches, err := parabolaCircleCutDisplayHeights(ctx, r.cut.sources)
	if err != nil {
		return empty, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleObliqueTangency(ctx, patches[0], patches[1], r.cut.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	second := [2]*big.Rat{proof.ParabolaSecondDerivative, proof.CircleSecondDerivative}
	lifts, err := parabolaCircleObliqueModelSecond(ctx, patches, proof, second)
	if err != nil {
		return empty, err
	}
	source := parabolaCircleTangentMeshSource{
		name: "oblique", axis: proof.FreeAxis, diskRegion: proof.DiskRegion, regionSides: proof.RegionSides,
		parameters: proof.Parameters, point: proof.Point, slope: proof.Slope, second: second,
	}
	strips, disk := parabolaCircleTangentSignedMoments(height, proof.ParabolaControls, proof.FreeAxis,
		geom.GraphContactCircle{Center: proof.CircleCenter, RadiusSquared: proof.RadiusSquared})
	volumes := parabolaCircleTangentCutVolumes(strips[1-proof.DiskRegion], disk)
	display, err := displayParabolaCircleTangentCut(ctx, r.cut, height, source, volumes, o)
	if err != nil {
		return empty, err
	}
	return CertifiedParabolaCircleObliqueTangencyMesh{
		Components: display.components, Assembly: display.assembly,
		Contact: ParabolaCircleObliqueMeshContact{
			Source: r.Contact(), Display: display.contact, Owners: proof.Owners,
			Slope: proof.Slope, DependentSecond: second, ModelSecond: lifts,
		},
	}, nil
}

func parabolaCircleObliqueModelSecond(ctx context.Context, patches [2]*geom.SurfacePatch,
	p *geom.SplineGraphParabolaCircleObliqueTangency, second [2]*big.Rat) (lifts [2][2][3]*big.Rat, err error) {
	twiceSlope := new(big.Rat).Add(p.Slope, p.Slope)
	squaredSlope := new(big.Rat).Mul(p.Slope, p.Slope)
	for owner, patch := range patches {
		u, v := p.Owners[owner].Parameters[0], p.Owners[owner].Parameters[1]
		jet, err := patch.EvaluateSecondJetRational(ctx, u, v, geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			return lifts, err
		}
		freeFree, dependentDependent := jet.Duu, jet.Dvv
		if p.FreeAxis == 1 {
			freeFree, dependentDependent = dependentDependent, freeFree
		}
		// Differentiate the original curve through the original surface.
		// Its oblique tangent contributes S_ff + 2m*S_fd + m²*S_dd.
		// The curvature of each separate branch adds y''*S_d.
		for axis := range freeFree {
			hessian := new(big.Rat).Mul(twiceSlope, jet.Duv[axis])
			hessian.Add(hessian, freeFree[axis])
			hessian.Add(hessian, new(big.Rat).Mul(squaredSlope, dependentDependent[axis]))
			for branch, curvature := range second {
				q := new(big.Rat).Mul(curvature, p.Owners[owner].DependentPartial[axis])
				lifts[branch][owner][axis] = q.Add(q, hessian)
			}
		}
	}
	for _, branch := range lifts {
		if !exactPointCoordinatesEqual(branch[0], branch[1]) {
			return lifts, geom.ErrConvergence
		}
	}
	return lifts, nil
}
