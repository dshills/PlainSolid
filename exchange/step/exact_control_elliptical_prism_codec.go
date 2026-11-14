package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"unicode/utf8"
)

// ExactControlEllipticalPrism retains nonbinary rational model vertices and
// homogeneous cap/wall coefficients. Source axes and original ancestry are
// available through the embedded descriptor; this is a separate native domain.
type ExactControlEllipticalPrism struct {
	PythagoreanEllipticalPrism
	volumePi *big.Rat
}

// ReadExactControlEllipticalPrism proves the same exact model-only source graph
// while preserving required nonbinary native control positions and vertices.
// Source radii/height remain lossless positive binary64; decimal origins and
// source model points are exact rationals, never snapped to display vertices.
func ReadExactControlEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64) (ExactControlEllipticalPrism, error) {
	m, e := readPythagoreanEllipticalPrism(ctx, r, l, tolerance, true)
	if e != nil {
		return ExactControlEllipticalPrism{}, e
	}
	q := new(big.Rat).Mul(new(big.Rat).SetFloat64(m.MajorRadius), new(big.Rat).SetFloat64(m.MinorRadius))
	q.Mul(q, new(big.Rat).SetFloat64(m.Height))
	return ExactControlEllipticalPrism{PythagoreanEllipticalPrism: m, volumePi: q}, nil
}
func fillExactControlPrism(f *File, b mappedRationalBasis, reference Value, aVal, bVal Value, a, h float64) error {
	var e error
	if e = exactPrismBasis(b); e != nil {
		return e
	}
	zero := new(big.Rat)
	ar, hr := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(h)
	origin := exactPrismPoint(b, zero, zero, zero)
	top := exactPrismPoint(b, zero, zero, hr)
	v0 := exactPrismPoint(b, ar, zero, zero)
	v1 := exactPrismPoint(b, ar, zero, hr)
	zr := b.Z
	neg := [3]*big.Rat{}
	for i := range neg {
		neg[i] = new(big.Rat).Neg(zr[i])
	}
	set := func(id int64, p [3]*big.Rat) error {
		en := f.Entities[id]
		items := []Value{}
		for _, x := range p {
			s, e := mappedTerminatingDecimal(x)
			if e != nil {
				return e
			}
			items = append(items, Value{Kind: Number, Text: s})
		}
		en.Records[0].Args[1].Items = items
		f.Entities[id] = en
		return nil
	}
	for _, x := range []struct {
		id int64
		p  [3]*big.Rat
	}{{23, v0}, {25, v1}, {28, v0}, {30, zr}, {35, origin}, {36, zr}, {39, neg}, {58, top}, {59, zr}, {71, top}, {72, zr}, {86, origin}, {87, zr}, {99, origin}, {100, zr}} {
		if e = set(x.id, x.p); e != nil {
			return e
		}
	}
	for _, id := range []int64{37, 60, 73, 88, 101} {
		en := f.Entities[id]
		en.Records[0].Args[1] = reference
		f.Entities[id] = en
	}
	for _, id := range []int64{33, 56, 84} {
		en := f.Entities[id]
		en.Records[0].Args[2] = aVal
		en.Records[0].Args[3] = bVal
		f.Entities[id] = en
	}
	return nil
}

// ExactControlEllipticalPrismDefinition uses caller-owned rational origin
// coordinates, copied at entry. Other dimensions retain the prior exact domain.
type ExactControlEllipticalPrismDefinition struct {
	Name                             string
	Origin                           [3]*big.Rat
	Reference, Axis                  geom.Vec3
	MajorRadius, MinorRadius, Height float64
}

