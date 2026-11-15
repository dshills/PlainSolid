package step

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"plainsolid/geom"
)

// GroupedCapFiniteTrim describes a proved radial correspondence, not equality
// of source parameterizations. EndStation is the unique root of EndPolynomial
// on the first span's normalized [0,1] domain. SourceEnd encloses its original
// knot parameter. Angular selection is exactly quadrant one [0,pi/2], with
// increasing original CIRCLE parameter and opposite cylindrical U direction.
// The rational cap endpoint itself need not lie exactly on the model circle.
type GroupedCapFiniteTrim struct {
	CapOwner                                ConvexArcPCurveAgreement
	CylinderOwner                           ConvexArcPCurveAgreement
	CircleID, StartVertexID, EndVertexID    int64
	CapCoedgeID, CylinderCoedgeID           int64
	CapCoedgeForward, CylinderCoedgeForward bool
	FirstSpan                               [2]*big.Rat
	EndPolynomial                           []*big.Rat
	EndStation                              *geom.ExactAlgebraicRoot
	SourceStart                             *big.Rat
	SourceEnd                               geom.ExactParameterInterval
	EndpointDistanceUpper                   [2]*big.Rat
	StartsAtSeam, Wraps                     bool
}

// GroupedFiniteTrimEvidence is a source-derived observation. It does not
// promote material or change any prior reader's acceptance.
type GroupedFiniteTrimEvidence struct {
	FullDomain                       GroupedPCurveEvidence
	Trims                            []GroupedCapFiniteTrim
	RootOperations, RootSubdivisions int
}

func groupedQuadraticPower(v []*big.Rat) []*big.Rat {
	return []*big.Rat{new(big.Rat).Set(v[0]), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(v[1], v[0])), new(big.Rat).Add(new(big.Rat).Sub(v[0], new(big.Rat).Mul(big.NewRat(2, 1), v[1])), v[2])}
}

