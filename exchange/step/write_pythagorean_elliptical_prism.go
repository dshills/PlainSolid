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

// PythagoreanEllipticalPrismDefinition authors an exact in-plane rotated prism.
// Axis is a signed cardinal unit vector. Reference contains original direction
// ratios, e.g.(3,4,0), not rounded normalized binary64 coefficients. Its exact
// norm must be rational and it must be orthogonal to Axis. Units are millimeters.
type PythagoreanEllipticalPrismDefinition struct {
	Name                             string
	Origin, Reference, Axis          geom.Vec3
	MajorRadius, MinorRadius, Height float64
}

// WritePythagoreanEllipticalPrismSTEP authors a complete model-only product only
// after the strict reader proves its geometry and lossless native controls.
// There is no support/endpoint snapping, repair budget or rounded unit frame.
func WritePythagoreanEllipticalPrismSTEP(ctx context.Context, w io.Writer, d PythagoreanEllipticalPrismDefinition, tolerance float64) error {
	if ctx == nil || w == nil || !d.Origin.IsFinite() || !d.Reference.IsFinite() || !d.Axis.IsFinite() || !finiteNumber(d.MajorRadius) || !finiteNumber(d.MinorRadius) || !finiteNumber(d.Height) || d.MajorRadius <= d.MinorRadius || d.MinorRadius <= 0 || d.Height <= 0 || len(d.Name) > 1024 || !utf8.ValidString(d.Name) {
		return geom.ErrInvalid
	}
	for _, c := range d.Name {
		if c < 32 || c == 127 {
			return geom.ErrInvalid
		}
	}
	basis := mappedRationalBasis{Origin: mappedRatVec(d.Origin), X: mappedRatVec(d.Reference), Z: mappedRatVec(d.Axis)}
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
	axis, e := rationalPrismAxis(basis)
	if e != nil || axis != d.Axis {
		return geom.ErrInvalid
	}
	basis.Y = mappedRationalCross(basis.Z, basis.X)
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
	if e = fillRationalPrism(f, basis, reference, a, b, d.MajorRadius, d.Height); e != nil {
		return e
	}
	name := d.Name
	if name == "" {
		name = "Exact Pythagorean elliptical prism"
	}
	en := f.Entities[7]
	en.Records[0].Args[0] = Value{Kind: String, Text: name}
	en.Records[0].Args[1] = Value{Kind: String, Text: name}
	f.Entities[7] = en
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Exact Pythagorean elliptical prism'),'2;1');\nFILE_NAME('elliptical-prism.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	identity := map[int64]int64{}
	for id := range f.Entities {
		identity[id] = id
	}
	for _, id := range f.Order {
		fmt.Fprintf(&out, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], identity))
	}
	fmt.Fprintln(&out, "ENDSEC;\nEND-ISO-10303-21;")
	if _, e = ReadPythagoreanEllipticalPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, tolerance); e != nil {
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

// WritePythagoreanEllipticalPrismSource preserves original direction ratios and
// entity ancestry after re-proving the complete source and exact native domain.
func WritePythagoreanEllipticalPrismSource(ctx context.Context, w io.Writer, f *File, l Limits, tolerance float64) error {
	if w == nil || !ellipseOwnedSourceBound(f, 76) {
		return geom.ErrInvalid
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		return e
	}
	if _, e = ReadPythagoreanEllipticalPrism(ctx, bytes.NewReader(b), l, tolerance); e != nil {
		return e
	}
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}
