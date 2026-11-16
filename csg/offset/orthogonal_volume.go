package offset

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/solid"
)

// VolumeOptions bounds exact orthogonal volume offsets. Zero selects 65,536
// source cells, 262,144 grid entries per arrangement, 20 million work units,
// and coordinate rounding bounded by 1e-9 in each axis. Arrangement grids
// are additionally capped at 4,194,304 entries. Native conversion receives
// the same grid and work limits independently.
type VolumeOptions struct {
	MaxSourceCells, MaxGridEntries, MaxWork int
	MaxCoordinateError                      float64
}

// VolumeCell is one positive-volume cuboid in the exact offset result.
// The rational coordinates belong to the caller.
type VolumeCell struct{ Min, Max [3]*big.Rat }

// VolumeResult stores the authoritative disjoint exact cells, exact volume,
// and a native planar solid. The largest rounding displacement is measured
// separately in each coordinate. Empty material has exact volume zero.
type VolumeResult struct {
	Solid           brep.Solid
	Cells           []VolumeCell
	ExactVolume     *big.Rat
	MaxDisplacement float64
}

// ComputeVolume dilates or erodes an axis-aligned cell solid in all three
// dimensions by the cube [-|distance|,|distance|]^3. Positive distance is
// dilation; negative distance is erosion. It returns regularized material
// and rejects output that cannot form an embedded manifold planar B-rep.
// This is a cubical, rather than Euclidean ball, offset.
func ComputeVolume(ctx context.Context, source solid.Solid, distance float64, o VolumeOptions) (VolumeResult, error) {
	o, err := volumeOffsetOptions(ctx, distance, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, source, distance, o, false, nil)
}

// FromOrthogonalVolume proves an outward axis-aligned B-rep and offsets its
// full volume. Different vertical intervals and internal cavities are allowed.
func FromOrthogonalVolume(ctx context.Context, source brep.Solid, distance float64, o VolumeOptions) (VolumeResult, error) {
	o, err := volumeOffsetOptions(ctx, distance, o)
	if err != nil {
		return VolumeResult{}, err
	}
	cells, err := orthogonalVolumeSource(source, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, cells, distance, o, false, nil)
}

// ThickenVolume retains the wall layer of positive thickness inside an
// orthogonal cell solid. It subtracts the complete cubical erosion from
// the original material in one exact arrangement. A vanished inner volume
// leaves the corresponding original material solid. No opening is cut.
func ThickenVolume(ctx context.Context, source solid.Solid, thickness float64, o VolumeOptions) (VolumeResult, error) {
	if !(thickness > 0) {
		return VolumeResult{}, ErrInvalid
	}
	o, err := volumeOffsetOptions(ctx, thickness, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, source, -thickness, o, true, nil)
}

// ThickenOrthogonalVolume first proves a native outward orthogonal B-rep
// before retaining its closed wall layer. Original cavities are allowed.
func ThickenOrthogonalVolume(ctx context.Context, source brep.Solid, thickness float64, o VolumeOptions) (VolumeResult, error) {
	if !(thickness > 0) {
		return VolumeResult{}, ErrInvalid
	}
	o, err := volumeOffsetOptions(ctx, thickness, o)
	if err != nil {
		return VolumeResult{}, err
	}
	cells, err := orthogonalVolumeSource(source, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, cells, -thickness, o, true, nil)
}

