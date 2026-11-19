//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"sort"

	"plainsolid/geom"
)

type AlgebraicGraphCapSeamOwner struct {
	Wire, Use int
}

// AlgebraicGraphCapPairSeam pairs opposite directed occurrences of one whole
// exact line segment. The two owners traverse it in opposite directions.
type AlgebraicGraphCapPairSeam struct {
	Owners [2]AlgebraicGraphCapSeamOwner
}

// AlgebraicGraphCapPair retains both independently clipped cap boundaries
// and a complete bijection of their generated seam occurrences. It does
// not attach either cap to the original stock faces.
type AlgebraicGraphCapPair struct {
	Caps                          [2]AlgebraicGraphCapClip
	Seams                         []AlgebraicGraphCapPairSeam
	ExactOperations, Subdivisions int
	caps                          [2]AlgebraicGraphCapClip
	seams                         []AlgebraicGraphCapPairSeam
}

// ClipAlgebraicGraphCapPair independently clips both native stock sections
// by the opposite plane, then proves whole mutual seam ownership. Both
// section planes must point out of the retained halfspaces, so the negative
// side of each plane is material. This regular crossing path requires at
// least one common material interval and rejects incomplete pairing.
func (s Solid) ClipAlgebraicGraphCapPair(ctx context.Context, planes [2][3]geom.Vec3, section RationalGraphShellSectionOptions, o RationalGraphShellRegionOptions) (AlgebraicGraphCapPair, error) {
	return s.clipAlgebraicGraphCapPair(ctx, planes, section, o, false)
}

// ClipAlgebraicGraphBand additionally accepts two caps whose intersection
// stays completely outside the original stock. In that case both cap
// sections must lie strictly in the other's retained halfspace. Parallel
// or diverging support planes can bound such a band without any common seam.
func (s Solid) ClipAlgebraicGraphBand(ctx context.Context, planes [2][3]geom.Vec3, section RationalGraphShellSectionOptions,
	o RationalGraphShellRegionOptions) (AlgebraicGraphCapPair, error) {
	return s.clipAlgebraicGraphCapPair(ctx, planes, section, o, true)
}

