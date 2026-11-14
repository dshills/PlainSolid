package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// FrustumHierarchyBoundsOptions bounds exact vertex/extremum evaluations and
// rational coefficients. Source independently bounds full graph reproof.
type FrustumHierarchyBoundsOptions struct {
	MaxPointEvaluations, MaxCoefficientBits int
	Source                                  Limits
}

// BoundIndependentFrustumHierarchy returns tight rational world AABBs for every
// retained occurrence and their aggregate. Proper signed-axis frames preserve
// local extrema exactly; no square-root enclosure or geometry baking is needed.
// The boxes concern complete retained bodies, not individual trimmed faces.
func BoundIndependentFrustumHierarchy(ctx context.Context, m IndependentFrustumHierarchy, o FrustumHierarchyBoundsOptions) (RationalHierarchyBounds, error) {
	var out RationalHierarchyBounds
	bad := func(e error) (RationalHierarchyBounds, error) { return RationalHierarchyBounds{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	if o.MaxPointEvaluations == 0 {
		o.MaxPointEvaluations = 65536
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.MaxPointEvaluations < 1 || o.MaxPointEvaluations > 1048576 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 4096 {
		return bad(geom.ErrInvalid)
	}
	actual, c, e := frustumHierarchyEnvelope(ctx, m)
	if e != nil {
		return bad(e)
	}
	proof, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewBufferString(actual.Proof.Source), o.Source, actual.Proof.Tolerance)
	if e != nil {
		return bad(e)
	}
	expected, d, e := frustumHierarchyEnvelope(ctx, proof)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	out.LengthUnit = proof.Assembly.LengthUnit
	a := hierarchyPointArithmetic{bits: o.MaxCoefficientBits}
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		out.PointEvaluations++
		if out.PointEvaluations > o.MaxPointEvaluations {
			return geom.ErrConvergence
		}
		return nil
	}
	for _, u := range proof.Instances {
		p := proof.Products[u.ProductID]
		leaf := RationalHierarchyLeafBounds{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...)}
		body, e := simple(proof.Assembly.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return bad(e)
		}
		shell, e := simple(proof.Assembly.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return bad(e)
		}
		leaf.SourceFaceIDs, e = listRefs(shell.Args[1], 1)
		if e != nil {
			return bad(e)
		}
		var lo, hi [3]*big.Rat
		if p.Planar != nil {
			leaf.Family = "planar"
			for _, v := range p.Planar.Solid.Vertices() {
				if e = tick(); e != nil {
					return bad(e)
				}
				q := mappedRatVec(v)
				for j, x := range q {
					a.check(x)
					if lo[j] == nil || x.Cmp(lo[j]) < 0 {
						lo[j] = new(big.Rat).Set(x)
					}
					if hi[j] == nil || x.Cmp(hi[j]) > 0 {
						hi[j] = new(big.Rat).Set(x)
					}
				}
			}
		} else if p.Frustum != nil {
			leaf.Family = "frustum"
			q := p.Frustum.Embedding
			// Each positive-weight quadrant lies in its control square and reaches
			// both cardinal endpoints. Linear positive endpoint scale attains its
			// maximum at one cap station, including noncircular rational profiles.
			r := q.LowerRadius
			if q.UpperRadius.Cmp(r) > 0 {
				r = q.UpperRadius
			}
			lo = [3]*big.Rat{a.sub(q.CenterX, r), a.sub(q.CenterY, r), new(big.Rat).Set(q.LowerZ)}
			hi = [3]*big.Rat{a.add(q.CenterX, r), a.add(q.CenterY, r), new(big.Rat).Set(q.UpperZ)}
		} else {
			var frame geom.Frame
			var r, h, minor *big.Rat
			switch {
			case p.Cylinder != nil:
				leaf.Family = "cylinder"
				frame = p.Cylinder.Frame
				r = new(big.Rat).SetFloat64(p.Cylinder.Radius)
				h = new(big.Rat).SetFloat64(p.Cylinder.Height)
			case p.Sphere != nil:
				leaf.Family = "sphere"
				frame = p.Sphere.Frame
				r = new(big.Rat).SetFloat64(p.Sphere.Radius)
			case p.Torus != nil:
				leaf.Family = "torus"
				frame = p.Torus.Frame
				r = new(big.Rat).SetFloat64(p.Torus.MajorRadius)
				minor = new(big.Rat).SetFloat64(p.Torus.MinorRadius)
			default:
				return bad(geom.ErrDomain)
			}
			center, axis := mappedRatVec(frame.Origin()), mappedRatVec(frame.ZAxis())
			for j := 0; j < 3; j++ {
				extent := r
				if p.Cylinder != nil && axis[j].Sign() != 0 {
					if new(big.Rat).Abs(axis[j]).Cmp(big.NewRat(1, 1)) != 0 {
						return bad(geom.ErrDomain)
					}
					end := a.add(center[j], a.mul(axis[j], h))
					lo[j], hi[j] = new(big.Rat).Set(center[j]), end
					if lo[j].Cmp(hi[j]) > 0 {
						lo[j], hi[j] = hi[j], lo[j]
					}
					continue
				}
				if p.Torus != nil {
					if axis[j].Sign() == 0 {
						extent = a.add(r, minor)
					} else {
						if new(big.Rat).Abs(axis[j]).Cmp(big.NewRat(1, 1)) != 0 {
							return bad(geom.ErrDomain)
						}
						extent = minor
					}
				}
				lo[j], hi[j] = a.sub(center[j], extent), a.add(center[j], extent)
			}
		}
		if a.err != nil {
			return bad(a.err)
		}
		for j := 0; j < 3; j++ {
			if lo[j] == nil || hi[j] == nil {
				return bad(geom.ErrInvalid)
			}
			a.check(lo[j])
			a.check(hi[j])
			leaf.Bounds.padding[j] = new(big.Rat)
		}
		for mask := 0; mask < 8; mask++ {
			if e = tick(); e != nil {
				return bad(e)
			}
			for i := 0; i < 3; i++ {
				v := a.check(new(big.Rat).SetFloat64(u.Transform[i][3]))
				for j := 0; j < 3; j++ {
					x := lo[j]
					if mask&(1<<j) != 0 {
						x = hi[j]
					}
					v = a.add(v, a.mul(new(big.Rat).SetFloat64(u.Transform[i][j]), x))
				}
				if a.err != nil {
					return bad(a.err)
				}
				if leaf.Bounds.lower[i] == nil || v.Cmp(leaf.Bounds.lower[i]) < 0 {
					leaf.Bounds.lower[i] = new(big.Rat).Set(v)
				}
				if leaf.Bounds.upper[i] == nil || v.Cmp(leaf.Bounds.upper[i]) > 0 {
					leaf.Bounds.upper[i] = new(big.Rat).Set(v)
				}
			}
		}
		for i := 0; i < 3; i++ {
			if out.Aggregate.lower[i] == nil || leaf.Bounds.lower[i].Cmp(out.Aggregate.lower[i]) < 0 {
				out.Aggregate.lower[i] = new(big.Rat).Set(leaf.Bounds.lower[i])
			}
			if out.Aggregate.upper[i] == nil || leaf.Bounds.upper[i].Cmp(out.Aggregate.upper[i]) > 0 {
				out.Aggregate.upper[i] = new(big.Rat).Set(leaf.Bounds.upper[i])
			}
			out.Aggregate.padding[i] = new(big.Rat)
		}
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
