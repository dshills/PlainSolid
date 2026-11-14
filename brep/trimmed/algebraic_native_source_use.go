package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

// nativeAlgebraicSourceUse preserves the complete original p-curve rather
// than replacing a curved retained boundary with its endpoint chord. Only a
// unique occurrence is accepted; periodic duplicate uses require a selector.
func nativeAlgebraicSourceUse(f Face, edge int, reverse bool, limits [2]*big.Rat) (Use, error) {
	var source Use
	found := false
	for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
		for _, u := range loop {
			if u.Edge == edge {
				if found {
					return Use{}, geom.ErrConvergence
				}
				source = u
				found = true
			}
		}
	}
	if !found || limits[0] == nil || limits[1] == nil || limits[0].Sign() < 0 || limits[1].Cmp(big.NewRat(1, 1)) > 0 || limits[0].Cmp(limits[1]) >= 0 {
		return Use{}, geom.ErrInvalid
	}
	for _, q := range limits {
		if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
			return Use{}, geom.ErrConvergence
		}
	}
	if source.Curve == nil {
		// The old exact line path is retained, including source/model orientation.
		singleton := Face{Outer: []Use{source}}
		p, q, e := nativeAlgebraicSourceUV(singleton, edge, reverse, limits)
		if e != nil {
			return Use{}, e
		}
		return Use{Edge: edge, Reversed: reverse, UVStart: p, UVEnd: q}, nil
	}
	var controls [][3]*big.Rat
	curve := source.Curve
	wrapperReversed := false
	if q, ok := curve.(reverseTrim); ok {
		curve = q.differentiableTrim
		wrapperReversed = true
	}
	switch q := curve.(type) {
	case ExactQuadraticUV:
		h := q.HomogeneousControls()
		controls = append(controls, h[:]...)
	case ExactBezierUV:
		controls = q.HomogeneousControls()
	default:
		return Use{}, geom.ErrUnrepresentable
	}
	if len(controls) < 3 || len(controls) > 5 {
		return Use{}, geom.ErrInvalid
	}
	if wrapperReversed {
		for i, j := 0, len(controls)-1; i < j; i, j = i+1, j-1 {
			controls[i], controls[j] = controls[j], controls[i]
		}
	}
	// Check exact authored endpoint binding even for rational-endpoint variants.
	for i, h := range [2][3]*big.Rat{controls[0], controls[len(controls)-1]} {
		p, e := exactGraphPartitionUV([2]*big.Rat{new(big.Rat).Quo(h[0], h[2]), new(big.Rat).Quo(h[1], h[2])})
		if e != nil {
			return Use{}, e
		}
		if p != [2]geom.Vec2{source.UVStart, source.UVEnd}[i] {
			return Use{}, geom.ErrConvergence
		}
	}
	if source.Reversed {
		for i, j := 0, len(controls)-1; i < j; i, j = i+1, j-1 {
			controls[i], controls[j] = controls[j], controls[i]
		}
	}
	// Blossom at lo repeated n-i and hi repeated i gives exact restriction.
	n := len(controls) - 1
	restricted := make([][3]*big.Rat, n+1)
	for i := range restricted {
		work := copyHomogeneousUV(controls)
		for level := 0; level < n; level++ {
			t := limits[0]
			if level >= n-i {
				t = limits[1]
			}
			oneMinus := new(big.Rat).Sub(big.NewRat(1, 1), t)
			for j := 0; j < n-level; j++ {
				for axis := 0; axis < 3; axis++ {
					q := new(big.Rat).Add(new(big.Rat).Mul(oneMinus, work[j][axis]), new(big.Rat).Mul(t, work[j+1][axis]))
					if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
						return Use{}, geom.ErrConvergence
					}
					work[j][axis] = q
				}
			}
		}
		restricted[i] = work[0]
	}
	if reverse {
		for i, j := 0, n; i < j; i, j = i+1, j-1 {
			restricted[i], restricted[j] = restricted[j], restricted[i]
		}
	}
	out := Use{Edge: edge, Reversed: reverse}
	if n == 2 {
		q, e := NewExactQuadraticUV([3][3]*big.Rat{restricted[0], restricted[1], restricted[2]})
		if e != nil {
			return Use{}, e
		}
		out.Curve = q
		out.UVStart, out.UVEnd = q.ends[0], q.ends[1]
	} else {
		q, e := NewExactBezierUV(restricted)
		if e != nil {
			return Use{}, e
		}
		out.Curve = q
		out.UVStart, out.UVEnd = q.ends[0], q.ends[1]
	}
	return out, nil
}
