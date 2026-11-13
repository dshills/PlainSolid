package geom

import "math/big"

// BlindBoreContactKind describes a circular footprint whose center is in the
// closed transverse rectangle of the stock. It does not construct a solid.
type BlindBoreContactKind uint8

const (
	BlindBoreStrictInterior BlindBoreContactKind = iota + 1
	BlindBoreSideTangency
	BlindBoreMultipleTangency
	BlindBoreCrossesSide
)

// BlindBoreGenerator is an exact contact with one stock side. Side is twice
// the XYZ coordinate index, plus one for its maximum side. Start and End are
// caller-owned rational model coordinates at the entrance and blind floor.
type BlindBoreGenerator struct {
	Side       int
	Start, End [3]*big.Rat
}

// AxialBlindBoreContact retains signed axial travel in model units. Tangent
// generators have non-manifold boundary incidence: four local half-germs meet
// along each open generator. They must not be silently sewn as manifold edges.
// ManifoldExcluded is true for either tangent classification. Crossing cases
// need a different clipped-boundary construction and are not classified here
// as manifold or non-manifold.
type AxialBlindBoreContact struct {
	Kind             BlindBoreContactKind
	SignedTravel     *big.Rat
	Floor            [3]*big.Rat
	Generators       []BlindBoreGenerator
	ManifoldExcluded bool
}

// ClassifyAxialBlindBoreContact compares represented binary inputs exactly.
// axis is 0=X, 1=Y, 2=Z and direction is +1 or -1. entrance must lie on the
// corresponding inward-facing stock cap, with its other coordinates in the
// closed stock rectangle. radius and depth are positive; depth must be strictly
// less than the stock thickness (through cuts are deliberately out of scope).
// The returned rational floor is not rounded to a representable model point.
// No tolerances, topology construction, or numerical volume tests are used.
func ClassifyAxialBlindBoreContact(stock Box, entrance Vec3, radius, depth float64, axis, direction int) (AxialBlindBoreContact, error) {
	var out AxialBlindBoreContact
	if !stock.Valid() || !entrance.IsFinite() || axis < 0 || axis > 2 || (direction != 1 && direction != -1) || !finite(radius) || !finite(depth) || radius <= 0 || depth <= 0 {
		return out, ErrInvalid
	}
	lo := [3]float64{stock.Min.X, stock.Min.Y, stock.Min.Z}
	hi := [3]float64{stock.Max.X, stock.Max.Y, stock.Max.Z}
	p := [3]float64{entrance.X, entrance.Y, entrance.Z}
	for i := 0; i < 3; i++ {
		if lo[i] >= hi[i] || p[i] < lo[i] || p[i] > hi[i] {
			return out, ErrDomain
		}
	}
	cap := lo[axis]
	if direction < 0 {
		cap = hi[axis]
	}
	if p[axis] != cap {
		return out, ErrDomain
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	travel := rat(depth)
	if travel.Cmp(new(big.Rat).Sub(rat(hi[axis]), rat(lo[axis]))) >= 0 {
		return out, ErrDomain
	}
	if direction < 0 {
		travel.Neg(travel)
	}
	out.SignedTravel = new(big.Rat).Set(travel)
	for i := 0; i < 3; i++ {
		out.Floor[i] = rat(p[i])
	}
	out.Floor[axis].Add(out.Floor[axis], travel)
	crossing := false
	for i := 0; i < 3; i++ {
		if i == axis {
			continue
		}
		for side := 0; side < 2; side++ {
			distance := new(big.Rat).Sub(rat(p[i]), rat(lo[i]))
			wall := lo[i]
			if side == 1 {
				distance.Sub(rat(hi[i]), rat(p[i]))
				wall = hi[i]
			}
			cmp := distance.Cmp(rat(radius))
			if cmp < 0 {
				crossing = true
			}
			if cmp != 0 {
				continue
			}
			g := BlindBoreGenerator{Side: 2*i + side}
			for j := 0; j < 3; j++ {
				g.Start[j] = rat(p[j])
				g.End[j] = new(big.Rat).Set(out.Floor[j])
			}
			g.Start[i] = rat(wall)
			g.End[i] = rat(wall)
			out.Generators = append(out.Generators, g)
		}
	}
	out.Kind = BlindBoreStrictInterior
	if crossing {
		out.Kind = BlindBoreCrossesSide
	} else if len(out.Generators) > 1 {
		out.Kind = BlindBoreMultipleTangency
		out.ManifoldExcluded = true
	} else if len(out.Generators) == 1 {
		out.Kind = BlindBoreSideTangency
		out.ManifoldExcluded = true
	}
	return out, nil
}
