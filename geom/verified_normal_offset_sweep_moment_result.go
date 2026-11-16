package geom

import "math/big"

func (w *sweepMomentWork) derive(raw sweepRawMoments) (out NormalOffsetSweepMassMoments, err error) {
	if raw[0].lo.Sign() <= 0 {
		return out, ErrConvergence
	}
	a := offsetIntervalArithmetic{b: w.budget}
	var shifted, origin [3]exactInterval
	out.Volume = copySweepInterval(raw[0])
	for i, r := range w.reference {
		origin[i] = exactInterval{r, r}
		shifted[i] = raw[1+i]
		c := a.add(origin[i], a.div(shifted[i], raw[0]))
		f := a.add(shifted[i], a.mul(origin[i], raw[0]))
		if a.err != nil {
			return NormalOffsetSweepMassMoments{}, a.err
		}
		out.Centroid[i], out.FirstMoment[i] = copySweepInterval(c), copySweepInterval(f)
	}
	var world, central [3][3]exactInterval
	for k, axes := range sweepMomentTensorAxes {
		i, j := axes[0], axes[1]
		mixed := a.add(a.mul(origin[i], shifted[j]), a.mul(origin[j], shifted[i]))
		vary := a.mul(a.mul(origin[i], origin[j]), raw[0])
		world[i][j] = a.add(a.add(raw[4+k], mixed), vary)
		central[i][j] = a.sub(raw[4+k], a.div(sweepMomentProduct(&a, shifted, i, j), raw[0]))
		if a.err != nil {
			return NormalOffsetSweepMassMoments{}, a.err
		}
		if i == j {
			world[i][j].lo = maxSweepMomentZero(world[i][j].lo)
			central[i][j].lo = maxSweepMomentZero(central[i][j].lo)
			if world[i][j].lo.Cmp(world[i][j].hi) > 0 || central[i][j].lo.Cmp(central[i][j].hi) > 0 {
				return NormalOffsetSweepMassMoments{}, ErrConvergence
			}
		}
		world[j][i], central[j][i] = world[i][j], central[i][j]
	}
	for i := range world {
		for j := range world[i] {
			x, y := world[i][j], central[i][j]
			out.SecondMoment[i][j] = copySweepInterval(x)
			if i == j {
				k, l := (i+1)%3, (i+2)%3
				x, y = a.add(world[k][k], world[l][l]), a.add(central[k][k], central[l][l])
			} else {
				x, y = a.sub(offsetZeroInterval(), x), a.sub(offsetZeroInterval(), y)
			}
			if a.err != nil {
				return NormalOffsetSweepMassMoments{}, a.err
			}
			out.InertiaOrigin[i][j], out.InertiaCentroid[i][j] = copySweepInterval(x), copySweepInterval(y)
		}
	}
	return out, nil
}

func (w *sweepMomentWork) complete(m NormalOffsetSweepMassMoments) (bool, error) {
	a := boundedRatArithmetic{b: w.budget}
	within := func(x ExactParameterInterval, target *big.Rat) bool {
		difference := a.sub(x.Upper, x.Lower)
		return a.err == nil && difference.Cmp(target) <= 0
	}
	o := w.options
	if !within(m.Volume, o.VolumeTolerance) {
		return false, a.err
	}
	for i := range m.Centroid {
		if !within(m.Centroid[i], o.CentroidTolerance) {
			return false, a.err
		}
		for j := range m.SecondMoment[i] {
			if !within(m.SecondMoment[i][j], o.SecondMomentTolerance) ||
				!within(m.InertiaOrigin[i][j], o.InertiaTolerance) ||
				!within(m.InertiaCentroid[i][j], o.InertiaTolerance) {
				return false, a.err
			}
		}
	}
	return true, nil
}
