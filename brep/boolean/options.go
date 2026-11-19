package boolean

import (
	"math"

	"plainsolid/brep/parametric"
)

// Operation selects the regularized set operation of Combine.
type Operation uint8

const (
	// Union is A ∪ B.
	Union Operation = iota
	// Common is A ∩ B.
	Common
	// Cut is A − B.
	Cut
)

// Options carries the tolerances and work limits of one call (P3). There is
// no hidden default tolerance: Linear and Relative must not both be zero.
type Options struct {
	// Linear is an absolute linear tolerance in model units.
	Linear float64
	// Relative is scaled by the diagonal of the box of both operands.
	Relative float64
	// Angular bounds the absolute sine used in parallel tests and the
	// absolute cosine used in perpendicular tests, in (0, 0.5].
	Angular float64
	// Volume is the relative bound of the volume identities (V5).
	Volume float64
	// Samples is the number of membership samples (V6); zero selects 64.
	Samples int
	// MaxFaces bounds the faces of each operand; zero selects 256.
	MaxFaces int
	// MaxPairs bounds the geometric pair tests; zero selects 65,536.
	MaxPairs int
	// MaxNodes bounds the nodes of the arrangement; zero selects 16,384.
	MaxNodes int
	// MaxPieces bounds the pieces; zero selects 4,096.
	MaxPieces int
	// MaxGrowth bounds a node radius as a multiple of the working
	// tolerance (T4); zero selects 4, and a nonzero value must be at
	// least 1.
	MaxGrowth float64
	// Integration is passed to the mass-property integration of V5.
	Integration parametric.IntegrationOptions
}

// normalized checks P4 and fills the defaults of P3 and section 10.
func (o Options) normalized() (Options, error) {
	bad := func(what string) (Options, error) { return Options{}, fail(ErrInvalid, "options", "%s", what) }
	for _, v := range []float64{o.Linear, o.Relative, o.Angular, o.Volume, o.MaxGrowth, o.Integration.RelativeTolerance} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return bad("a non-finite option")
		}
	}
	switch {
	case o.Linear < 0 || o.Relative < 0:
		return bad("a negative tolerance")
	case o.Linear == 0 && o.Relative == 0:
		return bad("Linear and Relative are both zero")
	case o.Angular <= 0 || o.Angular > 0.5:
		return bad("Angular outside (0, 0.5]")
	case o.Volume <= 0:
		return bad("Volume is not positive")
	case o.Samples < 0 || o.MaxFaces < 0 || o.MaxPairs < 0 || o.MaxNodes < 0 || o.MaxPieces < 0:
		return bad("a negative count")
	case o.MaxGrowth != 0 && o.MaxGrowth < 1:
		return bad("MaxGrowth below 1")
	case o.Integration.RelativeTolerance < 0 || o.Integration.RelativeTolerance >= 1 ||
		o.Integration.MaxSubdivisions < 0 || o.Integration.MaxSubdivisions > 512 || o.Integration.MaxSamples < 0:
		return bad("integration options out of range")
	}
	for _, d := range []struct {
		value *int
		zero  int
	}{{&o.Samples, 64}, {&o.MaxFaces, 256}, {&o.MaxPairs, 65536}, {&o.MaxNodes, 16384}, {&o.MaxPieces, 4096}} {
		if *d.value == 0 {
			*d.value = d.zero
		}
	}
	if o.MaxGrowth == 0 {
		o.MaxGrowth = 4
	}
	return o, nil
}
