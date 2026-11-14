package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
)

// HierarchyBoundaryNormal is one original face's outward normal ray. Vector is
// exact but is not generally unit length. SquaredNorm permits exact comparisons
// without replacing a necessary square root by a rounded normalization.
type HierarchyBoundaryNormal struct {
	SourceFaceID, SourceSurfaceID int64
	Role                          string
	vector                        [3]*big.Rat
	squared                       *big.Rat
}

func (n HierarchyBoundaryNormal) Vector() [3]*big.Rat { return copyHierarchyRats(n.vector) }
func (n HierarchyBoundaryNormal) SquaredNorm() *big.Rat {
	if n.squared == nil {
		return nil
	}
	return new(big.Rat).Set(n.squared)
}

type HierarchyBoundaryNormalLeaf struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path, ContactEdgeIDs, ContactVertexIDs   []int64
	Family                                   string
	Location                                 AnalyticPointLocation
	Normals                                  []HierarchyBoundaryNormal
}

// RationalHierarchyNormals preserves all incident face normals, including equal
// rays owned by distinct seam faces. It does not average corners or union uses.
type RationalHierarchyNormals struct {
	Leaves                     []HierarchyBoundaryNormalLeaf
	LengthUnit                 float64
	PlanarTests, RayDirections int
}
type RationalHierarchyNormalOptions struct {
	Point      RationalHierarchyPointOptions
	MaxNormals int
}

// BoundaryNormalsRationalHierarchy returns exact outward normal rays at every
// incident authenticated face, with general planar finite-loop ownership.
func BoundaryNormalsRationalHierarchy(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
	return boundaryNormalsHierarchy(ctx, m, p, o, false)
}

