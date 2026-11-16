package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// ParabolaCircleApexMeshContact retains the independently indexed display
// pair and both original source rays in each component. DependentSecond
// orders the parabola and circle second derivatives in the free source clock.
// ModelSecond records their separate lifts on each original roof, in
// component order then roof order.
type ParabolaCircleApexMeshContact struct {
	Source          ParabolaCircleApexCutContact
	Display         collision.AssemblyVertexContact
	Owners          [2]geom.ParabolaCircleApexOwner
	DependentSecond [2]*big.Rat
	ModelSecond     [2][2][3]*big.Rat
}

// CertifiedParabolaCircleApexTangencyMesh retains the separately manifold
// wall layer and disk lens and their single qualified vertex contact.
type CertifiedParabolaCircleApexTangencyMesh struct {
	Components [2]CertifiedParabolaTwoCirclesComponentMesh
	Contact    ParabolaCircleApexMeshContact
	Assembly   collision.VertexContactAssemblyReport
}

// TessellateCertified certifies the original identity-XY apex Cut with
// two-sided distance, signed volume, optional angular bounds and an
// exact assembly contact test.
func (r *ParabolaCircleApexTangencyCut) TessellateCertified(ctx context.Context,
	o ParabolaTwoCirclesMeshOptions) (CertifiedParabolaCircleApexTangencyMesh, error) {
	var empty CertifiedParabolaCircleApexTangencyMesh
	if ctx == nil || r == nil || r.sources[0] == nil {
		return empty, geom.ErrInvalid
	}
	o, err := checkedParabolaCircleMeshOptions(o)
	if err != nil {
		return empty, err
	}
	height, patches, err := parabolaCircleCutDisplayHeights(ctx, r.sources)
	if err != nil {
		return empty, err
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, patches[0], patches[1], r.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	result := CertifiedParabolaCircleApexTangencyMesh{Contact: ParabolaCircleApexMeshContact{
		Source: r.Contact(), Owners: proof.Owners,
		DependentSecond: [2]*big.Rat{proof.ParabolaSecondDerivative, proof.CircleSecondDerivative},
	}}
	for i, second := range result.Contact.DependentSecond {
		for owner, first := range proof.Owners {
			lift := &result.Contact.ModelSecond[i][owner]
			for axis, slope := range first.DependentPartial {
				lift[axis] = new(big.Rat).Mul(slope, second)
			}
			// With no dependent first derivative at the apex, the only
			// other second derivative is the free surface derivative.
			lift[2].Add(lift[2], parabolaCircleApexFreeSecond(height[owner], first.Parameters, proof.FreeAxis))
		}
		if !exactPointCoordinatesEqual(result.Contact.ModelSecond[i][0], result.Contact.ModelSecond[i][1]) {
			return empty, geom.ErrConvergence
		}
	}
	source := parabolaCircleTangentMeshSource{
		name: "apex", axis: proof.FreeAxis, diskRegion: proof.DiskRegion, regionSides: proof.RegionSides,
		parameters: proof.Parameters, point: proof.Point, slope: new(big.Rat), second: result.Contact.DependentSecond,
	}
	volumes := parabolaCircleApexCutMeshVolumes(height, proof)
	display, err := displayParabolaCircleTangentCut(ctx, r, height, source, volumes, o)
	if err != nil {
		return empty, err
	}
	result.Components, result.Contact.Display, result.Assembly = display.components, display.contact, display.assembly
	return result, nil
}

func parabolaCircleCutDisplayHeights(ctx context.Context, sources [3]*geom.BSplineSurface) (
	height [2]parabolaCircleMeshHeight, patches [2]*geom.SurfacePatch, err error) {
	for i := range height {
		height[i], patches[i], err = parabolaCircleDisplayHeight(ctx, sources[i+1])
		if err != nil {
			return height, patches, err
		}
		height[i].normalHessian = parabolaCircleMeshHessian(sources[i+1])
		height[i].hessian = height[i].normalHessian
	}
	return height, patches, nil
}

type parabolaCircleTangentMeshSource struct {
	name             string
	axis, diskRegion int
	regionSides      [3]int
	parameters       [2]*big.Rat
	point            [3]*big.Rat
	slope            *big.Rat
	second           [2]*big.Rat
}

type parabolaCircleTangentMeshDisplay struct {
	components [2]CertifiedParabolaTwoCirclesComponentMesh
	contact    collision.AssemblyVertexContact
	assembly   collision.VertexContactAssemblyReport
}

func displayParabolaCircleTangentCut(ctx context.Context, r *ParabolaCircleApexTangencyCut, height [2]parabolaCircleMeshHeight,
	p parabolaCircleTangentMeshSource, volumes [2][2]*big.Rat, o ParabolaTwoCirclesMeshOptions) (parabolaCircleTangentMeshDisplay, error) {
	var empty parabolaCircleTangentMeshDisplay
	plane, err := parabolaCircleMeshUVPlane(ctx)
	if err != nil {
		return empty, err
	}
	source := r.Contact()
	if source.Parameters[0].Cmp(p.parameters[0]) != 0 || source.Parameters[1].Cmp(p.parameters[1]) != 0 ||
		!exactPointCoordinatesEqual(source.Point, p.point) {
		return empty, geom.ErrConvergence
	}
	var result parabolaCircleTangentMeshDisplay
	var meshes [2]mesh.Mesh
	var vertices [2]collision.AssemblyVertex
	triangleBudget := o.MaxTriangles
	for i, s := range r.components {
		h := r.history[i]
		region := [2]int{1 - p.diskRegion, 2}[i]
		if h.Region != region || p.regionSides[region] <= 0 || volumes[i][0].Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		if o.MaxTriangles < len(s.faces) {
			return empty, parametric.ErrBudget
		}
		native := source.Vertices[i]
		if native < 0 || native >= len(s.vertices) {
			return empty, geom.ErrConvergence
		}
		second := p.second[i]
		for direction, ray := range source.Rays[i] {
			if ray < 0 || ray >= len(s.edges) {
				return empty, geom.ErrConvergence
			}
			sign, err := parabolaCircleTangentRay(s, ray, native, p.axis, p.slope, second)
			if err != nil || sign != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
		owners := ParabolaCircleCutComponentHistory{Region: h.Region, RoofFaces: h.RoofFaces, WallFaces: h.WallFaces}
		m, err := parabolaTwoCirclesComponentDisplay(ctx, s, owners, height, plane, volumes[i], o, nil, true)
		if err != nil {
			return empty, fmt.Errorf("trimmed: parabola-circle %s display region %d: %w", p.name, region, err)
		}
		vertex := m.OriginalVertices[native]
		if !exactPointCoordinatesEqual(exactBoundPoint(m.Mesh.Vertices[vertex]), p.point) {
			return empty, geom.ErrConvergence
		}
		for direction, ray := range source.Rays[i] {
			edge := m.OriginalEdges[ray].Vertices
			adjacent := edge[1]
			if edge[len(edge)-1] == vertex {
				adjacent = edge[len(edge)-2]
			} else if edge[0] != vertex {
				return empty, geom.ErrConvergence
			}
			station := exactBoundPoint(m.Mesh.Vertices[adjacent])[p.axis]
			if station.Cmp(source.Parameters[p.axis]) != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
		result.components[i], meshes[i] = m, m.Mesh
		vertices[i] = collision.AssemblyVertex{Solid: i, Vertex: vertex}
		o.MaxTriangles -= len(m.Mesh.Triangles)
	}
	result.contact = collision.AssemblyVertexContact{A: vertices[0], B: vertices[1]}
	pairs := o.Collision.MaxPairs
	if pairs == 0 {
		pairs = collision.DefaultMaxPairs
	}
	result.assembly, err = collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], []collision.AssemblyVertexContact{result.contact},
		collision.VertexContactAssemblyOptions{MaxSolids: 2, MaxDeclaredContacts: 1, MaxCrossPairs: pairs, CrossBVH: true,
			Validation: collision.LocationOptions{MaxTriangles: triangleBudget, MaxCollisionPairs: pairs}})
	if err != nil {
		return empty, fmt.Errorf("trimmed: parabola-circle %s display assembly: %w", p.name, err)
	}
	signed := new(big.Rat).Add(result.components[0].SignedDisplayVolume, result.components[1].SignedDisplayVolume)
	if len(result.assembly.Contacts) != 1 || result.assembly.Contacts[0] != result.contact ||
		result.assembly.SignedVolume.Cmp(signed) != 0 {
		return empty, geom.ErrConvergence
	}
	return result, nil
}

func parabolaCircleApexFreeSecond(h parabolaCircleMeshHeight, uv [2]*big.Rat, free int) *big.Rat {
	var powers [2][9]*big.Rat
	for axis, at := range uv {
		powers[axis][0] = big.NewRat(1, 1)
		for i := 1; i <= h.degree[axis]; i++ {
			powers[axis][i] = new(big.Rat).Mul(powers[axis][i-1], at)
		}
	}
	out := new(big.Rat)
	for u, row := range h.coefficients {
		for v, c := range row {
			exponents := [2]int{u, v}
			d := exponents[free]
			if c == nil || d < 2 {
				continue
			}
			exponents[free] -= 2
			term := new(big.Rat).Mul(c, big.NewRat(int64(d*(d-1)), 1))
			term.Mul(term, powers[0][exponents[0]])
			out.Add(out, term.Mul(term, powers[1][exponents[1]]))
		}
	}
	return out
}

func parabolaCircleApexCutMeshVolumes(height [2]parabolaCircleMeshHeight, p *geom.SplineGraphParabolaCircleApexTangency) [2][2]*big.Rat {
	// Only the strip opposite the disk survives the native Cut gate.
	strips, disk := parabolaCircleApexSignedMoments(height, p)
	return parabolaCircleTangentCutVolumes(strips[1-p.DiskRegion], disk)
}

func parabolaCircleTangentCutVolumes(strip, disk *big.Rat) [2][2]*big.Rat {
	pi := parabolaCircleMeshPi()
	return [2][2]*big.Rat{{strip, new(big.Rat).Set(strip)},
		{new(big.Rat).Mul(disk, pi[0]), new(big.Rat).Mul(disk, pi[1])}}
}

// The whole strips are rational. The disk moment excludes its factor pi.
func parabolaCircleApexSignedMoments(height [2]parabolaCircleMeshHeight, p *geom.SplineGraphParabolaCircleApexTangency) (
	strips [2]*big.Rat, disk *big.Rat) {
	return parabolaCircleTangentSignedMoments(height, p.ParabolaControls, p.FreeAxis,
		geom.GraphContactCircle{Center: p.CircleCenter, RadiusSquared: p.RadiusSquared})
}

func parabolaCircleTangentSignedMoments(height [2]parabolaCircleMeshHeight, controls [3][2]*big.Rat,
	free int, circle geom.GraphContactCircle) (strips [2]*big.Rat, disk *big.Rat) {
	powers := parabolaCircleMeshStripPowers(controls, free)
	lower, unit := new(big.Rat), new(big.Rat)
	disk = new(big.Rat)
	for owner, h := range height {
		for u, row := range h.coefficients {
			for v, c := range row {
				if c == nil {
					continue
				}
				coefficient := new(big.Rat).Set(c)
				if owner == 1 {
					coefficient.Neg(coefficient)
				}
				x, y := u, v
				if free == 1 {
					x, y = v, u
				}
				for k, term := range powers[y+1] {
					moment := new(big.Rat).Quo(term, big.NewRat(int64((y+1)*(x+k+1)), 1))
					lower.Add(lower, moment.Mul(moment, coefficient))
				}
				unit.Add(unit, new(big.Rat).Quo(coefficient, big.NewRat(int64((u+1)*(v+1)), 1)))
				moment := parabolaCircleDiskMoment(circle, u, v)
				disk.Add(disk, moment.Mul(moment, coefficient))
			}
		}
	}
	return [2]*big.Rat{lower, new(big.Rat).Sub(unit, lower)}, disk
}
