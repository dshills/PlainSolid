package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
)

// HierarchyNearestBoundaryPoint owns one exact world point and all original
// incident face/edge/vertex identities. Distinct tied points remain distinct.
type HierarchyNearestBoundaryPoint struct {
	SourceFaceIDs, SourceEdgeIDs, SourceVertexIDs []int64
	point                                         [3]*big.Rat
}

func (p HierarchyNearestBoundaryPoint) Point() [3]*big.Rat { return copyHierarchyRats(p.point) }

// HierarchyPlanarDistanceLeaf distinguishes boundary distance from material
// distance: an interior point has zero material distance but positive boundary
// distance. NearestBoundary always describes boundary points, even inside.
type HierarchyPlanarDistanceLeaf struct {
	ProductID, SourceProductID, SourceBodyID int64
	Path                                     []int64
	Supported                                bool
	UnsupportedReason                        string
	Location                                 AnalyticPointLocation
	NearestBoundary                          []HierarchyNearestBoundaryPoint
	boundarySquared, materialSquared         *big.Rat
}

func (p HierarchyPlanarDistanceLeaf) BoundarySquaredDistance() *big.Rat {
	if p.boundarySquared == nil {
		return nil
	}
	return new(big.Rat).Set(p.boundarySquared)
}
func (p HierarchyPlanarDistanceLeaf) MaterialSquaredDistance() *big.Rat {
	if p.materialSquared == nil {
		return nil
	}
	return new(big.Rat).Set(p.materialSquared)
}

// RationalHierarchyPlanarDistance contains no aggregate minimum: unsupported
// analytic leaves prevent inferring a whole-assembly distance from this subset.
type RationalHierarchyPlanarDistance struct {
	Leaves                                                []HierarchyPlanarDistanceLeaf
	LengthUnit                                            float64
	PlanarTests, RayDirections, Candidates, NearestPoints int
}
type RationalHierarchyPlanarDistanceOptions struct {
	MaxCoefficientBits, MaxPlanarTests, MaxRayDirections, MaxCandidates, MaxNearestPoints int
	Source                                                                                Limits
}

// DistanceRationalHierarchyPlanarMaterials authenticates and measures general
// finite planar materials, including holes and nonconvex faces, exactly.
func DistanceRationalHierarchyPlanarMaterials(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error) {
	return distanceHierarchyPlanar(ctx, m, p, o, false)
}

