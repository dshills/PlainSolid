package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicShellMeasureOptions controls conservative subdivision of the
// original face charts for certified shell Boolean measures.
type AlgebraicShellMeasureOptions struct {
	Shells                               SplineShellBooleanOptions
	MaxDepth, MaxCells, TubeSubdivisions int
}

type AlgebraicShellVolumeOptions = AlgebraicShellMeasureOptions

type AlgebraicShellVolumeBounds struct {
	Volume                       geom.ExactParameterInterval
	VisitedCells, AmbiguousCells int
}

type booleanSurfaceCellCounts struct{ VisitedCells, AmbiguousCells int }

// BoundVolume encloses the signed volume of the original curved Boolean.
// Exact tube enclosures cover every potential cut in each UV chart. A cell
// clear of that cover is classified with a certified ray from an exact
// point on its original support. The oriented flux interval of every
// unclassified cell is charged conservatively, including zero. Increasing
// the depth and tube subdivisions can tighten the enclosure.
func (s AlgebraicShellBoolean) BoundVolume(ctx context.Context, o AlgebraicShellVolumeOptions) (out AlgebraicShellVolumeBounds, err error) {
	volume := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	counts, err := s.visitBooleanSurfaceCells(ctx, o, func(p *geom.SurfacePatch, origin geom.Vec3, partial, reversed bool) error {
		lo, hi, e := booleanVolumeCellFlux(ctx, p, origin, partial, o.Shells.Embedding.Patches)
		if e != nil {
			return e
		}
		if reversed {
			lo, hi = hi.Neg(hi), lo.Neg(lo)
		}
		volume.Lower.Add(volume.Lower, lo)
		volume.Upper.Add(volume.Upper, hi)
		return nil
	})
	out.VisitedCells, out.AmbiguousCells = counts.VisitedCells, counts.AmbiguousCells
	if err == nil {
		out.Volume = volume
	}
	return out, err
}

func (s AlgebraicShellBoolean) visitBooleanSurfaceCells(ctx context.Context, o AlgebraicShellMeasureOptions,
	visit func(*geom.SurfacePatch, geom.Vec3, bool, bool) error) (out booleanSurfaceCellCounts, err error) {
	if ctx == nil || s.source == nil || o.MaxDepth < 0 || o.MaxDepth > 16 || o.MaxCells < 0 || o.MaxCells > 262144 ||
		o.TubeSubdivisions < 0 || o.TubeSubdivisions > 128 || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 8
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if o.TubeSubdivisions == 0 {
		o.TubeSubdivisions = 8
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	if len(s.faces) == 0 {
		return out, nil
	}
	sources := [2]Solid{s.source.a, s.source.b}
	var patches [2][]*geom.SurfacePatch
	var bounds [2]geom.Box
	var cover [2][][]geom.UVBox
	for op, b := range sources {
		cover[op] = make([][]geom.UVBox, len(b.faces))
		for i, f := range b.faces {
			x, e := exactFaceSupportPatch(f, o.Shells.Embedding.Patches)
			if e != nil {
				return out, e
			}
			d, e := faceUVBounds(f)
			if e != nil {
				return out, e
			}
			x, e = x.Trim(d)
			if e != nil {
				return out, e
			}
			patches[op] = append(patches[op], x)
			b, e := x.Bounds()
			if e != nil {
				return out, e
			}
			if i == 0 {
				bounds[op] = b
			} else {
				bounds[op], e = bounds[op].Union(b)
				if e != nil {
					return out, e
				}
			}
		}
	}
	for _, b := range s.source.branches {
		if b.rational != nil {
			boxes, e := rationalShellBranchCover(ctx, b, o.TubeSubdivisions, o.Shells.Embedding.Patches)
			if e != nil {
				return out, e
			}
			for op, row := range boxes {
				cover[op][b.Faces[op]] = append(cover[op][b.Faces[op]], row...)
			}
			continue
		}
		for _, x := range b.Segments {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			_, domain, _, e := x.ExactFreeDomain()
			if e != nil {
				return out, e
			}
			lo, e := geom.RepresentedExactScalar(domain.Lower)
			if e != nil {
				return out, e
			}
			hi, e := geom.RepresentedExactScalar(domain.Upper)
			if e != nil {
				return out, e
			}
			a := lo
			for i := 1; i <= o.TubeSubdivisions; i++ {
				t := float64(i) / float64(o.TubeSubdivisions)
				z := min(hi, (1-t)*lo+t*hi)
				if i == o.TubeSubdivisions {
					z = hi
				}
				if z <= a {
					continue
				}
				q, e := x.EncloseFreeInterval(ctx, a, z, geom.SurfaceTubeIntervalOptions{MaxIterations: 4,
					MaxOperations: o.Shells.Embedding.Patches.MaxOperations, MaxCoefficientBits: o.Shells.Embedding.Patches.MaxCoefficientBits})
				if !q.HasBounds || e != nil && !errors.Is(e, geom.ErrConvergence) {
					if e == nil {
						e = geom.ErrConvergence
					}
					return out, e
				}
				for op, x := range [2]geom.UVBox{q.ParametersA, q.ParametersB} {
					cover[op][b.Faces[op]] = append(cover[op][b.Faces[op]], x)
				}
				a = z
			}
		}
	}
	var origin geom.Vec3
	if len(sources[0].vertices) > 0 {
		origin = sources[0].vertices[0]
	} else {
		origin = sources[1].vertices[0]
	}
	type cell struct {
		p          *geom.SurfacePatch
		depth, id  int
		state      int // zero unknown, negative omitted, positive retained
		candidates []geom.UVBox
	}
	for op, q := range patches {
		var stack []cell
		for i, p := range q {
			stack = append(stack, cell{p: p, id: i, candidates: cover[op][i]})
		}
		want := s.op == parametric.Intersection || s.op == parametric.Difference && op == 1
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if out.VisitedCells >= o.MaxCells {
				return out, geom.ErrConvergence
			}
			out.VisitedCells++
			x := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			d := x.p.Domain()
			if x.state == 0 && len(x.candidates) == 0 {
				if len(sources[op^1].faces) == 0 {
					x.state = -1
					if !want {
						x.state = 1
					}
				} else {
					v, e := volumeShellWitness(ctx, x.p, sources[op^1], bounds[op^1], want, o.Shells)
					if e != nil {
						return out, e
					}
					x.state = v
				}
			}
			if x.state < 0 {
				continue
			}
			if x.state == 0 && x.depth < o.MaxDepth {
				l, r := d, d
				if d.UMax-d.UMin >= d.VMax-d.VMin {
					l.UMax = d.UMin/2 + d.UMax/2
					r.UMin = l.UMax
				} else {
					l.VMax = d.VMin/2 + d.VMax/2
					r.VMin = l.VMax
				}
				for _, d := range [2]geom.ParameterRectangle{l, r} {
					a, e := x.p.Trim(d)
					if e != nil {
						return out, e
					}
					var candidates []geom.UVBox
					for _, b := range x.candidates {
						if b.Min.X <= d.UMax && b.Max.X >= d.UMin && b.Min.Y <= d.VMax && b.Max.Y >= d.VMin {
							candidates = append(candidates, b)
						}
					}
					stack = append(stack, cell{p: a, depth: x.depth + 1, id: x.id, candidates: candidates})
				}
				continue
			}
			if x.state == 0 {
				out.AmbiguousCells++
			}
			reversed := sources[op].faces[x.id].Reversed != (s.op == parametric.Difference && op == 1)
			if e := visit(x.p, origin, x.state == 0, reversed); e != nil {
				return out, e
			}
		}
	}
	return out, nil
}

