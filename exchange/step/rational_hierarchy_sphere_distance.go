package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// HierarchySphereNearestSet describes the exact nearest boundary set. If
// Nonunique is true it is the complete sphere (the query was its center).
// Otherwise the unique point is Center + RadialScale*(query-Center), where the
// positive immutable algebraic RadialScale is exposed separately.
type HierarchySphereNearestSet struct {
	Nonunique         bool
	center, direction [3]*big.Rat
	radius            *big.Rat
	scale             *geom.ExactAlgebraicRoot
}

func (s HierarchySphereNearestSet) Center() [3]*big.Rat    { return copyHierarchyRats(s.center) }
func (s HierarchySphereNearestSet) Direction() [3]*big.Rat { return copyHierarchyRats(s.direction) }
func (s HierarchySphereNearestSet) Radius() *big.Rat {
	if s.radius == nil {
		return nil
	}
	return new(big.Rat).Set(s.radius)
}
func (s HierarchySphereNearestSet) RadialScale() *geom.ExactAlgebraicRoot { return s.scale }

type HierarchySphereNearestEnclosure struct {
	Lower, Upper             [3]*big.Rat
	Operations, Subdivisions int
}

// Enclose certifies each unique nearest coordinate to the requested maximum
// width. A whole-sphere nearest set returns ErrDomain rather than choosing a
// representative. Returned interval endpoints are caller-owned rationals.
func (s HierarchySphereNearestSet) Enclose(ctx context.Context, width *big.Rat, o geom.ExactCurvePlaneEventOptions) (HierarchySphereNearestEnclosure, error) {
	var zero HierarchySphereNearestEnclosure
	if ctx == nil || width == nil || width.Sign() <= 0 || s.radius == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if s.Nonunique {
		return zero, geom.ErrDomain
	}
	if s.scale == nil {
		return zero, geom.ErrInvalid
	}
	o, e := sphereDistanceRootOptions(o)
	if e != nil {
		return zero, e
	}
	a := hierarchyPointArithmetic{bits: o.MaxCoefficientBits}
	a.check(width)
	a.check(s.radius)
	for _, q := range s.center {
		a.check(q)
	}
	max := big.NewRat(1, 1)
	for _, q := range s.direction {
		a.check(q)
		if new(big.Rat).Abs(q).Cmp(max) > 0 {
			max = new(big.Rat).Abs(q)
		}
	}
	parameterWidth := a.check(new(big.Rat).Quo(width, max))
	if a.err != nil {
		return zero, a.err
	}
	// Reserve a fixed bound for this small exact affine interval calculation.
	if o.MaxOperations <= 32 {
		return zero, geom.ErrConvergence
	}
	o.MaxOperations -= 32
	r, e := s.scale.Enclose(ctx, parameterWidth, o)
	if e != nil {
		return zero, e
	}
	if !r.Converged {
		return zero, geom.ErrConvergence
	}
	out := HierarchySphereNearestEnclosure{Operations: r.Operations + 32, Subdivisions: r.Subdivisions}
	for i, d := range s.direction {
		lo, hi := r.Interval.Lower, r.Interval.Upper
		if d.Sign() < 0 {
			lo, hi = hi, lo
		}
		out.Lower[i] = a.add(s.center[i], a.mul(d, lo))
		out.Upper[i] = a.add(s.center[i], a.mul(d, hi))
		if a.sub(out.Upper[i], out.Lower[i]).Cmp(width) > 0 {
			return zero, geom.ErrConvergence
		}
	}
	if a.err != nil {
		return zero, a.err
	}
	return out, nil
}

type HierarchySphereDistanceLeaf struct {
	ProductID, SourceProductID, SourceBodyID, SourceFaceID int64
	Path                                                   []int64
	Supported                                              bool
	UnsupportedReason                                      string
	Location                                               AnalyticPointLocation
	NearestBoundary                                        HierarchySphereNearestSet
	boundary, material                                     *geom.ExactAlgebraicRoot
}

func (l HierarchySphereDistanceLeaf) BoundarySquaredDistance() *geom.ExactAlgebraicRoot {
	return l.boundary
}
func (l HierarchySphereDistanceLeaf) MaterialSquaredDistance() *geom.ExactAlgebraicRoot {
	return l.material
}

// RationalHierarchySphereDistance has per-occurrence results only. Unsupported
// other families cannot be silently omitted from a claimed global minimum.
type RationalHierarchySphereDistance struct {
	Leaves                   []HierarchySphereDistanceLeaf
	LengthUnit               float64
	Operations, Subdivisions int
}
type RationalHierarchySphereDistanceOptions struct {
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
	Source                                             Limits
}

func sphereDistanceRootOptions(o geom.ExactCurvePlaneEventOptions) (geom.ExactCurvePlaneEventOptions, error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 4096
	}
	if o.MaxOperations < 1 || o.MaxOperations > 20000000 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 || o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return o, geom.ErrInvalid
	}
	return o, nil
}

// DistanceRationalHierarchySpheres proves exact algebraic sphere distance and
// unique/nonunique nearest boundary sets under proper rational occurrences.
func DistanceRationalHierarchySpheres(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchySphereDistanceOptions) (RationalHierarchySphereDistance, error) {
	return distanceHierarchySpheres(ctx, m, p, o, false)
}

