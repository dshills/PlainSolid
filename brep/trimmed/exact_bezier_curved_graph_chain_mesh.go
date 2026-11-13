package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// CurvedBezierGraphChainOptions independently bounds the exact UV graph-chain
// proof and surface lifting. Interior.Deflection is the total requested model
// distance; Boundary.Display.Deflection is derived conservatively from it.
// Surface support is one positive rational span of degree<=2 in each axis.
type CurvedBezierGraphChainOptions struct {
	Boundary ExactBezierGraphFaceOptions
	Interior CurvedQuadraticTrimMeshOptions
}

// CertifiedCurvedBezierGraphChain bounds both directed distances from the
// original curved trimmed surface image to its displayed triangles. GraphChain
// is the source trim authority and contains the UV-plane seed mesh, not the
// lifted model mesh (legacy Boundary.Region source fields are empty).
// Original graph-chain boundary indices, piece indices and junctions remain
// unchanged: only interior edges are refined. The corresponding model samples
// are exact surface evaluations rounded once, with rounding in the residual.
// This is not an injectivity, embedding, cross-face sewing or normal certificate.
type CertifiedCurvedBezierGraphChain struct {
	CertifiedCurvedQuadraticTrimMesh
	GraphChain CertifiedExactBezierGraphChain
}

// TessellateCurvedBezierGraphChainCertified transports the proved UV fill
// correspondence by an exact global chart Lipschitz bound, then adds a proved
// two-sided common-parameter surface/triangle residual. Fixed boundary sampling
// may prevent convergence; that case rejects atomically, never silently inserts
// off-source seam samples or relaxes the requested error.
func TessellateCurvedBezierGraphChainCertified(ctx context.Context, p *geom.SurfacePatch, pieces []ExactBezierUV, baseline *big.Rat, o CurvedBezierGraphChainOptions) (CertifiedCurvedBezierGraphChain, error) {
	return tessellateCurvedBezierGraphChainCuts(ctx, p, pieces, baseline, nil, o)
}

func tessellateCurvedBezierGraphChainCuts(ctx context.Context, p *geom.SurfacePatch, pieces []ExactBezierUV, baseline *big.Rat, cuts [][]*big.Rat, o CurvedBezierGraphChainOptions) (CertifiedCurvedBezierGraphChain, error) {
	var empty CertifiedCurvedBezierGraphChain
	if ctx == nil || p == nil || !finite(o.Interior.Deflection) || o.Interior.Deflection <= 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	m := o.Interior
	if m.MaxSubdivisions == 0 {
		m.MaxSubdivisions = 32
	}
	if m.MaxTriangleChecks == 0 {
		m.MaxTriangleChecks = 100000
	}
	if m.MaxTriangles == 0 {
		m.MaxTriangles = 100000
	}
	if m.MaxSubdivisions < 1 || m.MaxSubdivisions > 128 || m.MaxSubdivisions&(m.MaxSubdivisions-1) != 0 || m.MaxTriangleChecks < 1 || m.MaxTriangles < 1 || m.MaxTriangles > 1000000 {
		return empty, geom.ErrInvalid
	}
	l2, l1, ok := quadraticPatchLipschitz(p)
	if !ok || l1.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	tolerance := new(big.Rat).Quo(ratInset(m.Deflection), new(big.Rat).Mul(big.NewRat(16, 1), l1))
	uvError, _ := tolerance.Float64()
	if !finite(uvError) || uvError <= 0 {
		return empty, geom.ErrConvergence
	}
	if ratInset(uvError).Cmp(tolerance) > 0 {
		uvError = math.Nextafter(uvError, 0)
	}
	if !finite(uvError) || uvError <= 0 {
		return empty, geom.ErrConvergence
	}
	d := p.Domain()
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: d.UMin, Y: d.VMin}, {X: d.UMin, Y: d.VMax}, {X: d.UMax, Y: d.VMin}, {X: d.UMax, Y: d.VMax}}, []float64{d.UMin, d.UMin, d.UMax, d.UMax}, []float64{d.VMin, d.VMin, d.VMax, d.VMax}, nil)
	if e != nil {
		return empty, e
	}
	identity, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		return empty, e
	}
	o.Boundary.Display.Deflection = uvError
	if o.Boundary.MaxTriangles == 0 || o.Boundary.MaxTriangles > m.MaxTriangles {
		o.Boundary.MaxTriangles = m.MaxTriangles
	}
	var chain CertifiedExactBezierGraphChain
	if cuts == nil {
		chain, e = TessellateExactBezierGraphChainCertified(ctx, identity, pieces, baseline, o.Boundary)
	} else {
		chain, e = TessellateExactBezierGraphChainWithCutsCertified(ctx, identity, pieces, baseline, cuts, o.Boundary)
	}
	if e != nil {
		return empty, e
	}
	boundary := CertifiedAffineQuadraticTrimMesh{Mesh: chain.Mesh, ExactUV: chain.ExactUV, ExactPoints: chain.ExactPoints, BoundaryLoops: [][]int{chain.Boundary}, Region: AffineQuadraticTrimRegionArea{FillSquaredDistance: chain.FillSquaredDistance}}
	lifted, e := tessellateCertifiedTrimBoundaryMode(ctx, p, boundary, m, 1, nil, l2, true)
	if e != nil {
		return empty, e
	}
	return CertifiedCurvedBezierGraphChain{CertifiedCurvedQuadraticTrimMesh: lifted, GraphChain: chain}, nil
}
