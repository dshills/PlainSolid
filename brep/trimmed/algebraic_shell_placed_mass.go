package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// BoundPlacedMassProperties encloses a rigidly placed original curved Boolean
// using its certified original-support flux bounds. Volume is invariant; first
// moments, centroid and inertia are in world units about the world origin.
// The placement arithmetic uses the patch coefficient bit limit (default 16384).
func (s AlgebraicShellBoolean) BoundPlacedMassProperties(ctx context.Context, pose placement.RationalRigid, o AlgebraicShellMeasureOptions) (AlgebraicShellMassBounds, error) {
	if ctx == nil {
		return AlgebraicShellMassBounds{}, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return AlgebraicShellMassBounds{}, err
	}
	matrix, err := pose.Matrix()
	if err != nil {
		return AlgebraicShellMassBounds{}, err
	}
	bits, err := placedMassBits(o.Shells.Embedding.Patches.MaxCoefficientBits, matrix)
	if err != nil {
		return AlgebraicShellMassBounds{}, err
	}
	local, err := s.BoundMassProperties(ctx, o)
	if err != nil {
		return AlgebraicShellMassBounds{}, err
	}
	return transformAlgebraicMass(ctx, local, matrix, bits)
}

// BoundPlacedMassProperties applies a proper exact occurrence pose to
// the complete original mixed material, preserving conservative flux bounds.
func (s *MixedAlgebraicShellBoolean) BoundPlacedMassProperties(ctx context.Context, pose placement.RationalRigid, o AlgebraicShellMeasureOptions) (AlgebraicShellMassBounds, error) {
	if s == nil || s.section == nil {
		return AlgebraicShellMassBounds{}, geom.ErrInvalid
	}
	return s.boundary.BoundPlacedMassProperties(ctx, pose, o)
}

func placedMassBits(bits int, m [3][4]*big.Rat) (int, error) {
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 || bits > 65536 {
		return 0, geom.ErrInvalid
	}
	for _, r := range m {
		for _, x := range r {
			if !mixedRoofFits(x, bits) {
				return 0, geom.ErrConvergence
			}
		}
	}
	return bits, nil
}

// The original result stores inertia J = tr(Q)I-Q, where Q=integral(xxᵀ).
// Rotate Q directly, then apply Q_world=RQRᵀ+t(Rm)ᵀ+(Rm)tᵀ+Vttᵀ.
func transformAlgebraicMass(ctx context.Context, m AlgebraicShellMassBounds, r [3][4]*big.Rat, bits int) (AlgebraicShellMassBounds, error) {
	var zero AlgebraicShellMassBounds
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	check := func(q geom.ExactParameterInterval) bool {
		return q.Lower != nil && q.Upper != nil && mixedRoofFits(q.Lower, bits) && mixedRoofFits(q.Upper, bits)
	}
	point := func(v *big.Rat) geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat).Set(v), Upper: new(big.Rat).Set(v)}
	}
	accumulate := func(sum *geom.ExactParameterInterval, term geom.ExactParameterInterval, scale *big.Rat) error {
		if !mixedRoofFits(scale, bits) || !check(term) {
			return geom.ErrConvergence
		}
		if scale.Sign() == 0 {
			return nil
		}
		x := scaleBooleanMeasureInterval(term, scale)
		if !check(x) {
			return geom.ErrConvergence
		}
		*sum = addBooleanMeasureIntervals(*sum, x)
		if !check(*sum) {
			return geom.ErrConvergence
		}
		return nil
	}
	if !check(m.Volume) {
		return zero, geom.ErrConvergence
	}
	out := AlgebraicShellMassBounds{AlgebraicShellVolumeBounds: AlgebraicShellVolumeBounds{
		Volume:       geom.ExactParameterInterval{Lower: new(big.Rat).Set(m.Volume.Lower), Upper: new(big.Rat).Set(m.Volume.Upper)},
		VisitedCells: m.VisitedCells, AmbiguousCells: m.AmbiguousCells}}
	var raw, world [3][3]geom.ExactParameterInterval
	var rotated [3]geom.ExactParameterInterval
	var err error
	for i := 0; i < 3; i++ {
		rotated[i] = point(new(big.Rat))
		for j := 0; j < 3; j++ {
			if err = accumulate(&rotated[i], m.FirstMoment[j], r[i][j]); err != nil {
				return zero, err
			}
			if !check(m.Inertia[i][j]) {
				return zero, geom.ErrConvergence
			}
			if i != j {
				raw[i][j] = scaleBooleanMeasureInterval(m.Inertia[i][j], big.NewRat(-1, 1))
			}
		}
		out.FirstMoment[i] = rotated[i]
		if err = accumulate(&out.FirstMoment[i], m.Volume, r[i][3]); err != nil {
			return zero, err
		}
		if m.Volume.Lower.Sign() > 0 {
			out.Centroid[i] = point(r[i][3])
			for j := 0; j < 3; j++ {
				if err = accumulate(&out.Centroid[i], m.Centroid[j], r[i][j]); err != nil {
					return zero, err
				}
			}
		}
		raw[i][i] = point(new(big.Rat))
		for j := 0; j < 3; j++ {
			scale := big.NewRat(1, 2)
			if i == j {
				scale.Neg(scale)
			}
			if err = accumulate(&raw[i][i], m.Inertia[j][j], scale); err != nil {
				return zero, err
			}
		}
		if raw[i][i].Lower.Sign() < 0 {
			raw[i][i].Lower.SetInt64(0)
		}
	}
	for i := 0; i < 3; i++ {
		if err = ctx.Err(); err != nil {
			return zero, err
		}
		for j := i; j < 3; j++ {
			x := point(new(big.Rat))
			for a := 0; a < 3; a++ {
				for b := a; b < 3; b++ {
					scale := new(big.Rat).Mul(r[i][a], r[j][b])
					if a != b {
						scale.Add(scale, new(big.Rat).Mul(r[i][b], r[j][a]))
					}
					if err = accumulate(&x, raw[a][b], scale); err != nil {
						return zero, err
					}
				}
			}
			for k, t := range [3]geom.ExactParameterInterval{rotated[i], rotated[j], m.Volume} {
				f := [3]*big.Rat{r[j][3], r[i][3], new(big.Rat).Mul(r[i][3], r[j][3])}[k]
				if err = accumulate(&x, t, f); err != nil {
					return zero, err
				}
			}
			world[i][j] = x
			if i != j {
				out.Inertia[i][j] = scaleBooleanMeasureInterval(x, big.NewRat(-1, 1))
				out.Inertia[j][i] = scaleBooleanMeasureInterval(x, big.NewRat(-1, 1))
			}
		}
	}
	for i := 0; i < 3; i++ {
		x := addBooleanMeasureIntervals(world[(i+1)%3][(i+1)%3], world[(i+2)%3][(i+2)%3])
		if !check(x) {
			return zero, geom.ErrConvergence
		}
		if x.Lower.Sign() < 0 {
			x.Lower.SetInt64(0)
		}
		out.Inertia[i][i] = x
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out, nil
}
