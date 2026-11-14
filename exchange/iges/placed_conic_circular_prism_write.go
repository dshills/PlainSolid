package iges

import (
	"bytes"
	"context"
	"io"
	"plainsolid/geom"
	"strings"
)

func writePlacedCircularConic(ctx context.Context, w io.Writer, a topologyAuthor, pose geom.Affine, unit, tol float64) error {
	for i, r := range a.records {
		if r.kind != 104 {
			continue
		}
		budget := 32
		v, e := parseParameters(r.data, ',', ';', &budget)
		if e != nil {
			return e
		}
		v[3] = v[1]
		text := make([]string, len(v))
		for j, p := range v {
			text[j] = p.text
		}
		a.records[i].data = strings.Join(text, ",") + ";"
	}
	return writePlacedConic(ctx, w, a, pose, unit, tol, func(raw []byte) error {
		_, e := ReadPlacedConicCircularPrismMaterial(ctx, bytes.NewReader(raw), Limits{}, tol)
		return e
	})
}
func WritePlacedConicCircularPrism(ctx context.Context, w io.Writer, d EllipticalPrismDescriptor, pose geom.Affine, unit, tol float64) error {
	p, e := circularConicProofDescriptor(d)
	if e != nil {
		return e
	}
	a, e := ellipticalPrismAuthor(p)
	if e != nil {
		return e
	}
	return writePlacedCircularConic(ctx, w, a, pose, unit, tol)
}
func WritePlacedConicCircularAnnularPrism(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDescriptor, pose geom.Affine, unit, tol float64) error {
	outer, e := circularConicProofDescriptor(EllipticalPrismDescriptor{d.OuterCoefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	inner, e := circularConicProofDescriptor(EllipticalPrismDescriptor{d.InnerCoefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	a, e := ellipticalAnnularPrismAuthor(EllipticalAnnularPrismDescriptor{outer.Coefficients, inner.Coefficients, d.LowerZ, d.UpperZ})
	if e != nil {
		return e
	}
	return writePlacedCircularConic(ctx, w, a, pose, unit, tol)
}
