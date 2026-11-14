package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"unicode/utf8"
)

// EllipticalAnnularPrism retains a concentric homothetic elliptical through hole.
// Native face order is lower/upper annuli, four outer and four inner wall charts.
type EllipticalAnnularPrism struct {
	Name                                                                             string
	MajorRadius, MinorRadius, InnerMajorRadius, InnerMinorRadius, Height, LengthUnit float64
	Solid                                                                            trimmed.Solid
	Source                                                                           *File
	SourceBodyID, SourceShellID                                                      int64
	SourceFaceIDs                                                                    [4]int64 // outer wall, inner wall, lower annulus, upper annulus
	SourceEdgeIDs                                                                    [6]int64 // outer seam/top/bottom, inner seam/top/bottom
	NativeFaceSourceIDs                                                              [10]int64
	basis                                                                            mappedRationalBasis
}

// RationalFrame returns independent exact origin/X/Y/Z coefficients.
func (m EllipticalAnnularPrism) RationalFrame() [4][3]*big.Rat {
	var out [4][3]*big.Rat
	for i, v := range [4][3]*big.Rat{m.basis.Origin, m.basis.X, m.basis.Y, m.basis.Z} {
		for j, q := range v {
			if q != nil {
				out[i][j] = new(big.Rat).Set(q)
			}
		}
	}
	return out
}
func annularPrismTemplate() (*File, error) {
	return Parse(bytes.NewBufferString("ISO-10303-21;HEADER;FILE_DESCRIPTION((''),'2;1');FILE_NAME('','',(''),(''),'','','');FILE_SCHEMA(('AUTOMOTIVE_DESIGN'));ENDSEC;DATA;"+ellipticalAnnularPrismTemplate+"ENDSEC;END-ISO-10303-21;"), Limits{})
}
func annularDimensions(a, b, c, d, h float64) bool {
	for _, v := range []float64{a, b, c, d, h} {
		if !finiteNumber(v) || v <= 0 {
			return false
		}
	}
	if a <= b || c <= d || c >= a || d >= b {
		return false
	}
	return new(big.Rat).Mul(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(d)).Cmp(new(big.Rat).Mul(new(big.Rat).SetFloat64(b), new(big.Rat).SetFloat64(c))) == 0
}
func fillAnnularPrism(f *File, b mappedRationalBasis, ref Value, a, r, ia, ir, h float64) error {
	if e := exactPrismBasis(b); e != nil {
		return e
	}
	if !nestedAnnularDimensions(a, r, ia, ir, h) {
		return geom.ErrInvalid
	}
	zero := new(big.Rat)
	hr := new(big.Rat).SetFloat64(h)
	origin, top := exactPrismPoint(b, zero, zero, zero), exactPrismPoint(b, zero, zero, hr)
	neg := [3]*big.Rat{}
	for i := range neg {
		neg[i] = new(big.Rat).Neg(b.Z[i])
	}
	set := func(id int64, v [3]*big.Rat) error {
		en := f.Entities[id]
		items := []Value{}
		for _, q := range v {
			s, e := mappedTerminatingDecimal(q)
			if e != nil {
				return e
			}
			items = append(items, Value{Kind: Number, Text: s})
		}
		en.Records[0].Args[1] = Value{Kind: Aggregate, Items: items}
		f.Entities[id] = en
		return nil
	}
	for _, id := range []int64{42, 48, 78, 84, 98} {
		if e := set(id, origin); e != nil {
			return e
		}
	}
	for _, id := range []int64{34, 70, 110} {
		if e := set(id, top); e != nil {
			return e
		}
	}
	for _, id := range []int64{29, 35, 43, 49, 65, 71, 79, 85, 99, 111} {
		if e := set(id, b.Z); e != nil {
			return e
		}
	}
	for _, id := range []int64{52, 88} {
		if e := set(id, neg); e != nil {
			return e
		}
	}
	for _, spec := range []struct {
		radius            float64
		bottom, top, line int64
	}{{a, 23, 25, 27}, {ia, 59, 61, 63}} {
		x := new(big.Rat).SetFloat64(spec.radius)
		for _, v := range []struct {
			id int64
			p  [3]*big.Rat
		}{{spec.bottom, exactPrismPoint(b, x, zero, zero)}, {spec.top, exactPrismPoint(b, x, zero, hr)}, {spec.line, exactPrismPoint(b, x, zero, zero)}} {
			if e := set(v.id, v.p); e != nil {
				return e
			}
		}
	}
	for _, id := range []int64{36, 44, 50, 72, 80, 86, 100, 112} {
		en := f.Entities[id]
		en.Records[0].Args[1] = ref
		f.Entities[id] = en
	}
	for _, spec := range []struct {
		ids  []int64
		a, b float64
	}{{[]int64{32, 40, 46}, a, r}, {[]int64{68, 76, 82}, ia, ir}} {
		av, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(spec.a))
		if e != nil {
			return e
		}
		bv, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(spec.b))
		if e != nil {
			return e
		}
		for _, id := range spec.ids {
			en := f.Entities[id]
			en.Records[0].Args[2] = Value{Kind: Number, Text: av}
			en.Records[0].Args[3] = Value{Kind: Number, Text: bv}
			f.Entities[id] = en
		}
	}
	return nil
}

