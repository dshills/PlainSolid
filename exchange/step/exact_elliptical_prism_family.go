package step

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

type ExactEllipticalPrismVariant string

const (
	EllipticalCardinal          ExactEllipticalPrismVariant = "cardinal"
	EllipticalPythagorean       ExactEllipticalPrismVariant = "pythagorean"
	EllipticalExactControl      ExactEllipticalPrismVariant = "exact-control"
	EllipticalTilted            ExactEllipticalPrismVariant = "tilted"
	EllipticalHomotheticAnnular ExactEllipticalPrismVariant = "homothetic-annular"
	EllipticalNestedAnnular     ExactEllipticalPrismVariant = "nested-annular"
	EllipticalOrthogonalAnnular ExactEllipticalPrismVariant = "orthogonal-annular"
)

// ExactEllipticalPrismFamily is a tagged result with exactly one typed payload.
// Source is retained only after complete ownership/geometry proof. Common fields
// describe the original material; later caller edits require a fresh proof.
type ExactEllipticalPrismFamily struct {
	Variant           ExactEllipticalPrismVariant
	Name              string
	LengthUnit        float64
	Solid             trimmed.Solid
	Source            *File
	Cardinal          *ModelOnlyEllipticalPrism
	Pythagorean       *PythagoreanEllipticalPrism
	ExactControl      *ExactControlEllipticalPrism
	Tilted            *TiltedEllipticalPrism
	HomotheticAnnular *EllipticalAnnularPrism
	NestedAnnular     *NestedEllipticalAnnularPrism
	OrthogonalAnnular *OrthogonalEllipticalAnnularPrism
}

