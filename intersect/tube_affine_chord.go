package intersect

import (
	"context"
	"plainsolid/geom"
)

// CertifyAffineChord proposes the represented paired endpoints of this trim's
// underlying segment and independently certifies their complete affine UV
// path and affine model image. Rounded endpoint proposals never suffice by
// themselves: Proven requires exact polynomial identities and tube uniqueness.
// The endpoints follow the segment's traversal, before external trim reversal.
func (p TubeTrim) CertifyAffineChord(ctx context.Context, o geom.TubeAffineLineOptions) (geom.TubeAffineLineCertificate, error) {
	if ctx == nil {
		return geom.TubeAffineLineCertificate{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return geom.TubeAffineLineCertificate{}, e
	}
	a, e := p.segment.EvaluatePair(0)
	if e != nil {
		return geom.TubeAffineLineCertificate{}, e
	}
	b, e := p.segment.EvaluatePair(1)
	if e != nil {
		return geom.TubeAffineLineCertificate{}, e
	}
	return p.segment.CertifyAffineLine(ctx, Parameters{A: a.A, B: a.B}, Parameters{A: b.A, B: b.B}, o)
}
