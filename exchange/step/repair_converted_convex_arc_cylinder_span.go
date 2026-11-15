package step

import (
	"math/big"

	"plainsolid/geom"
)

type convertedConvexArcLength struct {
	ownerScale *big.Rat // exact source length per millimetre
	modelSpeed *big.Rat // exact original LINE magnitude token
}

// ConvexArcCylinderSpanOwner records an original clamped degree-one UV owner.
// Its two controls define UVStart + (t-ParameterInterval[0])*UVDerivative
// on ParameterInterval.
type ConvexArcCylinderSpanOwner struct {
	PCurveID          int64
	ParameterInterval [2]*big.Rat
	UVStart           [2]*big.Rat
	UVDerivative      [2]*big.Rat
	// RepairedToOwner is [offset, scale] for the finite affine map from
	// repaired model parameter to original spline parameter, when needed.
	RepairedToOwner *[2]*big.Rat `json:",omitempty"`
	// ClippedOwnerParameters are the actual directed stations after bounding
	// an original model projection that falls outside the authored knots.
	ClippedOwnerParameters *[2]*big.Rat `json:",omitempty"`
	// ModelAngleBounds encloses the two directed model angles independently.
	// When present, normalized travel pairs the model arc with the full knots.
	ModelAngleBounds *[2][2]*big.Rat `json:",omitempty"`
}

func (m RationalPosedPCurveRepairedOCCTConvexArcExtrusion) CylindricalSpanOwners() []ConvexArcCylinderSpanOwner {
	return copyConvexArcCylinderSpanOwners(m.spans)
}

func copyConvexArcCylinderSpanOwners(spans []ConvexArcCylinderSpanOwner) []ConvexArcCylinderSpanOwner {
	out := append([]ConvexArcCylinderSpanOwner(nil), spans...)
	for i := range out {
		for j := range out[i].UVStart {
			out[i].ParameterInterval[j] = new(big.Rat).Set(out[i].ParameterInterval[j])
			out[i].UVStart[j] = new(big.Rat).Set(out[i].UVStart[j])
			out[i].UVDerivative[j] = new(big.Rat).Set(out[i].UVDerivative[j])
		}
		if mapping := out[i].RepairedToOwner; mapping != nil {
			out[i].RepairedToOwner = &[2]*big.Rat{new(big.Rat).Set(mapping[0]), new(big.Rat).Set(mapping[1])}
		}
		if clipped := out[i].ClippedOwnerParameters; clipped != nil {
			out[i].ClippedOwnerParameters = &[2]*big.Rat{new(big.Rat).Set(clipped[0]), new(big.Rat).Set(clipped[1])}
		}
		if bounds := out[i].ModelAngleBounds; bounds != nil {
			out[i].ModelAngleBounds = new([2][2]*big.Rat)
			for j, interval := range bounds {
				out[i].ModelAngleBounds[j] = [2]*big.Rat{new(big.Rat).Set(interval[0]), new(big.Rat).Set(interval[1])}
			}
		}
	}
	return out
}

func readConvexArcCylinderSpan(f *File, q ConvexArcPCurveAgreement) (ConvexArcCylinderSpanOwner, error) {
	var zero ConvexArcCylinderSpanOwner
	r, err := simple(f, q.GeometryID, "B_SPLINE_CURVE_WITH_KNOTS", 9)
	if err != nil || r.Args[0].Kind != String || r.Args[3].Kind != Enumeration || r.Args[3].Text != "UNSPECIFIED" ||
		r.Args[8].Kind != Enumeration || r.Args[8].Text != "PIECEWISE_BEZIER_KNOTS" {
		return zero, geom.ErrDomain
	}
	degree, err := liftSourceRat(r.Args[1])
	if err != nil || degree.Cmp(big.NewRat(1, 1)) != 0 {
		return zero, geom.ErrDomain
	}
	for _, flag := range r.Args[4:6] {
		value, err := boolean(flag)
		if err != nil || value {
			return zero, geom.ErrDomain
		}
	}
	controls, err := listRefs(r.Args[2], 2)
	if err != nil || len(controls) != 2 || r.Args[6].Kind != Aggregate || len(r.Args[6].Items) != 2 ||
		r.Args[7].Kind != Aggregate || len(r.Args[7].Items) != 2 {
		return zero, geom.ErrDomain
	}
	out := ConvexArcCylinderSpanOwner{PCurveID: q.PCurveID}
	var uv [2][]*big.Rat
	for i, id := range controls {
		mult, err := liftSourceRat(r.Args[6].Items[i])
		if err != nil || mult.Cmp(big.NewRat(2, 1)) != 0 {
			return zero, geom.ErrDomain
		}
		knot, err := liftSourceRat(r.Args[7].Items[i])
		if err != nil || knot.Num().BitLen() > 256 || knot.Denom().BitLen() > 256 {
			return zero, geom.ErrDomain
		}
		out.ParameterInterval[i] = knot
		uv[i], err = convexUVVector(f, id, "CARTESIAN_POINT", 2)
		if err != nil {
			return zero, err
		}
	}
	lo, hi := out.ParameterInterval[0], out.ParameterInterval[1]
	if lo.Sign() < 0 || hi.Cmp(lo) <= 0 {
		return zero, geom.ErrDomain
	}
	width := new(big.Rat).Sub(hi, lo)
	for i := range out.UVStart {
		out.UVStart[i] = uv[0][i]
		out.UVDerivative[i] = new(big.Rat).Quo(new(big.Rat).Sub(uv[1][i], uv[0][i]), width)
	}
	return out, nil
}