// ReadExactEllipticalPrismFamily parses the input once under cumulative bounds,
// then selects and proves one supported exact model-only material graph. The
// lossless Pythagorean representation wins its overlap with exact controls;
// only an explicit unrepresentable result permits that exact-control fallback.
// Rounded pcurve repair and unknown/multiple material graphs remain excluded.
func ReadExactEllipticalPrismFamily(ctx context.Context, r io.Reader, l Limits, tol float64) (ExactEllipticalPrismFamily, error) {
	bad := func() (ExactEllipticalPrismFamily, error) {
		return ExactEllipticalPrismFamily{}, fmt.Errorf("step: unsupported exact elliptical prism family")
	}
	if ctx == nil || r == nil || !finiteNumber(tol) || tol <= 0 {
		return bad()
	}
	if e := ctx.Err(); e != nil {
		return ExactEllipticalPrismFamily{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return bad()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 118)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return ExactEllipticalPrismFamily{}, e
	}
	type profile struct {
		basis mappedRationalBasis
		a, b  float64
	}
	profiles := []profile{}
	for _, en := range f.Entities {
		if len(en.Records) != 1 || en.Records[0].Type != "SURFACE_OF_LINEAR_EXTRUSION" {
			continue
		}
		v := en.Records[0]
		if len(v.Args) != 3 || v.Args[1].Kind != Reference {
			return bad()
		}
		ellipse, err := simple(f, v.Args[1].Ref, "ELLIPSE", 4)
		if err != nil || ellipse.Args[1].Kind != Reference {
			return bad()
		}
		basis, e := mappedRationalFrame(f, ellipse.Args[1].Ref)
		if e != nil {
			return bad()
		}
		a, e := cylinderSupportNumber(ellipse.Args[2])
		if e != nil {
			return bad()
		}
		b, e := cylinderSupportNumber(ellipse.Args[3])
		if e != nil {
			return bad()
		}
		profiles = append(profiles, profile{basis, a, b})
	}
	out := ExactEllipticalPrismFamily{Source: f}
	if len(f.Entities) == 76 && len(profiles) == 1 {
		if l.MaxFaces < 6 || l.MaxVertices < 8 || l.MaxEdgeUses < 24 {
			return bad()
		}
		b := profiles[0].basis
		cardinal := func(v [3]*big.Rat) bool {
			n := 0
			for _, q := range v {
				if q.Sign() != 0 {
					if q.Cmp(big.NewRat(1, 1)) != 0 && q.Cmp(big.NewRat(-1, 1)) != 0 {
						return false
					}
					n++
				}
			}
			return n == 1
		}
		if cardinal(b.Z) && cardinal(b.X) {
			m, e := readModelOnlyEllipticalPrismFile(ctx, f, l, tol)
			if e != nil {
				return ExactEllipticalPrismFamily{}, e
			}
			out.Variant, out.Cardinal = EllipticalCardinal, &m
			out.Name, out.LengthUnit, out.Solid = m.Name, m.LengthUnit, m.Solid
			return out, nil
		}
		tilted := !cardinal(b.Z)
		m, e := readRationalFrameEllipticalPrismFile(ctx, f, l, tol, tilted, tilted)
		exact := false
		if !tilted && errors.Is(e, geom.ErrUnrepresentable) {
			m, e = readRationalFrameEllipticalPrismFile(ctx, f, l, tol, true, false)
			exact = true
		}
		if e != nil {
			return ExactEllipticalPrismFamily{}, e
		}
		out.Name, out.LengthUnit, out.Solid = m.Name, m.LengthUnit, m.Solid
		if tilted {
			out.Variant = EllipticalTilted
			out.Tilted = &TiltedEllipticalPrism{m}
		} else if exact {
			out.Variant = EllipticalExactControl
			q := new(big.Rat).Mul(new(big.Rat).SetFloat64(m.MajorRadius), new(big.Rat).SetFloat64(m.MinorRadius))
			q.Mul(q, new(big.Rat).SetFloat64(m.Height))
			out.ExactControl = &ExactControlEllipticalPrism{m, q}
		} else {
			out.Variant = EllipticalPythagorean
			out.Pythagorean = &m
		}
		return out, nil
	}
	if len(f.Entities) != 118 || len(profiles) != 2 || l.MaxFaces < 10 || l.MaxVertices < 16 || l.MaxEdgeUses < 48 {
		return bad()
	}
	a, b := profiles[0], profiles[1]
	if a.a < b.a {
		a, b = b, a
	}
	orthogonal := true
	aligned := true
	for i := 0; i < 3; i++ {
		orthogonal = orthogonal && b.basis.X[i].Cmp(a.basis.Y[i]) == 0
		aligned = aligned && b.basis.X[i].Cmp(a.basis.X[i]) == 0
	}
	if !orthogonal && !aligned {
		return bad()
	}
	nonhomothetic := !annularDimensions(a.a, a.b, b.a, b.b, 1)
	m, e := readOrientedEllipticalAnnularPrismFile(ctx, f, l, tol, nonhomothetic, orthogonal)
	if e != nil {
		return ExactEllipticalPrismFamily{}, e
	}
	out.Name, out.LengthUnit, out.Solid = m.Name, m.LengthUnit, m.Solid
	if orthogonal {
		out.Variant = EllipticalOrthogonalAnnular
		out.OrthogonalAnnular = &OrthogonalEllipticalAnnularPrism{m}
	} else if nonhomothetic {
		out.Variant = EllipticalNestedAnnular
		out.NestedAnnular = &NestedEllipticalAnnularPrism{m}
	} else {
		out.Variant = EllipticalHomotheticAnnular
		out.HomotheticAnnular = &m
	}
	return out, nil
}

// WriteExactEllipticalPrismFamilySource revalidates and re-emits the retained
// source graph, not caller edits to decoded fields. Names, units and original
// entity IDs survive; malformed or unsupported graphs produce no output.
func WriteExactEllipticalPrismFamilySource(ctx context.Context, w io.Writer, source *File, l Limits, tol float64) error {
	if ctx == nil || w == nil || source == nil || (len(source.Entities) != 76 && len(source.Entities) != 118) || !ellipseOwnedSourceBound(source, len(source.Entities)) {
		return geom.ErrInvalid
	}
	b, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return e
	}
	if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(b), l, tol); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
