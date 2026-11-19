package shape

// Solid is reached from another package, so its file is core.
type Solid struct{ n int }

// New is a root: consumer/use.go calls it.
func New() Solid { return Solid{n: scale()} }
