//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// CertifiedWrappedConeMesh has exactly one apex vertex, justified by exact
// equality of the complete collapsed homogeneous control row. Profile is the
// certified auxiliary cylinder used to bound base-curve chord interpolation;
// it is not the source cone. OriginalFace is 0 for cone side and 1 for base.
// SquaredErrors and SourceImageSquaredDistance certify both directed surface
// distances by the radial/ruled parameter correspondence, including the pole.
type CertifiedWrappedConeMesh struct {
	Mesh                                            mesh.Mesh
	Profile                                         CertifiedWrappedFrustumMesh
	ApexVertex, BaseCenterVertex                    int
	RimVertices                                     []int
	OriginalFace                                    []int
	SquaredErrors                                   []*big.Rat
	SourceImageSquaredDistance, SignedDisplayVolume *big.Rat
	Collision                                       collision.Report
}

// TessellateWrappedConeClosedCertified accepts an upper collapsed pole and a
// canonical rational quadratic lower profile, with affine height and equal V
// weights. The pole must lie exactly on the profile axis above its base.
// Lossless spline export is required; no sampled or tolerance-based collapse is
// permitted. Lifting the top control row to the lower XY profile yields an
// independently certified cylinder, proving regular source cone sectors away
// from the apex and one circular apex link. Auxiliary-cylinder construction and
// final mesh both obey MaxTriangles; all other budgets retain the frustum API
// semantics (the final contact proof uses a separate Collision budget).
func TessellateWrappedConeClosedCertified(ctx context.Context, w *geom.WrappedSurfacePatch, o CertifiedWrappedFrustumMeshOptions) (out CertifiedWrappedConeMesh, err error) {
	defer func() {
		if err != nil {
			out = CertifiedWrappedConeMesh{}
		}
	}()
	if ctx == nil || w == nil || w.FundamentalPatch() == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	periods := w.Periods()
	if periods[0] == nil || periods[1] != nil {
		return out, geom.ErrConvergence
	}
	spline, e := w.FundamentalPatch().ToBSplineSurface()
	if e != nil {
		return out, e
	}
	d := spline.Data()
	if d.DegreeU != 2 || d.DegreeV != 1 || d.CountU != 9 || d.CountV != 2 || len(d.Poles) != 18 {
		return out, geom.ErrConvergence
	}
	apex := d.Poles[1]
	for i := 0; i < 9; i++ {
		if d.Poles[2*i+1] != apex {
			return out, geom.ErrConvergence
		}
		d.Poles[2*i+1].X = d.Poles[2*i].X
		d.Poles[2*i+1].Y = d.Poles[2*i].Y
	}
	lifted, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		return out, e
	}
	patch, e := geom.NewSurfacePatch(lifted, o.Embedding)
	if e != nil {
		return out, e
	}
	wrapped, _, e := geom.NewWrappedSurfacePatch(ctx, patch, geom.WrappedSurfaceOptions{WrapU: true, MaxPoles: o.Embedding.MaxPoles, MaxOperations: o.Embedding.MaxOperations, MaxCoefficientBits: o.Embedding.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	profile, e := TessellateWrappedFrustumClosedCertified(ctx, wrapped, o)
	if e != nil {
		return out, e
	}
	if profile.Embedding.LowerRadius.Cmp(profile.Embedding.UpperRadius) != 0 || profile.Embedding.CenterX.Cmp(ratInset(apex.X)) != 0 || profile.Embedding.CenterY.Cmp(ratInset(apex.Y)) != 0 || profile.Embedding.UpperZ.Cmp(ratInset(apex.Z)) != 0 {
		return out, geom.ErrConvergence
	}
	out.Profile = profile
	out.SourceImageSquaredDistance = new(big.Rat).Set(profile.SourceImageSquaredDistance)
	n := len(profile.RimVertices[0])
	if n < 4 {
		return out, geom.ErrConvergence
	}
	for _, id := range profile.RimVertices[0] {
		out.RimVertices = append(out.RimVertices, len(out.Mesh.Vertices))
		out.Mesh.Vertices = append(out.Mesh.Vertices, profile.Mesh.Vertices[id])
	}
	out.ApexVertex = len(out.Mesh.Vertices)
	out.Mesh.Vertices = append(out.Mesh.Vertices, apex)
	lower, _ := profile.Embedding.LowerZ.Float64()
	out.BaseCenterVertex = len(out.Mesh.Vertices)
	out.Mesh.Vertices = append(out.Mesh.Vertices, geom.Vec3{X: apex.X, Y: apex.Y, Z: lower})
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		out.Mesh.Triangles = append(out.Mesh.Triangles, [3]int{i, j, out.ApexVertex}, [3]int{out.BaseCenterVertex, j, i})
		out.OriginalFace = append(out.OriginalFace, 0, 1)
		out.SquaredErrors = append(out.SquaredErrors, new(big.Rat).Set(out.SourceImageSquaredDistance), new(big.Rat).Set(out.SourceImageSquaredDistance))
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
	if !out.Collision.Complete || len(out.Collision.Contacts) > 0 {
		return out, geom.ErrConvergence
	}
	return out, nil
}
