// Package offset implements regularized L-infinity offsets of orthogonal XY
// footprints extruded between common Z planes. Positive distance dilates by an
// axis-aligned square; negative distance erodes by that square. Z is unchanged.
package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/csg/prism"
	"plainsolid/geom"
	"plainsolid/solid"
	"sort"
)

var ErrInvalid = errors.New("offset: invalid orthogonal footprint or options")
var ErrLimit = errors.New("offset: resource budget exceeded")
var ErrPrecision = errors.New("offset: coordinate conversion loses required precision or order")

// Options bounds input sides (default 256, maximum 2048), each coordinate grid
// (default 262144 cells), and work (default 20000000). MaxWork separately applies
// to the exact grid phase and each downstream cell-solid conversion pass.
// MaxCoordinateError defaults to 1e-9 model units; no exact coordinate ordering
// may collapse regardless of that bound. Validation/conversion passes are
// bounded but not internally cancellable; exact grid loops check context.
type Options struct {
	MaxSegments, MaxCells, MaxWork int
	MaxCoordinateError             float64
}

// Rectangle is an exact positive-area cell in a disjoint footprint cover.
// Coordinates are caller-owned. Adjacent cells may share boundary edges.
type Rectangle struct{ Min, Max [2]*big.Rat }

// Result contains an authoritative exact footprint and its rounded planar BRep.
// Empty offsets have no cells and a zero-value Solid. ExactArea excludes Z;
// extrusion planes remain the supplied float64 values. MaxDisplacement is the
// greatest rounded XY coordinate error (one coordinate, not Euclidean distance).
type Result struct {
	Solid           brep.Solid
	Cells           []Rectangle
	ExactArea       *big.Rat
	MaxDisplacement float64
}
type q = *big.Rat
type pt struct{ x, y q }

func num(x float64) q       { return new(big.Rat).SetFloat64(x) }
func add(a, b q) q          { return new(big.Rat).Add(a, b) }
func sub(a, b q) q          { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q          { return new(big.Rat).Mul(a, b) }
func mid(a, b q) q          { return new(big.Rat).Quo(add(a, b), big.NewRat(2, 1)) }
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

type budget struct {
	ctx       context.Context
	remaining int
}

func (b *budget) tick(n int) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if n > b.remaining {
		return ErrLimit
	}
	b.remaining -= n
	return nil
}
func unique(a []q) []q {
	sort.Slice(a, func(i, j int) bool { return a[i].Cmp(a[j]) < 0 })
	out := a[:0]
	for _, x := range a {
		if len(out) == 0 || x.Cmp(out[len(out)-1]) != 0 {
			out = append(out, x)
		}
	}
	return out
}
func contains(r []pt, p pt, b *budget) (bool, error) {
	inside := false
	for i, a := range r {
		if e := b.tick(1); e != nil {
			return false, e
		}
		c := r[(i+1)%len(r)]
		if a.x.Cmp(c.x) != 0 {
			continue
		}
		if (a.y.Cmp(p.y) > 0) != (c.y.Cmp(p.y) > 0) && a.x.Cmp(p.x) > 0 {
			inside = !inside
		}
	}
	return inside, nil
}
func occupied(profiles [][][]pt, p pt, b *budget) (bool, error) {
	for _, profile := range profiles {
		in, e := contains(profile[0], p, b)
		if e != nil {
			return false, e
		}
		if !in {
			continue
		}
		for _, hole := range profile[1:] {
			h, e := contains(hole, p, b)
			if e != nil {
				return false, e
			}
			if h {
				in = false
				break
			}
		}
		if in {
			return true, nil
		}
	}
	return false, nil
}
func gridSize(x, y, limit int) (int, error) {
	if x <= 0 || y <= 0 {
		return 0, nil
	}
	if x > limit/y {
		return 0, ErrLimit
	}
	return x * y, nil
}

