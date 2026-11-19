package consumer

import "plainsolid/tools/refgraph/testdata/fixture/shape"

// Make is the only production use of shape outside it.
func Make() shape.Solid { return shape.New() }

// Size reaches a method, not a package-level name.
func Size() int { return Make().Size() }

type measured interface{ Area() int }

// Measured uses a Solid as an interface without naming the method.
func Measured() measured { return Make() }
