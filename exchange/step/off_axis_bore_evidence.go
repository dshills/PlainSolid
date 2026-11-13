package step

import (
	"bytes"
	"io"
	"math/big"

	"plainsolid/geom"
)

// OffAxisBoreEvidence bundles independent topology, one-turn and approximate
// lift proofs. WithinRequestedTolerance checks numerical curve/endpoint errors
// only; it does not certify face coverage, embedding or material equivalence.
type OffAxisBoreEvidence struct {
	Topology                 OffAxisTopologyEvidence
	Winding                  PolynomialWindingEvidence
	Cylinder                 CylinderLiftAgreement
	Sphere                   SphereLiftAgreement
	RimEndpointSquaredGaps   [2]*big.Rat
	WithinRequestedTolerance bool
}

// InspectOffAxisBoreEvidence currently requires the source cylinder's exact
// positive model-Z axis and positive model-X reference direction. The selected
// model spline must have one positive turn and exact closure. Entire source
// curve domains are checked; no trimmed-interval/material inference is made.
func InspectOffAxisBoreEvidence(r io.Reader, l Limits, bodyID int64, tolerance float64, o PolynomialWindingOptions) (out OffAxisBoreEvidence, err error) {
	if r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return out, geom.ErrDomain
	}
	l, e := l.defaults()
	if e != nil {
		return out, e
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return out, geom.ErrDomain
	}
	topology, e := ValidateOffAxisBoreTopology(bytes.NewReader(raw), l, bodyID)
	if e != nil {
		return out, e
	}
	source := topology.Intersection
	if !source.IncreasingU || source.DecreasingU {
		return out, geom.ErrDomain
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return out, e
	}
	cyl, e := simple(f, source.CylinderID, "CYLINDRICAL_SURFACE", 3)
	if e != nil {
		return out, e
	}
	pid, e := ref(cyl.Args[1])
	if e != nil {
		return out, e
	}
	origin, axes, e := sphereLiftFrame(f, pid)
	if e != nil {
		return out, e
	}
	for a := 0; a < 3; a++ {
		for c := 0; c < 3; c++ {
			want := int64(0)
			if a == c {
				want = 1
			}
			if axes[a][c].Cmp(big.NewRat(want, 1)) != 0 {
				return out, geom.ErrDomain
			}
		}
	}
	x, exactX := origin[0].Float64()
	y, exactY := origin[1].Float64()
	if !exactX || !exactY {
		return out, geom.ErrDomain
	}
	winding, e := BoundPolynomialSplineWinding(bytes.NewReader(raw), l, source.ModelCurveID, geom.Vec3{X: x, Y: y}, o)
	if e != nil {
		return out, e
	}
	if winding.Turns != 1 {
		return out, geom.ErrDomain
	}
	cylinder, e := BoundCylinderPCurveLift(bytes.NewReader(raw), l, source.EdgeID, tolerance)
	if e != nil {
		return out, e
	}
	sphere, e := BoundSpherePCurveLift(bytes.NewReader(raw), l, source.EdgeID, tolerance)
	if e != nil {
		return out, e
	}
	_, poles, _, e := readResidualPolynomial(bytes.NewReader(raw), l, source.ModelCurveID)
	if e != nil {
		return out, e
	}
	edge, e := simple(f, source.EdgeID, "EDGE_CURVE", 5)
	if e != nil {
		return out, e
	}
	out = OffAxisBoreEvidence{Topology: topology, Winding: winding, Cylinder: cylinder, Sphere: sphere, WithinRequestedTolerance: cylinder.WithinTolerance && sphere.WithinTolerance}
	for end := 0; end < 2; end++ {
		vid, e := ref(edge.Args[1+end])
		if e != nil {
			return OffAxisBoreEvidence{}, e
		}
		vertex, e := simple(f, vid, "VERTEX_POINT", 2)
		if e != nil {
			return OffAxisBoreEvidence{}, e
		}
		pointID, e := ref(vertex.Args[1])
		if e != nil {
			return OffAxisBoreEvidence{}, e
		}
		point, e := simple(f, pointID, "CARTESIAN_POINT", 2)
		if e != nil || len(point.Args[1].Items) != 3 {
			return OffAxisBoreEvidence{}, geom.ErrDomain
		}
		index := 0
		if end == 1 {
			index = len(poles) - 1
		}
		gap := new(big.Rat)
		for c, v := range point.Args[1].Items {
			q, e := liftSourceRat(v)
			if e != nil {
				return OffAxisBoreEvidence{}, e
			}
			d := new(big.Rat).Sub(q, poles[index][c])
			gap.Add(gap, new(big.Rat).Mul(d, d))
		}
		out.RimEndpointSquaredGaps[end] = gap
		tol := new(big.Rat).SetFloat64(tolerance)
		out.WithinRequestedTolerance = out.WithinRequestedTolerance && gap.Cmp(new(big.Rat).Mul(tol, tol)) <= 0
	}
	return out, nil
}
