package step

import (
	"bytes"
	"io"
	"math/big"
	"plainsolid/geom"
)

// OffAxisRimLiftAgreement bounds the distance between the two authored owner
// pcurve lifts at the same full-domain parameter. It does not certify exact
// closure, authored trim selection, rephased boundaries or material validity.
type OffAxisRimLiftAgreement struct {
	BodyID, EdgeID, ModelCurveID, SphereID, CylinderID                                      int64
	SpherePCurveID, CylinderPCurveID, SphereCurveID, CylinderCurveID                        int64
	ParameterStart, ParameterEnd                                                            *big.Rat
	SphereWorldCoordinateBounds, CylinderWorldCoordinateBounds                              [3]*big.Rat
	SphereSpans, CylinderSpans                                                              int
	WorldCoordinateBounds                                                                   [3]*big.Rat
	SphereModelSquaredBound, CylinderModelSquaredBound                                      *big.Rat
	DistanceSquaredUpper, Tolerance                                                         *big.Rat
	WithinTolerance, FullDomainSameParameter                                                bool
	AuthoredTrimSelectionProven, RepairedBoundaryAgreementProven, MaterialEquivalenceProven bool
}

// BoundOffAxisRimLiftAgreement composes independent continuous error bounds
// through one shared model curve: |S_i(t)-C_i(t)| <= |S_i(t)-M_i(t)| +
// |M_i(t)-C_i(t)|. Owner-local residual bounds are transformed to model-space
// coordinates before addition. Both source frames must be signed-coordinate
// orthonormal frames. Caller tolerance uses unconverted model length units;
// source uncertainty metadata is not an acceptance threshold. A false
// WithinTolerance is inconclusive, not proof that the curves disagree.
func BoundOffAxisRimLiftAgreement(r io.Reader, l Limits, bodyID int64, tolerance float64) (out OffAxisRimLiftAgreement, err error) {
	fail := func() (OffAxisRimLiftAgreement, error) { return OffAxisRimLiftAgreement{}, geom.ErrDomain }
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
	edgeID := top.Intersection.EdgeID
	cylinder, err := BoundCylinderPCurveLift(bytes.NewReader(raw), l, edgeID, tolerance)
	if err != nil {
		return out, err
	}
	sphere, err := BoundSpherePCurveLift(bytes.NewReader(raw), l, edgeID, tolerance)
	if err != nil {
		return out, err
	}
	// Both certifiers parse this same bounded byte buffer. Preserve and verify
	// the shared source identity explicitly rather than combining caller reports.
	a, b := cylinder.Source, sphere.Source
	if a.BodyID != bodyID || b.BodyID != bodyID || a.EdgeID != b.EdgeID || a.ModelCurveID != b.ModelCurveID || a.ParameterStart.Cmp(b.ParameterStart) != 0 || a.ParameterEnd.Cmp(b.ParameterEnd) != 0 {
		return fail()
	}
	f, err := Parse(bytes.NewReader(raw), l)
	if err != nil {
		return out, err
	}
	frame := func(id int64, kind string) ([3][3]*big.Rat, error) {
		var axes [3][3]*big.Rat
		support, e := simple(f, id, kind, 3)
		if e != nil {
			return axes, e
		}
		pid, e := ref(support.Args[1])
		if e != nil {
			return axes, e
		}
		_, axes, e = sphereLiftFrame(f, pid)
		return axes, e
	}
	sa, e := frame(sphere.SphereID, "SPHERICAL_SURFACE")
	if e != nil {
		return fail()
	}
	ca, e := frame(a.CylinderID, "CYLINDRICAL_SURFACE")
	if e != nil {
		return fail()
	}
	out = OffAxisRimLiftAgreement{BodyID: bodyID, EdgeID: edgeID, ModelCurveID: a.ModelCurveID, SphereID: sphere.SphereID, CylinderID: a.CylinderID, SpherePCurveID: sphere.PCurveID, CylinderPCurveID: a.PCurveID, SphereCurveID: sphere.ParameterCurveID, CylinderCurveID: a.ParameterCurveID, ParameterStart: new(big.Rat).Set(a.ParameterStart), ParameterEnd: new(big.Rat).Set(a.ParameterEnd), SphereModelSquaredBound: new(big.Rat).Set(sphere.DistanceSquaredUpper), CylinderModelSquaredBound: new(big.Rat).Set(cylinder.DistanceSquaredUpper), DistanceSquaredUpper: new(big.Rat), Tolerance: new(big.Rat).SetFloat64(tolerance), FullDomainSameParameter: true, SphereSpans: sphere.Spans, CylinderSpans: cylinder.Spans}
	for i := 0; i < 3; i++ {
		bound := new(big.Rat)
		sb, cb := new(big.Rat), new(big.Rat)
		for j := 0; j < 3; j++ {
			sb.Add(sb, new(big.Rat).Mul(new(big.Rat).Abs(sa[j][i]), sphere.CoordinateBounds[j]))
			cb.Add(cb, new(big.Rat).Mul(new(big.Rat).Abs(ca[j][i]), cylinder.CoordinateBounds[j]))
		}
		out.SphereWorldCoordinateBounds[i], out.CylinderWorldCoordinateBounds[i] = sb, cb
		bound.Add(sb, cb)
		out.WorldCoordinateBounds[i] = bound
		out.DistanceSquaredUpper.Add(out.DistanceSquaredUpper, new(big.Rat).Mul(bound, bound))
	}
	out.WithinTolerance = out.DistanceSquaredUpper.Cmp(new(big.Rat).Mul(out.Tolerance, out.Tolerance)) <= 0
	return out, nil
}