// InspectGroupedConvexArcFiniteTrims adds uniquely selected first-quadrant
// cap/negative-cylinder-U trims to the full-domain evidence. MaxDistance bounds
// each source-vertex-to-cap-endpoint distance separately. Options bound the
// cumulative algebraic root proof over both caps; zero fields use fixed caps.
func InspectGroupedConvexArcFiniteTrims(ctx context.Context, r io.Reader, l Limits, o ConvexArcRepairOptions, rootOptions geom.ExactCurvePlaneEventOptions) (GroupedFiniteTrimEvidence, error) {
	var zero GroupedFiniteTrimEvidence
	if ctx == nil || r == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	l, e := l.defaults()
	if e != nil {
		return zero, e
	}
	l.MaxBytes = min(l.MaxBytes, 1<<20)
	l.MaxEntities = min(l.MaxEntities, 1024)
	l.MaxValues = min(l.MaxValues, 8192)
	l.MaxDepth = min(l.MaxDepth, 16)
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return zero, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return zero, geom.ErrConvergence
	}
	full, e := InspectGroupedConvexArcPCurves(ctx, bytes.NewReader(raw), l, o)
	if e != nil {
		return zero, e
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return zero, e
	}
	if rootOptions.MaxOperations < 0 || rootOptions.MaxCoefficientBits < 0 || rootOptions.MaxSubdivisions < 0 {
		return zero, geom.ErrInvalid
	}
	if rootOptions.MaxOperations == 0 {
		rootOptions.MaxOperations = 100000
	}
	rootOptions.MaxOperations = min(rootOptions.MaxOperations, 100000)
	if rootOptions.MaxCoefficientBits == 0 {
		rootOptions.MaxCoefficientBits = 4096
	}
	rootOptions.MaxCoefficientBits = min(rootOptions.MaxCoefficientBits, 4096)
	if rootOptions.MaxSubdivisions == 0 {
		rootOptions.MaxSubdivisions = 256
	}
	rootOptions.MaxSubdivisions = min(rootOptions.MaxSubdivisions, 256)
	out := GroupedFiniteTrimEvidence{FullDomain: full}
	budget := func() (geom.ExactCurvePlaneEventOptions, error) {
		q := rootOptions
		q.MaxOperations -= out.RootOperations
		q.MaxSubdivisions -= out.RootSubdivisions
		if q.MaxOperations <= 0 || q.MaxSubdivisions <= 0 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	for _, cap := range full.Caps {
		edge, e := simple(f, cap.Owner.EdgeID, "EDGE_CURVE", 5)
		if e != nil {
			return zero, e
		}
		same, e := boolean(edge.Args[4])
		if e != nil || !same {
			return zero, geom.ErrDomain
		}
		circle, e := simple(f, cap.CircleID, "CIRCLE", 3)
		if e != nil {
			return zero, e
		}
		center, x, y, z, e := convexUVFrame(f, circle.Args[1].Ref)
		if e != nil {
			return zero, e
		}
		radius, e := liftSourceRat(circle.Args[2])
		if e != nil {
			return zero, e
		}
		trim := GroupedCapFiniteTrim{CapOwner: cap.Owner, CircleID: cap.CircleID, StartVertexID: edge.Args[1].Ref, EndVertexID: edge.Args[2].Ref, FirstSpan: [2]*big.Rat{new(big.Rat).Set(cap.Knots[1]), new(big.Rat).Set(cap.Knots[2])}, StartsAtSeam: true, SourceStart: new(big.Rat).Set(cap.Knots[1])}
		for i, id := range []int64{trim.StartVertexID, trim.EndVertexID} {
			v, e := simple(f, id, "VERTEX_POINT", 2)
			if e != nil {
				return zero, e
			}
			p, e := occtLoopTriple(f, v.Args[1].Ref, "CARTESIAN_POINT")
			if e != nil {
				return zero, e
			}
			d := occtLoopSub(p, center)
			a, b, c := occtLoopDot(d, x), occtLoopDot(d, y), occtLoopDot(d, z)
			if c.Sign() != 0 || (i == 0 && (a.Sign() <= 0 || b.Sign() != 0)) || (i == 1 && (a.Sign() != 0 || b.Sign() <= 0)) {
				return zero, geom.ErrDomain
			}
			rad := a
			if i == 1 {
				rad = b
			}
			trim.EndpointDistanceUpper[i] = new(big.Rat).Add(new(big.Rat).Abs(new(big.Rat).Sub(rad, radius)), cap.RadialDistanceUpper)
			if trim.EndpointDistanceUpper[i].Cmp(new(big.Rat).SetFloat64(o.MaxDistance)) > 0 {
				return zero, geom.ErrDomain
			}
		}
		// The whole-domain proof supplies a strictly increasing one-turn angle.
		// At the first span start X>0,Y=0; the unique end event has X=0,Y>0.
		poly := groupedQuadraticPower(cap.firstSpan[0])
		opts, e := budget()
		if e != nil {
			return zero, e
		}
		roots, e := geom.IsolateExactPolynomialRoots(ctx, poly, big.NewRat(0, 1), big.NewRat(1, 1), opts)
		out.RootOperations += roots.Operations
		out.RootSubdivisions += roots.Subdivisions
		if e != nil {
			return zero, e
		}
		if len(roots.Roots) != 1 || roots.Roots[0].Multiplicity != 1 || roots.LowerSign != 1 || roots.UpperSign != -1 {
			return zero, geom.ErrDomain
		}
		root := roots.Roots[0].Root
		for _, coeff := range [][]*big.Rat{cap.firstSpan[1], cap.firstSpan[2]} {
			opts, e = budget()
			if e != nil {
				return zero, e
			}
			sign, e := root.SignPolynomial(ctx, groupedQuadraticPower(coeff), opts)
			out.RootOperations += sign.Operations
			out.RootSubdivisions += sign.Subdivisions
			if e != nil {
				return zero, e
			}
			if sign.Sign != 1 {
				return zero, geom.ErrDomain
			}
		}
		opts, e = budget()
		if e != nil {
			return zero, e
		}
		width := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 48))
		enc, e := root.Enclose(ctx, width, opts)
		out.RootOperations += enc.Operations
		out.RootSubdivisions += enc.Subdivisions
		if e != nil {
			return zero, e
		}
		if !enc.Converged || !enc.HasBounds {
			return zero, geom.ErrConvergence
		}
		trim.EndPolynomial = poly
		trim.EndStation = root
		scale := new(big.Rat).Sub(trim.FirstSpan[1], trim.FirstSpan[0])
		trim.SourceEnd = geom.ExactParameterInterval{Lower: new(big.Rat).Add(trim.FirstSpan[0], new(big.Rat).Mul(scale, enc.Interval.Lower)), Upper: new(big.Rat).Add(trim.FirstSpan[0], new(big.Rat).Mul(scale, enc.Interval.Upper))}
		for _, q := range full.OtherOwners {
			if q.EdgeID == cap.Owner.EdgeID {
				if trim.CylinderOwner.EdgeID != 0 || q.Kind != "cylinder-circle" {
					return zero, geom.ErrDomain
				}
				trim.CylinderOwner = q
			}
		}
		if trim.CylinderOwner.EdgeID == 0 {
			return zero, geom.ErrDomain
		}
		for i, owner := range []ConvexArcPCurveAgreement{trim.CapOwner, trim.CylinderOwner} {
			face, e := occtLoopReadFace(f, owner.FaceID)
			if e != nil {
				return zero, e
			}
			found := false
			for _, id := range face.oriented {
				oe, e := simple(f, id, "ORIENTED_EDGE", 5)
				if e != nil {
					return zero, e
				}
				if oe.Args[3].Ref == owner.EdgeID {
					if found {
						return zero, geom.ErrDomain
					}
					found = true
					forward, e := boolean(oe.Args[4])
					if e != nil {
						return zero, e
					}
					if i == 0 {
						trim.CapCoedgeID = id
						trim.CapCoedgeForward = forward
					} else {
						trim.CylinderCoedgeID = id
						trim.CylinderCoedgeForward = forward
					}
				}
			}
			if !found {
				return zero, geom.ErrDomain
			}
		}
		out.Trims = append(out.Trims, trim)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