func boundConvertedConvexArcCylinderSpan(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat, conversion *convertedConvexArcLength, trim *ConvexArcAxialTrimRepair, reversal *ConvexArcRepairReversal, segments int) (ConvexArcCylinderSpanOwner, string, *big.Rat, error) {
	var zero ConvexArcCylinderSpanOwner
	span, err := readConvexArcCylinderSpan(src, q)
	if err != nil {
		return zero, "", nil, err
	}
	surface, err := simple(src, q.SupportID, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		return zero, "", nil, err
	}
	o, x, y, z, err := convexUVFrame(src, surface.Args[1].Ref)
	if err != nil {
		return zero, "", nil, err
	}
	radius, err := liftSourceRat(surface.Args[2])
	if err != nil || radius.Sign() <= 0 {
		return zero, "", nil, geom.ErrDomain
	}
	curve := src.Entities[model].Records
	if len(curve) != 1 || len(curve[0].Args) != 3 {
		return zero, "", nil, geom.ErrDomain
	}
	uv, slope := span.UVStart, span.UVDerivative
	switch curve[0].Type {
	case "LINE":
		p, d, intervals, err := convexArcOwnerLineIntervals(src, corrected, q, model, tolSquared, trim)
		if err != nil || span.ParameterInterval[0].Sign() != 0 || slope[0].Sign() != 0 || slope[1].Cmp(conversion.ownerScale) != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		original, repaired := intervals[0], intervals[1]
		owner := original
		clipped := false
		for i, t := range original {
			switch {
			case t.Cmp(span.ParameterInterval[0]) < 0:
				owner[i] = span.ParameterInterval[0]
			case t.Cmp(span.ParameterInterval[1]) > 0:
				owner[i] = span.ParameterInterval[1]
			default:
				continue
			}
			excess := new(big.Rat).Sub(t, owner[i])
			excess.Mul(excess, conversion.modelSpeed)
			if (segments != 8 && segments != 10) || trim == nil || excess.Mul(excess, excess).Cmp(tolSquared) > 0 {
				return zero, "", nil, geom.ErrDomain
			}
			clipped = true
		}
		if owner[1].Cmp(owner[0]) != original[1].Cmp(original[0]) {
			return zero, "", nil, geom.ErrDomain
		}
		for i := range d {
			if d[i].Cmp(new(big.Rat).Mul(z[i], conversion.modelSpeed)) != 0 {
				return zero, "", nil, geom.ErrDomain
			}
		}
		if trim != nil {
			// Map corresponding endpoints, preserving direction inside the
			// authored knots. The trim retains the original LINE stations.
			scale := new(big.Rat).Quo(new(big.Rat).Sub(owner[1], owner[0]), new(big.Rat).Sub(repaired[1], repaired[0]))
			offset := new(big.Rat).Sub(owner[0], new(big.Rat).Mul(scale, repaired[0]))
			span.RepairedToOwner = &[2]*big.Rat{offset, scale}
		}
		var originalPoint []*big.Rat
		if clipped {
			span.ClippedOwnerParameters = &owner
			originalPoint, _, err = convexUVLine(src, model, 3)
			if err != nil {
				return zero, "", nil, err
			}
		}
		// With constant u, the difference between the traversed spline and
		// either LINE is affine. Its squared norm is bounded at endpoints.
		upper := new(big.Rat)
		for i, t := range owner {
			v := new(big.Rat).Add(uv[1], new(big.Rat).Mul(slope[1], t))
			repairedV := new(big.Rat).Sub(v, new(big.Rat).Mul(conversion.modelSpeed, repaired[i]))
			bound, err := boundConvexArcCylinderLine(o, x, y, z, p, radius, uv[0], repairedV)
			if err != nil {
				return zero, "", nil, err
			}
			if bound.Cmp(upper) > 0 {
				upper = bound
			}
			if clipped {
				originalV := new(big.Rat).Sub(v, new(big.Rat).Mul(conversion.modelSpeed, original[i]))
				bound, err = boundConvexArcCylinderLine(o, x, y, z, [3]*big.Rat(originalPoint), radius, uv[0], originalV)
				if err != nil {
					return zero, "", nil, err
				}
				if bound.Cmp(upper) > 0 {
					upper = bound
				}
			}
		}
		return span, "finite-cylinder-line", upper, nil
	case "CIRCLE":
		allowQuarter := segments == 8 || segments == 10
		mo, mx, my, mz, err := convexUVFrame(src, curve[0].Args[1].Ref)
		increment, kind := big.NewRat(1, 1), "finite-cylinder-circle"
		if reversal != nil {
			increment, kind = big.NewRat(-1, 1), "finite-reversed-cylinder-circle"
		}
		if err != nil || slope[1].Sign() != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		r, err := liftSourceRat(curve[0].Args[2])
		if err != nil || r.Cmp(radius) != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		var center [3]*big.Rat
		for i := range o {
			center[i] = new(big.Rat).Add(o[i], new(big.Rat).Mul(z[i], uv[1]))
		}
		delta := occtLoopSub(center, mo)
		if span.ParameterInterval[0].Sign() != 0 {
			if (segments != 6 && !allowQuarter) || reversal != nil || !convexUVSame(z, mz) {
				return zero, "", nil, geom.ErrDomain
			}
			return boundShiftedConvexArcCylinderSpan(src, corrected, q, model, span, delta, x, y, mo, mx, my, mz, radius, tolSquared, allowQuarter, segments == 10)
		}
		if uv[0].Sign() != 0 || slope[0].Cmp(increment) != 0 || !convexUVSame(x, mx) {
			return zero, "", nil, geom.ErrDomain
		}
		for i := range y {
			if my[i].Cmp(new(big.Rat).Mul(y[i], increment)) != 0 || mz[i].Cmp(new(big.Rat).Mul(z[i], increment)) != 0 {
				return zero, "", nil, geom.ErrDomain
			}
		}
		if err := coverConvexArcMinorCircleSpan(src, corrected, q, model, mo, mx, my, mz, span.ParameterInterval[1], tolSquared, reversal, allowQuarter); err != nil {
			return zero, "", nil, err
		}
		return span, kind, occtLoopDot(delta, delta), nil
	default:
		return zero, "", nil, geom.ErrDomain
	}
}

