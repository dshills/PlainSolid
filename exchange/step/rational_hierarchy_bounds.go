package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// RationalHierarchyBoundsOptions controls exact square-root enclosure and work.
// Zero fields select defaults:64 fraction bits,65536 points,192 square roots,
// 2048 coefficient bits. Source limits independently bound complete reproof.
type RationalHierarchyBoundsOptions struct {
	FractionBits, MaxPointEvaluations, MaxSquareRoots, MaxCoefficientBits int
	Source                                                                Limits
}

// RationalWorldBounds owns an outward AABB and a per-axis maximum excess of
// either extreme over the true support extremum. Accessors return owned copies.
type RationalWorldBounds struct{ lower, upper, padding [3]*big.Rat }

func copyHierarchyRats(a [3]*big.Rat) (b [3]*big.Rat) {
	for i, q := range a {
		if q != nil {
			b[i] = new(big.Rat).Set(q)
		}
	}
	return
}
func (b RationalWorldBounds) Min() [3]*big.Rat                { return copyHierarchyRats(b.lower) }
func (b RationalWorldBounds) Max() [3]*big.Rat                { return copyHierarchyRats(b.upper) }
func (b RationalWorldBounds) MaximumAxisPadding() [3]*big.Rat { return copyHierarchyRats(b.padding) }

// RationalHierarchyLeafBounds binds a complete material's bounds to original
// source faces and one original occurrence path; it is not a per-face trim box.
type RationalHierarchyLeafBounds struct {
	ProductID, SourceProductID, SourceBodyID int64
	SourceFaceIDs, Path                      []int64
	Family                                   string
	Bounds                                   RationalWorldBounds
}

// RationalHierarchyBounds reports complete bounded work only. An error returns
// no partial leaves. Coordinates remain in the source common LengthUnit.
type RationalHierarchyBounds struct {
	Leaves                                      []RationalHierarchyLeafBounds
	Aggregate                                   RationalWorldBounds
	LengthUnit                                  float64
	FractionBits, PointEvaluations, SquareRoots int
}

// BoundRationalSphericalHierarchy re-proves source/native/pose claims once and
// computes outward exact rational support bounds without baking local geometry.
func BoundRationalSphericalHierarchy(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyBoundsOptions) (RationalHierarchyBounds, error) {
	return boundRationalHierarchy(ctx, m, o, false)
}

