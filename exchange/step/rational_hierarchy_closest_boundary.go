package step

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// RationalHierarchyClosestOptions shares one root work budget across every
// analytic family, planar rational-root construction, and minimum comparison.
// Planar finite-face work has its own explicit cumulative quotas.
type RationalHierarchyClosestOptions struct {
	MaxOperations, MaxCoefficientBits, MaxSubdivisions                int
	MaxPlanarTests, MaxRayDirections, MaxCandidates, MaxNearestPoints int
	Source                                                            Limits
}

// HierarchyClosestBoundaryOwner retains a complete per-occurrence nearest set.
// Exactly one typed payload is nonnil. Payloads belong to this result; changing
// them cannot change the authenticated hierarchy or the immutable minimum root.
type HierarchyClosestBoundaryOwner struct {
	Planar   *HierarchyPlanarDistanceLeaf
	Sphere   *HierarchySphereDistanceLeaf
	Cylinder *HierarchyCylinderDistanceLeaf
	Torus    *HierarchyTorusDistanceLeaf
}

// RationalHierarchyClosestBoundary measures the union of retained BOUNDARIES,
// including internal surfaces of overlapping occurrences. It does not measure
// a regularized material union. Every exactly tied occurrence is retained.
type RationalHierarchyClosestBoundary struct {
	Owners                                            []HierarchyClosestBoundaryOwner
	LengthUnit                                        float64
	Operations, Subdivisions, PlanarTests, Candidates int
	squared                                           *geom.ExactAlgebraicRoot
}

// SquaredDistance returns the exact minimum in squared original model units.
func (r RationalHierarchyClosestBoundary) SquaredDistance() *geom.ExactAlgebraicRoot {
	return r.squared
}

// ClosestRationalHierarchyBoundary searches the neutral-child hierarchy.
func ClosestRationalHierarchyBoundary(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyClosestOptions) (RationalHierarchyClosestBoundary, error) {
	return closestHierarchyBoundary(ctx, m, p, o, false)
}