// DistanceOCCTRationalHierarchyPlanarMaterials uses the explicit OCCT frame rule.
func DistanceOCCTRationalHierarchyPlanarMaterials(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error) {
	return distanceHierarchyPlanar(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// PlanarDistance retains original affine-pcurve material face/edge/vertex IDs.
func (m AffinePCurveHierarchy) PlanarDistance(ctx context.Context, p [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions) (RationalHierarchyPlanarDistance, error) {
	return distanceHierarchyPlanar(ctx, m.model, p, o, m.vendor)
}
func distanceHierarchyPlanar(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyPlanarDistanceOptions, vendor bool) (RationalHierarchyPlanarDistance, error) {
	bad := func(e error) (RationalHierarchyPlanarDistance, error) { return RationalHierarchyPlanarDistance{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxPlanarTests == 0 {
		o.MaxPlanarTests = 65536
	}
	if o.MaxRayDirections == 0 {
		o.MaxRayDirections = 256
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 65536
	}
	if o.MaxNearestPoints == 0 {
		o.MaxNearestPoints = 4096
	}
	if o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 8192 || o.MaxPlanarTests < 1 || o.MaxPlanarTests > 1048576 || o.MaxRayDirections < 1 || o.MaxRayDirections > 4096 || o.MaxCandidates < 1 || o.MaxCandidates > 1048576 || o.MaxNearestPoints < 1 || o.MaxNearestPoints > 65536 {
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
	out := RationalHierarchyPlanarDistance{LengthUnit: proof.LengthUnit}
	prepared := map[int64]*hierarchyGeneralPlanarProof{}
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > o.MaxPlanarTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	classify := func(g *hierarchyGeneralPlanarProof, p [3]*big.Rat) (AnalyticPointLocation, []int64, []int64, []int64, error) {
		where, fs, es, vs, n, e := g.classify(p, &a, tick, o.MaxRayDirections-out.RayDirections)
		out.RayDirections += n
		return where, fs, es, vs, e
	}
	quotient := func(x, y *big.Rat) *big.Rat {
		if y.Sign() == 0 {
			a.err = geom.ErrInvalid
			return new(big.Rat)
		}
		return a.check(new(big.Rat).Quo(x, y))
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[u.ProductID]
		leaf := HierarchyPlanarDistanceLeaf{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...), Location: AnalyticPointUnsupported}
		if p.Planar == nil {
			leaf.UnsupportedReason = "exact planar distance only; analytic material distance not evaluated"
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		g := prepared[u.ProductID]
		if g == nil {
			body, e := simple(proof.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
			if e != nil {
				return bad(e)
			}
			shell, e := simple(proof.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
			if e != nil {
				return bad(e)
			}
			faces, e := listRefs(shell.Args[1], 1)
			if e != nil {
				return bad(e)
			}
			g, e = prepareHierarchyGeneralPlanar(ctx, p, faces, &a, tick)
			if e != nil {
				return bad(e)
			}
			prepared[u.ProductID] = g
		}
		local := a.inverse(point, u.pose.Origin, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z})
		if a.err != nil {
			return bad(a.err)
		}
		leaf.Location, _, _, _, e = classify(g, local)
		if e != nil {
			return bad(e)
		}
		var best *big.Rat
		nearest := map[string][3]*big.Rat{}
		candidate := func(q [3]*big.Rat) error {
			delta := a.planarSub(q, local)
			squared := a.planarDot(delta, delta)
			if a.err != nil {
				return a.err
			}
			cmp := -1
			if best != nil {
				cmp = squared.Cmp(best)
			}
			if cmp < 0 {
				best = new(big.Rat).Set(squared)
				nearest = map[string][3]*big.Rat{}
			}
			if cmp <= 0 {
				key := q[0].RatString() + "," + q[1].RatString() + "," + q[2].RatString()
				nearest[key] = copyHierarchyRats(q)
				if len(nearest) > o.MaxNearestPoints {
					return geom.ErrConvergence
				}
			}
			return nil
		}
		charge := func() error {
			out.Candidates++
			if out.Candidates > o.MaxCandidates {
				return geom.ErrConvergence
			}
			return ctx.Err()
		}
		// An interior closest point of a finite planar face is its orthogonal
		// projection. If that projection falls in a hole/outside, a closest point
		// lies on a closed original boundary segment (including its endpoints).
		for _, face := range g.faces {
			if e = charge(); e != nil {
				return bad(e)
			}
			n := face.plane.n
			norm := a.planarDot(n, n)
			if norm.Sign() <= 0 {
				return bad(geom.ErrInvalid)
			}
			t := quotient(a.add(a.planarDot(n, local), face.plane.c), norm)
			var q [3]*big.Rat
			for i := range q {
				q[i] = a.sub(local[i], a.mul(t, n[i]))
			}
			if a.err != nil {
				return bad(a.err)
			}
			inside, e := g.finiteFace(face, q, &a, tick)
			if e != nil {
				return bad(e)
			}
			if inside >= 0 {
				if e = candidate(q); e != nil {
					return bad(e)
				}
			}
		}
		for _, edge := range g.base.edges {
			if e = charge(); e != nil {
				return bad(e)
			}
			v, w := g.base.vertices[edge.Start], g.base.vertices[edge.End]
			d := a.planarSub(w, v)
			den := a.planarDot(d, d)
			if den.Sign() <= 0 {
				return bad(geom.ErrInvalid)
			}
			t := quotient(a.planarDot(a.planarSub(local, v), d), den)
			if t.Sign() < 0 {
				t = new(big.Rat)
			} else if t.Cmp(big.NewRat(1, 1)) > 0 {
				t = big.NewRat(1, 1)
			}
			var q [3]*big.Rat
			for i := range q {
				q[i] = a.add(v[i], a.mul(t, d[i]))
			}
			if e = candidate(q); e != nil {
				return bad(e)
			}
		}
		if a.err != nil {
			return bad(a.err)
		}
		if best == nil {
			return bad(geom.ErrInvalid)
		}
		leaf.boundarySquared = new(big.Rat).Set(best)
		leaf.materialSquared = new(big.Rat).Set(best)
		if leaf.Location == AnalyticPointInside {
			leaf.materialSquared.SetInt64(0)
		}
		if (leaf.Location == AnalyticPointBoundary) != (best.Sign() == 0) {
			return bad(geom.ErrInvalid)
		}
		ordered := make([][3]*big.Rat, 0, len(nearest))
		for _, q := range nearest {
			ordered = append(ordered, q)
		}
		sort.Slice(ordered, func(i, j int) bool {
			for k := 0; k < 3; k++ {
				if c := ordered[i][k].Cmp(ordered[j][k]); c != 0 {
					return c < 0
				}
			}
			return false
		})
		for _, q := range ordered {
			out.NearestPoints++
			if out.NearestPoints > o.MaxNearestPoints {
				return bad(geom.ErrConvergence)
			}
			where, fs, es, vs, e := classify(g, q)
			if e != nil {
				return bad(e)
			}
			if where != AnalyticPointBoundary || len(fs) == 0 {
				return bad(geom.ErrInvalid)
			}
			world := copyHierarchyRats(u.pose.Origin)
			axes := [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z}
			for i := range world {
				for j := 0; j < 3; j++ {
					world[i] = a.add(world[i], a.mul(axes[j][i], q[j]))
				}
			}
			if a.err != nil {
				return bad(a.err)
			}
			leaf.NearestBoundary = append(leaf.NearestBoundary, HierarchyNearestBoundaryPoint{append([]int64(nil), fs...), append([]int64(nil), es...), append([]int64(nil), vs...), copyHierarchyRats(world)})
		}
		sort.Slice(leaf.NearestBoundary, func(i, j int) bool {
			for k := 0; k < 3; k++ {
				if c := leaf.NearestBoundary[i].point[k].Cmp(leaf.NearestBoundary[j].point[k]); c != 0 {
					return c < 0
				}
			}
			return false
		})
		leaf.Supported = true
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
