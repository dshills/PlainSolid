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
	var height [2]parabolaCircleMeshHeight
	var patches [2]*geom.SurfacePatch
	for i := range height {
		height[i], patches[i], err = parabolaCircleDisplayHeight(ctx, r.sources[i+1])
		if err != nil {
			return empty, err
		}
		height[i].normalHessian = parabolaCircleMeshHessian(r.sources[i+1])
		height[i].hessian = height[i].normalHessian
	}
	proof, err := geom.DiscoverSplineGraphParabolaCircleApexTangency(ctx, patches[0], patches[1], r.axis, geom.GraphPointContactOptions{})
	if err != nil {
		return empty, err
	}
	volumes := parabolaCircleApexCutMeshVolumes(height, proof)
	plane, err := parabolaCircleMeshUVPlane(ctx)
	if err != nil {
		return empty, err
	}
	source := r.Contact()
	if source.Parameters[0].Cmp(proof.Parameters[0]) != 0 || source.Parameters[1].Cmp(proof.Parameters[1]) != 0 ||
		!exactPointCoordinatesEqual(source.Point, proof.Point) {
		return empty, geom.ErrConvergence
	}
	result := CertifiedParabolaCircleApexTangencyMesh{Contact: ParabolaCircleApexMeshContact{
		Source: source, Owners: proof.Owners,
		DependentSecond: [2]*big.Rat{proof.ParabolaSecondDerivative, proof.CircleSecondDerivative},
	}}
	var meshes [2]mesh.Mesh
	var vertices [2]collision.AssemblyVertex
	triangleBudget := o.MaxTriangles
	for i, s := range r.components {
		h := r.history[i]
		region := [2]int{1 - proof.DiskRegion, 2}[i]
		if h.Region != region || proof.RegionSides[region] <= 0 || volumes[i][0].Sign() <= 0 {
			return empty, geom.ErrConvergence
		}
		if o.MaxTriangles < len(s.faces) {
			return empty, parametric.ErrBudget
		}
		native := source.Vertices[i]
		if native < 0 || native >= len(s.vertices) {
			return empty, geom.ErrConvergence
		}
		second := result.Contact.DependentSecond[i]
		for direction, ray := range source.Rays[i] {
			if ray < 0 || ray >= len(s.edges) {
				return empty, geom.ErrConvergence
			}
			sign, err := parabolaCircleApexRay(s, ray, native, proof.FreeAxis, second)
			if err != nil || sign != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
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
		owners := ParabolaCircleCutComponentHistory{Region: h.Region, RoofFaces: h.RoofFaces, WallFaces: h.WallFaces}
		m, err := parabolaTwoCirclesComponentDisplay(ctx, s, owners, height, plane, volumes[i], o, nil, true)
		if err != nil {
			return empty, fmt.Errorf("trimmed: parabola-circle apex display region %d: %w", region, err)
		}
		vertex := m.OriginalVertices[native]
		if !exactPointCoordinatesEqual(exactBoundPoint(m.Mesh.Vertices[vertex]), proof.Point) {
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
			station := exactBoundPoint(m.Mesh.Vertices[adjacent])[proof.FreeAxis]
			if station.Cmp(source.Parameters[proof.FreeAxis]) != 2*direction-1 {
				return empty, geom.ErrConvergence
			}
		}
		result.Components[i], meshes[i] = m, m.Mesh
		vertices[i] = collision.AssemblyVertex{Solid: i, Vertex: vertex}
		o.MaxTriangles -= len(m.Mesh.Triangles)
	}
	result.Contact.Display = collision.AssemblyVertexContact{A: vertices[0], B: vertices[1]}
	pairs := o.Collision.MaxPairs
	if pairs == 0 {
		pairs = collision.DefaultMaxPairs
	}
	result.Assembly, err = collision.ValidateMeshVertexContactAssembly(ctx, meshes[:], []collision.AssemblyVertexContact{result.Contact.Display},
		collision.VertexContactAssemblyOptions{MaxSolids: 2, MaxDeclaredContacts: 1, MaxCrossPairs: pairs, CrossBVH: true,
			Validation: collision.LocationOptions{MaxTriangles: triangleBudget, MaxCollisionPairs: pairs}})
	if err != nil {
		return empty, fmt.Errorf("trimmed: parabola-circle apex display assembly: %w", err)
	}
	signed := new(big.Rat).Add(result.Components[0].SignedDisplayVolume, result.Components[1].SignedDisplayVolume)
	if len(result.Assembly.Contacts) != 1 || result.Assembly.Contacts[0] != result.Contact.Display ||
		result.Assembly.SignedVolume.Cmp(signed) != 0 {
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
	// Only the strip opposite the disk survives the native Cut gate. Its
	// contribution is rational. A full disk moment is a rational times pi.
	free := p.FreeAxis
	powers := parabolaCircleMeshStripPowers(p.ParabolaControls, free)
	circle := geom.GraphContactCircle{Center: p.CircleCenter, RadiusSquared: p.RadiusSquared}
	lower, unit, disk := new(big.Rat), new(big.Rat), new(big.Rat)
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
	strip := lower
	if p.DiskRegion == 0 {
		strip = new(big.Rat).Sub(unit, lower)
	}
	pi := parabolaCircleMeshPi()
	return [2][2]*big.Rat{{strip, new(big.Rat).Set(strip)},
		{new(big.Rat).Mul(disk, pi[0]), new(big.Rat).Mul(disk, pi[1])}}
}