// ClosestOCCTRationalHierarchyBoundary searches the vendor-frame hierarchy.
func ClosestOCCTRationalHierarchyBoundary(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyClosestOptions) (RationalHierarchyClosestBoundary, error) {
	return closestHierarchyBoundary(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// ClosestBoundary searches the verified affine-pcurve occurrence boundaries.
func (m AffinePCurveHierarchy) ClosestBoundary(ctx context.Context, p [3]*big.Rat, o RationalHierarchyClosestOptions) (RationalHierarchyClosestBoundary, error) {
	return closestHierarchyBoundary(ctx, m.model, p, o, m.vendor)
}
func closestHierarchyBoundary(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyClosestOptions, vendor bool) (RationalHierarchyClosestBoundary, error) {
	var out RationalHierarchyClosestBoundary
	bad := func(e error) (RationalHierarchyClosestBoundary, error) { return RationalHierarchyClosestBoundary{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	ro, e := sphereDistanceRootOptions(geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
	if e != nil {
		return bad(e)
	}
	remaining := func() (geom.ExactCurvePlaneEventOptions, error) {
		q := ro
		q.MaxOperations -= out.Operations
		q.MaxSubdivisions -= out.Subdivisions
		if q.MaxOperations <= 0 || q.MaxSubdivisions <= 0 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	// Each existing family query independently authenticates the complete source
	// and occurrence table. No unsupported leaf may disappear from the minimum.
	pl, e := distanceHierarchyPlanar(ctx, m, p, RationalHierarchyPlanarDistanceOptions{MaxCoefficientBits: ro.MaxCoefficientBits, MaxPlanarTests: o.MaxPlanarTests, MaxRayDirections: o.MaxRayDirections, MaxCandidates: o.MaxCandidates, MaxNearestPoints: o.MaxNearestPoints, Source: o.Source}, vendor)
	if e != nil {
		return bad(e)
	}
	out.LengthUnit = pl.LengthUnit
	out.PlanarTests = pl.PlanarTests
	out.Candidates = pl.Candidates
	q, e := remaining()
	if e != nil {
		return bad(e)
	}
	sp, e := distanceHierarchySpheres(ctx, m, p, RationalHierarchySphereDistanceOptions{MaxOperations: q.MaxOperations, MaxCoefficientBits: q.MaxCoefficientBits, MaxSubdivisions: q.MaxSubdivisions, Source: o.Source}, vendor)
	if e != nil {
		return bad(e)
	}
	out.Operations += sp.Operations
	out.Subdivisions += sp.Subdivisions
	q, e = remaining()
	if e != nil {
		return bad(e)
	}
	cy, e := distanceHierarchyCylinders(ctx, m, p, RationalHierarchyCylinderDistanceOptions{MaxOperations: q.MaxOperations, MaxCoefficientBits: q.MaxCoefficientBits, MaxSubdivisions: q.MaxSubdivisions, Source: o.Source}, vendor)
	if e != nil {
		return bad(e)
	}
	out.Operations += cy.Operations
	out.Subdivisions += cy.Subdivisions
	q, e = remaining()
	if e != nil {
		return bad(e)
	}
	tr, e := distanceHierarchyTori(ctx, m, p, RationalHierarchyTorusDistanceOptions{MaxOperations: q.MaxOperations, MaxCoefficientBits: q.MaxCoefficientBits, MaxSubdivisions: q.MaxSubdivisions, Source: o.Source}, vendor)
	if e != nil {
		return bad(e)
	}
	out.Operations += tr.Operations
	out.Subdivisions += tr.Subdivisions
	n := len(pl.Leaves)
	if n == 0 || len(sp.Leaves) != n || len(cy.Leaves) != n || len(tr.Leaves) != n {
		return bad(geom.ErrInvalid)
	}
	for i := 0; i < n; i++ {
		if e := ctx.Err(); e != nil {
			return bad(e)
		}
		count := 0
		var root *geom.ExactAlgebraicRoot
		var owner HierarchyClosestBoundaryOwner
		if pl.Leaves[i].Supported {
			count++
			v := pl.Leaves[i]
			owner.Planar = &v
			q, e = remaining()
			if e != nil {
				return bad(e)
			}
			var work int
			root, work, e = geom.NewExactRationalRoot(ctx, v.BoundarySquaredDistance(), q)
			if e != nil {
				return bad(e)
			}
			out.Operations += work
		}
		if sp.Leaves[i].Supported {
			count++
			v := sp.Leaves[i]
			owner.Sphere = &v
			root = v.BoundarySquaredDistance()
		}
		if cy.Leaves[i].Supported {
			count++
			v := cy.Leaves[i]
			owner.Cylinder = &v
			root = v.BoundarySquaredDistance()
		}
		if tr.Leaves[i].Supported {
			count++
			v := tr.Leaves[i]
			owner.Torus = &v
			root = v.BoundarySquaredDistance()
		}
		if count != 1 || root == nil {
			return bad(geom.ErrDomain)
		}
		q, e = remaining()
		if e != nil {
			return bad(e)
		}
		if e = out.acceptClosest(ctx, root, owner, q); e != nil {
			return bad(e)
		}
	}
	return out, nil
}

// acceptClosest compares complete local minima; equality retains a distinct use
// even when its product or nearest coordinates coincide with another use.
func (out *RationalHierarchyClosestBoundary) acceptClosest(ctx context.Context, root *geom.ExactAlgebraicRoot, owner HierarchyClosestBoundaryOwner, q geom.ExactCurvePlaneEventOptions) error {
	if out.squared == nil {
		out.squared = root
		out.Owners = []HierarchyClosestBoundaryOwner{owner}
		return nil
	}
	cmp, e := root.Compare(ctx, out.squared, q)
	if e != nil {
		return e
	}
	out.Operations += cmp.Operations
	out.Subdivisions += cmp.Subdivisions
	if !cmp.Proven {
		return geom.ErrConvergence
	}
	if cmp.Order < 0 {
		out.squared = root
		out.Owners = []HierarchyClosestBoundaryOwner{owner}
	} else if cmp.Order == 0 {
		out.Owners = append(out.Owners, owner)
	}
	return nil
}