// WriteExactControlEllipticalPrismSTEP emits exact analytic model geometry and
// verifies the exact-control native material before writing. A required source
// Cartesian point with a nonterminating STEP decimal fails explicitly.
func WriteExactControlEllipticalPrismSTEP(ctx context.Context, w io.Writer, d ExactControlEllipticalPrismDefinition, tolerance float64) error {
	return writeRationalFrameEllipticalPrismSTEP(ctx, w, d, tolerance, false)
}
func writeRationalFrameEllipticalPrismSTEP(ctx context.Context, w io.Writer, d ExactControlEllipticalPrismDefinition, tolerance float64, tilted bool) error {
	if ctx == nil || w == nil || !d.Reference.IsFinite() || !d.Axis.IsFinite() || !finiteNumber(d.MajorRadius) || !finiteNumber(d.MinorRadius) || !finiteNumber(d.Height) || d.MajorRadius <= d.MinorRadius || d.MinorRadius <= 0 || d.Height <= 0 || len(d.Name) > 1024 || !utf8.ValidString(d.Name) {
		return geom.ErrInvalid
	}
	for _, c := range d.Name {
		if c < 32 || c == 127 {
			return geom.ErrInvalid
		}
	}
	basis := mappedRationalBasis{X: mappedRatVec(d.Reference), Z: mappedRatVec(d.Axis)}
	for i, q := range d.Origin {
		if q == nil || q.Num().BitLen() > 1024 || q.Denom().BitLen() > 1024 {
			return geom.ErrInvalid
		}
		basis.Origin[i] = new(big.Rat).Set(q)
	}
	if mappedRationalDot(basis.X, basis.Z).Sign() != 0 {
		return geom.ErrInvalid
	}
	var e error
	basis.X, e = mappedRationalUnit(basis.X)
	if e != nil {
		return e
	}
	basis.Z, e = mappedRationalUnit(basis.Z)
	if e != nil {
		return e
	}
	basis.Y = mappedRationalCross(basis.Z, basis.X)
	if tilted {
		if e = tiltedPrismBasis(basis); e != nil {
			return e
		}
	} else {
		axis, e := rationalPrismAxis(basis)
		if e != nil || axis != d.Axis {
			return geom.ErrInvalid
		}
	}
	number := func(v float64) (Value, error) {
		s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(v))
		return Value{Kind: Number, Text: s}, e
	}
	reference := Value{Kind: Aggregate}
	for _, v := range []float64{d.Reference.X, d.Reference.Y, d.Reference.Z} {
		n, e := number(v)
		if e != nil {
			return e
		}
		reference.Items = append(reference.Items, n)
	}
	a, e := number(d.MajorRadius)
	if e != nil {
		return e
	}
	b, e := number(d.MinorRadius)
	if e != nil {
		return e
	}
	f, e := rationalPrismTemplate()
	if e != nil {
		return e
	}
	if e = fillExactControlPrism(f, basis, reference, a, b, d.MajorRadius, d.Height); e != nil {
		return e
	}
	name := d.Name
	if name == "" {
		name = "Exact control elliptical prism"
	}
	en := f.Entities[7]
	en.Records[0].Args[0] = Value{Kind: String, Text: name}
	en.Records[0].Args[1] = Value{Kind: String, Text: name}
	f.Entities[7] = en
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Exact control elliptical prism'),'2;1');\nFILE_NAME('elliptical-prism.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	ids := map[int64]int64{}
	for id := range f.Entities {
		ids[id] = id
	}
	for _, id := range f.Order {
		fmt.Fprintf(&out, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], ids))
	}
	fmt.Fprintln(&out, "ENDSEC;\nEND-ISO-10303-21;")
	if _, e = readRationalFrameEllipticalPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, tolerance, true, tilted); e != nil {
		return e
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}

// ExactVolumePiCoefficient is the exact volume divided by π, from the proved
// positive-radius/right-handed extrusion. Returned rationals are caller-owned.
func (m ExactControlEllipticalPrism) ExactVolumePiCoefficient() *big.Rat {
	if m.volumePi == nil {
		return nil
	}
	return new(big.Rat).Set(m.volumePi)
}