// ReadEllipticalAnnularPrism proves the complete model-only 118-entity owning
// graph. Both ellipse dimensions must be exactly homothetic and strictly nested.
// No rounded pcurve, offset hole, finite angular trim or tolerance repair enters.
func ReadEllipticalAnnularPrism(ctx context.Context, r io.Reader, l Limits, tol float64) (EllipticalAnnularPrism, error) {
	return readEllipticalAnnularPrism(ctx, r, l, tol, false)
}
func readEllipticalAnnularPrism(ctx context.Context, r io.Reader, l Limits, tol float64, nonhomothetic bool) (EllipticalAnnularPrism, error) {
	return readOrientedEllipticalAnnularPrism(ctx, r, l, tol, nonhomothetic, false)
}
func readOrientedEllipticalAnnularPrism(ctx context.Context, r io.Reader, l Limits, tol float64, nonhomothetic, orthogonal bool) (EllipticalAnnularPrism, error) {
	fail := func() (EllipticalAnnularPrism, error) {
		return EllipticalAnnularPrism{}, fmt.Errorf("step: unsupported exact elliptical annular prism")
	}
	if ctx == nil || r == nil || !finiteNumber(tol) || tol <= 0 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return EllipticalAnnularPrism{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	if l.MaxFaces < 10 || l.MaxVertices < 16 || l.MaxEdgeUses < 48 {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 118)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return EllipticalAnnularPrism{}, e
	}
	if len(f.Entities) != 118 {
		return fail()
	}
	want, e := annularPrismTemplate()
	if e != nil {
		return fail()
	}
	roots := [][2]int64{{1, 0}, {3, 0}, {118, 0}}
	for id, en := range f.Entities {
		if len(en.Records) != 1 {
			continue
		}
		for i, k := range []string{"APPLICATION_PROTOCOL_DEFINITION", "SHAPE_DEFINITION_REPRESENTATION", "PRODUCT_RELATED_PRODUCT_CATEGORY"} {
			if en.Records[0].Type == k {
				if roots[i][1] != 0 {
					return fail()
				}
				roots[i][1] = id
			}
		}
	}
	roles, ok := ellipsePrismDiscovery(want, f, roots)
	if !ok {
		return fail()
	}
	basis, e := mappedRationalFrame(f, roles[47])
	if e != nil || exactPrismBasis(basis) != nil {
		return fail()
	}
	dims := []float64{}
	for _, id := range []int64{46, 82} {
		p, _ := simple(f, roles[id], "ELLIPSE", 4)
		for _, v := range p.Args[2:] {
			q, e := cylinderSupportNumber(v)
			if e != nil {
				return fail()
			}
			dims = append(dims, q)
		}
	}
	top, e := mappedRationalCoordinate(f, roles[34], "CARTESIAN_POINT", 3)
	if e != nil {
		return fail()
	}
	height := new(big.Rat)
	for i := range top {
		height.Add(height, new(big.Rat).Mul(new(big.Rat).Sub(top[i], basis.Origin[i]), basis.Z[i]))
	}
	h, exact := height.Float64()
	if !exact || !orientedAnnularDimensions(dims[0], dims[1], dims[2], dims[3], h, nonhomothetic, orthogonal) {
		return fail()
	}
	ref, _ := simple(f, roles[50], "DIRECTION", 2)
	if e = fillAnnularPrism(want, basis, ref.Args[1], dims[0], dims[1], dims[2], dims[3], h); e != nil {
		return EllipticalAnnularPrism{}, e
	}
	if orthogonal {
		inner, e := mappedRationalFrame(f, roles[83])
		if e != nil {
			return fail()
		}
		for i := 0; i < 3; i++ {
			if inner.X[i].Cmp(basis.Y[i]) != 0 {
				return fail()
			}
		}
		ir, _ := simple(f, roles[86], "DIRECTION", 2)
		if e = fillOrthogonalAnnularPrism(want, basis, ir.Args[1], dims[2], h); e != nil {
			return EllipticalAnnularPrism{}, e
		}
	}
	proved, ok := ellipseFaceGraph(want, f, roots)
	if !ok {
		return fail()
	}
	for id, v := range roles {
		if proved[id] != v {
			return fail()
		}
	}
	unit, e := contextUnits(f, roles[113])
	if e != nil || unit != .001 {
		return fail()
	}
	s, e := sewOrientedEllipticalAnnularPrism(ctx, basis, dims[0], dims[1], dims[2], dims[3], h, tol, orthogonal)
	if e != nil {
		return EllipticalAnnularPrism{}, e
	}
	product, _ := simple(f, roles[7], "PRODUCT", 4)
	return EllipticalAnnularPrism{Name: product.Args[1].Text, MajorRadius: dims[0], MinorRadius: dims[1], InnerMajorRadius: dims[2], InnerMinorRadius: dims[3], Height: h, LengthUnit: unit, Solid: s, Source: f, SourceBodyID: roles[15], SourceShellID: roles[16], SourceFaceIDs: [4]int64{roles[17], roles[53], roles[89], roles[101]}, SourceEdgeIDs: [6]int64{roles[21], roles[31], roles[39], roles[57], roles[67], roles[75]}, NativeFaceSourceIDs: [10]int64{roles[89], roles[101], roles[17], roles[17], roles[17], roles[17], roles[53], roles[53], roles[53], roles[53]}, basis: basis}, nil
}

