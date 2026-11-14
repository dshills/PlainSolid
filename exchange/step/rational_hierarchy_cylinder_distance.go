package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
)

// HierarchyCylinderNearestSet is either a unique point or an entire circle.
// SourceFaceIDs are exact incident owners for a point; for a circle they are
// the source faces covering that complete nearest set, not a claim that every
// point lies on every listed face. No arbitrary axis-circle witness is chosen.
type HierarchyCylinderNearestSet struct {
	Kind                    string // "point" or "circle"
	SourceFaceIDs           []int64
	center, direction, axis [3]*big.Rat
	radius                  *big.Rat
	scale                   *geom.ExactAlgebraicRoot
}

func (s HierarchyCylinderNearestSet) Center() [3]*big.Rat    { return copyHierarchyRats(s.center) }
func (s HierarchyCylinderNearestSet) Direction() [3]*big.Rat { return copyHierarchyRats(s.direction) }
func (s HierarchyCylinderNearestSet) Axis() [3]*big.Rat      { return copyHierarchyRats(s.axis) }
func (s HierarchyCylinderNearestSet) Radius() *big.Rat {
	if s.radius == nil {
		return nil
	}
	return new(big.Rat).Set(s.radius)
}
func (s HierarchyCylinderNearestSet) RadialScale() *geom.ExactAlgebraicRoot { return s.scale }

// Enclose bounds Center+RadialScale*Direction for a unique point. For a circle
// it returns ErrDomain: center/axis/radius describe the entire exact set.
func (s HierarchyCylinderNearestSet) Enclose(ctx context.Context, width *big.Rat, o geom.ExactCurvePlaneEventOptions) (HierarchySphereNearestEnclosure, error) {
	if ctx == nil {
		return HierarchySphereNearestEnclosure{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return HierarchySphereNearestEnclosure{}, e
	}
	if s.Kind == "circle" {
		return HierarchySphereNearestEnclosure{}, geom.ErrDomain
	}
	if s.Kind != "point" {
		return HierarchySphereNearestEnclosure{}, geom.ErrInvalid
	}
	return (HierarchySphereNearestSet{center: s.center, direction: s.direction, radius: s.radius, scale: s.scale}).Enclose(ctx, width, o)
}

type HierarchyCylinderDistanceLeaf struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path                                     []int64
	Supported                                bool
	UnsupportedReason                        string
	Location                                 AnalyticPointLocation
	NearestBoundary                          []HierarchyCylinderNearestSet
	boundary, material                       *geom.ExactAlgebraicRoot
}

func (l HierarchyCylinderDistanceLeaf) BoundarySquaredDistance() *geom.ExactAlgebraicRoot {
	return l.boundary
}
func (l HierarchyCylinderDistanceLeaf) MaterialSquaredDistance() *geom.ExactAlgebraicRoot {
	return l.material
}

type RationalHierarchyCylinderDistance struct {
	Leaves                   []HierarchyCylinderDistanceLeaf
	LengthUnit               float64
	Operations, Subdivisions int
}
type RationalHierarchyCylinderDistanceOptions struct {
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
	Source                                             Limits
}

// DistanceRationalHierarchyCylinders proves finite closed-cylinder distances
// and all nearest sets independently for each authenticated occurrence.
func DistanceRationalHierarchyCylinders(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyCylinderDistanceOptions) (RationalHierarchyCylinderDistance, error) {
	return distanceHierarchyCylinders(ctx, m, p, o, false)
}

