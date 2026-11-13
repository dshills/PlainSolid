package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// SphericalRephaseCoverage describes an explicit theoretical repair of source
// parameter curves, not acceptance or reconstruction of the original STEP face.
// The repaired disk includes the south pole and excludes the north pole.
type SphericalRephaseCoverage struct {
	BodyID, FaceID, SphereID, RimPCurveID, RimCurveID, PoleVertexID int64
	DownwardSeamPCurveID, UpwardSeamPCurveID                        int64
	SourceWidth, LatitudeLower, LatitudeUpper                       *big.Rat
	LongitudeDisplacementUpper, PoleDisplacementSquared, Tolerance  *big.Rat
	RepairedDiskCoverage                                            bool
	AssumesFullRimAndCornerSegments                                 bool
	ModelMeridianAgreementProven, SourceMaterialEquivalenceProven   bool
}

// CertifyOffAxisSphericalRephase proves coverage after replacing source U by
// 2*pi*U/W and replacing the source pole vertex with the exact south pole.
// Source U must start at zero, increase strictly, and end at W; endpoint
// latitudes must agree. Both positional corrections must fit tolerance in model
// units. Full rim and meridian corner-to-pole interval selection is assumed.
// The source model meridian curve is not certified by this operation.
func CertifyOffAxisSphericalRephase(r io.Reader, l Limits, bodyID int64, tolerance float64) (out SphericalRephaseCoverage, err error) {
	fail := func() (SphericalRephaseCoverage, error) { return SphericalRephaseCoverage{}, geom.ErrDomain }
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return fail()
	}
	l, err = l.defaults()
	if err != nil {
		return out, err
	}
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return out, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	top, err := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	seams, err := InspectSphericalSeamPCurves(bytes.NewReader(raw), l, bodyID)
	if err != nil {
		return out, err
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return out, err
	}
	var face OffAxisTopologyFace
	for _, x := range top.Faces {
		if x.SurfaceKind == "SPHERICAL_SURFACE" {
			face = x
		}
	}
	if !face.SameSense || len(face.Bounds) != 1 || len(face.Bounds[0].Uses) != 3 {
		return fail()
	}
	var pcID, curveID int64
	for _, e := range top.EdgeCurves {
		if e.EdgeID == top.Intersection.EdgeID {
			for i, s := range e.SupportIDs {
				if s == face.SurfaceID {
					pcID = e.PCurveIDs[i]
					curveID = e.ParameterCurveIDs[i]
				}
			}
		}
	}
	if pcID == 0 {
		return fail()
	}
	_, poles, _, err := readResidualPolynomialDimension(bytes.NewReader(raw), l, curveID, 2)
	if err != nil {
		return out, err
	}
	if len(poles) < 2 || poles[0][0].Sign() != 0 || poles[0][1].Cmp(poles[len(poles)-1][1]) != 0 {
		return fail()
	}
	width := new(big.Rat).Set(poles[len(poles)-1][0])
	if width.Sign() <= 0 {
		return fail()
	}
	piLo, piHi := pcurvePiBounds()
	halfLo := new(big.Rat).Quo(piLo, big.NewRat(2, 1))
	minusHalfLo := new(big.Rat).Neg(halfLo)
	low, high := new(big.Rat).Set(poles[0][1]), new(big.Rat).Set(poles[0][1])
	for i, p := range poles {
		if p[1].Cmp(minusHalfLo) <= 0 || p[1].Cmp(halfLo) >= 0 {
			return fail()
		}
		if i > 0 && p[0].Cmp(poles[i-1][0]) <= 0 {
			return fail()
		}
		if p[1].Cmp(low) < 0 {
			low.Set(p[1])
		}
		if p[1].Cmp(high) > 0 {
			high.Set(p[1])
		}
	}
	uses := face.Bounds[0].Uses
	at := -1
	for i, u := range uses {
		if u.EdgeID == top.Intersection.EdgeID {
			at = i
		}
	}
	if at < 0 {
		return fail()
	}
	rim, down, up := uses[at], uses[(at+1)%3], uses[(at+2)%3]
	if rim.Forward || down.EdgeID != seams.EdgeID || down.Forward || up.EdgeID != seams.EdgeID || !up.Forward || down.EndVertex != up.StartVertex {
		return fail()
	}
	var downID, upID int64
	for _, b := range seams.Branches {
		if b.Direction[1].Sign() <= 0 {
			return fail()
		}
		if b.Origin[0].Sign() == 0 {
			downID = b.PCurveID
		} else if b.Origin[0].Cmp(width) == 0 {
			upID = b.PCurveID
		} else {
			return fail()
		}
	}
	if downID == 0 || upID == 0 || downID == upID {
		return fail()
	}
	sphere, e := simple(f, face.SurfaceID, "SPHERICAL_SURFACE", 3)
	if e != nil {
		return fail()
	}
	radius, e := liftSourceRat(sphere.Args[2])
	if e != nil || radius.Sign() <= 0 {
		return fail()
	}
	frameID, e := ref(sphere.Args[1])
	if e != nil {
		return fail()
	}
	center, axes, e := sphereLiftFrame(f, frameID)
	if e != nil {
		return fail()
	}
	vertex, e := simple(f, down.EndVertex, "VERTEX_POINT", 2)
	if e != nil {
		return fail()
	}
	pointID, e := ref(vertex.Args[1])
	if e != nil {
		return fail()
	}
	point, e := simple(f, pointID, "CARTESIAN_POINT", 2)
	if e != nil || point.Args[1].Kind != Aggregate || len(point.Args[1].Items) != 3 {
		return fail()
	}
	gap := new(big.Rat)
	for i, v := range point.Args[1].Items {
		q, e := liftSourceRat(v)
		if e != nil {
			return fail()
		}
		target := new(big.Rat).Sub(center[i], new(big.Rat).Mul(radius, axes[2][i]))
		d := new(big.Rat).Sub(q, target)
		gap.Add(gap, new(big.Rat).Mul(d, d))
	}
	defectLo := new(big.Rat).Abs(new(big.Rat).Sub(width, new(big.Rat).Mul(big.NewRat(2, 1), piLo)))
	defectHi := new(big.Rat).Abs(new(big.Rat).Sub(width, new(big.Rat).Mul(big.NewRat(2, 1), piHi)))
	if defectHi.Cmp(defectLo) > 0 {
		defectLo = defectHi
	}
	displacement := new(big.Rat).Mul(radius, defectLo)
	tol := new(big.Rat).SetFloat64(tolerance)
	if displacement.Cmp(tol) > 0 || gap.Cmp(new(big.Rat).Mul(tol, tol)) > 0 {
		return fail()
	}
	out = SphericalRephaseCoverage{BodyID: bodyID, FaceID: face.FaceID, SphereID: face.SurfaceID, RimPCurveID: pcID, RimCurveID: curveID, PoleVertexID: down.EndVertex, DownwardSeamPCurveID: downID, UpwardSeamPCurveID: upID, SourceWidth: width, LatitudeLower: low, LatitudeUpper: high, LongitudeDisplacementUpper: displacement, PoleDisplacementSquared: gap, Tolerance: tol, RepairedDiskCoverage: true, AssumesFullRimAndCornerSegments: true}
	return out, nil
}
