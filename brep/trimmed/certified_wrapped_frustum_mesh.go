package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedFrustumMeshOptions bounds the lateral interpolation proof,
// planar fan checks and mandatory exact display-contact check independently.
type CertifiedWrappedFrustumMeshOptions struct {
	Patch                       CertifiedPatchMeshOptions
	Embedding                   geom.SurfacePatchOptions
	Collision                   collision.Options
	MaxTriangles, MaxPredicates int
}

// CertifiedWrappedFrustumMesh includes the periodic side and both caps.
// OriginalFace is 0 for the lateral chart, 1 for the lower disk, 2 for the upper
// disk. RimVertices indexes the single common display samples for each cap.
// SquaredErrors bounds both directions per side triangle or radial cap sector;
// cap sectors use the same normalized boundary interpolation as the side.
// Source embedding and display embedding are proved separately. No certified
// material-distance or volume-error bound is asserted.
type CertifiedWrappedFrustumMesh struct {
	Mesh                                            mesh.Mesh
	Lateral                                         CertifiedKnotPatchMesh
	Embedding                                       geom.WrappedFrustumEmbedding
	OriginalFace                                    []int
	RimVertices                                     [2][]int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
	Predicates                                      int
}

// TessellateWrappedFrustumClosedCertified supports the canonical positive-radius
// rational frustum certificate, including circular and noncircular cylinders.
// Exact centers must be representable, and each represented rim must remain a
// simple star polygon about that center. The cap map is C+s*(curve(u)-C);
// replacing curve(u) by its synchronized side chord multiplies its error by
// s in [0,1]. Star-shaped source and proved polygon sectors give surjectivity
// in both directions, including the common center. No pole is collapsed and
// no tolerance-based welding is used. Failures return no partial mesh.
func TessellateWrappedFrustumClosedCertified(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedFrustumMesh, err error) {
	return tessellateWrappedFrustumFrom(ctx, w, o, 1)
}
func tessellateWrappedFrustumFrom(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedWrappedFrustumMeshOptions, minimum int) (out CertifiedWrappedFrustumMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedWrappedFrustumMesh{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 1000000
	}
	if o.MaxPredicates == 0 {
		o.MaxPredicates = 1000000
	}
	if o.MaxTriangles < 1 || o.MaxTriangles > 8000000 || o.MaxPredicates < 1 || o.MaxPredicates > 100000000 {
		return out, geom.ErrInvalid
	}
	if o.MaxTriangles < 16 {
		return out, geom.ErrConvergence
	}
	proof, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Embedding)
	if e != nil {
		return out, e
	}
	if !proof.Complete {
		return out, geom.ErrConvergence
	}
	side, e := tessellateCertifiedKnotGridFrom(ctx, w.FundamentalPatch(), [2]bool{true, false}, o.Patch, minimum)
	if e != nil {
		return out, e
	}
	if !side.BothDirections {
		return out, geom.ErrConvergence
	}
	nu, nv := len(side.UParameters)-1, len(side.VParameters)-1
	if nu < 4 || nv < 1 || len(side.Mesh.Vertices) != nu*(nv+1) || len(side.Mesh.Triangles)+2*nu > o.MaxTriangles {
		return out, geom.ErrConvergence
	}
	out.Embedding, out.Lateral = proof, side
	out.Mesh.Vertices = append([]geom.Vec3(nil), side.Mesh.Vertices...)
	out.Mesh.Triangles = append([][3]int(nil), side.Mesh.Triangles...)
	out.SourceImageSquaredDistance = new(big.Rat)
	for _, bound := range side.SquaredErrors {
		if bound == nil {
			return out, geom.ErrInvalid
		}
		out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(bound))
		if bound.Cmp(out.SourceImageSquaredDistance) > 0 {
			out.SourceImageSquaredDistance.Set(bound)
		}
	}
	out.OriginalFace = make([]int, len(out.Mesh.Triangles))
	for cap := 0; cap < 2; cap++ {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		z := proof.LowerZ
		if cap == 1 {
			z = proof.UpperZ
		}
		exact := [3]*big.Rat{proof.CenterX, proof.CenterY, z}
		var xyz [3]float64
		for i, r := range exact {
			xyz[i], _ = r.Float64()
			if !finite(xyz[i]) || ratInset(xyz[i]).Cmp(r) != 0 {
				return out, geom.ErrConvergence
			}
		}
		center := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		points := make([][2]*big.Rat, nu+1)
		ring := make([]int, nu)
		for i := 0; i < nu; i++ {
			id := i*(nv+1) + cap*nv
			p := out.Mesh.Vertices[id]
			if p.Z != xyz[2] {
				return out, geom.ErrConvergence
			}
			out.RimVertices[cap] = append(out.RimVertices[cap], id)
			ring[i] = i
			points[i] = [2]*big.Rat{ratInset(p.X), ratInset(p.Y)}
		}
		points[nu] = [2]*big.Rat{new(big.Rat).Set(proof.CenterX), new(big.Rat).Set(proof.CenterY)}
		budget := partitionFillProof{ctx: ctx, limit: o.MaxPredicates - out.Predicates, points: points}
		if budget.limit < 1 {
			return out, geom.ErrConvergence
		}
		if e := budget.simple(ring); e != nil {
			return out, e
		}
		// Every consecutive ray turns strictly positively; together with simplicity
		// this proves a nonoverlapping fan covering the represented closed disk.
		for i := 0; i < nu; i++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			a, b := points[i], points[(i+1)%nu]
			ax := new(big.Rat).Sub(a[0], points[nu][0])
			ay := new(big.Rat).Sub(a[1], points[nu][1])
			bx := new(big.Rat).Sub(b[0], points[nu][0])
			by := new(big.Rat).Sub(b[1], points[nu][1])
			if new(big.Rat).Sub(new(big.Rat).Mul(ax, by), new(big.Rat).Mul(ay, bx)).Sign() <= 0 {
				return out, geom.ErrConvergence
			}
		}
		out.Predicates += budget.work + nu
		if out.Predicates > o.MaxPredicates {
			return out, geom.ErrConvergence
		}
		centerID := len(out.Mesh.Vertices)
		out.Mesh.Vertices = append(out.Mesh.Vertices, center)
		for i, a := range out.RimVertices[cap] {
			b := out.RimVertices[cap][(i+1)%nu]
			tri := [3]int{centerID, a, b}
			if cap == 0 {
				tri[1], tri[2] = tri[2], tri[1]
			}
			out.Mesh.Triangles = append(out.Mesh.Triangles, tri)
			out.OriginalFace = append(out.OriginalFace, cap+1)
			out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(out.SourceImageSquaredDistance))
		}
	}
	if e := out.Mesh.ValidateSolid(); e != nil {
		return out, e
	}
	ids := make([]int, len(out.Mesh.Vertices))
	for i := range ids {
		ids[i] = i
	}
	if e := validateQuadraticJunctionFans(out.Mesh, ids); e != nil {
		return out, e
	}
	out.SignedDisplayVolume = new(big.Rat)
	for _, tri := range out.Mesh.Triangles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		a, b, c := exactBoundPoint(out.Mesh.Vertices[tri[0]]), exactBoundPoint(out.Mesh.Vertices[tri[1]]), exactBoundPoint(out.Mesh.Vertices[tri[2]])
		for i := 0; i < 3; i++ {
			j, k := (i+1)%3, (i+2)%3
			out.SignedDisplayVolume.Add(out.SignedDisplayVolume, new(big.Rat).Mul(a[i], new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))))
		}
	}
	out.SignedDisplayVolume.Quo(out.SignedDisplayVolume, big.NewRat(6, 1))
	if out.SignedDisplayVolume.Sign() <= 0 {
		return out, geom.ErrDegenerate
	}
	out.Collision, e = collision.SelfIntersectionsBVHContext(ctx, out.Mesh, o.Collision)
	if e != nil {
		return out, e
	}
	if !out.Collision.Complete || len(out.Collision.Contacts) != 0 {
		return out, geom.ErrConvergence
	}
	return out, nil
}
