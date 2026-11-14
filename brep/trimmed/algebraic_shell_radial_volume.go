package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// RadialTubeRegionVolume binds exact radial integration to every retained
// original face and complete closed section wire. Components follow native
// outer-branch order. The band volumes follow increasing radial order.
type RadialTubeRegionVolume struct {
	Volume     geom.RadialGraphVolume
	Components []RadialTubeRegionVolumeComponent
}
type RadialTubeRegionVolumeComponent struct {
	OriginalFaces                  [2]int
	OuterBranch, InnerBranch, Band int
}

// BoundRadialTubeRegionVolume proves the complete material is a collection of
// disks or adjacent circular annuli between two original identity-XY graphs.
// Both original supports must cover the unit chart; all retained tube boxes
// must lie there and each must identify exactly one proved circular factor.
// It rejects clipped/source-edge boundaries, nonconstant height denominators,
// ambiguous circle ownership, and any unsupported component. The supplied
// center/radii are proposals re-proved against all original coefficients.
// MaxOperations is cumulative, including original support recovery. No partial
// result is published. This does not broaden native serialization limits.
func (s AlgebraicShellBoolean) BoundRadialTubeRegionVolume(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphVolumeOptions) (RadialTubeRegionVolume, error) {
	bound, e := s.nativeRadialRegions(ctx, center, radii, o)
	if e != nil {
		return RadialTubeRegionVolume{}, e
	}
	options := bound.options
	options.MaxOperations -= bound.work
	volume, e := bound.proof.BoundVolumeBands(ctx, bound.bands, options)
	if e != nil {
		return RadialTubeRegionVolume{}, e
	}
	volume.ExactOperations += bound.work
	return RadialTubeRegionVolume{Volume: volume, Components: bound.components}, nil
}

type nativeRadialRegions struct {
	proof      *geom.SplineGraphRadialContours
	bands      []int
	components []RadialTubeRegionVolumeComponent
	work       int
	options    geom.RadialGraphVolumeOptions
}

