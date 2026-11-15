package collision

import (
	"context"
	"math/big"
	"math/bits"
	"sort"
)

// ExactAreaOptions bounds the rational area calculation. PrecisionBits is an
// absolute width of 2^-PrecisionBits in squared model units. Zero defaults to
// 40, with supported values from 8 through 512.
type ExactAreaOptions struct {
	PrecisionBits int
	Arithmetic    ExactMassOptions
}

type PlanarAreaBounds struct{ Lower, Upper *big.Rat }

type ExactPlanarFaceArea struct {
	AssemblyFace
	Area PlanarAreaBounds
}

// ExactPlanarArea contains rational bounds for the representative boundary's
// total surface area and its original face contributions. Interior cavity
// boundaries contribute positive area. Point contacts contribute no shared area.
type ExactPlanarArea struct {
	Area       PlanarAreaBounds
	Faces      []ExactPlanarFaceArea // sorted by original solid and face ID
	Operations int
}

// SurfaceAreaExactBounds uses exact triangle areas and rigorous square-root
// enclosures. It reuses the already validated planar point-locator snapshot.
func (l *PointLocator) SurfaceAreaExactBounds(ctx context.Context, o ExactAreaOptions) (ExactPlanarArea, error) {
	if l == nil {
		return ExactPlanarArea{}, ErrInvalid
	}
	return exactPlanarArea(ctx, []*PointLocator{l}, o)
}

// SurfaceAreaExactBounds measures the complete point-contact compound. The
// declared shared vertices do not change any of its surface areas.
func (l *VertexContactAssemblyLocator) SurfaceAreaExactBounds(ctx context.Context, o ExactAreaOptions) (ExactPlanarArea, error) {
	if l == nil {
		return ExactPlanarArea{}, ErrInvalid
	}
	parts := make([]*PointLocator, len(l.components))
	for i, component := range l.components {
		parts[i] = component.loc
	}
	return exactPlanarArea(ctx, parts, o)
}

func exactPlanarArea(ctx context.Context, parts []*PointLocator, o ExactAreaOptions) (ExactPlanarArea, error) {
	var zero ExactPlanarArea
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 40
	}
	if o.PrecisionBits < 8 || o.PrecisionBits > 512 {
		return zero, ErrInvalid
	}
	a, err := newPlanarMassArithmetic(ctx, o.Arithmetic)
	if err != nil {
		return zero, err
	}
	count := 0
	for _, c := range parts {
		count += len(c.triangles)
	}
	// Each inexact magnitude interval has width at most 2^-places. After
	// the factor 1/2, that makes the aggregate error smaller than requested.
	places := o.PrecisionBits + bits.Len(uint(count))
	half := big.NewRat(1, 2)
	r := ExactPlanarArea{Area: PlanarAreaBounds{new(big.Rat), new(big.Rat)}}
	faces := map[AssemblyFace]PlanarAreaBounds{}
	for solid, c := range parts {
		for _, triangle := range c.triangles {
			length2 := new(big.Rat)
			for _, component := range triangle.n {
				length2 = a.add(length2, a.mul(component, component))
			}
			if a.err != nil {
				return zero, a.err
			}
			positive, err := planarAreaSquareRoot(length2, places, &a)
			if err != nil {
				return zero, err
			}
			record := AssemblyFace{Solid: solid, Face: triangle.face}
			prior, ok := faces[record]
			if !ok {
				prior = PlanarAreaBounds{new(big.Rat), new(big.Rat)}
			}
			lo, hi := a.mul(positive.Lower, half), a.mul(positive.Upper, half)
			faces[record] = PlanarAreaBounds{a.add(prior.Lower, lo), a.add(prior.Upper, hi)}
			r.Area = PlanarAreaBounds{a.add(r.Area.Lower, lo), a.add(r.Area.Upper, hi)}
			if a.err != nil {
				return zero, a.err
			}
		}
	}
	for face, area := range faces {
		r.Faces = append(r.Faces, ExactPlanarFaceArea{face, area})
	}
	if err := a.budget.tick(len(r.Faces) * bits.Len(uint(len(r.Faces)))); err != nil {
		return zero, err
	}
	sort.Slice(r.Faces, func(i, j int) bool {
		if r.Faces[i].Solid != r.Faces[j].Solid {
			return r.Faces[i].Solid < r.Faces[j].Solid
		}
		return r.Faces[i].Face < r.Faces[j].Face
	})
	r.Operations = a.budget.work
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return r, nil
}

func planarAreaSquareRoot(n *big.Rat, places int, a *planarMassArithmetic) (PlanarAreaBounds, error) {
	if n.Sign() <= 0 {
		return PlanarAreaBounds{}, ErrInvalid
	}
	if err := a.budget.tick(1); err != nil {
		return PlanarAreaBounds{}, err
	}
	p := new(big.Int).Sqrt(n.Num())
	q := new(big.Int).Sqrt(n.Denom())
	if new(big.Int).Mul(p, p).Cmp(n.Num()) == 0 && new(big.Int).Mul(q, q).Cmp(n.Denom()) == 0 {
		x := a.check(new(big.Rat).SetFrac(p, q))
		return PlanarAreaBounds{x, new(big.Rat).Set(x)}, a.err
	}
	if n.Num().BitLen()+2*places > a.bits || places+1 > a.bits {
		return PlanarAreaBounds{}, ErrLimit
	}
	// floor(sqrt(floor(n * 2^(2*places)))) isolates the exact radical.
	value := new(big.Int).Lsh(new(big.Int).Set(n.Num()), uint(2*places))
	value.Quo(value, n.Denom())
	p.Sqrt(value)
	q.Lsh(big.NewInt(1), uint(places))
	hi := new(big.Int).Add(p, big.NewInt(1))
	return PlanarAreaBounds{a.check(new(big.Rat).SetFrac(p, q)), a.check(new(big.Rat).SetFrac(hi, q))}, a.err
}