// BoundaryNormalsOCCTRationalHierarchy uses the explicit OCCT frame convention.
func BoundaryNormalsOCCTRationalHierarchy(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
	return boundaryNormalsHierarchy(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// BoundaryNormals preserves the original affine-pcurve material face IDs.
func (m AffinePCurveHierarchy) BoundaryNormals(ctx context.Context, p [3]*big.Rat, o RationalHierarchyNormalOptions) (RationalHierarchyNormals, error) {
	return boundaryNormalsHierarchy(ctx, m.model, p, o, m.vendor)
}
func boundaryNormalsHierarchy(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyNormalOptions, vendor bool) (RationalHierarchyNormals, error) {
	bad := func(e error) (RationalHierarchyNormals, error) { return RationalHierarchyNormals{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxNormals == 0 {
		o.MaxNormals = 4096
	}
	if o.MaxNormals < 1 || o.MaxNormals > 65536 {
		return bad(geom.ErrInvalid)
	}
	// Own a complete source snapshot before evaluating supports. The existing
	// locator also re-proves this immutable snapshot through its unchanged API.
	actual, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m, vendor)
	if e != nil {
		return bad(e)
	}
	proof, e := readRationalSphericalHierarchy(ctx, bytes.NewBufferString(actual.Source), o.Point.Source, actual.LocalProof.Proof.Tolerance, vendor)
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
	locations, e := locateHierarchyAnalyticPoint(ctx, proof, point, o.Point, vendor, true, true)
	if e != nil {
		return bad(e)
	}
	bits := o.Point.MaxCoefficientBits
	if bits == 0 {
		bits = 4096
	}
	a := hierarchyPointArithmetic{bits: bits}
	out := RationalHierarchyNormals{LengthUnit: locations.LengthUnit, PlanarTests: locations.PlanarTests, RayDirections: locations.RayDirections}
	count := 0
	rotate := func(v [3]*big.Rat, axes [3][3]*big.Rat) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat)
			for j := 0; j < 3; j++ {
				r[i] = a.add(r[i], a.mul(axes[j][i], v[j]))
			}
		}
		return
	}
	for k, location := range locations.Leaves {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		u := proof.Instances[k]
		product := proof.Products[u.ProductID]
		leaf := HierarchyBoundaryNormalLeaf{ProductID: location.ProductID, SourceProductID: location.SourceProductID, SourceBodyID: location.SourceBodyID, Path: append([]int64(nil), location.Path...), ContactEdgeIDs: append([]int64(nil), location.ContactEdgeIDs...), ContactVertexIDs: append([]int64(nil), location.ContactVertexIDs...), Family: location.Family, Location: location.Location}
		if location.Location != AnalyticPointBoundary {
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		seen := map[int64]bool{}
		add := func(fid int64, role string, v [3]*big.Rat) error {
			if seen[fid] {
				return nil
			}
			seen[fid] = true
			count++
			if count > o.MaxNormals {
				return geom.ErrConvergence
			}
			fr, e := simple(proof.Source, fid, "ADVANCED_FACE", 4)
			if e != nil {
				return e
			}
			sid, e := ref(fr.Args[2])
			if e != nil {
				return e
			}
			world := rotate(v, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z})
			norm := a.planarDot(world, world)
			if a.err != nil {
				return a.err
			}
			if norm.Sign() <= 0 {
				return geom.ErrInvalid
			}
			leaf.Normals = append(leaf.Normals, HierarchyBoundaryNormal{fid, sid, role, copyHierarchyRats(world), new(big.Rat).Set(norm)})
			return ctx.Err()
		}
		if product.Planar != nil {
			for _, fid := range location.ContactFaceIDs {
				fr, e := simple(proof.Source, fid, "ADVANCED_FACE", 4)
				if e != nil {
					return bad(e)
				}
				pl, e := simple(proof.Source, fr.Args[2].Ref, "PLANE", 2)
				if e != nil {
					return bad(e)
				}
				f, e := mappedExactFrame(proof.Source, pl.Args[1].Ref)
				if e != nil {
					return bad(e)
				}
				n := mappedRatVec(f.ZAxis())
				same, e := boolean(fr.Args[3])
				if e != nil {
					return bad(e)
				}
				if !same {
					for i := range n {
						n[i] = a.mul(n[i], big.NewRat(-1, 1))
					}
				}
				if e = add(fid, "planar-face", n); e != nil {
					return bad(e)
				}
			}
		} else {
			local := location.AnalyticCoordinates()
			x, y, z := local[0], local[1], local[2]
			var frame geom.Frame
			switch {
			case product.Cylinder != nil:
				frame = product.Cylinder.Frame
			case product.Sphere != nil:
				frame = product.Sphere.Frame
			case product.Torus != nil:
				frame = product.Torus.Frame
			default:
				return bad(geom.ErrDomain)
			}
			axes := [3][3]*big.Rat{mappedRatVec(frame.XAxis()), mappedRatVec(frame.YAxis()), mappedRatVec(frame.ZAxis())}
			analytic := func(fid int64, role string, n [3]*big.Rat) error { return add(fid, role, rotate(n, axes)) }
			switch {
			case product.Cylinder != nil:
				cylinder := product.Cylinder
				for _, role := range location.BoundaryRoles {
					switch role {
					case "lower-cap":
						e = analytic(cylinder.NativeFaceSourceIDs[0], role, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(-1, 1)})
					case "upper-cap":
						e = analytic(cylinder.NativeFaceSourceIDs[1], role, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)})
					case "wall":
						// Closed quadrant predicates include both incident faces on a real cell
						// seam; canonical half cells map two adjacent quadrants to one face.
						quadrants := [4]bool{x.Sign() >= 0 && y.Sign() >= 0, x.Sign() <= 0 && y.Sign() >= 0, x.Sign() <= 0 && y.Sign() <= 0, x.Sign() >= 0 && y.Sign() <= 0}
						for q, hit := range quadrants {
							if hit {
								if e = analytic(cylinder.NativeFaceSourceIDs[2+q], role, [3]*big.Rat{x, y, new(big.Rat)}); e != nil {
									break
								}
							}
						}
					default:
						e = geom.ErrInvalid
					}
					if e != nil {
						return bad(e)
					}
				}
			case product.Sphere != nil:
				e = analytic(product.Sphere.SourceFaceID, "sphere", local)
			case product.Torus != nil:
				R, r := new(big.Rat).SetFloat64(product.Torus.MajorRadius), new(big.Rat).SetFloat64(product.Torus.MinorRadius)
				R2 := a.mul(R, R)
				s := a.sub(a.add(a.add(a.mul(x, x), a.mul(y, y)), a.add(a.mul(z, z), R2)), a.mul(r, r))
				t := a.sub(s, a.mul(big.NewRat(2, 1), R2))
				// One quarter of the outward gradient of the authenticated ring-torus
				// implicit polynomial. Ring radii ensure a nonzero regular gradient.
				e = analytic(product.Torus.SourceFaceID, "torus", [3]*big.Rat{a.mul(x, t), a.mul(y, t), a.mul(z, s)})
			}
			if e != nil {
				return bad(e)
			}
		}
		if len(leaf.Normals) == 0 {
			return bad(geom.ErrInvalid)
		}
		sort.Slice(leaf.Normals, func(i, j int) bool { return leaf.Normals[i].SourceFaceID < leaf.Normals[j].SourceFaceID })
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
