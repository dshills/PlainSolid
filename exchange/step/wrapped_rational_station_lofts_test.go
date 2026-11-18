package step

// Independent Green integration of four rational quadratic arcs. This uses
// neither the native source evaluator nor its certified mass implementation.
func rationalStationSTEPArea() float64 {
	total := 0.
	for _, w := range []float64{.5, .75, .75, .75} {
		f := func(t float64) float64 {
			a := 1 - t
			d := a*a + 2*w*a*t + t*t
			dd := -2*a + 2*w*(1-2*t) + 2*t
			x := a*a + 2*w*a*t
			y := 2*w*a*t + t*t
			dx := -2*a + 2*w*(1-2*t)
			dy := 2*w*(1-2*t) + 2*t
			return (x*(dy*d-y*dd) - y*(dx*d-x*dd)) / (2 * d * d * d)
		}
		const n = 8192
		sum := f(0) + f(1)
		for i := 1; i < n; i++ {
			factor := 2.
			if i%2 == 1 {
				factor = 4
			}
			sum += factor * f(float64(i)/n)
		}
		total += sum / (3 * n)
	}
	return total
}
