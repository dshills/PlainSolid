package boolean

import (
	"math"
	"testing"
)

// trigWithZeros is a polynomial of degree two with the four zeros given,
// positive just after the first when they are distinct and in order:
// the product of sin((t − r)/2) over the four.
func trigWithZeros(r [4]float64) trig {
	// sin((t−p)/2)·sin((t−q)/2) = (cos((q−p)/2) − cos(t − (p+q)/2))/2.
	c12, m12 := math.Cos((r[1]-r[0])/2), (r[0]+r[1])/2
	c34, m34 := math.Cos((r[3]-r[2])/2), (r[2]+r[3])/2
	// (c12 − cos(t−m12))(c34 − cos(t−m34))/4, with the product of the two
	// cosines written as (cos(2t − m12 − m34) + cos(m34 − m12))/2.
	return trig{
		(c12*c34 + math.Cos(m34-m12)/2) / 4,
		(-c12*math.Cos(m34) - c34*math.Cos(m12)) / 4,
		(-c12*math.Sin(m34) - c34*math.Sin(m12)) / 4,
		math.Cos(m12+m34) / 8,
		math.Sin(m12+m34) / 8,
	}
}

func TestTrigHits(t *testing.T) {
	const tol = 1e-7
	type want struct {
		at       float64
		touching bool
	}
	for _, c := range []struct {
		name string
		f    trig
		want []want
		all  bool
	}{
		{"a constant", trig{3}, nil, false},
		{"zero all the way round", trig{tol / 2}, nil, true},
		{"cos t", trig{0, 1}, []want{{math.Pi / 2, false}, {3 * math.Pi / 2, false}}, false},
		{"cos 2t", trig{0, 0, 0, 1}, []want{{math.Pi / 4, false}, {3 * math.Pi / 4, false}, {5 * math.Pi / 4, false}, {7 * math.Pi / 4, false}}, false},
		{"touching at pi", trig{1, 1}, []want{{math.Pi, true}}, false},
		{"touching from below at the origin", trig{-1, 1}, []want{{0, true}}, false},
		{"within the tolerance of touching", trig{1 + tol/2, 1}, []want{{math.Pi, true}}, false},
		{"clear of touching by twice the tolerance", trig{1 + 2*tol, 1}, nil, false},
		{"dipping through by half the tolerance", trig{1 - tol/2, 1}, []want{{math.Pi, true}}, false},
		{"four zeros", trigWithZeros([4]float64{0.3, 1.1, 2.9, 5.2}), []want{{0.3, false}, {1.1, false}, {2.9, false}, {5.2, false}}, false},
		{"a double zero and two single ones", trigWithZeros([4]float64{2, 2, 3, 5}), []want{{2, true}, {3, false}, {5, false}}, false},
		{"zeros either side of the origin", trigWithZeros([4]float64{0.01, 2, 4, 2*math.Pi - 0.01}), []want{{0.01, false}, {2, false}, {4, false}, {2*math.Pi - 0.01, false}}, false},
	} {
		got, all := c.f.hits(tol)
		if all != c.all || len(got) != len(c.want) {
			t.Errorf("%s: hits %+v all round %v, want %+v", c.name, got, all, c.want)
			continue
		}
		for _, w := range c.want {
			found := false
			for _, h := range got {
				d := math.Abs(h.t - w.at)
				found = found || math.Min(d, 2*math.Pi-d) < 1e-6 && h.touching == w.touching
			}
			if !found {
				t.Errorf("%s: no hit at %.9g (touching %v) among %+v", c.name, w.at, w.touching, got)
			}
		}
	}
}

func TestTrigAbove(t *testing.T) {
	f := trig{1, 1} // 1 + cos t: 2 at the origin, 0 at pi
	for _, c := range []struct {
		t0, t1, level float64
		above, below  bool
	}{
		{0, 1, 1.5, true, false},
		{0, 2, 0.5, true, false},
		{0, math.Pi, -1e-9, true, false},
		{0, math.Pi, 1e-9, false, false},
		{2, 4, 0.6, false, true},
		{2, 4, 0.58, false, false},
		{0, 2 * math.Pi, 2 + 1e-9, false, true},
	} {
		if got := f.above(c.t0, c.t1, c.level); got != c.above {
			t.Errorf("above %g over [%g, %g]: %v", c.level, c.t0, c.t1, got)
		}
		if got := f.below(c.t0, c.t1, c.level); got != c.below {
			t.Errorf("below %g over [%g, %g]: %v", c.level, c.t0, c.t1, got)
		}
	}
}
