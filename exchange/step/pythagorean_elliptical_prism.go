package step

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// PythagoreanEllipticalPrism has exact rational source axes and lossless native
// quarter controls. Native plane charts are independently cardinal; no rounded
// geom.Frame is substituted for the original rational ellipse frame.
type PythagoreanEllipticalPrism struct {
	Name                                         string
	MajorRadius, MinorRadius, Height, LengthUnit float64
	Solid                                        trimmed.Solid
	Source                                       *File
	SourceBodyID, SourceShellID                  int64
	SourceFaceIDs, SourceEdgeIDs                 [3]int64
	NativeFaceSourceIDs                          [6]int64
	NativeRimEdges                               [2][4]int
	basis                                        mappedRationalBasis
}

// RationalFrame returns owned exact origin and normalized X/Y/Z axes.
func (m PythagoreanEllipticalPrism) RationalFrame() (origin, x, y, z [3]*big.Rat) {
	for i := 0; i < 3; i++ {
		if m.basis.Origin[i] != nil {
			origin[i] = new(big.Rat).Set(m.basis.Origin[i])
			x[i] = new(big.Rat).Set(m.basis.X[i])
			y[i] = new(big.Rat).Set(m.basis.Y[i])
			z[i] = new(big.Rat).Set(m.basis.Z[i])
		}
	}
	return
}
func rationalPrismPoint(b mappedRationalBasis, x, y, z float64) (geom.Vec3, error) {
	var v [3]float64
	for i := range v {
		q := new(big.Rat).Set(b.Origin[i])
		for j, axis := range [3][3]*big.Rat{b.X, b.Y, b.Z} {
			q.Add(q, new(big.Rat).Mul(axis[i], new(big.Rat).SetFloat64([]float64{x, y, z}[j])))
		}
		var ok bool
		v[i], ok = q.Float64()
		if !ok || !finiteNumber(v[i]) {
			return geom.Vec3{}, geom.ErrUnrepresentable
		}
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}
func rationalPrismAxis(b mappedRationalBasis) (geom.Vec3, error) {
	v := [3]float64{}
	count := 0
	noncardinal := false
	for i := range v {
		var exact bool
		v[i], exact = b.Z[i].Float64()
		if !exact || v[i] != 0 && v[i] != 1 && v[i] != -1 {
			return geom.Vec3{}, geom.ErrInvalid
		}
		if v[i] != 0 {
			count++
		}
		if b.X[i].Sign() != 0 && b.X[i].Cmp(big.NewRat(1, 1)) != 0 && b.X[i].Cmp(big.NewRat(-1, 1)) != 0 {
			noncardinal = true
		}
	}
	if count != 1 || !noncardinal {
		return geom.Vec3{}, geom.ErrInvalid
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}
func rationalPrismTemplate() (*File, error) {
	var b bytes.Buffer
	analyticSupportHeader(&b)
	b.WriteString(modelOnlyEllipticalPrismTemplate)
	b.WriteString("ENDSEC;END-ISO-10303-21;")
	return Parse(&b, Limits{})
}
func fillRationalPrism(f *File, b mappedRationalBasis, reference Value, aVal, bVal Value, a, h float64) error {
	z, e := rationalPrismAxis(b)
	if e != nil {
		return e
	}
	origin, e := rationalPrismPoint(b, 0, 0, 0)
	if e != nil {
		return e
	}
	top, e := rationalPrismPoint(b, 0, 0, h)
	if e != nil {
		return e
	}
	v0, e := rationalPrismPoint(b, a, 0, 0)
	if e != nil {
		return e
	}
	v1, e := rationalPrismPoint(b, a, 0, h)
	if e != nil {
		return e
	}
	set := func(id int64, v geom.Vec3) error {
		en := f.Entities[id]
		items := []Value{}
		for _, x := range []float64{v.X, v.Y, v.Z} {
			s, e := mappedTerminatingDecimal(new(big.Rat).SetFloat64(x))
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
		v  geom.Vec3
	}{{23, v0}, {25, v1}, {28, v0}, {30, z}, {35, origin}, {36, z}, {39, z.Scale(-1)}, {58, top}, {59, z}, {71, top}, {72, z}, {86, origin}, {87, z}, {99, origin}, {100, z}} {
		if e = set(x.id, x.v); e != nil {
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
func sewRationalEllipticalPrism(ctx context.Context, b mappedRationalBasis, a, r, h, tol float64) (trimmed.Solid, [2][4]int, error) {
	fail := func(e error) (trimmed.Solid, [2][4]int, error) { return trimmed.Solid{}, [2][4]int{}, e }
	axis, e := rationalPrismAxis(b)
	if e != nil {
		return fail(e)
	}
	caps := [2]geom.Frame{}
	for k, z := range []float64{0, h} {
		p, e := rationalPrismPoint(b, 0, 0, z)
		if e != nil {
			return fail(e)
		}
		caps[k], e = geom.FrameFromNormal(p, axis)
		if e != nil {
			return fail(e)
		}
	}
	uv := [4][]geom.Vec3{}
	world := [2][4][]geom.Vec3{}
	for q := 0; q < 4; q++ {
		for _, v := range [][2]float64{{1, 0}, {1, 1}, {0, 1}} {
			x, y := v[0], v[1]
			for i := 0; i < q; i++ {
				x, y = -y, x
			}
			x *= a
			y *= r
			// Project exact rational local offsets into the independent cardinal plane.
			local := [2]float64{}
			for j, c := range []geom.Vec3{caps[0].XAxis(), caps[0].YAxis()} {
				sum := new(big.Rat)
				for i, ci := range []float64{c.X, c.Y, c.Z} {
					offset := new(big.Rat).Add(new(big.Rat).Mul(b.X[i], new(big.Rat).SetFloat64(x)), new(big.Rat).Mul(b.Y[i], new(big.Rat).SetFloat64(y)))
					sum.Add(sum, offset.Mul(offset, new(big.Rat).SetFloat64(ci)))
				}
				var exact bool
				local[j], exact = sum.Float64()
				if !exact || !finiteNumber(local[j]) {
					return fail(geom.ErrUnrepresentable)
				}
			}
			uv[q] = append(uv[q], geom.Vec3{X: local[0], Y: local[1]})
			for k, z := range []float64{0, h} {
				p, e := rationalPrismPoint(b, x, y, z)
				if e != nil {
					return fail(e)
				}
				world[k][q] = append(world[k][q], p)
			}
		}
	}
	return sewEllipticalPrismControls(ctx, uv, world, caps, tol)
}

// ReadPythagoreanEllipticalPrism is an additive exact model-only reader with a
// cardinal extrusion axis and noncardinal in-plane direction ratios whose norm
// has an exact rational square root. All source frames must retain the same
// declared ratios. Every native world/UV pole must be losslessly representable.
func ReadPythagoreanEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64) (PythagoreanEllipticalPrism, error) {
	return readPythagoreanEllipticalPrism(ctx, r, l, tolerance, false)
}
func readPythagoreanEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64, exactControls bool) (PythagoreanEllipticalPrism, error) {
	return readRationalFrameEllipticalPrism(ctx, r, l, tolerance, exactControls, false)
}
func readRationalFrameEllipticalPrism(ctx context.Context, r io.Reader, l Limits, tolerance float64, exactControls, tilted bool) (PythagoreanEllipticalPrism, error) {
	fail := func() (PythagoreanEllipticalPrism, error) {
		return PythagoreanEllipticalPrism{}, fmt.Errorf("step: unsupported exact Pythagorean ellipse prism")
	}
	if ctx == nil || r == nil || !finiteNumber(tolerance) || tolerance <= 0 {
		return fail()
	}
	if e := ctx.Err(); e != nil {
		return PythagoreanEllipticalPrism{}, e
	}
	l, e := l.defaults()
	if e != nil {
		return fail()
	}
	if l.MaxFaces < 6 || l.MaxVertices < 8 || l.MaxEdgeUses < 24 {
		return fail()
	}
	l.MaxBytes = min(l.MaxBytes, 65536)
	l.MaxEntities = min(l.MaxEntities, 76)
	l.MaxValues = min(l.MaxValues, 4096)
	l.MaxDepth = min(l.MaxDepth, 16)
	f, e := Parse(r, l)
	if e != nil {
		return PythagoreanEllipticalPrism{}, e
	}
	if len(f.Entities) != 76 {
		return fail()
	}
	want, e := rationalPrismTemplate()
	if e != nil {
		return fail()
	}
	roots := [][2]int64{{1, 0}, {3, 0}, {121, 0}}
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
	basis, e := mappedRationalFrame(f, roles[34])
	if e != nil {
		return fail()
	}
	if tilted {
		e = tiltedPrismBasis(basis)
	} else {
		_, e = rationalPrismAxis(basis)
	}
	if e != nil {
		return fail()
	}
	profile, _ := simple(f, roles[33], "ELLIPSE", 4)
	a, e := cylinderSupportNumber(profile.Args[2])
	if e != nil {
		return fail()
	}
	b, e := cylinderSupportNumber(profile.Args[3])
	if e != nil || b <= 0 || a <= b {
		return fail()
	}
	top, e := mappedRationalCoordinate(f, roles[58], "CARTESIAN_POINT", 3)
	if e != nil {
		return fail()
	}
	height := new(big.Rat)
	for i := range top {
		height.Add(height, new(big.Rat).Mul(new(big.Rat).Sub(top[i], basis.Origin[i]), basis.Z[i]))
	}
	h, exact := height.Float64()
	if !exact || !finiteNumber(h) || h <= 0 {
		return fail()
	}
	ref, _ := simple(f, roles[37], "DIRECTION", 2)
	if exactControls {
		e = fillExactControlPrism(want, basis, ref.Args[1], profile.Args[2], profile.Args[3], a, h)
	} else {
		e = fillRationalPrism(want, basis, ref.Args[1], profile.Args[2], profile.Args[3], a, h)
	}
	if e != nil {
		return PythagoreanEllipticalPrism{}, e
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
	unit, e := contextUnits(f, roles[116])
	if e != nil || unit != .001 {
		return fail()
	}
	var solid trimmed.Solid
	var rims [2][4]int
	if exactControls {
		solid, rims, e = sewExactControlEllipticalPrism(ctx, basis, a, b, h, tolerance)
	} else {
		solid, rims, e = sewRationalEllipticalPrism(ctx, basis, a, b, h, tolerance)
	}
	if e != nil {
		return PythagoreanEllipticalPrism{}, e
	}
	product, _ := simple(f, roles[7], "PRODUCT", 4)
	return PythagoreanEllipticalPrism{Name: product.Args[1].Text, MajorRadius: a, MinorRadius: b, Height: h, LengthUnit: unit, Solid: solid, Source: f, SourceBodyID: roles[15], SourceShellID: roles[16], SourceFaceIDs: [3]int64{roles[17], roles[108], roles[112]}, SourceEdgeIDs: [3]int64{roles[21], roles[54], roles[82]}, NativeFaceSourceIDs: [6]int64{roles[108], roles[112], roles[17], roles[17], roles[17], roles[17]}, NativeRimEdges: rims, basis: basis}, nil
}
