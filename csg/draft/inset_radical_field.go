package draft

import "math/big"

// A basis index is the mask of independent square-root generators in a term.
// The empty mask is rational. A nil coefficient is exactly zero.
type insetAlgebraic [16]*big.Rat

func (a insetAlgebraic) coefficient(i int) *big.Rat {
	if a[i] == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Set(a[i])
}

type insetRadicalField struct {
	work     *budget
	radicals []*big.Rat
	roots    map[int][][2]*big.Rat
}

func (f *insetRadicalField) includeRoot(squared *big.Rat, maxGenerators int) (int, *big.Rat, error) {
	f.work.check(squared)
	for index, basis := range f.radicals {
		if !f.work.tick() {
			return 0, nil, f.work.err
		}
		ratio := div(squared, basis)
		f.work.check(ratio)
		if coefficient, ok := rationalSqrt(ratio); ok {
			return index, coefficient, f.work.err
		}
	}
	previous := len(f.radicals)
	if previous == 1<<maxGenerators {
		return 0, nil, ErrUnsupported
	}
	for i := 0; i < previous; i++ {
		radical := mul(squared, f.radicals[i])
		f.work.check(radical)
		f.radicals = append(f.radicals, radical)
	}
	return previous, big.NewRat(1, 1), f.work.err
}

func (f *insetRadicalField) addTerm(out *insetAlgebraic, i int, value *big.Rat) {
	if value == nil || value.Sign() == 0 {
		return
	}
	if out[i] != nil {
		value = add(out[i], value)
	}
	f.work.check(value)
	if value.Sign() == 0 {
		out[i] = nil
	} else {
		out[i] = value
	}
}

func (f *insetRadicalField) plus(a, b insetAlgebraic) insetAlgebraic {
	for i := range f.radicals {
		f.addTerm(&a, i, b[i])
	}
	return a
}

func (f *insetRadicalField) minus(a, b insetAlgebraic) insetAlgebraic {
	for i := range f.radicals {
		if b[i] != nil {
			f.addTerm(&a, i, new(big.Rat).Neg(b[i]))
		}
	}
	return a
}

func (f *insetRadicalField) scaled(a insetAlgebraic, scale *big.Rat) insetAlgebraic {
	var out insetAlgebraic
	if scale.Sign() == 0 {
		return out
	}
	for i := range f.radicals {
		if a[i] != nil {
			f.addTerm(&out, i, mul(a[i], scale))
		}
	}
	return out
}

func (f *insetRadicalField) product(a, b insetAlgebraic) insetAlgebraic {
	var out insetAlgebraic
	for i := range f.radicals {
		if a[i] == nil {
			continue
		}
		for j := range f.radicals {
			if b[j] == nil {
				continue
			}
			if !f.work.tick() {
				return out
			}
			// Repeated generators become a rational factor.
			f.addTerm(&out, i^j, mul(mul(a[i], b[j]), f.radicals[i&j]))
		}
	}
	return out
}

func (f *insetRadicalField) sign(a insetAlgebraic) int {
	if f.work.err != nil {
		return 0
	}
	var positive, negative int
	pos, neg := -1, -1
	for i := range f.radicals {
		if a[i] == nil {
			continue
		}
		if a[i].Sign() > 0 {
			positive++
			pos = i
		} else {
			negative++
			neg = i
		}
	}
	if positive == 0 || negative == 0 {
		switch {
		case positive > 0:
			return 1
		case negative > 0:
			return -1
		default:
			return 0
		}
	}
	if positive == 1 && negative == 1 {
		x := mul(mul(a[pos], a[pos]), f.radicals[pos])
		y := mul(mul(a[neg], a[neg]), f.radicals[neg])
		f.work.check(x)
		f.work.check(y)
		return x.Cmp(y)
	}
	// Distinct independent square classes are linearly independent over Q.
	// A nonzero combination has a sign even if a bounded search cannot prove it.
	maxPrecision := min(4096, f.work.o.MaxBits/2)
	for precision := min(64, maxPrecision); precision > 0; precision = min(2*precision, maxPrecision) {
		if !f.work.tick() {
			return 0
		}
		lo, hi := f.bounds(a, precision)
		if f.work.err != nil {
			return 0
		}
		if lo.Sign() > 0 {
			return 1
		}
		if hi.Sign() < 0 {
			return -1
		}
		if precision == maxPrecision {
			break
		}
	}
	f.work.err = ErrLimit
	return 0
}

func (f *insetRadicalField) bounds(a insetAlgebraic, precision int) (*big.Rat, *big.Rat) {
	roots := f.roots[precision]
	if roots == nil {
		if f.roots == nil {
			f.roots = make(map[int][][2]*big.Rat)
		}
		roots = make([][2]*big.Rat, len(f.radicals))
		roots[0] = [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}
		den := new(big.Int).Lsh(big.NewInt(1), uint(precision))
		for i, radical := range f.radicals[1:] {
			n := new(big.Int).Lsh(radical.Num(), uint(2*precision))
			n.Quo(n, radical.Denom()).Sqrt(n)
			roots[i+1] = [2]*big.Rat{new(big.Rat).SetFrac(n, den), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), den)}
		}
		f.roots[precision] = roots
	}
	lower, upper := new(big.Rat), new(big.Rat)
	for i, root := range roots {
		if a[i] == nil {
			continue
		}
		if a[i].Sign() < 0 {
			root[0], root[1] = root[1], root[0]
		}
		lower.Add(lower, mul(a[i], root[0]))
		upper.Add(upper, mul(a[i], root[1]))
		f.work.check(lower)
		f.work.check(upper)
	}
	return lower, upper
}