func (s Solid) clipAlgebraicGraphCapPair(ctx context.Context, planes [2][3]geom.Vec3, section RationalGraphShellSectionOptions,
	o RationalGraphShellRegionOptions, separated bool) (AlgebraicGraphCapPair, error) {
	var zero AlgebraicGraphCapPair
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	topo, e := o.Topology.WithDefaults()
	if e != nil {
		return zero, e
	}
	b := capClipBudget{ctx, topo.MaxWork, topo.MaxSubdivisions, topo.MaxBits}
	var out AlgebraicGraphCapPair
	for i := range planes {
		limits := o
		limits.Topology = topo
		limits.Topology.MaxWork, limits.Topology.MaxSubdivisions = b.work, b.subdiv
		var cap AlgebraicGraphCapClip
		if separated {
			// Authenticate the no-contact case before invoking the
			// crossing engine. A strict entire original ring hull is
			// enough to prove containment in the other halfspace.
			cap, e = s.separatedAlgebraicCap(ctx, planes[i], planes[i^1], section, limits)
			if e != nil && !errors.Is(e, geom.ErrUnrepresentable) {
				return zero, e
			}
		}
		if cap.recipe == nil {
			cap, e = s.ClipAlgebraicGraphCap(ctx, planes[i], planes[i^1], false, section, limits)
			if e != nil {
				return zero, e
			}
		}
		if e := b.charge(cap.ExactOperations, cap.Subdivisions); e != nil {
			return zero, e
		}
		out.Caps[i] = cap
	}
	if separated {
		a, z := out.Caps[0], out.Caps[1]
		if a.separated != z.separated {
			return zero, geom.ErrConvergence
		}
		if a.separated {
			if len(a.wires) == 0 || len(z.wires) == 0 {
				return zero, geom.ErrUnrepresentable
			}
			out.caps = out.Caps
			out.ExactOperations, out.Subdivisions = topo.MaxWork-b.work, topo.MaxSubdivisions-b.subdiv
			return out, nil
		}
	}
	// Any nonconstant world axis on the exact plane intersection orders the
	// same material intervals on both cap charts. The other two coordinates
	// are then uniquely determined by the two independent cap equations.
	a, z := out.Caps[0].regions.plane.normal, out.Caps[1].regions.plane.normal
	axis := -1
	for i := range a {
		j, k := (i+1)%3, (i+2)%3
		x := new(big.Rat).Sub(new(big.Rat).Mul(a[j], z[k]), new(big.Rat).Mul(a[k], z[j]))
		if x.Sign() != 0 {
			axis = i
			break
		}
	}
	if axis < 0 {
		return zero, geom.ErrConvergence
	}
	compare := func(a, z *geom.ExactAlgebraicRoot) (int, error) {
		q, e := a.Compare(ctx, z, b.limits())
		if e != nil {
			return 0, e
		}
		if e := b.charge(q.Operations, q.Subdivisions); e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, geom.ErrConvergence
		}
		return q.Order, nil
	}
	type span struct {
		owner      AlgebraicGraphCapSeamOwner
		start, end *geom.ExactAlgebraicRoot
		increasing bool
	}
	var data [2][]span
	for side, cap := range out.Caps {
		for wi, wire := range cap.wires {
			for ui, use := range wire {
				if use.Seam < 0 {
					continue
				}
				q := span{owner: AlgebraicGraphCapSeamOwner{wi, ui}, increasing: true}
				var roots [2]*geom.ExactAlgebraicRoot
				for end := range roots {
					local := end
					if use.Reversed {
						local ^= 1
					}
					p, _ := use.Cell.EndpointRoot(local)
					r, work, e := use.Cell.CoordinateRoot(ctx, p, axis, b.limits())
					if e != nil {
						return zero, e
					}
					if e := b.charge(work, 0); e != nil {
						return zero, e
					}
					roots[end] = r
				}
				order, e := compare(roots[0], roots[1])
				if e != nil || order == 0 {
					return zero, errors.Join(geom.ErrConvergence, e)
				}
				if order > 0 {
					roots[0], roots[1] = roots[1], roots[0]
					q.increasing = false
				}
				q.start, q.end = roots[0], roots[1]
				data[side] = append(data[side], q)
			}
		}
		row := data[side]
		// Only use enclosures to propose the sort; every adjacent relation
		// is reproved on the exact roots before comparing the two ledgers.
		sort.SliceStable(row, func(i, j int) bool {
			return row[i].start.Interval().Lower.Cmp(row[j].start.Interval().Lower) < 0
		})
		for i := 1; i < len(row); i++ {
			for j := i; j > 0; j-- {
				order, e := compare(row[j-1].start, row[j].start)
				if e != nil || order == 0 {
					return zero, errors.Join(geom.ErrConvergence, e)
				}
				if order < 0 {
					break
				}
				row[j-1], row[j] = row[j], row[j-1]
			}
		}
		for i := 1; i < len(row); i++ {
			order, e := compare(row[i-1].end, row[i].start)
			if e != nil || order >= 0 {
				return zero, errors.Join(geom.ErrConvergence, e)
			}
		}
	}
	if len(data[0]) == 0 || len(data[0]) != len(data[1]) {
		return zero, geom.ErrConvergence
	}
	for i, a := range data[0] {
		z := data[1][i]
		if a.increasing == z.increasing {
			return zero, geom.ErrConvergence
		}
		for _, endpoint := range [][2]*geom.ExactAlgebraicRoot{{a.start, z.start}, {a.end, z.end}} {
			order, e := compare(endpoint[0], endpoint[1])
			if e != nil {
				return zero, e
			}
			if order != 0 {
				return zero, geom.ErrConvergence
			}
		}
		out.Seams = append(out.Seams, AlgebraicGraphCapPairSeam{[2]AlgebraicGraphCapSeamOwner{a.owner, z.owner}})
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	out.caps = out.Caps
	out.seams = append([]AlgebraicGraphCapPairSeam(nil), out.Seams...)
	out.ExactOperations, out.Subdivisions = topo.MaxWork-b.work, topo.MaxSubdivisions-b.subdiv
	return out, nil
}
