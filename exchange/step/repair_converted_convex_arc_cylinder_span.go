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
// Its two controls define UVStart + t*UVDerivative on ParameterInterval,
// starting at t=0. Here t is also the original model LINE or CIRCLE parameter.
type ConvexArcCylinderSpanOwner struct {
	PCurveID          int64
	ParameterInterval [2]*big.Rat
	UVStart           [2]*big.Rat
	UVDerivative      [2]*big.Rat
	// RepairedToOwner is [offset, scale] for the finite affine map from
	// repaired model parameter to original spline parameter, when needed.
	RepairedToOwner *[2]*big.Rat `json:",omitempty"`
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
	if lo.Sign() != 0 || hi.Sign() <= 0 {
		return zero, geom.ErrDomain
	}
	for i := range out.UVStart {
		out.UVStart[i] = uv[0][i]
		out.UVDerivative[i] = new(big.Rat).Quo(new(big.Rat).Sub(uv[1][i], uv[0][i]), hi)
	}
	return out, nil
}

func boundConvertedConvexArcCylinderSpan(src, corrected *File, q ConvexArcPCurveAgreement, model int64, tolSquared *big.Rat, conversion *convertedConvexArcLength, trim *ConvexArcAxialTrimRepair, reversal *ConvexArcRepairReversal) (ConvexArcCylinderSpanOwner, string, *big.Rat, error) {
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
		if err != nil || slope[0].Sign() != 0 || slope[1].Cmp(conversion.ownerScale) != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		original, repaired := intervals[0], intervals[1]
		for _, t := range original {
			if t.Cmp(span.ParameterInterval[0]) < 0 || t.Cmp(span.ParameterInterval[1]) > 0 {
				return zero, "", nil, geom.ErrDomain
			}
		}
		for i := range d {
			if d[i].Cmp(new(big.Rat).Mul(z[i], conversion.modelSpeed)) != 0 {
				return zero, "", nil, geom.ErrDomain
			}
		}
		if trim != nil {
			// Map corresponding endpoints, preserving direction and staying
			// inside the authored spline knots even if a repaired end moved.
			scale := new(big.Rat).Quo(new(big.Rat).Sub(original[1], original[0]), new(big.Rat).Sub(repaired[1], repaired[0]))
			offset := new(big.Rat).Sub(original[0], new(big.Rat).Mul(scale, repaired[0]))
			span.RepairedToOwner = &[2]*big.Rat{offset, scale}
		}
		// With constant u, the difference between the traversed spline and
		// repaired line is affine. Its squared norm is bounded at endpoints.
		upper := new(big.Rat)
		for i, t := range original {
			v := new(big.Rat).Add(uv[1], new(big.Rat).Mul(slope[1], t))
			v.Sub(v, new(big.Rat).Mul(conversion.modelSpeed, repaired[i]))
			bound, err := boundConvexArcCylinderLine(o, x, y, z, p, radius, uv[0], v)
			if err != nil {
				return zero, "", nil, err
			}
			if bound.Cmp(upper) > 0 {
				upper = bound
			}
		}
		return span, "finite-cylinder-line", upper, nil
	case "CIRCLE":
		mo, mx, my, mz, err := convexUVFrame(src, curve[0].Args[1].Ref)
		increment, kind := big.NewRat(1, 1), "finite-cylinder-circle"
		if reversal != nil {
			increment, kind = big.NewRat(-1, 1), "finite-reversed-cylinder-circle"
		}
		if err != nil || uv[0].Sign() != 0 || slope[0].Cmp(increment) != 0 || slope[1].Sign() != 0 || !convexUVSame(x, mx) {
			return zero, "", nil, geom.ErrDomain
		}
		for i := range y {
			if my[i].Cmp(new(big.Rat).Mul(y[i], increment)) != 0 || mz[i].Cmp(new(big.Rat).Mul(z[i], increment)) != 0 {
				return zero, "", nil, geom.ErrDomain
			}
		}
		r, err := liftSourceRat(curve[0].Args[2])
		if err != nil || r.Cmp(radius) != 0 {
			return zero, "", nil, geom.ErrDomain
		}
		if err := coverConvexArcMinorCircleSpan(src, corrected, q, model, mo, mx, my, mz, span.ParameterInterval[1], tolSquared, reversal); err != nil {
			return zero, "", nil, err
		}
		var center [3]*big.Rat
		for i := range o {
			center[i] = new(big.Rat).Add(o[i], new(big.Rat).Mul(z[i], uv[1]))
		}
		delta := occtLoopSub(center, mo)
		return span, kind, occtLoopDot(delta, delta), nil
	default:
		return zero, "", nil, geom.ErrDomain
	}
}

// Use the original axis to order the endpoint rays, accounting for a proved
// circle reversal in the repaired edge. The cross product against the
// knot-angle ray proves containment without a rounded inverse trigonometric
// evaluation for either the ordinary minor sweep or the reversed quarter.
func coverConvexArcMinorCircleSpan(src, corrected *File, q ConvexArcPCurveAgreement, model int64, o, x, y, z [3]*big.Rat, end, tolSquared *big.Rat, reversal *ConvexArcRepairReversal) error {
	minimum, maximum := big.NewRat(2, 1), big.NewRat(3, 1)
	if reversal != nil {
		if reversal.EdgeID != q.EdgeID || reversal.CircleID != model {
			return geom.ErrDomain
		}
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
			if b.Sign() <= 0 || (reversal == nil && a.Sign() >= 0) || (reversal != nil && a.Sign() != 0) {
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
