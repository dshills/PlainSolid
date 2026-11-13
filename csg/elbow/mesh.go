package elbow

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimmed"
)

// CertifiedDisplay retains the exact ideal volume coefficient and the existing
// two-sided affine-torus display certificate. VolumePiCoefficient describes
// ideal material; Mesh.SignedDisplayVolume describes represented triangles.
// Neither is silently substituted for the other's volume.
type CertifiedDisplay struct {
	Mesh                trimmed.CertifiedRevolvedCircleMesh
	VolumePiCoefficient *big.Rat
}

// TessellateCertified uses the immutable retained chart and positive chart
// angle. Negative NewSigned segments already reversed their frame Y/Z axes;
// applying the original negative angle here would incorrectly reverse twice.
// Native cap sewing remains numerical, while the certificate targets exactly
// the same represented-affine ideal material as VolumePiCoefficient.
func (s Segment) TessellateCertified(ctx context.Context, o trimmed.CertifiedRevolvePolygonOptions) (CertifiedDisplay, error) {
	if ctx == nil || s.angle == 0 {
		return CertifiedDisplay{}, ErrInvalid
	}
	result, e := trimmed.RevolveCircleCertified(ctx, s.frame, s.major, 0, s.minor, math.Abs(s.angle), o)
	if e != nil {
		return CertifiedDisplay{}, e
	}
	if result.FrameDeterminant.Cmp(s.determinant()) != 0 {
		return CertifiedDisplay{}, ErrUnresolved
	}
	v, e := s.VolumePiCoefficient()
	if e != nil {
		return CertifiedDisplay{}, e
	}
	return CertifiedDisplay{Mesh: result, VolumePiCoefficient: v}, nil
}

// CertifiedContinuation regenerates one exterior shell; the common meridian
// between the two input angular intervals is an internal material interface,
// never an additional cap or coincident pair of display faces.
type CertifiedContinuation struct {
	Continuation Continuation
	Display      CertifiedDisplay
}

func ContinueCertified(ctx context.Context, s Segment, delta float64, o trimmed.CertifiedRevolvePolygonOptions) (CertifiedContinuation, error) {
	joined, e := Continue(ctx, s, delta)
	if e != nil {
		return CertifiedContinuation{}, e
	}
	display, e := joined.Union.TessellateCertified(ctx, o)
	if e != nil {
		return CertifiedContinuation{}, e
	}
	return CertifiedContinuation{Continuation: joined, Display: display}, nil
}