// BoundOCCTRationalSphericalHierarchy applies the same query under the separately
// proved pinned-OCCT inverse(from)*to convention.
func BoundOCCTRationalSphericalHierarchy(ctx context.Context, m OCCTRationalSphericalHierarchy, o RationalHierarchyBoundsOptions) (RationalHierarchyBounds, error) {
	return boundRationalHierarchy(ctx, RationalSphericalHierarchy(m), o, true)
}
func boundRationalHierarchy(ctx context.Context, m RationalSphericalHierarchy, o RationalHierarchyBoundsOptions, occt bool) (RationalHierarchyBounds, error) {
	bad := func() (RationalHierarchyBounds, error) { return RationalHierarchyBounds{}, geom.ErrInvalid }
	if ctx == nil {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return RationalHierarchyBounds{}, e
	}
	if o.FractionBits == 0 {
		o.FractionBits = 64
	}
	if o.MaxPointEvaluations == 0 {
		o.MaxPointEvaluations = 65536
	}
	if o.MaxSquareRoots == 0 {
		o.MaxSquareRoots = 192
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 2048
	}
	if o.FractionBits < 1 || o.FractionBits > 128 || o.MaxPointEvaluations < 1 || o.MaxPointEvaluations > 1048576 || o.MaxSquareRoots < 1 || o.MaxSquareRoots > 192 || o.MaxCoefficientBits < 256 || o.MaxCoefficientBits > 4096 {
		return bad()
	}
	actual, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m, occt)
	if e != nil {
		return RationalHierarchyBounds{}, e
	}
	proof, e := readRationalSphericalHierarchy(ctx, bytes.NewBufferString(actual.Source), o.Source, actual.LocalProof.Proof.Tolerance, occt)
	if e != nil {
		return RationalHierarchyBounds{}, e
	}
	expected, d, e := rationalSphericalHierarchyEnvelopeFor(ctx, proof, occt)
	if e != nil {
		return RationalHierarchyBounds{}, e
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad()
	}
	out := RationalHierarchyBounds{LengthUnit: proof.LengthUnit, FractionBits: o.FractionBits}
	check := func(q *big.Rat) error {
		if q == nil || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return nil
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return RationalHierarchyBounds{}, e
		}
		p := proof.Products[u.ProductID]
		leaf := RationalHierarchyLeafBounds{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...)}
		body, e := simple(proof.Source, p.SourceBodyID, "MANIFOLD_SOLID_BREP", 2)
		if e != nil {
			return bad()
		}
		shell, e := simple(proof.Source, body.Args[1].Ref, "CLOSED_SHELL", 2)
		if e != nil {
			return bad()
		}
		leaf.SourceFaceIDs, e = listRefs(shell.Args[1], 1)
		if e != nil {
			return bad()
		}
		matrix := u.Matrix()
		apply := func(v [3]*big.Rat, point bool) (q [3]*big.Rat, err error) {
			if point {
				out.PointEvaluations++
				if out.PointEvaluations > o.MaxPointEvaluations {
					return q, geom.ErrConvergence
				}
			}
			for _, x := range v {
				if err = check(x); err != nil {
					return
				}
			}
			for i := range q {
				q[i] = new(big.Rat)
				if point {
					q[i].Set(matrix[i][3])
				}
				for j, x := range v {
					term := new(big.Rat).Mul(matrix[i][j], x)
					if err = check(term); err != nil {
						return
					}
					q[i].Add(q[i], term)
					if err = check(q[i]); err != nil {
						return
					}
				}
				if err = check(q[i]); err != nil {
					return
				}
			}
			return
		}
		for i := 0; i < 3; i++ {
			leaf.Bounds.padding[i] = new(big.Rat)
		}
		if p.Planar != nil {
			leaf.Family = "planar"
			for _, v := range p.Planar.Solid.Vertices() {
				if e = ctx.Err(); e != nil {
					return RationalHierarchyBounds{}, e
				}
				q, e := apply(mappedRatVec(v), true)
				if e != nil {
					return RationalHierarchyBounds{}, e
				}
				for i, x := range q {
					if leaf.Bounds.lower[i] == nil || x.Cmp(leaf.Bounds.lower[i]) < 0 {
						leaf.Bounds.lower[i] = new(big.Rat).Set(x)
					}
					if leaf.Bounds.upper[i] == nil || x.Cmp(leaf.Bounds.upper[i]) > 0 {
						leaf.Bounds.upper[i] = new(big.Rat).Set(x)
					}
				}
			}
		} else {
			var frame geom.Frame
			var radius, height, minor *big.Rat
			switch {
			case p.Cylinder != nil:
				leaf.Family = "cylinder"
				frame = p.Cylinder.Frame
				radius = new(big.Rat).SetFloat64(p.Cylinder.Radius)
				height = new(big.Rat).SetFloat64(p.Cylinder.Height)
			case p.Torus != nil:
				leaf.Family = "torus"
				frame = p.Torus.Frame
				radius = new(big.Rat).SetFloat64(p.Torus.MajorRadius)
				minor = new(big.Rat).SetFloat64(p.Torus.MinorRadius)
			case p.Sphere != nil:
				leaf.Family = "sphere"
				frame = p.Sphere.Frame
				radius = new(big.Rat).SetFloat64(p.Sphere.Radius)
			default:
				return bad()
			}
			if e = check(radius); e != nil {
				return RationalHierarchyBounds{}, e
			}
			if radius.Sign() <= 0 {
				return bad()
			}
			local := mappedRatVec(frame.Origin())
			axis := mappedRatVec(frame.ZAxis())
			half := new(big.Rat)
			if height != nil {
				if e = check(height); e != nil {
					return RationalHierarchyBounds{}, e
				}
				half.Quo(height, big.NewRat(2, 1))
				for i := range local {
					local[i].Add(local[i], new(big.Rat).Mul(half, axis[i]))
				}
			}
			center, e := apply(local, true)
			if e != nil {
				return RationalHierarchyBounds{}, e
			}
			axis, e = apply(axis, false)
			if e != nil {
				return RationalHierarchyBounds{}, e
			}
			norm := new(big.Rat)
			for _, x := range axis {
				term := new(big.Rat).Mul(x, x)
				if e = check(term); e != nil {
					return RationalHierarchyBounds{}, e
				}
				norm.Add(norm, term)
				if e = check(norm); e != nil {
					return RationalHierarchyBounds{}, e
				}
			}
			if norm.Cmp(big.NewRat(1, 1)) != 0 {
				return bad()
			}
			for i := 0; i < 3; i++ {
				extent := new(big.Rat).Set(radius)
				if leaf.Family != "sphere" {
					out.SquareRoots++
					if out.SquareRoots > o.MaxSquareRoots {
						return RationalHierarchyBounds{}, geom.ErrConvergence
					}
					q := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(axis[i], axis[i]))
					lo, hi, e := hierarchySqrtEnclosure(q, o.FractionBits, o.MaxCoefficientBits)
					if e != nil {
						return RationalHierarchyBounds{}, e
					}
					extent.Mul(radius, hi)
					leaf.Bounds.padding[i].Mul(radius, new(big.Rat).Sub(hi, lo))
					if leaf.Family == "cylinder" {
						extent.Add(extent, new(big.Rat).Mul(half, new(big.Rat).Abs(axis[i])))
					} else {
						if e = check(minor); e != nil {
							return RationalHierarchyBounds{}, e
						}
						extent.Add(extent, minor)
					}
				}
				leaf.Bounds.lower[i] = new(big.Rat).Sub(center[i], extent)
				leaf.Bounds.upper[i] = new(big.Rat).Add(center[i], extent)
			}
		}
		for i := 0; i < 3; i++ {
			for _, q := range []*big.Rat{leaf.Bounds.lower[i], leaf.Bounds.upper[i], leaf.Bounds.padding[i]} {
				if e = check(q); e != nil {
					return RationalHierarchyBounds{}, e
				}
			}
			if out.Aggregate.lower[i] == nil || leaf.Bounds.lower[i].Cmp(out.Aggregate.lower[i]) < 0 {
				out.Aggregate.lower[i] = new(big.Rat).Set(leaf.Bounds.lower[i])
			}
			if out.Aggregate.upper[i] == nil || leaf.Bounds.upper[i].Cmp(out.Aggregate.upper[i]) > 0 {
				out.Aggregate.upper[i] = new(big.Rat).Set(leaf.Bounds.upper[i])
			}
			if out.Aggregate.padding[i] == nil || leaf.Bounds.padding[i].Cmp(out.Aggregate.padding[i]) > 0 {
				out.Aggregate.padding[i] = new(big.Rat).Set(leaf.Bounds.padding[i])
			}
		}
		out.Leaves = append(out.Leaves, leaf)
	}
	if e = ctx.Err(); e != nil {
		return RationalHierarchyBounds{}, e
	}
	return out, nil
}