// Pair the original spline knots and the model's true endpoint angles by
// the same normalized arc fraction. A rational enclosure proves that the
// two parameters differ by at most angularError throughout both intervals.
func boundShiftedConvexArcCylinderSpan(src, corrected *File, q ConvexArcPCurveAgreement, model int64, span ConvexArcCylinderSpanOwner,
	delta, x, y, origin, mx, my, mz [3]*big.Rat, radius, tolSquared *big.Rat, allowQuarter, pentagonal bool) (ConvexArcCylinderSpanOwner, string, *big.Rat, error) {
	var zero ConvexArcCylinderSpanOwner
	angularError := big.NewRat(1, 1000000000000)
	lo, hi := span.ParameterInterval[0], span.ParameterInterval[1]
	u, speed := span.UVStart[0], span.UVDerivative[0]
	one := big.NewRat(1, 1)
	if speed.Sign() <= 0 || (!allowQuarter && (u.Sign() != 0 || speed.Cmp(one) != 0)) {
		return zero, "", nil, geom.ErrDomain
	}
	// The authored linear UV chart can differ from u=t-lo. Its maximum
	// angular deviation from that law occurs at one of the authored knots.
	width := new(big.Rat).Sub(hi, lo)
	startError := new(big.Rat).Abs(u)
	endError := new(big.Rat).Sub(speed, one)
	endError.Mul(endError, width).Add(endError, u).Abs(endError)
	uvError := startError
	if endError.Cmp(startError) > 0 {
		uvError = endError
	}
	if uvError.Cmp(angularError) > 0 {
		return zero, "", nil, geom.ErrDomain
	}

	// The stored knots select a candidate branch. The exact model rays
	// and their enclosures below certify both endpoints of that branch.
	minimum := [2]int64{2, 4}
	raySigns := [2][2]int{{-1, 1}, {0, -1}}
	switch {
	case allowQuarter && lo.Cmp(big.NewRat(2, 1)) < 0:
		minimum = [2]int64{1, 2}
		raySigns = [2][2]int{{0, 1}, {-1, 1}}
	case pentagonal && lo.Cmp(big.NewRat(3, 1)) > 0:
		// A positive quarter starts on the negative cosine axis in the
		// model frame. The original cylinder reference is antipodal.
		if occtLoopDot(x, mx).Cmp(big.NewRat(-1, 1)) != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		minimum = [2]int64{3, 4}
		raySigns = [2][2]int{{-1, 0}, {0, -1}}
	case pentagonal && hi.Cmp(big.NewRat(4, 1)) < 0:
		minimum = [2]int64{2, 3}
		raySigns = [2][2]int{{-1, 1}, {-1, 0}}
	}
	var bounds [2][2]*big.Rat
	type trig struct{ sine, cosine, remainder *big.Rat }
	var enclosures [2][2]trig
	for i, knot := range span.ParameterInterval {
		interval := [2]*big.Rat{new(big.Rat).Sub(knot, angularError), new(big.Rat).Add(knot, angularError)}
		if interval[0].Cmp(big.NewRat(minimum[i], 1)) < 0 || interval[1].Cmp(big.NewRat(minimum[i]+1, 1)) > 0 {
			return zero, "", nil, geom.ErrDomain
		}
		bounds[i] = interval
		for j, angle := range interval {
			sine, cosine, remainder, err := liftTrigBernstein([]*big.Rat{angle, angle})
			if err != nil {
				return zero, "", nil, err
			}
			enclosures[i][j] = trig{sine[0], cosine[0], remainder}
		}
	}
	var vertices [2]int64
	for _, f := range []*File{src, corrected} {
		edge, err := simple(f, q.EdgeID, "EDGE_CURVE", 5)
		if err != nil {
			return zero, "", nil, err
		}
		link := model
		if f == src {
			link = q.SurfaceCurveID
			vertices = [2]int64{edge.Args[1].Ref, edge.Args[2].Ref}
		}
		sense, err := boolean(edge.Args[4])
		if err != nil || !sense || edge.Args[3].Ref != link || vertices != [2]int64{edge.Args[1].Ref, edge.Args[2].Ref} {
			return zero, "", nil, geom.ErrDomain
		}
		for i, id := range vertices {
			vertex, err := simple(f, id, "VERTEX_POINT", 2)
			if err != nil {
				return zero, "", nil, err
			}
			point, err := occtLoopTriple(f, vertex.Args[1].Ref, "CARTESIAN_POINT")
			if err != nil {
				return zero, "", nil, err
			}
			ray := occtLoopSub(point, origin)
			a, b, axial := occtLoopDot(ray, mx), occtLoopDot(ray, my), occtLoopDot(ray, mz)
			if axial.Sign() != 0 && (f == corrected || axial.Mul(axial, axial).Cmp(tolSquared) > 0) {
				return zero, "", nil, geom.ErrDomain
			}
			if [2]int{a.Sign(), b.Sign()} != raySigns[i] {
				return zero, "", nil, geom.ErrDomain
			}
			for j, enclosure := range enclosures[i] {
				cross := new(big.Rat).Sub(new(big.Rat).Mul(a, enclosure.sine), new(big.Rat).Mul(b, enclosure.cosine))
				errorBound := new(big.Rat).Add(new(big.Rat).Abs(a), new(big.Rat).Abs(b))
				errorBound.Mul(errorBound, enclosure.remainder)
				if j == 0 {
					cross.Neg(cross)
				}
				if cross.Cmp(errorBound) < 0 {
					return zero, "", nil, geom.ErrDomain
				}
			}
		}
	}
	phase := new(big.Rat).Neg(span.ParameterInterval[0])
	bound, err := boundGroupedCylinderPhase(delta, x, y, mx, my, radius, phase)
	if err != nil {
		return zero, "", nil, err
	}
	// In the orthonormal cylinder frame, normalized travel and the authored
	// coefficients move the lift by at most radius times both errors. Combine
	// with the same-parameter phase bound using (a+b)^2 <= 2*(a^2+b^2).
	drift := new(big.Rat).Mul(radius, new(big.Rat).Add(angularError, uvError))
	bound.Add(bound, new(big.Rat).Mul(drift, drift)).Mul(bound, big.NewRat(2, 1))
	span.ModelAngleBounds = &bounds
	return span, "finite-phase-cylinder-circle", bound, nil
}