// EllipticalAnnularPrismDefinition owns a copied rational origin and represented
// direction ratios. Normalized axes and all STEP model coordinates must admit
// bounded terminating decimals; native poles/endpoints remain exact rationals.
type EllipticalAnnularPrismDefinition struct {
	Name                                                                 string
	Origin                                                               [3]*big.Rat
	Reference, Axis                                                      geom.Vec3
	MajorRadius, MinorRadius, InnerMajorRadius, InnerMinorRadius, Height float64
}

func WriteEllipticalAnnularPrismSTEP(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDefinition, tol float64) error {
	return writeEllipticalAnnularPrismSTEP(ctx, w, d, tol, false)
}
func writeEllipticalAnnularPrismSTEP(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDefinition, tol float64, nonhomothetic bool) error {
	return writeOrientedEllipticalAnnularPrismSTEP(ctx, w, d, tol, nonhomothetic, false)
}
func writeOrientedEllipticalAnnularPrismSTEP(ctx context.Context, w io.Writer, d EllipticalAnnularPrismDefinition, tol float64, nonhomothetic, orthogonal bool) error {
	if ctx == nil || w == nil || !d.Reference.IsFinite() || !d.Axis.IsFinite() || !utf8.ValidString(d.Name) || len(d.Name) > 1024 || !orientedAnnularDimensions(d.MajorRadius, d.MinorRadius, d.InnerMajorRadius, d.InnerMinorRadius, d.Height, nonhomothetic, orthogonal) {
		return geom.ErrInvalid
	}
	for _, c := range d.Name {
		if c < 32 || c == 127 {
			return geom.ErrInvalid
		}
	}
	b := mappedRationalBasis{X: mappedRatVec(d.Reference), Z: mappedRatVec(d.Axis)}
	for i, q := range d.Origin {
		if q == nil || q.Num().BitLen() > 1024 || q.Denom().BitLen() > 1024 {
			return geom.ErrInvalid
		}
		b.Origin[i] = new(big.Rat).Set(q)
	}
	var e error
	b.X, e = mappedRationalUnit(b.X)
	if e != nil {
		return e
	}
	b.Z, e = mappedRationalUnit(b.Z)
	if e != nil {
		return e
	}
	b.Y = mappedRationalCross(b.Z, b.X)
	if e = exactPrismBasis(b); e != nil {
		return e
	}
	ref := Value{Kind: Aggregate}
	for _, q := range mappedRatVec(d.Reference) {
		v, e := mappedTerminatingDecimal(q)
		if e != nil {
			return e
		}
		ref.Items = append(ref.Items, Value{Kind: Number, Text: v})
	}
	f, e := annularPrismTemplate()
	if e != nil {
		return e
	}
	if e = fillAnnularPrism(f, b, ref, d.MajorRadius, d.MinorRadius, d.InnerMajorRadius, d.InnerMinorRadius, d.Height); e != nil {
		return e
	}
	if orthogonal {
		ir := Value{Kind: Aggregate}
		for _, q := range b.Y {
			v, e := mappedTerminatingDecimal(q)
			if e != nil {
				return e
			}
			ir.Items = append(ir.Items, Value{Kind: Number, Text: v})
		}
		if e = fillOrthogonalAnnularPrism(f, b, ir, d.InnerMajorRadius, d.Height); e != nil {
			return e
		}
	}
	name := d.Name
	if name == "" {
		name = "Elliptical annular prism"
	}
	en := f.Entities[7]
	en.Records[0].Args[0] = Value{Kind: String, Text: name}
	en.Records[0].Args[1] = Value{Kind: String, Text: name}
	f.Entities[7] = en
	var out bytes.Buffer
	fmt.Fprintln(&out, "ISO-10303-21;\nHEADER;\nFILE_DESCRIPTION(('Exact elliptical annular prism'),'2;1');\nFILE_NAME('annulus.step','1970-01-01T00:00:00',(''),(''),'PlainSolid','PlainSolid','');\nFILE_SCHEMA(('AUTOMOTIVE_DESIGN'));\nENDSEC;\nDATA;")
	ids := map[int64]int64{}
	for id := range f.Entities {
		ids[id] = id
	}
	for _, id := range f.Order {
		fmt.Fprintf(&out, "#%d=%s;\n", id, assemblyEntity(f.Entities[id], ids))
	}
	fmt.Fprintln(&out, "ENDSEC;END-ISO-10303-21;")
	if _, e = readOrientedEllipticalAnnularPrism(ctx, bytes.NewReader(out.Bytes()), Limits{}, tol, nonhomothetic, orthogonal); e != nil {
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
