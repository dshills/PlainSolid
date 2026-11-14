package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"plainsolid/geom"
)

// ExactEllipticalPrismCollection retains one to sixteen independently rooted
// materials. It records explicit source geometry without inferring instances.
type ExactEllipticalPrismCollection struct {
	Members    []ExactEllipticalPrismFamily
	Source     *File
	LengthUnit float64
}

// ReadExactEllipticalPrismCollection validates every member, cumulative native
// budgets and exact original graph ownership. Only neutral identity placement
// records may be shared; material/support/unit/product aliases fail atomically.
func ReadExactEllipticalPrismCollection(ctx context.Context, r io.Reader, l Limits, tol float64) (ExactEllipticalPrismCollection, error) {
	return readExactEllipticalPrismCollection(ctx, r, l, tol, 1, 16)
}

// WriteExactEllipticalPrismCollection independently proves one to sixteen source
// products and revalidates their combined graph before output. Source frames and
// coordinates are retained literally; no rigid transform is baked or inferred.
func WriteExactEllipticalPrismCollection(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64) error {
	return writeExactEllipticalPrismCollection(ctx, w, sources, l, tol, 1, 16)
}
func readExactEllipticalPrismCollection(ctx context.Context, r io.Reader, l Limits, tol float64, minRoots, maxRoots int) (ExactEllipticalPrismCollection, error) {
	var e error
	l, e = l.defaults()
	if e != nil {
		return ExactEllipticalPrismCollection{}, e
	}
	l.MaxBytes = min(l.MaxBytes, int64(65536*maxRoots))
	l.MaxEntities = min(l.MaxEntities, 118*maxRoots)
	l.MaxValues = min(l.MaxValues, 4096*maxRoots)
	p, e := readIndependentExactMaterialCollection(ctx, r, l, tol, minRoots, maxRoots, false)
	if e != nil {
		return ExactEllipticalPrismCollection{}, e
	}
	out := ExactEllipticalPrismCollection{Source: p.Source, LengthUnit: p.LengthUnit}
	for _, m := range p.Members {
		if m.Elliptical == nil {
			return ExactEllipticalPrismCollection{}, geom.ErrInvalid
		}
		out.Members = append(out.Members, *m.Elliptical)
	}
	return out, nil
}

func writeExactEllipticalPrismCollection(ctx context.Context, w io.Writer, sources []*File, l Limits, tol float64, minRoots, maxRoots int) error {
	if ctx == nil || w == nil || len(sources) < minRoots || len(sources) > maxRoots {
		return geom.ErrInvalid
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Independent exact ellipse materials'),'2;1');\nFILE_NAME('ellipse-compound.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	next := int64(1)
	for _, source := range sources {
		if source == nil || (len(source.Entities) != 76 && len(source.Entities) != 118) || !ellipseOwnedSourceBound(source, len(source.Entities)) {
			return geom.ErrInvalid
		}
		raw, e := mappedPlanarSourceBytes(source)
		if e != nil {
			return e
		}
		if _, e = ReadExactEllipticalPrismFamily(ctx, bytes.NewReader(raw), l, tol); e != nil {
			return e
		}
		ids := map[int64]int64{}
		for _, id := range source.Order {
			ids[id] = next
			next++
		}
		for _, id := range source.Order {
			fmt.Fprintf(&out, "#%d=%s;\n", ids[id], assemblyEntity(source.Entities[id], ids))
		}
	}
	fmt.Fprintln(&out, "ENDSEC;END-ISO-10303-21;")
	if _, e := readExactEllipticalPrismCollection(ctx, bytes.NewReader(out.Bytes()), l, tol, minRoots, maxRoots); e != nil {
		return e
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	n, e := w.Write(out.Bytes())
	if e == nil && n != out.Len() {
		e = io.ErrShortWrite
	}
	return e
}