// DistanceOCCTRationalHierarchyCylinders selects the explicit OCCT frame rule.
func DistanceOCCTRationalHierarchyCylinders(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyCylinderDistanceOptions) (RationalHierarchyCylinderDistance, error) {
	return distanceHierarchyCylinders(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// CylinderDistance retains source ancestry from the affine-pcurve hierarchy.
func (m AffinePCurveHierarchy) CylinderDistance(ctx context.Context, p [3]*big.Rat, o RationalHierarchyCylinderDistanceOptions) (RationalHierarchyCylinderDistance, error) {
	return distanceHierarchyCylinders(ctx, m.model, p, o, m.vendor)
}
func distanceHierarchyCylinders(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyCylinderDistanceOptions, vendor bool) (RationalHierarchyCylinderDistance, error) {
	bad := func(e error) (RationalHierarchyCylinderDistance, error) {
		return RationalHierarchyCylinderDistance{}, e
	}
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
	out := RationalHierarchyCylinderDistance{LengthUnit: proof.LengthUnit}
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
		q, e := remaining()
		if e != nil {
			return nil, e
		}
		r, n, e := geom.NewExactRationalRoot(ctx, x, q)
		out.Operations += n
		return r, e
	}
	isolate := func(coeff []*big.Rat, upper *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		q, e := remaining()
		if e != nil {
			return nil, e
		}
		r, e := geom.IsolateExactPolynomialRoots(ctx, coeff, new(big.Rat), upper, q)
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
	compare := func(x, y *geom.ExactAlgebraicRoot) (int, error) {
		q, e := remaining()
		if e != nil {
			return 0, e
		}
		r, e := x.Compare(ctx, y, q)
		out.Operations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e != nil {
			return 0, e
		}
		if !r.Proven {
			return 0, geom.ErrConvergence
		}
		return r.Order, nil
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[u.ProductID]
		leaf := HierarchyCylinderDistanceLeaf{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...), Location: AnalyticPointUnsupported}
		if p.Cylinder == nil {
			leaf.UnsupportedReason = "finite cylinder distance only; other material family not evaluated"
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		out.Operations += 512
		if _, e = remaining(); e != nil {
			return bad(e)
		}
		cyl := p.Cylinder
		r, h := new(big.Rat).SetFloat64(cyl.Radius), new(big.Rat).SetFloat64(cyl.Height)
		frameAxes := [3][3]*big.Rat{mappedRatVec(cyl.Frame.XAxis()), mappedRatVec(cyl.Frame.YAxis()), mappedRatVec(cyl.Frame.ZAxis())}
		productPoint := a.inverse(point, u.pose.Origin, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z})
		local := a.inverse(productPoint, mappedRatVec(cyl.Frame.Origin()), frameAxes)
		x, y, z := local[0], local[1], local[2]
		s := a.add(a.mul(x, x), a.mul(y, y))
		r2 := a.mul(r, r)
		radialSign := s.Cmp(r2)
		leaf.Location = AnalyticPointOutside
		if radialSign <= 0 && z.Sign() >= 0 && z.Cmp(h) <= 0 {
			leaf.Location = AnalyticPointInside
			if radialSign == 0 || z.Sign() == 0 || z.Cmp(h) == 0 {
				leaf.Location = AnalyticPointBoundary
			}
		}
		world := func(v [3]*big.Rat, vector bool) (w [3]*big.Rat) {
			q := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			if !vector {
				q = mappedRatVec(cyl.Frame.Origin())
			}
			for i := range q {
				for j := 0; j < 3; j++ {
					q[i] = a.add(q[i], a.mul(frameAxes[j][i], v[j]))
				}
			}
			for i := range w {
				w[i] = new(big.Rat)
				if !vector {
					w[i] = new(big.Rat).Set(u.pose.Origin[i])
				}
				axes := [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z}
				for j := 0; j < 3; j++ {
					w[i] = a.add(w[i], a.mul(axes[j][i], q[j]))
				}
			}
			return
		}
		axis := world([3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, true)
		var scale *geom.ExactAlgebraicRoot
		if s.Sign() > 0 {
			if radialSign == 0 {
				scale, e = rational(big.NewRat(1, 1))
			} else {
				upper := a.add(big.NewRat(1, 1), a.check(new(big.Rat).Quo(r2, s)))
				if a.err != nil {
					return bad(a.err)
				}
				scale, e = isolate([]*big.Rat{new(big.Rat).Neg(r2), new(big.Rat), s}, upper)
			}
			if e != nil {
				return bad(e)
			}
		}
		one, e := rational(big.NewRat(1, 1))
		if e != nil {
			return bad(e)
		}
		radialDistance := func(shift *big.Rat) (*geom.ExactAlgebraicRoot, error) {
			if s.Sign() == 0 {
				return rational(a.add(r2, shift))
			}
			if radialSign == 0 {
				return rational(shift)
			}
			sum := a.add(s, r2)
			diff := a.sub(s, r2)
			constant := a.add(a.add(a.mul(shift, shift), a.mul(big.NewRat(2, 1), a.mul(sum, shift))), a.mul(diff, diff))
			upper := a.add(sum, shift)
			linear := a.mul(big.NewRat(-2, 1), upper)
			if a.err != nil {
				return nil, a.err
			}
			return isolate([]*big.Rat{constant, linear, big.NewRat(1, 1)}, upper)
		}
		owners := func(station *big.Rat, wall bool) []int64 {
			ids := map[int64]bool{}
			if wall {
				if s.Sign() == 0 {
					for i := 0; i < cyl.Cells; i++ {
						ids[cyl.SourceFaces[i]] = true
					}
				} else {
					hit := [4]bool{x.Sign() >= 0 && y.Sign() >= 0, x.Sign() <= 0 && y.Sign() >= 0, x.Sign() <= 0 && y.Sign() <= 0, x.Sign() >= 0 && y.Sign() <= 0}
					for i, v := range hit {
						if v {
							ids[cyl.NativeFaceSourceIDs[2+i]] = true
						}
					}
				}
			}
			if station.Sign() == 0 {
				ids[cyl.NativeFaceSourceIDs[0]] = true
			}
			if station.Cmp(h) == 0 {
				ids[cyl.NativeFaceSourceIDs[1]] = true
			}
			out := make([]int64, 0, len(ids))
			for id := range ids {
				out = append(out, id)
			}
			sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
			return out
		}
		type candidate struct {
			key      string
			distance *geom.ExactAlgebraicRoot
			set      HierarchyCylinderNearestSet
		}
		var candidates []candidate
		radialWitness := func(station *big.Rat) HierarchyCylinderNearestSet {
			set := HierarchyCylinderNearestSet{Kind: "point", SourceFaceIDs: owners(station, true), center: world([3]*big.Rat{new(big.Rat), new(big.Rat), station}, false), direction: world([3]*big.Rat{x, y, new(big.Rat)}, true), axis: copyHierarchyRats(axis), radius: new(big.Rat).Set(r), scale: scale}
			if s.Sign() == 0 {
				set.Kind = "circle"
			}
			return set
		}
		station := new(big.Rat).Set(z)
		if station.Sign() < 0 {
			station.SetInt64(0)
		} else if station.Cmp(h) > 0 {
			station.Set(h)
		}
		dz := a.sub(z, station)
		distance, e := radialDistance(a.mul(dz, dz))
		if e != nil {
			return bad(e)
		}
		candidates = append(candidates, candidate{"radial:" + station.RatString(), distance, radialWitness(station)})
		for _, capZ := range []*big.Rat{new(big.Rat), h} {
			dz := a.sub(z, capZ)
			shift := a.mul(dz, dz)
			var set HierarchyCylinderNearestSet
			key := "radial:" + capZ.RatString()
			if radialSign >= 0 {
				distance, e = radialDistance(shift)
				set = radialWitness(capZ)
			} else {
				distance, e = rational(shift)
				key = "disk:" + capZ.RatString()
				set = HierarchyCylinderNearestSet{Kind: "point", SourceFaceIDs: owners(capZ, false), center: world([3]*big.Rat{x, y, capZ}, false), direction: [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, axis: copyHierarchyRats(axis), radius: new(big.Rat).Set(r), scale: one}
			}
			if e != nil {
				return bad(e)
			}
			candidates = append(candidates, candidate{key, distance, set})
		}
		if a.err != nil {
			return bad(a.err)
		}
		seen := map[string]bool{}
		for _, c := range candidates {
			order := -1
			if leaf.boundary != nil {
				order, e = compare(c.distance, leaf.boundary)
				if e != nil {
					return bad(e)
				}
			}
			if order < 0 {
				leaf.boundary = c.distance
				leaf.NearestBoundary = nil
				seen = map[string]bool{}
			}
			if order <= 0 && !seen[c.key] {
				seen[c.key] = true
				leaf.NearestBoundary = append(leaf.NearestBoundary, c.set)
			}
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