// nativeRadialRegions is the sole original-material ownership gate shared by
// volume and projected moments. Public diagnostic tables are never authority.
func (s AlgebraicShellBoolean) nativeRadialRegions(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphVolumeOptions) (nativeRadialRegions, error) {
	zero := nativeRadialRegions{}
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 4096
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 64
	}
	if o.MaxOperations < 1 || o.MaxOperations > 128000000 || o.MaxCoefficientBits < 64 || o.MaxCoefficientBits > 16384 || o.PrecisionBits < 16 || o.PrecisionBits > 256 {
		return zero, geom.ErrInvalid
	}
	work := 0
	charge := func(n int) error {
		if n < 0 || n > o.MaxOperations-work {
			return geom.ErrConvergence
		}
		work += n
		return ctx.Err()
	}
	check := func(q *big.Rat) error {
		if q == nil {
			return geom.ErrInvalid
		}
		if q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return charge(1)
	}
	if len(radii) < 1 || len(radii) > 4 {
		return zero, geom.ErrInvalid
	}
	for _, q := range append([]*big.Rat{center[0], center[1]}, radii...) {
		if e := check(q); e != nil {
			return zero, e
		}
	}
	layouts, e := s.nativeTubeRegions(ctx)
	if e != nil {
		return zero, e
	}
	if len(layouts) == 0 {
		return zero, geom.ErrConvergence
	}
	pair := s.source.branches[layouts[0].Branches[0]].Faces
	var patches [2]*geom.SurfacePatch
	for i, f := range layouts[0].Faces {
		base, m := parametric.SurfaceMapping(f.Surface)
		if work >= o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		p, e := exactSplineSupportPatch(base, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations - work, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		if e = charge(p.ExactOperations()); e != nil {
			return zero, e
		}
		if m != geom.Identity() {
			if work >= o.MaxOperations {
				return zero, geom.ErrConvergence
			}
			var n int
			p, n, e = p.TransformedWithWorkLimit(m, o.MaxOperations-work)
			if e != nil {
				return zero, e
			}
			if e = charge(n); e != nil {
				return zero, e
			}
		}
		if work >= o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		p, n, e := p.TrimWithWorkLimit(geom.ParameterRectangle{UMin: 0, UMax: 1, VMin: 0, VMax: 1}, o.MaxOperations-work)
		if e != nil {
			return zero, e
		}
		if e = charge(n); e != nil {
			return zero, e
		}
		patches[i] = p
	}
	if work >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	proof, e := geom.CertifySplineGraphRadialContours(ctx, patches[0], patches[1], center, radii, geom.GraphPointContactOptions{MaxOperations: o.MaxOperations - work, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	if e = charge(proof.ExactOperations()); e != nil {
		return zero, e
	}
	squares := make([]*big.Rat, len(radii))
	for i, r := range radii {
		squares[i] = new(big.Rat).Mul(r, r)
		if e = check(squares[i]); e != nil {
			return zero, e
		}
	}
	// Every branch is an authentic simple closed original section. Once every
	// segment lies on one regular circle, closedness and local regularity make
	// it that whole circle. Boxes are used only for exact factor ownership,
	// never to infer a crossing or approximate a boundary.
	used := map[int]bool{}
	bands := []int{}
	result := nativeRadialRegions{}
	for _, layout := range layouts {
		if len(layout.Branches) > 2 || s.source.branches[layout.Branches[0]].Faces != pair {
			return zero, geom.ErrConvergence
		}
		indices := []int{}
		for _, branch := range layout.Branches {
			selected := -1
			for _, segment := range s.source.branches[branch].Segments {
				if e = charge(1); e != nil {
					return zero, e
				}
				if !segment.TrimA().CertifiesPairedTubeOnSurfaces(segment.TrimB(), layout.Faces[0].Surface, layout.Faces[1].Surface) {
					return zero, geom.ErrConvergence
				}
				box, e := segment.TrimB().BoundsUV()
				if e != nil {
					return zero, e
				}
				if box.Min.X < 0 || box.Min.Y < 0 || box.Max.X > 1 || box.Max.Y > 1 {
					return zero, geom.ErrConvergence
				}
				lo, hi := new(big.Rat), new(big.Rat)
				for axis, ends := range [][2]float64{{box.Min.X, box.Max.X}, {box.Min.Y, box.Max.Y}} {
					a := new(big.Rat).SetFloat64(ends[0])
					b := new(big.Rat).SetFloat64(ends[1])
					if a == nil || b == nil {
						return zero, geom.ErrInvalid
					}
					a.Sub(a, center[axis])
					b.Sub(b, center[axis])
					if e = check(a); e != nil {
						return zero, e
					}
					if e = check(b); e != nil {
						return zero, e
					}
					aa := new(big.Rat).Mul(a, a)
					bb := new(big.Rat).Mul(b, b)
					if e = check(aa); e != nil {
						return zero, e
					}
					if e = check(bb); e != nil {
						return zero, e
					}
					min, max := aa, bb
					if aa.Cmp(bb) > 0 {
						min, max = bb, aa
					}
					if a.Sign() <= 0 && b.Sign() >= 0 {
						min = new(big.Rat)
					}
					lo.Add(lo, min)
					hi.Add(hi, max)
					if e = check(lo); e != nil {
						return zero, e
					}
					if e = check(hi); e != nil {
						return zero, e
					}
				}
				hit := -1
				for i, r := range squares {
					if lo.Cmp(r) <= 0 && hi.Cmp(r) >= 0 {
						if hit >= 0 {
							return zero, geom.ErrConvergence
						}
						hit = i
					}
				}
				if hit < 0 || selected >= 0 && hit != selected {
					return zero, geom.ErrConvergence
				}
				selected = hit
			}
			if selected < 0 || used[selected] {
				return zero, geom.ErrConvergence
			}
			used[selected] = true
			indices = append(indices, selected)
		}
		band := indices[0]
		inner := -1
		if len(indices) == 1 {
			if band != 0 {
				return zero, geom.ErrConvergence
			}
		} else {
			if indices[1] != band-1 {
				return zero, geom.ErrConvergence
			}
			inner = layout.Branches[1]
		}
		sign := proof.BandSigns()[band]
		if layout.Faces[0].Reversed != (sign < 0) || layout.Faces[1].Reversed != (sign > 0) {
			return zero, geom.ErrConvergence
		}
		bands = append(bands, band)
		result.components = append(result.components, RadialTubeRegionVolumeComponent{OriginalFaces: pair, OuterBranch: layout.Branches[0], InnerBranch: inner, Band: band})
	}
	sort.Ints(bands)
	if work >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	result.proof = proof
	result.bands = bands
	result.work = work
	result.options = o
	return result, nil
}
