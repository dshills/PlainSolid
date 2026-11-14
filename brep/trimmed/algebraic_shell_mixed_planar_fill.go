package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// MixedPlanarFragmentProposal names an original Boolean face and an exact raw
// plane through three represented noncollinear points. ProjectionAxes are two
// distinct model axes (0=X,1=Y,2=Z), not original UV coordinates. Both whole
// support incidence and global projected injectivity are independently proved.
type MixedPlanarFragmentProposal struct {
	Face           int
	Plane          [3]geom.Vec3
	ProjectionAxes [2]int
}

// MixedPlanarFragment retains the source face and unchanged shared model ring.
// Coordinates are physical model-axis coordinates, NEVER nonlinear original UV.
// OriginalUVResolved is always false; no approximate inverse is asserted.
// ProjectionReversed records the original chart-to-projection orientation.
// PlaneResidual is displacement along ResidualAxis from the exact source plane.
// AreaProjected is polygonal projection area; ErrorUpper is a two-sided source
// fill-distance bound, including all original boundary and display rounding.
type MixedPlanarFragment struct {
	Face, Operand, Original                int
	BoundaryVertices                       []int
	Coordinates                            [][2]*big.Rat
	ProjectionAxes                         [2]int
	ProjectionReversed, OriginalUVResolved bool
	PlaneResidual                          []*big.Rat
	ResidualAxis                           int
	AreaProjected, ErrorUpper              *big.Rat
	Triangles                              [][3]int
}

// CertifiedMixedPlanarFragmentFill is an immutable PARTIAL open fill with the
// same distance, indexed incidence and no-contact guarantees as its affine
// counterpart. Unfilled interiors are not tested or claimed filled. Original
// nonlinear UV inverses remain unresolved; all original source edge recipes and
// their complete model correspondence remain owned by the input network.
type CertifiedMixedPlanarFragmentFill struct {
	base *CertifiedMixedAffineFragmentFill
}

func (r *CertifiedMixedPlanarFragmentFill) Mesh() mesh.Mesh {
	if r == nil || r.base == nil {
		return mesh.Mesh{}
	}
	return r.base.Mesh()
}
func (r *CertifiedMixedPlanarFragmentFill) UnfilledFaces() []int {
	if r == nil || r.base == nil {
		return nil
	}
	return r.base.UnfilledFaces()
}
func (r *CertifiedMixedPlanarFragmentFill) Deflection() float64 {
	if r == nil || r.base == nil {
		return 0
	}
	return r.base.Deflection()
}
func (r *CertifiedMixedPlanarFragmentFill) Work() (int, int, int) {
	if r == nil || r.base == nil {
		return 0, 0, 0
	}
	return r.base.Work()
}
func (r *CertifiedMixedPlanarFragmentFill) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}
func (r *CertifiedMixedPlanarFragmentFill) Fragments() []MixedPlanarFragment {
	if r == nil || r.base == nil {
		return nil
	}
	raw := r.base.Fragments()
	out := make([]MixedPlanarFragment, len(raw))
	for i, f := range raw {
		out[i] = MixedPlanarFragment{Face: f.Face, Operand: f.Operand, Original: f.Original, BoundaryVertices: f.BoundaryVertices, Coordinates: f.UV, ProjectionAxes: f.projectionAxes, ProjectionReversed: f.projectionReversed, PlaneResidual: f.PlaneResidual, ResidualAxis: f.ResidualAxis, AreaProjected: f.AreaUV, ErrorUpper: f.ErrorUpper, Triangles: f.Triangles}
	}
	return out
}

// FillPlanarFragmentsCertified accepts nonaffine original charts only after
// whole-support planarity and global projected injectivity proofs. Budgets are
// cumulative, including both proofs and all retained network contacts. It does
// not loosen FillAffineFragmentsCertified or construct guessed UV inverses.
func (n *CertifiedMixedAlgebraicShellBoundary) FillPlanarFragmentsCertified(ctx context.Context, proposals []MixedPlanarFragmentProposal, o MixedAffineFragmentFillOptions) (*CertifiedMixedPlanarFragmentFill, error) {
	if n == nil || n.source == nil || len(proposals) == 0 || len(proposals) > len(n.source.boundary.faces) {
		return nil, geom.ErrInvalid
	}
	planes := map[int]MixedPlanarFragmentProposal{}
	faces := make([]int, len(proposals))
	for i, p := range proposals {
		if _, ok := planes[p.Face]; ok {
			return nil, geom.ErrInvalid
		}
		axes := p.ProjectionAxes
		if axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
			return nil, geom.ErrInvalid
		}
		for _, v := range p.Plane {
			if !v.IsFinite() {
				return nil, geom.ErrInvalid
			}
		}
		planes[p.Face] = p
		faces[i] = p.Face
	}
	base, e := n.fillMixedPlanarFragments(ctx, faces, o, planes)
	if e != nil {
		return nil, e
	}
	return &CertifiedMixedPlanarFragmentFill{base}, nil
}

type mixedPhysicalPlane struct {
	axes                    [2]int
	normal                  [3]*big.Rat
	offset                  *big.Rat
	orientation, operations int
}

func proveMixedPhysicalPlane(ctx context.Context, patch *geom.SurfacePatch, p MixedPlanarFragmentProposal, maxOperations int) (*mixedPhysicalPlane, error) {
	if maxOperations < 1 {
		return nil, geom.ErrConvergence
	}
	proof, e := geom.CertifySurfaceSupportingPlane(ctx, patch, p.Plane[0], p.Plane[1], p.Plane[2], geom.SurfaceSupportingPlaneOptions{MaxOperations: maxOperations, MaxCoefficientBits: 4096})
	if e != nil {
		return nil, e
	}
	if !proof.Complete || !proof.Coincident {
		return nil, geom.ErrUnrepresentable
	}
	remaining := maxOperations - proof.ExactOperations
	if remaining <= 64 {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	embedding, e := geom.CertifySurfaceInjectivity(patch, geom.SurfaceInjectivityOptions{ProjectionAxes: p.ProjectionAxes, MaxOperations: remaining - 64, MaxCoefficientBits: 4096})
	if e != nil {
		return nil, e
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if !embedding.ProvenInjective || embedding.ProjectionOrientation == 0 {
		return nil, geom.ErrConvergence
	}
	r := &mixedPhysicalPlane{axes: p.ProjectionAxes, orientation: embedding.ProjectionOrientation, operations: proof.ExactOperations + embedding.ExactOperations + 64, offset: new(big.Rat)}
	xyz := func(v geom.Vec3) [3]*big.Rat { return [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)} }
	a, b, c := xyz(p.Plane[0]), xyz(p.Plane[1]), xyz(p.Plane[2])
	for i := 0; i < 3; i++ {
		b[i].Sub(b[i], a[i])
		c[i].Sub(c[i], a[i])
	}
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		r.normal[i] = new(big.Rat).Sub(new(big.Rat).Mul(b[j], c[k]), new(big.Rat).Mul(b[k], c[j]))
		r.offset.Add(r.offset, new(big.Rat).Mul(r.normal[i], a[i]))
	}
	if r.normal[3-r.axes[0]-r.axes[1]].Sign() == 0 {
		return nil, geom.ErrDegenerate
	}
	return r, nil
}
