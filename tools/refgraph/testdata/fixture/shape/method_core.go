package shape

// Size is declared away from Solid. Only a method call from another package
// makes this file core, which a scan for shape.Name selectors cannot see.
func (s Solid) Size() int { return s.n }