func orthogonalVolumeSource(source brep.Solid, o VolumeOptions) (solid.Solid, error) {
	cells, err := solid.FromOrthogonal(source, solid.Options{MaxCells: o.MaxGridEntries, MaxWork: o.MaxWork})
	if errors.Is(err, solid.ErrLimit) {
		return solid.Solid{}, ErrLimit
	}
	if err != nil {
		return solid.Solid{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return cells, nil
}

func volumeOffsetOptions(ctx context.Context, distance float64, o VolumeOptions) (VolumeOptions, error) {
	if ctx == nil || !finite(distance) || !finite(o.MaxCoordinateError) || o.MaxCoordinateError < 0 ||
		o.MaxSourceCells < 0 || o.MaxGridEntries < 0 || o.MaxWork < 0 || o.MaxGridEntries > 4_194_304 {
		return VolumeOptions{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return VolumeOptions{}, err
	}
	if o.MaxSourceCells == 0 {
		o.MaxSourceCells = 65_536
	}
	if o.MaxGridEntries == 0 {
		o.MaxGridEntries = 262_144
	}
	if o.MaxWork == 0 {
		o.MaxWork = 20_000_000
	}
	if o.MaxCoordinateError == 0 {
		o.MaxCoordinateError = 1e-9
	}
	return o, nil
}

type volumeGridBox struct{ lo, hi [3]int }

type volumeGrid struct {
	axis   [3][]q
	n      [3]int // numbers of positive-width intervals
	stride [3]int
	delta  []int
}

func newVolumeGrid(axis [3][]q, budget *budget, maxEntries int) (volumeGrid, error) {
	g := volumeGrid{axis: axis}
	size := 1
	for a := 2; a >= 0; a-- {
		n := len(axis[a])
		if n < 2 || size > maxEntries/n {
			return volumeGrid{}, ErrLimit
		}
		g.n[a], g.stride[a] = n-1, size
		size *= n
	}
	if err := budget.tick(size); err != nil {
		return volumeGrid{}, err
	}
	g.delta = make([]int, size)
	return g, nil
}

func (g volumeGrid) index(i, j, k int) int { return i*g.stride[0] + j*g.stride[1] + k }

func (g volumeGrid) position(a int, x q) int {
	return sort.Search(len(g.axis[a]), func(i int) bool { return g.axis[a][i].Cmp(x) >= 0 })
}

// Mark eight signed corners. Successive prefix passes along each axis turn
// these differences into coverage counts over the open cell intervals.
func (g volumeGrid) mark(box volumeGridBox) {
	for mask := range 8 {
		p, sign := box.lo, 1
		for a := range 3 {
			if mask&(1<<a) != 0 {
				p[a], sign = box.hi[a], -sign
			}
		}
		g.delta[g.index(p[0], p[1], p[2])] += sign
	}
}

func (g volumeGrid) accumulate(b *budget) error {
	for a, stride := range g.stride {
		for block := 0; block < len(g.delta); block += stride * (g.n[a] + 1) {
			for i := 1; i <= g.n[a]; i++ {
				if err := b.tick(stride); err != nil {
					return err
				}
				start := block + i*stride
				for k := start; k < start+stride; k++ {
					g.delta[k] += g.delta[k-stride]
				}
			}
		}
	}
	return nil
}

func computeVolumeOffset(ctx context.Context, source solid.Solid, distance float64, o VolumeOptions, keepWall bool, opening *VolumeOpenPlane) (VolumeResult, error) {
	var zero VolumeResult
	result := VolumeResult{ExactVolume: new(big.Rat)}
	b := &budget{ctx: ctx, remaining: o.MaxWork}
	if err := b.tick(1); err != nil {
		return zero, err
	}
	count := source.CellCount()
	if count == 0 {
		if opening != nil {
			return zero, ErrNoOpenCore
		}
		return result, nil
	}
	if count > o.MaxSourceCells || count > b.remaining/6 {
		return zero, ErrLimit
	}
	cells := source.Cells()
	var exact [3][]q
	seen := [3]map[float64]bool{{}, {}, {}}
	for _, c := range cells {
		if err := b.tick(6); err != nil {
			return zero, err
		}
		for a, lo := range [3]float64{c.Min.X, c.Min.Y, c.Min.Z} {
			hi := [3]float64{c.Max.X, c.Max.Y, c.Max.Z}[a]
			for _, value := range [2]float64{lo, hi} {
				if !seen[a][value] {
					seen[a][value] = true
					exact[a] = append(exact[a], num(value))
				}
			}
		}
	}
	radius := num(math.Abs(distance))
	erode := distance < 0
	var shifted [3][]q
	var expandedLow, expandedHigh [3][]q
	var vanished bool
	for a, axis := range exact {
		exact[a] = unique(axis)
		axis = exact[a]
		if err := b.tick(len(axis)); err != nil {
			return zero, err
		}
		for _, x := range axis {
			lo, hi := sub(x, radius), add(x, radius)
			expandedLow[a] = append(expandedLow[a], lo)
			expandedHigh[a] = append(expandedHigh[a], hi)
			shifted[a] = append(shifted[a], lo, hi)
		}
		if erode && expandedHigh[a][0].Cmp(expandedLow[a][len(axis)-1]) >= 0 {
			if !keepWall {
				return result, nil
			}
			vanished = true
		}
		if keepWall {
			shifted[a] = append(shifted[a], axis...)
		}
		shifted[a] = unique(shifted[a])
	}
	if vanished {
		if opening != nil {
			return zero, ErrNoOpenCore
		}
		return unchangedVolumeLayer(cells, exact, b, o)
	}
	g, err := newVolumeGrid(shifted, b, o.MaxGridEntries)
	if err != nil {
		return zero, err
	}
	lo, hi := [3]int{}, g.n
	if erode {
		original, err := newVolumeGrid(exact, b, o.MaxGridEntries)
		if err != nil {
			return zero, err
		}
		if err := populateVolumeSource(cells, original, b); err != nil {
			return zero, err
		}
		for a := range 3 {
			lo[a] = g.position(a, expandedHigh[a][0])
			hi[a] = g.position(a, expandedLow[a][original.n[a]])
		}
		if err := markErodedComplement(g, original, expandedLow, expandedHigh, b); err != nil {
			return zero, err
		}
	} else {
		for _, c := range cells {
			if err := b.tick(8); err != nil {
				return zero, err
			}
			var box volumeGridBox
			for a, x := range [3]float64{c.Min.X, c.Min.Y, c.Min.Z} {
				y := [3]float64{c.Max.X, c.Max.Y, c.Max.Z}[a]
				box.lo[a] = g.position(a, sub(num(x), radius))
				box.hi[a] = g.position(a, add(num(y), radius))
			}
			g.mark(box)
		}
	}
	if err := g.accumulate(b); err != nil {
		return zero, err
	}
	var take func(i, j, k int) bool
	if keepWall {
		original, err := newVolumeGrid(shifted, b, o.MaxGridEntries)
		if err != nil {
			return zero, err
		}
		if err := populateVolumeSource(cells, original, b); err != nil {
			return zero, err
		}
		insetLo, insetHi := lo, hi
		for a, axis := range exact {
			lo[a] = g.position(a, axis[0])
			hi[a] = g.position(a, axis[len(axis)-1])
		}
		var mouth orthogonalOpenMouth
		if opening != nil {
			mouth, err = proveVolumeOpening(g, original, lo, hi, insetLo, insetHi, *opening, b)
			if err != nil {
				return zero, err
			}
		}
		take = func(i, j, k int) bool {
			index := g.index(i, j, k)
			if original.delta[index] == 0 {
				return false
			}
			// The core is occupied only inside the inset limits and outside
			// every expanded complementary box.
			inside := i >= insetLo[0] && i < insetHi[0] && j >= insetLo[1] && j < insetHi[1] && k >= insetLo[2] && k < insetHi[2]
			if opening != nil && mouth.contains([3]int{i, j, k}) {
				return false
			}
			return !inside || g.delta[index] > 0
		}
	} else {
		take = func(i, j, k int) bool { return (g.delta[g.index(i, j, k)] > 0) != erode }
	}
	boxes, err := collectVolumeRuns(lo, hi, take, b)
	if err != nil {
		return zero, err
	}
	if len(boxes) == 0 {
		return result, nil
	}
	return volumeResult(boxes, g, b, o)
}

func unchangedVolumeLayer(cells []geom.Box, axis [3][]q, b *budget, o VolumeOptions) (VolumeResult, error) {
	g, err := newVolumeGrid(axis, b, o.MaxGridEntries)
	if err != nil {
		return VolumeResult{}, err
	}
	if err := populateVolumeSource(cells, g, b); err != nil {
		return VolumeResult{}, err
	}
	boxes, err := collectVolumeRuns([3]int{}, g.n, func(i, j, k int) bool { return g.delta[g.index(i, j, k)] > 0 }, b)
	if err != nil {
		return VolumeResult{}, err
	}
	return volumeResult(boxes, g, b, o)
}

func populateVolumeSource(cells []geom.Box, grid volumeGrid, b *budget) error {
	for _, c := range cells {
		if err := b.tick(8); err != nil {
			return err
		}
		var box volumeGridBox
		for a, lo := range [3]float64{c.Min.X, c.Min.Y, c.Min.Z} {
			hi := [3]float64{c.Max.X, c.Max.Y, c.Max.Z}[a]
			box.lo[a], box.hi[a] = grid.position(a, num(lo)), grid.position(a, num(hi))
		}
		grid.mark(box)
	}
	return grid.accumulate(b)
}

func markErodedComplement(g, original volumeGrid, lows, highs [3][]q, b *budget) error {
	var lower, upper [3][]int
	for a := range 3 {
		for i := range original.n[a] {
			if err := b.tick(2); err != nil {
				return err
			}
			lower[a] = append(lower[a], g.position(a, lows[a][i]))
			upper[a] = append(upper[a], g.position(a, highs[a][i+1]))
		}
	}
	for i := range original.n[0] {
		for j := range original.n[1] {
			for k := range original.n[2] {
				if err := b.tick(1); err != nil {
					return err
				}
				if original.delta[original.index(i, j, k)] == 0 {
					if err := b.tick(8); err != nil {
						return err
					}
					g.mark(volumeGridBox{lo: [3]int{lower[0][i], lower[1][j], lower[2][k]}, hi: [3]int{upper[0][i], upper[1][j], upper[2][k]}})
				}
			}
		}
	}
	return nil
}

func collectVolumeRuns(lo, hi [3]int, occupied func(i, j, k int) bool, b *budget) ([]volumeGridBox, error) {
	var result []volumeGridBox
	previousSlab := map[[4]int]int{}
	for i := lo[0]; i < hi[0]; i++ {
		var slab []volumeGridBox
		previousRow := map[[2]int]int{}
		for j := lo[1]; j < hi[1]; j++ {
			if err := b.tick(hi[2] - lo[2]); err != nil {
				return nil, err
			}
			currentRow := map[[2]int]int{}
			for k := lo[2]; k < hi[2]; {
				if !occupied(i, j, k) {
					k++
					continue
				}
				start := k
				for k++; k < hi[2] && occupied(i, j, k); k++ {
				}
				key := [2]int{start, k}
				n, exists := previousRow[key]
				if exists {
					slab[n].hi[1]++
				} else {
					n = len(slab)
					slab = append(slab, volumeGridBox{lo: [3]int{i, j, start}, hi: [3]int{i + 1, j + 1, k}})
				}
				currentRow[key] = n
			}
			previousRow = currentRow
		}
		currentSlab := make(map[[4]int]int, len(slab))
		for _, rect := range slab {
			if err := b.tick(1); err != nil {
				return nil, err
			}
			key := [4]int{rect.lo[1], rect.lo[2], rect.hi[1], rect.hi[2]}
			n, exists := previousSlab[key]
			if exists {
				result[n].hi[0]++
			} else {
				n = len(result)
				result = append(result, rect)
			}
			currentSlab[key] = n
		}
		previousSlab = currentSlab
	}
	return result, nil
}

func volumeResult(boxes []volumeGridBox, g volumeGrid, b *budget, o VolumeOptions) (VolumeResult, error) {
	var zero VolumeResult
	var used [3][]bool
	var coords [3][]float64
	for a := range 3 {
		used[a] = make([]bool, len(g.axis[a]))
		coords[a] = make([]float64, len(g.axis[a]))
	}
	for _, box := range boxes {
		if err := b.tick(6); err != nil {
			return zero, err
		}
		for a := range 3 {
			used[a][box.lo[a]], used[a][box.hi[a]] = true, true
		}
	}
	maxError := new(big.Rat)
	tol := num(o.MaxCoordinateError)
	for a, axis := range g.axis {
		prev := math.Inf(-1)
		for i, x := range axis {
			if !used[a][i] {
				continue
			}
			if err := b.tick(1); err != nil {
				return zero, err
			}
			f, _ := x.Float64()
			if !finite(f) || f <= prev {
				return zero, ErrPrecision
			}
			err := new(big.Rat).Abs(sub(num(f), x))
			if err.Cmp(tol) > 0 {
				return zero, ErrPrecision
			}
			if err.Cmp(maxError) > 0 {
				maxError.Set(err)
			}
			coords[a][i], prev = f, f
		}
	}
	result := VolumeResult{ExactVolume: new(big.Rat), Cells: make([]VolumeCell, 0, len(boxes))}
	result.MaxDisplacement, _ = maxError.Float64()
	if num(result.MaxDisplacement).Cmp(maxError) < 0 {
		result.MaxDisplacement = math.Nextafter(result.MaxDisplacement, math.Inf(1))
	}
	approx := make([]geom.Box, 0, len(boxes))
	for _, box := range boxes {
		if err := b.tick(1); err != nil {
			return zero, err
		}
		var c VolumeCell
		v := big.NewRat(1, 1)
		for a, axis := range g.axis {
			c.Min[a] = new(big.Rat).Set(axis[box.lo[a]])
			c.Max[a] = new(big.Rat).Set(axis[box.hi[a]])
			v.Mul(v, sub(c.Max[a], c.Min[a]))
		}
		result.Cells = append(result.Cells, c)
		result.ExactVolume.Add(result.ExactVolume, v)
		approx = append(approx, geom.Box{
			Min: geom.Vec3{X: coords[0][box.lo[0]], Y: coords[1][box.lo[1]], Z: coords[2][box.lo[2]]},
			Max: geom.Vec3{X: coords[0][box.hi[0]], Y: coords[1][box.hi[1]], Z: coords[2][box.hi[2]]},
		})
	}
	limits := solid.Options{MaxCells: o.MaxGridEntries, MaxWork: o.MaxWork}
	cells, err := solid.NewWithOptions(limits, approx...)
	if errors.Is(err, solid.ErrLimit) {
		return zero, ErrLimit
	}
	if err != nil {
		return zero, err
	}
	if err := b.tick(1); err != nil {
		return zero, err
	}
	result.Solid, err = cells.BRep(limits)
	if errors.Is(err, solid.ErrLimit) {
		return zero, ErrLimit
	}
	if err != nil {
		return zero, fmt.Errorf("offset volume result: %w", err)
	}
	if err := b.tick(1); err != nil {
		return zero, err
	}
	return result, nil
}
