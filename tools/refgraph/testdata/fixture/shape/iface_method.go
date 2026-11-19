package shape

// Area is declared away from Solid. Nothing names it outside the package;
// consumer passes a Solid where an interface with Area is wanted.
func (s Solid) Area() int { return s.n * s.n }
