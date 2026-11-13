package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// MixedExactCurvedMeshOptions bounds the independent UV topology/chord proof
// and fixed-boundary surface lift. Interior.Deflection is the total requested
// model-space distance; Boundary.Display.Deflection is derived from that target.
type MixedExactCurvedMeshOptions struct {
	// RectangularGrid optionally replaces an exactly rectangular UV polygon
	// seed when opposite boundary cuts match. It preserves all source boundary
	// IDs and adds only interior vertices, with fresh exact coverage/roundoff.
	// A separately proved affine support keeps its cheaper polygon seed; that
	// additional chart proof uses Boundary.Chart limits independently.
	RectangularGrid bool
	// Cuts optionally prescribes original normalized pcurve parameters, outer
	// then holes, retaining all coedge occurrences. Non-nil per-trim lists
	// include 0 and 1 in increasing order; nil leaves that trim adaptive.
	// At most2048 total occurrences are allowed. Additional adaptive samples
	// may be generated. Original hole direction is preserved by the mapping.
	Cuts     [][][]*big.Rat
	Boundary MixedExactAffineMeshOptions
	Interior CurvedQuadraticTrimMeshOptions
}

// CertifiedMixedExactCurvedMesh preserves UVRegion sample and vertex indices.
// UVRegion.Mesh is the identity-UV polygon seed, not the model-space display.
// The embedded Mesh and ExactPoints contain the lifted model geometry. The
// proved distance applies in both directions between the trimmed surface image
// and triangle union; no injectivity, shell or embeddedness claim is made.
type CertifiedMixedExactCurvedMesh struct {
	RectangularGrid bool
	CertifiedCurvedQuadraticTrimMesh
	UVRegion CertifiedMixedExactAffineMesh
}

// TessellateMixedExactCurvedRegionCertified accepts one positive-weight rational
// spline span with degrees at most two in each axis. Exact mixed native trim
// controls drive source topology and chord fill proofs on an identity UV chart.
// A global exact chart Lipschitz bound transports their two-sided fill distance;
// exact common-parameter surface/triangle residuals and display rounding are
// added independently. Only interior edges refine: insufficient fixed boundary
// sampling rejects atomically, without moving samples off their source trims.
func TessellateMixedExactCurvedRegionCertified(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactCurvedMeshOptions) (CertifiedMixedExactCurvedMesh, error) {
	return tessellateMixedExactCurvedRegion(ctx, p, outer, holes, o, nil)
}

func tessellateMixedExactCurvedRegion(ctx context.Context, p *geom.SurfacePatch, outer []PCurve, holes [][]PCurve, o MixedExactCurvedMeshOptions, spans *multiSpanMeshWork) (CertifiedMixedExactCurvedMesh, error) {
	var empty CertifiedMixedExactCurvedMesh
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
	var l2, l1 *big.Rat
	if spans == nil {
		var ok bool
		l2, l1, ok = quadraticPatchLipschitz(p)
		if !ok {
			return empty, geom.ErrConvergence
		}
	} else {
		var err error
		l2, l1, err = spans.lipschitz(ctx, p)
		if err != nil {
			return empty, err
		}
	}
	if l1.Sign() == 0 {
		return empty, geom.ErrConvergence
	}
	tol := new(big.Rat).Quo(ratInset(m.Deflection), new(big.Rat).Mul(big.NewRat(16, 1), l1))
	uvError, _ := tol.Float64()
	if !finite(uvError) || uvError <= 0 {
		return empty, geom.ErrConvergence
	}
	if ratInset(uvError).Cmp(tol) > 0 {
		uvError = math.Nextafter(uvError, 0)
	}
	if uvError <= 0 {
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
	uv, e := tessellateMixedExactAffineRegionCuts(ctx, identity, outer, holes, o.Cuts, o.Boundary)
	if e != nil {
		return empty, e
	}
	grid := false
	if o.RectangularGrid {
		_, affineError := geom.NewAffineSurfaceChart(ctx, p, o.Boundary.Chart)
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if affineError != nil {
			var seed CertifiedAffineQuadraticTrimMesh
			seed, grid, e = mixedRectangleGridSeed(ctx, uv.CertifiedAffineQuadraticTrimMesh, m.MaxTriangles)
			if e != nil {
				return empty, e
			}
			uv.CertifiedAffineQuadraticTrimMesh = seed
		}
	}
	lifted, e := tessellateCertifiedTrimBoundaryMode(ctx, p, uv.CertifiedAffineQuadraticTrimMesh, m, 1, spans, l2, true)
	if e != nil {
		return empty, e
	}
	return CertifiedMixedExactCurvedMesh{CertifiedCurvedQuadraticTrimMesh: lifted, UVRegion: uv, RectangularGrid: grid}, nil
}