// DistanceOCCTRationalHierarchySpheres selects the explicit OCCT frame rule.
func DistanceOCCTRationalHierarchySpheres(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchySphereDistanceOptions) (RationalHierarchySphereDistance, error) {
	return distanceHierarchySpheres(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// SphereDistance measures the sphere leaves in the source-bound hierarchy.
func (m AffinePCurveHierarchy) SphereDistance(ctx context.Context, p [3]*big.Rat, o RationalHierarchySphereDistanceOptions) (RationalHierarchySphereDistance, error) {
	return distanceHierarchySpheres(ctx, m.model, p, o, m.vendor)
}
func distanceHierarchySpheres(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchySphereDistanceOptions, vendor bool) (RationalHierarchySphereDistance, error) {
	bad := func(e error) (RationalHierarchySphereDistance, error) { return RationalHierarchySphereDistance{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	opts, e := sphereDistanceRootOptions(geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
	if e != nil {
		return bad(e)
	}
	a := hierarchyPointArithmetic{bits: opts.MaxCoefficientBits}
	for _, q := range point {
		if q == nil {
			return bad(geom.ErrInvalid)
		}
		a.check(q)
	}
	if a.err != nil {
		return bad(a.err)
	}
	point = copyHierarchyRats(point)
	actual, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m, vendor)
	if e != nil {
		return bad(e)
	}
	proof, e := readRationalSphericalHierarchy(ctx, bytes.NewBufferString(actual.Source), o.Source, actual.LocalProof.Proof.Tolerance, vendor)
	if e != nil {
		return bad(e)
	}
	expected, d, e := rationalSphericalHierarchyEnvelopeFor(ctx, proof, vendor)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	out := RationalHierarchySphereDistance{LengthUnit: proof.LengthUnit}
	remaining := func() (geom.ExactCurvePlaneEventOptions, error) {
		q := opts
		q.MaxOperations -= out.Operations
		q.MaxSubdivisions -= out.Subdivisions
		if q.MaxOperations < 1 || q.MaxSubdivisions < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	rational := func(x *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		o, e := remaining()
		if e != nil {
			return nil, e
		}
		r, n, e := geom.NewExactRationalRoot(ctx, x, o)
		out.Operations += n
		return r, e
	}
	isolate := func(coeff []*big.Rat, upper *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		o, e := remaining()
		if e != nil {
			return nil, e
		}
		r, e := geom.IsolateExactPolynomialRoots(ctx, coeff, new(big.Rat), upper, o)
		out.Operations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e != nil {
			return nil, e
		}
		if len(r.Roots) != 1 {
			return nil, geom.ErrConvergence
		}
		return r.Roots[0].Root, nil
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[u.ProductID]
		leaf := HierarchySphereDistanceLeaf{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...), Location: AnalyticPointUnsupported}
		if p.Sphere == nil {
			leaf.UnsupportedReason = "exact sphere distance only; other material family not evaluated"
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		// Fixed affine/polynomial coefficient work is charged per supported use.
		out.Operations += 128
		if _, e = remaining(); e != nil {
			return bad(e)
		}
		sphere := p.Sphere
		leaf.SourceFaceID = sphere.SourceFaceID
		r := new(big.Rat).SetFloat64(sphere.Radius)
		localCenter := mappedRatVec(sphere.Frame.Origin())
		matrix := u.Matrix()
		var center, dir [3]*big.Rat
		for i := range center {
			center[i] = new(big.Rat).Set(matrix[i][3])
			for j := 0; j < 3; j++ {
				center[i] = a.add(center[i], a.mul(matrix[i][j], localCenter[j]))
			}
			dir[i] = a.sub(point[i], center[i])
		}
		s := a.planarDot(dir, dir)
		r2 := a.mul(r, r)
		sum := a.add(s, r2)
		difference := a.sub(s, r2)
		if a.err != nil {
			return bad(a.err)
		}
		leaf.NearestBoundary = HierarchySphereNearestSet{center: copyHierarchyRats(center), direction: copyHierarchyRats(dir), radius: new(big.Rat).Set(r)}
		leaf.Location = AnalyticPointOutside
		if difference.Sign() < 0 {
			leaf.Location = AnalyticPointInside
		} else if difference.Sign() == 0 {
			leaf.Location = AnalyticPointBoundary
		}
		if s.Sign() == 0 {
			leaf.NearestBoundary.Nonunique = true
			leaf.boundary, e = rational(r2)
		} else if difference.Sign() == 0 {
			leaf.boundary, e = rational(new(big.Rat))
			if e == nil {
				leaf.NearestBoundary.scale, e = rational(big.NewRat(1, 1))
			}
		} else {
			coeff := []*big.Rat{a.mul(difference, difference), a.mul(big.NewRat(-2, 1), sum), big.NewRat(1, 1)}
			if a.err != nil {
				return bad(a.err)
			}
			leaf.boundary, e = isolate(coeff, sum)
			if e == nil {
				upper := a.add(big.NewRat(1, 1), a.check(new(big.Rat).Quo(r2, s)))
				if a.err != nil {
					return bad(a.err)
				}
				leaf.NearestBoundary.scale, e = isolate([]*big.Rat{new(big.Rat).Neg(r2), new(big.Rat), new(big.Rat).Set(s)}, upper)
			}
		}
		if e != nil {
			return bad(e)
		}
		leaf.material = leaf.boundary
		if leaf.Location == AnalyticPointInside {
			leaf.material, e = rational(new(big.Rat))
			if e != nil {
				return bad(e)
			}
		}
		leaf.Supported = true
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
