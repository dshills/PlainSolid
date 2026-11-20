package boolean

import "math"

// trig is the trigonometric polynomial of degree two
// a0 + a1·cos t + b1·sin t + a2·cos 2t + b2·sin 2t, held as those five
// coefficients. The square of the distance from a point of a circle or an
// ellipse to a line is one.
type trig [5]float64

func (f trig) at(t float64) float64 {
	s, c := math.Sincos(t)
	s2, c2 := math.Sincos(2 * t)
	return f[0] + f[1]*c + f[2]*s + f[3]*c2 + f[4]*s2
}

// bend bounds the second derivative of f. Over a part of width h, f
// leaves the chord between its end values by at most bend·h²/8.
func (f trig) bend() float64 {
	return math.Abs(f[1]) + math.Abs(f[2]) + 4*(math.Abs(f[3])+math.Abs(f[4]))
}

// trigDepth is how often a part of the period is halved before it is
// left undecided. A part is then about 1e-9 wide.
const trigDepth = 30

// above reports whether f is greater than level everywhere from t0 to t1.
// It answers true only where the bound of bend shows it, and false where
// it finds a value at or below the level or cannot decide.
func (f trig) above(t0, t1, level float64) bool {
	bend := f.bend()
	var visit func(lo, hi, flo, fhi float64, left int) bool
	visit = func(lo, hi, flo, fhi float64, left int) bool {
		least := math.Min(flo, fhi)
		if least <= level {
			return false
		}
		if h := hi - lo; least-bend*h*h/8 > level {
			return true
		}
		if left == 0 {
			return false
		}
		mid := lo + (hi-lo)/2
		fm := f.at(mid)
		return visit(lo, mid, flo, fm, left-1) && visit(mid, hi, fm, fhi, left-1)
	}
	// Parts of at most an eighth of a turn, so that the bound is useful
	// from the start.
	parts := int(math.Ceil((t1-t0)/(math.Pi/4))) + 1
	lo, flo := t0, f.at(t0)
	for k := 1; k <= parts; k++ {
		hi := t0 + (t1-t0)*float64(k)/float64(parts)
		fhi := f.at(hi)
		if !visit(lo, hi, flo, fhi, trigDepth) {
			return false
		}
		lo, flo = hi, fhi
	}
	return true
}

// below reports whether f is less than level everywhere from t0 to t1.
func (f trig) below(t0, t1, level float64) bool {
	return trig{-f[0], -f[1], -f[2], -f[3], -f[4]}.above(t0, t1, -level)
}

// trigHit is a place where f is zero within a tolerance: through it,
// where f passes from one side of zero to the other, or touching, where
// it comes within the tolerance of zero and turns back.
type trigHit struct {
	t        float64
	touching bool
}

// hits finds where f is zero within tol over one period. The period is
// cut into parts on each of which f is shown to stay above tol, to stay
// below −tol, or to stay within tol of zero; a part that cannot be shown
// to do any of these is halved. A run of parts within tol, between a part
// above and a part below, holds a hit through zero, placed at the change
// of sign; between two parts on the same side it holds a touching hit,
// placed where f turns back, which is between its two zeros where it
// dips through zero on the way. Deciding at tol, and not at zero, is what
// keeps the rounding of f, which is far below tol, from making hits. The
// second result is true when f is within tol of zero all the way round.
func (f trig) hits(tol float64) ([]trigHit, bool) {
	const pieces = 8
	type part struct {
		lo, hi float64
		side   int // 1 above tol, −1 below −tol, 0 within tol or undecided
	}
	bend := f.bend()
	var parts []part
	var visit func(lo, hi, flo, fhi float64, left int)
	visit = func(lo, hi, flo, fhi float64, left int) {
		h := hi - lo
		slack := bend * h * h / 8
		switch least, most := math.Min(flo, fhi), math.Max(flo, fhi); {
		case least-slack > tol:
			parts = append(parts, part{lo, hi, 1})
		case most+slack < -tol:
			parts = append(parts, part{lo, hi, -1})
		case least-slack >= -tol && most+slack <= tol, left == 0:
			parts = append(parts, part{lo, hi, 0})
		default:
			mid := lo + h/2
			fm := f.at(mid)
			visit(lo, mid, flo, fm, left-1)
			visit(mid, hi, fm, fhi, left-1)
		}
	}
	first := f.at(0)
	lo, flo := 0., first
	for k := 1; k <= pieces; k++ {
		hi, fhi := 2*math.Pi*float64(k)/pieces, first
		if k < pieces {
			fhi = f.at(hi)
		}
		visit(lo, hi, flo, fhi, trigDepth)
		lo, flo = hi, fhi
	}
	// Start the walk at a part with a side, so that no run is cut by the
	// origin of the parameter.
	start := -1
	for i, p := range parts {
		if p.side != 0 {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, true
	}
	var out []trigHit
	n := len(parts)
	for i := 0; i < n; {
		p := parts[(start+i)%n]
		if p.side != 0 {
			i++
			continue
		}
		// A run of parts within tol, from lo to hi, hi taken past the
		// period where the run goes through the origin.
		before := parts[(start+i-1+n)%n].side
		lo, hi := p.lo, p.hi
		for i++; parts[(start+i)%n].side == 0; i++ {
			hi = parts[(start+i)%n].hi
		}
		if hi < lo {
			hi += 2 * math.Pi
		}
		after := parts[(start+i)%n].side
		at, touching := f.turn(lo, hi, before), true
		if before != after {
			at, touching = f.zero(lo, hi, before), false
		}
		out = append(out, trigHit{math.Mod(at, 2*math.Pi), touching})
	}
	return out, false
}

// zero finds by bisection a change of sign of f between lo, where it has
// the sign given, and hi.
func (f trig) zero(lo, hi float64, sign int) float64 { return zeroOf(f.at, lo, hi, sign) }

// turn finds where f, coming from the side given, is farthest towards the
// other side between lo and hi: at a sample of the interval, improved by
// narrowing about it.
func (f trig) turn(lo, hi float64, side int) float64 {
	const samples = 64
	value := func(t float64) float64 { return float64(side) * f.at(t) }
	best, at := math.Inf(1), lo
	for k := 0; k <= samples; k++ {
		t := lo + (hi-lo)*float64(k)/samples
		if v := value(t); v < best {
			best, at = v, t
		}
	}
	a, b := math.Max(lo, at-(hi-lo)/samples), math.Min(hi, at+(hi-lo)/samples)
	for range 80 {
		p, q := a+(b-a)/3, b-(b-a)/3
		if value(p) <= value(q) {
			b = q
		} else {
			a = p
		}
	}
	return a + (b-a)/2
}
