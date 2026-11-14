package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// AnalyticPointLocation classifies a point against one material occurrence.
// Unsupported means no classification was attempted, not outside.
type AnalyticPointLocation string

const (
	AnalyticPointUnsupported AnalyticPointLocation = "unsupported"
	AnalyticPointOutside     AnalyticPointLocation = "outside"
	AnalyticPointInside      AnalyticPointLocation = "inside"
	AnalyticPointBoundary    AnalyticPointLocation = "boundary"
)

// RationalHierarchyPointLeaf identifies one original occurrence. SourceFaceIDs
// names its complete shell, not the selected face at a split-cylinder seam.
// BoundaryRoles identifies analytic supports (wall, lower-cap, upper-cap), not
// a guessed source coedge or periodic parameter branch.
type RationalHierarchyPointLeaf struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path, SourceFaceIDs                      []int64
	Family                                   string
	Location                                 AnalyticPointLocation
	BoundaryRoles                            []string
	local                                    [3]*big.Rat
}

// AnalyticCoordinates returns owned exact coordinates in the analytic support
// frame. Unsupported planar occurrences have no analytic coordinates.
func (p RationalHierarchyPointLeaf) AnalyticCoordinates() [3]*big.Rat {
	return copyHierarchyRats(p.local)
}

// RationalHierarchyPointLocations has no union or first-hit interpretation.
// Coincident or overlapping occurrences remain separate, in source path order.
type RationalHierarchyPointLocations struct {
	Leaves     []RationalHierarchyPointLeaf
	LengthUnit float64
}
type RationalHierarchyPointOptions struct {
	MaxCoefficientBits int
	Source             Limits
}

func LocateRationalHierarchyAnalyticPoint(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
	return locateHierarchyAnalyticPoint(ctx, m, point, o, false)
}
func LocateOCCTRationalHierarchyAnalyticPoint(ctx context.Context, m OCCTRationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyPointOptions) (RationalHierarchyPointLocations, error) {
	return locateHierarchyAnalyticPoint(ctx, RationalSphericalHierarchy(m), point, o, true)
}

type hierarchyPointArithmetic struct {
	bits int
	err  error
}

func (a *hierarchyPointArithmetic) check(q *big.Rat) *big.Rat {
	if q == nil || q.Num().BitLen() > a.bits || q.Denom().BitLen() > a.bits {
		a.err = geom.ErrConvergence
	}
	return q
}
func (a *hierarchyPointArithmetic) add(x, y *big.Rat) *big.Rat {
	return a.check(new(big.Rat).Add(x, y))
}
func (a *hierarchyPointArithmetic) sub(x, y *big.Rat) *big.Rat {
	return a.check(new(big.Rat).Sub(x, y))
}
func (a *hierarchyPointArithmetic) mul(x, y *big.Rat) *big.Rat {
	return a.check(new(big.Rat).Mul(x, y))
}
func (a *hierarchyPointArithmetic) inverse(p [3]*big.Rat, origin [3]*big.Rat, axes [3][3]*big.Rat) (q [3]*big.Rat) {
	var d [3]*big.Rat
	for i := range d {
		d[i] = a.sub(p[i], origin[i])
	}
	for i := range q {
		q[i] = new(big.Rat)
		for j := range d {
			q[i] = a.add(q[i], a.mul(axes[i][j], d[j]))
		}
	}
	return
}
func locateHierarchyAnalyticPoint(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyPointOptions, vendor bool) (RationalHierarchyPointLocations, error) {
	bad := func(e error) (RationalHierarchyPointLocations, error) { return RationalHierarchyPointLocations{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 {
		return bad(geom.ErrInvalid)
	}
	a := hierarchyPointArithmetic{bits: o.MaxCoefficientBits}
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
	out := RationalHierarchyPointLocations{LengthUnit: proof.LengthUnit}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[u.ProductID]
		v := RationalHierarchyPointLeaf{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...), Location: AnalyticPointUnsupported}
		body, e := simple(proof.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return bad(e)
		}
		shell, e := simple(proof.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return bad(e)
		}
		v.SourceFaceIDs, e = listRefs(shell.Args[1], 1)
		if e != nil {
			return bad(e)
		}
		var frame geom.Frame
		var radius, height, major *big.Rat
		switch {
		case p.Cylinder != nil:
			v.Family = "cylinder"
			frame = p.Cylinder.Frame
			radius = new(big.Rat).SetFloat64(p.Cylinder.Radius)
			height = new(big.Rat).SetFloat64(p.Cylinder.Height)
		case p.Sphere != nil:
			v.Family = "sphere"
			frame = p.Sphere.Frame
			radius = new(big.Rat).SetFloat64(p.Sphere.Radius)
		case p.Torus != nil:
			v.Family = "torus"
			frame = p.Torus.Frame
			radius = new(big.Rat).SetFloat64(p.Torus.MinorRadius)
			major = new(big.Rat).SetFloat64(p.Torus.MajorRadius)
		case p.Planar != nil:
			v.Family = "planar"
			out.Leaves = append(out.Leaves, v)
			continue
		default:
			return bad(geom.ErrInvalid)
		}
		local := a.inverse(point, u.pose.Origin, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z})
		v.local = a.inverse(local, mappedRatVec(frame.Origin()), [3][3]*big.Rat{mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis()), mappedRatVec(frame.ZAxis())})
		x, y, z := v.local[0], v.local[1], v.local[2]
		rho := a.add(a.mul(x, x), a.mul(y, y))
		r2 := a.mul(radius, radius)
		res := a.sub(rho, r2)
		v.Location = AnalyticPointOutside
		switch v.Family {
		case "cylinder":
			if res.Sign() <= 0 && z.Sign() >= 0 && z.Cmp(height) <= 0 {
				v.Location = AnalyticPointInside
				if res.Sign() == 0 {
					v.BoundaryRoles = append(v.BoundaryRoles, "wall")
				}
				if z.Sign() == 0 {
					v.BoundaryRoles = append(v.BoundaryRoles, "lower-cap")
				}
				if z.Cmp(height) == 0 {
					v.BoundaryRoles = append(v.BoundaryRoles, "upper-cap")
				}
			}
		case "sphere":
			res = a.add(res, a.mul(z, z))
			if res.Sign() <= 0 {
				v.Location = AnalyticPointInside
				if res.Sign() == 0 {
					v.BoundaryRoles = []string{"wall"}
				}
			}
		case "torus":
			R2 := a.mul(major, major)
			s := a.sub(a.add(a.add(rho, a.mul(z, z)), R2), r2)
			res = a.sub(a.mul(s, s), a.mul(big.NewRat(4, 1), a.mul(R2, rho)))
			if res.Sign() <= 0 {
				v.Location = AnalyticPointInside
				if res.Sign() == 0 {
					v.BoundaryRoles = []string{"wall"}
				}
			}
		}
		if len(v.BoundaryRoles) > 0 {
			v.Location = AnalyticPointBoundary
		}
		if a.err != nil {
			return bad(a.err)
		}
		out.Leaves = append(out.Leaves, v)
	}
	return out, nil
}