// Compute offsets the union of profiles. Each input profile must have a simple
// orthogonal outer loop and disjoint strictly contained orthogonal holes; either
// winding is accepted. Different profiles may overlap. Result topology can merge,
// split, close holes, or vanish. Point-only connections fail BRep validation.
// Exact cells describe square-metric morphology; this is not a Euclidean round
// offset or an arbitrary-plane/curved-surface offset.
func Compute(ctx context.Context, input []prism.Profile, zMin, zMax, distance float64, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil || !finite(zMin) || !finite(zMax) || !(zMin < zMax) || !finite(zMax-zMin) || !finite(distance) || o.MaxSegments < 0 || o.MaxSegments > 2048 || o.MaxCells < 0 || o.MaxWork < 0 || !finite(o.MaxCoordinateError) || o.MaxCoordinateError < 0 {
		return empty, ErrInvalid
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 256
	}
	if o.MaxCells == 0 {
		o.MaxCells = 262144
	}
	if o.MaxWork == 0 {
		o.MaxWork = 20000000
	}
	if o.MaxCoordinateError == 0 {
		o.MaxCoordinateError = 1e-9
	}
	b := &budget{ctx: ctx, remaining: o.MaxWork}
	if e := b.tick(1); e != nil {
		return empty, e
	}
	var profiles [][][]pt
	var xs, ys []q
	segments := 0
	for _, profile := range input {
		var rings [][]pt
		var outer []geom.Vec3
		var holes [][]geom.Vec3
		for j, ring := range append([][]geom.Vec2{profile.Outer}, profile.Holes...) {
			if len(ring) < 4 {
				return empty, ErrInvalid
			}
			if len(ring) > o.MaxSegments-segments {
				return empty, ErrLimit
			}
			segments += len(ring)
			r := make([]pt, len(ring))
			v := make([]geom.Vec3, len(ring))
			for i, p := range ring {
				if e := b.tick(1); e != nil {
					return empty, e
				}
				next := ring[(i+1)%len(ring)]
				if !finite(p.X) || !finite(p.Y) || (p.X == next.X) == (p.Y == next.Y) {
					return empty, ErrInvalid
				}
				r[i] = pt{num(p.X), num(p.Y)}
				v[i] = geom.Vec3{X: p.X, Y: p.Y}
				xs = append(xs, r[i].x)
				ys = append(ys, r[i].y)
			}
			rings = append(rings, r)
			if j == 0 {
				outer = v
			} else {
				holes = append(holes, v)
			}
		}
		if _, e := brep.PrismWithHoles(outer, holes, geom.Vec3{Z: 1}); e != nil {
			return empty, fmt.Errorf("%w: %v", ErrInvalid, e)
		}
		profiles = append(profiles, rings)
	}
	result := Result{ExactArea: new(big.Rat)}
	if len(input) == 0 {
		return result, nil
	}
	xs, ys = unique(xs), unique(ys)
	nx, ny := len(xs)-1, len(ys)-1
	size, e := gridSize(nx, ny, o.MaxCells)
	if e != nil {
		return empty, e
	}
	occ := make([]bool, size)
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			inside, e := occupied(profiles, pt{mid(xs[i], xs[i+1]), mid(ys[j], ys[j+1])}, b)
			if e != nil {
				return empty, e
			}
			occ[i*ny+j] = inside
		}
	}
	radius := num(math.Abs(distance))
	var ox, oy []q
	for _, x := range xs {
		ox = append(ox, sub(x, radius), add(x, radius))
	}
	for _, y := range ys {
		oy = append(oy, sub(y, radius), add(y, radius))
	}
	ox, oy = unique(ox), unique(oy)
	mx, my := len(ox)-1, len(oy)-1
	size, e = gridSize(len(ox), len(oy), o.MaxCells)
	if e != nil {
		return empty, e
	}
	deltas := make([]int, size)
	stride := len(oy)
	mark := func(x0, x1, y0, y1 q) {
		i0 := sort.Search(len(ox), func(i int) bool { return ox[i].Cmp(x0) >= 0 })
		i1 := sort.Search(len(ox), func(i int) bool { return ox[i].Cmp(x1) >= 0 })
		j0 := sort.Search(len(oy), func(i int) bool { return oy[i].Cmp(y0) >= 0 })
		j1 := sort.Search(len(oy), func(i int) bool { return oy[i].Cmp(y1) >= 0 })
		deltas[i0*stride+j0]++
		deltas[i1*stride+j0]--
		deltas[i0*stride+j1]--
		deltas[i1*stride+j1]++
	}
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			if e := b.tick(1); e != nil {
				return empty, e
			}
			use := occ[i*ny+j]
			if distance < 0 {
				use = !use
			}
			if use {
				mark(sub(xs[i], radius), add(xs[i+1], radius), sub(ys[j], radius), add(ys[j+1], radius))
			}
		}
	}
	for i := 0; i <= mx; i++ {
		for j := 0; j <= my; j++ {
			if e := b.tick(1); e != nil {
				return empty, e
			}
			k := i*stride + j
			if i > 0 {
				deltas[k] += deltas[k-stride]
			}
			if j > 0 {
				deltas[k] += deltas[k-1]
			}
			if i > 0 && j > 0 {
				deltas[k] -= deltas[k-stride-1]
			}
		}
	}
	fill := make([]bool, mx*my)
	inset := [4]q{add(xs[0], radius), sub(xs[nx], radius), add(ys[0], radius), sub(ys[ny], radius)}
	for i := 0; i < mx; i++ {
		for j := 0; j < my; j++ {
			if e := b.tick(1); e != nil {
				return empty, e
			}
			yes := deltas[i*stride+j] > 0
			if distance < 0 {
				yes = !yes && ox[i].Cmp(inset[0]) >= 0 && ox[i+1].Cmp(inset[1]) <= 0 && oy[j].Cmp(inset[2]) >= 0 && oy[j+1].Cmp(inset[3]) <= 0
			}
			fill[i*my+j] = yes
		}
	}
	// Merge equal horizontal runs across consecutive rows. This preserves exact
	// coverage while avoiding one extruded box for every fine arrangement cell.
	type rect struct{ x0, x1, y0, y1 int }
	var rects []rect
	active := map[[2]int]int{}
	for j := 0; j < my; j++ {
		next := map[[2]int]int{}
		for i := 0; i < mx; {
			if e := b.tick(1); e != nil {
				return empty, e
			}
			if !fill[i*my+j] {
				i++
				continue
			}
			start := i
			for i < mx && fill[i*my+j] {
				i++
			}
			key := [2]int{start, i}
			index, ok := active[key]
			if ok {
				rects[index].y1 = j + 1
			} else {
				index = len(rects)
				rects = append(rects, rect{start, i, j, j + 1})
			}
			next[key] = index
		}
		active = next
	}
	if len(rects) == 0 {
		return result, nil
	}
	fx, dx, e := roundedAxis(ox, o.MaxCoordinateError)
	if e != nil {
		return empty, e
	}
	fy, dy, e := roundedAxis(oy, o.MaxCoordinateError)
	if e != nil {
		return empty, e
	}
	result.MaxDisplacement = math.Max(dx, dy)
	var boxes []geom.Box
	for _, r := range rects {
		if e := b.tick(1); e != nil {
			return empty, e
		}
		result.Cells = append(result.Cells, Rectangle{Min: [2]q{new(big.Rat).Set(ox[r.x0]), new(big.Rat).Set(oy[r.y0])}, Max: [2]q{new(big.Rat).Set(ox[r.x1]), new(big.Rat).Set(oy[r.y1])}})
		result.ExactArea.Add(result.ExactArea, mul(sub(ox[r.x1], ox[r.x0]), sub(oy[r.y1], oy[r.y0])))
		boxes = append(boxes, geom.Box{Min: geom.Vec3{X: fx[r.x0], Y: fy[r.y0], Z: zMin}, Max: geom.Vec3{X: fx[r.x1], Y: fy[r.y1], Z: zMax}})
	}
	so := solid.Options{MaxCells: o.MaxCells, MaxWork: o.MaxWork}
	cellSolid, e := solid.NewWithOptions(so, boxes...)
	if e != nil {
		if errors.Is(e, solid.ErrLimit) {
			return empty, ErrLimit
		}
		return empty, e
	}
	if e := b.tick(1); e != nil {
		return empty, e
	}
	result.Solid, e = cellSolid.BRep(so)
	if e != nil {
		if errors.Is(e, solid.ErrLimit) {
			return empty, ErrLimit
		}
		return empty, fmt.Errorf("offset result: %w", e)
	}
	if e := b.tick(1); e != nil {
		return empty, e
	}
	return result, nil
}
func roundedAxis(axis []q, tolerance float64) ([]float64, float64, error) {
	out := make([]float64, len(axis))
	maxError := new(big.Rat)
	tol := num(tolerance)
	for i, x := range axis {
		f, _ := x.Float64()
		if !finite(f) || i > 0 && f <= out[i-1] {
			return nil, 0, ErrPrecision
		}
		delta := new(big.Rat).Abs(sub(num(f), x))
		if delta.Cmp(tol) > 0 {
			return nil, 0, ErrPrecision
		}
		if delta.Cmp(maxError) > 0 {
			maxError = delta
		}
		out[i] = f
	}
	d, _ := maxError.Float64()
	return out, d, nil
}
