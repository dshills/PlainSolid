package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlacedObliqueTorusCavityMeshOptions shares Mesh.Collision.MaxPairs between
// local and world checks. MaxWorldVertices defaults 100000 and caps exact pose
// transforms; each transform additionally has a 4096-bit coefficient limit.
// Nesting defaults to 2M work units and 64 exact ray attempts, separately bounded.
type PlacedObliqueTorusCavityMeshOptions struct {
	Mesh                           ObliqueTorusCavityMeshOptions
	MaxWorldVertices               int
	MaxNestingWork, MaxNestingRays int
}

// CertifiedPlacedObliqueTorusCavityMesh describes only a display of the exact
// source occurrence, not a rounded transformed native BRep. Source topology and
// original torus edge indices remain unchanged. DistanceUpper includes both
// local sampling/rounding and WorldRoundingUpper; proper rigidity preserves the
// local two-sided bound before the second explicitly measured rounding.
// Exact represented nesting additionally proves the stock shell at depth0 and
// its reversed torus cavity at depth1; no-contact alone is insufficient.
type CertifiedPlacedObliqueTorusCavityMesh struct {
	CertifiedObliqueTorusCavityMesh
	Pose               [3][4]*big.Rat
	WorldRoundingUpper *big.Rat
	LocalCollision     collision.Report
	ShellSignedVolumes [2]*big.Rat
	Shells             [2]collision.ShellInfo
}

func (p *PreparedObliqueTorusCavity) TessellatePlacedCertified(ctx context.Context, pose placement.RationalRigid, o PlacedObliqueTorusCavityMeshOptions) (out CertifiedPlacedObliqueTorusCavityMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedPlacedObliqueTorusCavityMesh{}
		}
	}()
	if p == nil || p.major == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxWorldVertices == 0 {
		o.MaxWorldVertices = 100000
	}
	if o.MaxWorldVertices < 1 || o.MaxWorldVertices > 100000 {
		return out, geom.ErrInvalid
	}
	n := o.Mesh.Subdivisions
	if n < 1 || n > 128 {
		return out, geom.ErrInvalid
	}
	if 16*n*n+8*n+2 > o.MaxWorldVertices {
		return out, geom.ErrConvergence
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return out, e
	}
	frame, e := p.source.TorusFrame()
	if e != nil {
		return out, e
	}
	d := p.dimensions
	base, e := TessellateObliqueTorusCavityCertified(ctx, d[0], d[1], d[2], d[3], d[4], frame, p.body.tolerance, o.Mesh)
	if e != nil {
		return out, e
	}
	if len(base.Mesh.Vertices) > o.MaxWorldVertices {
		return out, geom.ErrConvergence
	}
	collisionOptions := o.Mesh.Collision
	if collisionOptions.MaxPairs == 0 {
		collisionOptions.MaxPairs = collision.DefaultMaxPairs
	}
	collisionOptions.MaxPairs -= base.Collision.BroadPhasePairs
	if collisionOptions.MaxPairs < 1 {
		return out, collision.ErrLimit
	}
	out.Pose = matrix
	out.LocalCollision = base.Collision
	out.WorldRoundingUpper = new(big.Rat)
	for i, v := range base.Mesh.Vertices {
		q, e := pose.TransformPoint(ctx, exactBoundPoint(v), false, 4096)
		if e != nil {
			return out, e
		}
		display, e := rationalDisplayPoint(q)
		if e != nil {
			return out, e
		}
		rounded := exactBoundPoint(display)
		delta := new(big.Rat)
		for j := 0; j < 3; j++ {
			x := new(big.Rat).Sub(q[j], rounded[j])
			delta.Add(delta, x.Abs(x))
		}
		if delta.Cmp(out.WorldRoundingUpper) > 0 {
			out.WorldRoundingUpper.Set(delta)
		}
		base.Mesh.Vertices[i] = display
	}
	base.DistanceUpper = new(big.Rat).Add(base.DistanceUpper, out.WorldRoundingUpper)
	if base.DistanceUpper.Cmp(ratInset(o.Mesh.Deflection)) > 0 {
		return out, geom.ErrConvergence
	}
	locator, e := collision.NewMeshPointLocator(ctx, base.Mesh, base.OriginalFace, collision.LocationOptions{MaxTriangles: collision.MaxTriangles, MaxWork: o.MaxNestingWork, MaxRays: o.MaxNestingRays, MaxCollisionPairs: collisionOptions.MaxPairs})
	if e != nil {
		return out, e
	}
	shells := locator.Shells()
	if len(shells) != 2 || shells[0].Parent != -1 || shells[0].Depth != 0 || shells[1].Parent != 0 || shells[1].Depth != 1 {
		return out, geom.ErrDomain
	}
	for i, shell := range shells {
		for _, triangle := range shell.Triangles {
			if (base.OriginalFace[triangle] >= 3) != (i == 1) {
				return out, geom.ErrDomain
			}
		}
		out.Shells[i] = shell
		out.ShellSignedVolumes[i] = new(big.Rat).Set(shell.SignedVolume)
	}
	base.Collision = locator.BoundaryCollision()
	base.SignedDisplayVolume = new(big.Rat).Add(out.ShellSignedVolumes[0], out.ShellSignedVolumes[1])
	if base.SignedDisplayVolume.Sign() <= 0 {
		return out, geom.ErrDomain
	}
	out.CertifiedObliqueTorusCavityMesh = base
	return out, nil
}