func volumeShellWitness(ctx context.Context, p *geom.SurfacePatch, other Solid, bounds geom.Box, want bool, o SplineShellBooleanOptions) (int, error) {
	d := p.Domain()
	for _, q := range [][2]int64{{1, 1}, {1, 2}, {2, 1}, {2, 2}, {0, 0}} {
		at := func(a, b float64, k int64) *big.Rat {
			return new(big.Rat).Add(ratInset(a), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(b), ratInset(a)), big.NewRat(k, 2)))
		}
		j, e := p.EvaluateRational(ctx, at(d.UMin, d.UMax, q[0]), at(d.VMin, d.VMax, q[1]),
			geom.SurfacePatchEvaluationOptions{MaxOperations: o.Embedding.Patches.MaxOperations, MaxCoefficientBits: o.Embedding.Patches.MaxCoefficientBits})
		if e != nil {
			return 0, e
		}
		v, e := geom.RepresentedExactPoint(j.Point)
		if errors.Is(e, geom.ErrUnrepresentable) {
			continue
		}
		if e != nil {
			return 0, e
		}
		inside, e := classifySeparatedSplineWitness(ctx, other, bounds, v, o)
		if errors.Is(e, geom.ErrConvergence) {
			continue
		}
		if e != nil {
			return 0, e
		}
		if inside == want {
			return 1, nil
		}
		return -1, nil
	}
	return 0, nil
}

func booleanVolumeCellFlux(ctx context.Context, p *geom.SurfacePatch, origin geom.Vec3, partial bool,
	o geom.SurfacePatchOptions) (*big.Rat, *big.Rat, error) {
	return booleanCellFlux(ctx, p, origin, partial, -1, -1, o)
}

func booleanCellFlux(ctx context.Context, p *geom.SurfacePatch, origin geom.Vec3, partial bool,
	momentAxis, secondAxis int, o geom.SurfacePatchOptions) (*big.Rat, *big.Rat, error) {
	// A depth-zero flux enclosure bounds the integrand pointwise within
	// each original knot cell. A clipped subset may independently
	// contribute any measure between zero and the full cell.
	u, v := p.KnotBreaks()
	if len(u) < 2 || len(v) < 2 || (len(u)-1)*(len(v)-1) > 4096 {
		return nil, nil, geom.ErrConvergence
	}
	lo, hi := new(big.Rat), new(big.Rat)
	for i := 0; i+1 < len(u); i++ {
		for j := 0; j+1 < len(v); j++ {
			q, e := p.Trim(geom.ParameterRectangle{UMin: u[i], UMax: u[i+1], VMin: v[j], VMax: v[j+1]})
			if e != nil {
				return nil, nil, e
			}
			limit := geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
			var a, b *big.Rat
			switch {
			case momentAxis < 0:
				a, b, e = geom.BoundSurfaceOrientedVolumeFlux(ctx, q, origin, 0, limit)
			case secondAxis < 0:
				a, b, e = geom.BoundSurfaceOrientedFirstMomentFlux(ctx, q, origin, 0, momentAxis, limit)
			default:
				a, b, e = geom.BoundSurfaceOrientedSecondMomentFlux(ctx, q, origin, 0, momentAxis, secondAxis, limit)
			}
			if e != nil {
				return nil, nil, e
			}
			if !partial || a.Sign() < 0 {
				lo.Add(lo, a)
			}
			if !partial || b.Sign() > 0 {
				hi.Add(hi, b)
			}
		}
	}
	return lo, hi, nil
}
