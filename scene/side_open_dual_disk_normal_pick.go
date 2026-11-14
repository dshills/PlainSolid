package scene

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// SideOpenDualDiskNormalBoundary is satisfied by the native equal, unequal,
// and external-radical cuts. It returns separate original support normals.
type SideOpenDualDiskNormalBoundary interface {
	FirstBoundaryHit(context.Context, [3]*big.Rat, [3]*big.Rat, *big.Rat, *big.Rat, geom.ToroidLineOptions) (geom.BoxDualCircleFirstHit, error)
}

// SideOpenDualDiskNormalItem places an exact cut by a nonsingular represented
// affine map. Transform maps local model coordinates to world coordinates.
type SideOpenDualDiskNormalItem struct {
	ID        string
	Boundary  SideOpenDualDiskNormalBoundary
	Transform geom.Affine
}

// SideOpenDualDiskNormalPick contains the exact first contact in world camera
// ray units. Normals are outward, unnormalized, and separate for each source.
type SideOpenDualDiskNormalPick struct {
	ID  string
	Hit geom.BoxDualCircleFirstHit
}

// PickSideOpenDualDiskNormals compares the first contacts of visible instances
// exactly. A depth tie selects the lexicographically smallest ID. Neither
// the inverse placement nor the outward normal transform rounds to float64.
func PickSideOpenDualDiskNormals(ctx context.Context, camera Camera, screen geom.Vec2, items []SideOpenDualDiskNormalItem, o geom.ToroidLineOptions) (SideOpenDualDiskNormalPick, bool, error) {
	var zero SideOpenDualDiskNormalPick
	if ctx == nil || len(items) > 1024 || o.ParameterTolerance < 0 || !finite(o.ParameterTolerance) {
		return zero, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	ray, e := camera.Ray(screen)
	if e != nil {
		return zero, false, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	origin := sceneRationalVector(ray.Origin)
	direction := sceneRationalVector(ray.Direction)
	minT, maxT := new(big.Rat), new(big.Rat).SetFloat64(ray.Length)
	limits := geom.ExactCurvePlaneEventOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	seen := make(map[string]bool, len(items))
	best := zero
	var bestInverse [3][3]*big.Rat
	for _, item := range items {
		if e := ctx.Err(); e != nil {
			return zero, false, e
		}
		if item.ID == "" || seen[item.ID] || item.Boundary == nil || !item.Transform.IsFinite() {
			return zero, false, geom.ErrInvalid
		}
		seen[item.ID] = true
		inv, e := sidePickInverse(item.Transform)
		if e != nil {
			return zero, false, fmt.Errorf("scene: side opening %q: %w", item.ID, e)
		}
		var p, d [3]*big.Rat
		for i := range p {
			p[i], d[i] = new(big.Rat), new(big.Rat)
			for j := range origin {
				a := new(big.Rat).Sub(origin[j], new(big.Rat).SetFloat64(item.Transform[j][3]))
				p[i].Add(p[i], a.Mul(a, inv[i][j]))
				d[i].Add(d[i], new(big.Rat).Mul(direction[j], inv[i][j]))
			}
		}
		h, e := item.Boundary.FirstBoundaryHit(ctx, p, d, minT, maxT, o)
		if e != nil {
			return zero, false, fmt.Errorf("scene: side opening %q: %w", item.ID, e)
		}
		if !h.HasHit {
			continue
		}
		if h.Parameter == nil || h.Support == 0 || len(h.Normals) == 0 {
			return zero, false, geom.ErrInvalid
		}
		if best.Hit.HasHit {
			order, e := h.Parameter.Compare(ctx, best.Hit.Parameter, limits)
			if e != nil {
				return zero, false, e
			}
			if !order.Proven {
				return zero, false, geom.ErrConvergence
			}
			if order.Order > 0 || order.Order == 0 && item.ID >= best.ID {
				continue
			}
		}
		best, bestInverse = SideOpenDualDiskNormalPick{ID: item.ID, Hit: h}, inv
	}
	if e := ctx.Err(); e != nil {
		return zero, false, e
	}
	if !best.Hit.HasHit {
		return zero, false, nil
	}
	h := &best.Hit
	h.Bounds = h.Parameter.Interval()
	span := func(a, b *big.Rat) geom.ExactParameterInterval {
		x := new(big.Rat).Add(a, new(big.Rat).Mul(b, h.Bounds.Lower))
		y := new(big.Rat).Add(a, new(big.Rat).Mul(b, h.Bounds.Upper))
		if x.Cmp(y) > 0 {
			x, y = y, x
		}
		return geom.ExactParameterInterval{Lower: x, Upper: y}
	}
	for i := range h.PointBounds {
		h.PointBounds[i] = span(origin[i], direction[i])
	}
	normals := make([]geom.BoxDualCircleBoundaryNormal, len(h.Normals))
	for k, n := range h.Normals {
		v := &normals[k]
		v.Source, v.DirectionSign = n.Source, n.DirectionSign
		for i := range v.Constant {
			v.Constant[i], v.Linear[i] = new(big.Rat), new(big.Rat)
			for j := range n.Constant {
				if n.Constant[j] == nil || n.Linear[j] == nil {
					return zero, false, geom.ErrInvalid
				}
				v.Constant[i].Add(v.Constant[i], new(big.Rat).Mul(bestInverse[j][i], n.Constant[j]))
				v.Linear[i].Add(v.Linear[i], new(big.Rat).Mul(bestInverse[j][i], n.Linear[j]))
			}
			v.Bounds[i] = span(v.Constant[i], v.Linear[i])
		}
	}
	h.Normals = normals
	return best, true, nil
}

func sceneRationalVector(v geom.Vec3) [3]*big.Rat {
	return [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}

// A fixed-size rational solve bounds work independently of scene size and
// preserves nonbinary inverses of binary64 placement coefficients.
func sidePickInverse(m geom.Affine) (inv [3][3]*big.Rat, err error) {
	var r [3][6]*big.Rat
	for i := range r {
		for j := 0; j < 3; j++ {
			r[i][j] = new(big.Rat).SetFloat64(m[i][j])
			r[i][j+3] = new(big.Rat)
		}
		r[i][i+3].SetInt64(1)
	}
	for j := range r {
		p := j
		for p < 3 && r[p][j].Sign() == 0 {
			p++
		}
		if p == 3 {
			return inv, geom.ErrDegenerate
		}
		r[p], r[j] = r[j], r[p]
		v := new(big.Rat).Set(r[j][j])
		for k := j; k < 6; k++ {
			r[j][k].Quo(r[j][k], v)
		}
		for i := range r {
			if i == j {
				continue
			}
			v.Set(r[i][j])
			for k := j; k < 6; k++ {
				r[i][k].Sub(r[i][k], new(big.Rat).Mul(v, r[j][k]))
			}
		}
	}
	for i := range inv {
		copy(inv[i][:], r[i][3:])
	}
	return inv, nil
}