// Integer square roots give a proof, not a floating estimate. Perfect rational
// squares stay exact; otherwise the outward enclosure width is 2^-bits.
func hierarchySqrtEnclosure(q *big.Rat, bits, maxBits int) (*big.Rat, *big.Rat, error) {
	if q == nil || q.Sign() < 0 || bits < 1 || bits > 128 || q.Num().BitLen() > maxBits || q.Denom().BitLen() > maxBits {
		return nil, nil, geom.ErrConvergence
	}
	n, d := new(big.Int).Sqrt(q.Num()), new(big.Int).Sqrt(q.Denom())
	if new(big.Int).Mul(n, n).Cmp(q.Num()) == 0 && new(big.Int).Mul(d, d).Cmp(q.Denom()) == 0 {
		r := new(big.Rat).SetFrac(n, d)
		return r, new(big.Rat).Set(r), nil
	}
	scaled := new(big.Int).Lsh(new(big.Int).Set(q.Num()), uint(2*bits))
	if scaled.BitLen() > maxBits {
		return nil, nil, geom.ErrConvergence
	}
	scaled.Quo(scaled, q.Denom())
	n.Sqrt(scaled)
	den := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	lo := new(big.Rat).SetFrac(n, den)
	hi := new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), den)
	return lo, hi, nil
}
