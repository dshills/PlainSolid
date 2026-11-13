package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type AlgebraicShellVolumeOptions struct {
	Shells                               SplineShellBooleanOptions
	MaxDepth, MaxCells, TubeSubdivisions int
}

type AlgebraicShellVolumeBounds struct {
	Volume                       geom.ExactParameterInterval
	VisitedCells, AmbiguousCells int
}

// BoundVolume encloses the signed volume of the original curved Boolean.
// Exact tube enclosures cover every potential cut in each UV chart. A cell
// clear of that cover is classified with a certified ray from an exact
// point on its original support. The oriented flux interval of every
// unclassified cell is charged conservatively, including zero. Increasing
// the depth and tube subdivisions can tighten the enclosure.
func (s AlgebraicShellBoolean) BoundVolume(ctx context.Context, o AlgebraicShellVolumeOptions) (out AlgebraicShellVolumeBounds, err error) {
	if ctx == nil || s.source == nil || o.MaxDepth < 0 || o.MaxDepth > 16 || o.MaxCells < 0 || o.MaxCells > 262144 ||
		o.TubeSubdivisions < 0 || o.TubeSubdivisions > 128 || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return out, geom.ErrInvalid
	}
	defer func() {
		if err != nil {
			out.Volume = geom.ExactParameterInterval{}
		}
	}()
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
	out.Volume = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
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
		for _, x := range b.Segments {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			_, domain, _, e := x.ExactFreeDomain()
			if e != nil {
				return out, e
			}
			lo, _ := domain.Lower.Float64()
			hi, _ := domain.Upper.Float64()
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
			// The full-cell flux enclosure also bounds the integrand
			// pointwise (depth zero except exact source knot splits).
			// Split knots explicitly so a clipped subset of a cell may
			// independently contribute any measure between zero and full.
			lo, hi, e := booleanVolumeCellFlux(ctx, x.p, origin, x.state == 0, o.Shells.Embedding.Patches)
			if e != nil {
				return out, e
			}
			if x.state == 0 {
				out.AmbiguousCells++
			}
			if sources[op].faces[x.id].Reversed != (s.op == parametric.Difference && op == 1) {
				lo, hi = hi.Neg(hi), lo.Neg(lo)
			}
			out.Volume.Lower.Add(out.Volume.Lower, lo)
			out.Volume.Upper.Add(out.Volume.Upper, hi)
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
			a, b, e := geom.BoundSurfaceOrientedVolumeFlux(ctx, q, origin, 0,
				geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
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
