package offset

import (
	"context"
	"errors"
	"fmt"

	"plainsolid/brep"
	"plainsolid/solid"
)

// ErrNoOpenCore means no positive erosion cross section reaches the selected
// exterior support. An open wall layer cannot be constructed there.
var ErrNoOpenCore = errors.New("offset: no inner core reaches the selected open support")

// VolumeOpenPlane selects one entire extremal plane of a source cell solid.
// Axis is 0 for X, 1 for Y and 2 for Z. High chooses the maximum coordinate.
type VolumeOpenPlane struct {
	Axis int
	High bool
}

// ThickenVolumeOpenPlane forms a wall layer with an opening on one extremal
// plane. The cubical core's first or last section extends to that original
// plane. Only complete, positive cross sections of the exact core are
// opened. The lip retains original material and can contain several gaps.
func ThickenVolumeOpenPlane(ctx context.Context, source solid.Solid, thickness float64, opening VolumeOpenPlane, o VolumeOptions) (VolumeResult, error) {
	o, err := volumeOpenOptions(ctx, thickness, opening, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, source, -thickness, o, true, &opening)
}

// ThickenOrthogonalVolumeOpenPlane proves the native orthogonal solid before
// extending its cubical core to one original extremal plane.
func ThickenOrthogonalVolumeOpenPlane(ctx context.Context, source brep.Solid, thickness float64, opening VolumeOpenPlane, o VolumeOptions) (VolumeResult, error) {
	o, err := volumeOpenOptions(ctx, thickness, opening, o)
	if err != nil {
		return VolumeResult{}, err
	}
	cells, err := orthogonalVolumeSource(source, o)
	if err != nil {
		return VolumeResult{}, err
	}
	return computeVolumeOffset(ctx, cells, -thickness, o, true, &opening)
}

func volumeOpenOptions(ctx context.Context, thickness float64, opening VolumeOpenPlane, o VolumeOptions) (VolumeOptions, error) {
	if !(thickness > 0) || opening.Axis < 0 || opening.Axis > 2 {
		return VolumeOptions{}, ErrInvalid
	}
	return volumeOffsetOptions(ctx, thickness, o)
}

type orthogonalOpenMouth struct {
	a, u, v int
	lo, hi  int
	columns map[[2]int]struct{}
}

func (m orthogonalOpenMouth) contains(p [3]int) bool {
	if p[m.a] < m.lo || p[m.a] >= m.hi {
		return false
	}
	_, ok := m.columns[[2]int{p[m.u], p[m.v]}]
	return ok
}

func proveVolumeOpening(core, original volumeGrid, sourceLo, sourceHi, insetLo, insetHi [3]int, opening VolumeOpenPlane, b *budget) (orthogonalOpenMouth, error) {
	var zero orthogonalOpenMouth
	a := opening.Axis
	m := orthogonalOpenMouth{a: a, u: (a + 1) % 3, v: (a + 2) % 3, columns: map[[2]int]struct{}{}}
	coreStation := insetLo[a]
	m.lo, m.hi = sourceLo[a], insetLo[a]
	if opening.High {
		coreStation = insetHi[a] - 1
		m.lo, m.hi = insetHi[a], sourceHi[a]
	}
	if m.lo >= m.hi {
		return zero, ErrNoOpenCore
	}
	for i := insetLo[m.u]; i < insetHi[m.u]; i++ {
		for j := insetLo[m.v]; j < insetHi[m.v]; j++ {
			if err := b.tick(1); err != nil {
				return zero, err
			}
			var p [3]int
			p[a], p[m.u], p[m.v] = coreStation, i, j
			if core.delta[core.index(p[0], p[1], p[2])] != 0 {
				continue
			}
			// Prove the entire extension lies in original material. The
			// output grid contains both source and inset boundaries.
			for k := m.lo; k < m.hi; k++ {
				if err := b.tick(1); err != nil {
					return zero, err
				}
				p[a] = k
				if original.delta[original.index(p[0], p[1], p[2])] == 0 {
					return zero, fmt.Errorf("%w: an opening column leaves the original support slab", ErrInvalid)
				}
			}
			m.columns[[2]int{i, j}] = struct{}{}
		}
	}
	if len(m.columns) == 0 {
		return zero, ErrNoOpenCore
	}
	return m, nil
}
