package boolean

import (
	"math"
	"sort"
)

// The rows of the curve table for a carrier with no closed form (T2 and
// T3 of specs/general-boolean-quartic) find their hits by sampling a
// function of the carrier's parameter over one period.

// scanSteps is the number of equal steps of the period at which such a
// function is sampled.
const scanSteps = 256

// zeroOf finds by bisection a change of sign of f between lo, where it
// has the sign given, and hi.
func zeroOf(f func(float64) float64, lo, hi float64, sign int) float64 {
	for range 80 {
		m := lo + (hi-lo)/2
		if m <= lo || m >= hi {
			break
		}
		if (f(m) > 0) == (sign > 0) {
			lo = m
		} else {
			hi = m
		}
	}
	return lo + (hi-lo)/2
}

// leastOf finds where f is least between lo and hi, by narrowing about
// the least of 16 samples, and gives that value.
func leastOf(f func(float64) float64, lo, hi float64) (at, least float64) {
	const samples = 16
	least, at = math.Inf(1), lo
	for k := 0; k <= samples; k++ {
		t := lo + (hi-lo)*float64(k)/samples
		if v := f(t); v < least {
			least, at = v, t
		}
	}
	a, b := math.Max(lo, at-(hi-lo)/samples), math.Min(hi, at+(hi-lo)/samples)
	for range 80 {
		p, q := a+(b-a)/3, b-(b-a)/3
		if p <= a || q >= b {
			break
		}
		if f(p) <= f(q) {
			b = q
		} else {
			a = p
		}
	}
	at = a + (b-a)/2
	return at, f(at)
}

// zeros finds where f, of the given period, is zero within tol (T2). A
// hit through zero is placed where f changes sign between a sample above
// tol and one below −tol. Where f comes within tol of zero and turns
// back on the same side the hit is touching, and a dip past the other
// bound and back is two hits through. Between two neighbouring samples
// on one side a search is made only where one of them is a least value
// among its neighbours. The second result is true when every sample is
// within tol.
func zeros(f func(float64) float64, period, tol float64) ([]trigHit, bool) {
	const n = scanSteps
	var value [n]float64
	var side [n]int
	first := -1
	for i := range value {
		value[i] = f(period * float64(i) / n)
		switch {
		case value[i] > tol:
			side[i] = 1
		case value[i] < -tol:
			side[i] = -1
		}
		if side[i] != 0 && first < 0 {
			first = i
		}
	}
	if first < 0 {
		return nil, true
	}
	at := func(i int) float64 { return period * float64(i) / n }
	var out []trigHit
	add := func(t float64, touching bool) {
		if t = math.Mod(t, period); t < 0 {
			t += period
		}
		out = append(out, trigHit{t, touching})
	}
	// dip looks between lo and hi, where f is on the side given at both,
	// for the place it comes nearest the other side.
	dip := func(lo, hi float64, s int) {
		t, least := leastOf(func(t float64) float64 { return float64(s) * f(t) }, lo, hi)
		switch {
		case least > tol:
		case least >= -tol:
			add(t, true)
		default:
			add(zeroOf(f, lo, t, s), false)
			add(zeroOf(f, t, hi, -s), false)
		}
	}
	for i := first; i < first+n; {
		j := i + 1
		for side[j%n] == 0 {
			j++
		}
		s := side[i%n]
		switch {
		case side[j%n] != s:
			add(zeroOf(f, at(i), at(j), s), false)
		case j > i+1:
			dip(at(i), at(j), s)
		default:
			// Sample j is on the same side as its neighbour before it.
			// Where it is also a least value among its neighbours, f
			// may dip between them.
			v := func(k int) float64 { return float64(s) * value[k%n] }
			if side[(j+1)%n] == s && v(j) <= v(i) && v(j) < v(j+1) {
				dip(at(i), at(j+1), s)
			}
		}
		i = j
	}
	sort.Slice(out, func(a, b int) bool { return out[a].t < out[b].t })
	return out, false
}

// minima finds where g, of the given period and never negative, comes
// within reach of zero (T3). slope bounds how fast g changes, so that a
// least sample too far from zero to get within reach inside a step is
// left alone. The second result is true when every sample is within tol.
func minima(g func(float64) float64, period, reach, tol, slope float64) ([]float64, bool) {
	const n = scanSteps
	var value [n]float64
	all := true
	for i := range value {
		value[i] = g(period * float64(i) / n)
		all = all && value[i] <= tol
	}
	if all {
		return nil, true
	}
	var out []float64
	step := period / n
	for i := range value {
		before, after := value[(i+n-1)%n], value[(i+1)%n]
		if value[i] > before || value[i] >= after || value[i] > reach+slope*step {
			continue
		}
		t, least := leastOf(g, step*float64(i-1), step*float64(i+1))
		if least > reach {
			continue
		}
		if t = math.Mod(t, period); t < 0 {
			t += period
		}
		out = append(out, t)
	}
	sort.Float64s(out)
	return out, false
}
