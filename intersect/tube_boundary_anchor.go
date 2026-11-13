package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// TubeBoundaryAnchor contains a represented exact point in both charts at
// one free-coordinate endpoint of a tube. Rebuilding always verifies the
// actual original support equality and the inward continuation proof.
type TubeBoundaryAnchor struct {
	AtUpper    bool
	Parameters [4]float64
}

func cloneTubeBoundaryAnchor(a *TubeBoundaryAnchor) *TubeBoundaryAnchor {
	if a == nil {
		return nil
	}
	v := *a
	return &v
}

func (a *TubeBoundaryAnchor) exact(o geom.SurfacePairTubeOptions) (geom.SurfaceTubeBoundaryOptions, error) {
	r := geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: o, AtUpper: a.AtUpper}
	for i, v := range a.Parameters {
		if !finite(v) {
			return geom.SurfaceTubeBoundaryOptions{}, geom.ErrInvalid
		}
		r.Anchor[i] = new(big.Rat).SetFloat64(v)
	}
	return r, nil
}

func verifySplineTubeCell(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfacePairTubeOptions, anchor *TubeBoundaryAnchor) (geom.VerifiedSurfaceTube, error) {
	if anchor == nil {
		return geom.NewVerifiedSurfaceTube(a, b, o)
	}
	x, e := anchor.exact(o)
	if e != nil {
		return geom.VerifiedSurfaceTube{}, e
	}
	return geom.NewBoundaryAnchoredSurfaceTube(ctx, a, b, x)
}

func certifySplineTubeCell(ctx context.Context, a, b *geom.SurfacePatch, o geom.SurfacePairTubeOptions, anchor *TubeBoundaryAnchor) (geom.SurfaceTubeCertificate, error) {
	if anchor == nil {
		return geom.CertifySurfacePairTube(a, b, o)
	}
	x, e := anchor.exact(o)
	if e != nil {
		return geom.SurfaceTubeCertificate{}, e
	}
	return geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, a, b, x)
}