// Use the original axis to order the endpoint rays, accounting for a proved
// circle reversal in the repaired edge. The cross product against the
// knot-angle ray proves containment without a rounded inverse trigonometric
// evaluation for the qualified minor and quarter sweeps.
func coverConvexArcMinorCircleSpan(src, corrected *File, q ConvexArcPCurveAgreement, model int64, o, x, y, z [3]*big.Rat, end, tolSquared *big.Rat, reversal *ConvexArcRepairReversal, allowQuarter bool) error {
	minimum, maximum := big.NewRat(2, 1), big.NewRat(3, 1)
	if reversal != nil {
		if reversal.EdgeID != q.EdgeID || reversal.CircleID != model {
			return geom.ErrDomain
		}
	}
	quarter := reversal != nil || (allowQuarter && end.Cmp(minimum) < 0)
	if quarter {
		minimum, maximum = big.NewRat(1, 1), big.NewRat(2, 1)
	}
	if end.Cmp(minimum) < 0 || end.Cmp(maximum) > 0 {
		return geom.ErrDomain
	}
	sine, cosine, rem, err := liftTrigBernstein([]*big.Rat{end, end})
	if err != nil {
		return err
	}
	for _, f := range []*File{src, corrected} {
		edge, err := simple(f, q.EdgeID, "EDGE_CURVE", 5)
		if err != nil {
			return err
		}
		link := model
		if f == src {
			link = q.SurfaceCurveID
		}
		sense, err := boolean(edge.Args[4])
		if err != nil || !sense || edge.Args[3].Ref != link {
			return geom.ErrDomain
		}
		vertices := [2]int64{edge.Args[1].Ref, edge.Args[2].Ref}
		if reversal != nil {
			if f == corrected {
				vertices[0], vertices[1] = vertices[1], vertices[0]
			}
			if vertices != [2]int64{reversal.OriginalStart, reversal.OriginalEnd} {
				return geom.ErrDomain
			}
		}
		for i, id := range vertices {
			vertex, err := simple(f, id, "VERTEX_POINT", 2)
			if err != nil {
				return err
			}
			point, err := occtLoopTriple(f, vertex.Args[1].Ref, "CARTESIAN_POINT")
			if err != nil {
				return err
			}
			ray := occtLoopSub(point, o)
			a, b := occtLoopDot(ray, x), occtLoopDot(ray, y)
			axial := occtLoopDot(ray, z)
			if axial.Sign() != 0 && (reversal == nil || f == corrected || axial.Mul(axial, axial).Cmp(tolSquared) > 0) {
				return geom.ErrDomain
			}
			if i == 0 {
				if a.Sign() <= 0 || b.Sign() != 0 {
					return geom.ErrDomain
				}
				continue
			}
			if b.Sign() <= 0 || (quarter && a.Sign() != 0) || (!quarter && a.Sign() >= 0) {
				return geom.ErrDomain
			}
			cross := new(big.Rat).Sub(new(big.Rat).Mul(a, sine[0]), new(big.Rat).Mul(b, cosine[0]))
			errorBound := new(big.Rat).Mul(rem, new(big.Rat).Sub(b, a))
			if cross.Cmp(errorBound) < 0 {
				return geom.ErrDomain
			}
		}
	}
	return nil
}
