package shape

// Twist is a method on a core type that nothing outside calls.
func (s Solid) Twist() int { return -s.n }
