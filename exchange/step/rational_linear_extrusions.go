package step

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
	"strings"
	"unicode/utf8"
)

// RationalLinearExtrusionSurface retains exact finite-decimal source direction
// ratios. Construct it with NewRationalLinearExtrusionSurface. Ratios are owned
// immutable copies; DirectionRatios returns fresh copies. Profile geometry is
// immutable. Other public fields are revalidated by Surface and the writer.
type RationalLinearExtrusionSurface struct {
	Name         string
	Profile      SplineCurve
	VStart, VEnd float64
	ratios       [3]*big.Rat
}
type RationalLinearExtrusionModel struct {
	Surfaces   []RationalLinearExtrusionSurface
	LengthUnit float64
}

// NewRationalLinearExtrusionSurface requires exact rational normalization and
// losslessly representable endpoint control nets. Direction ratios are bounded
// to 256 numerator/denominator bits and 64-character finite decimal output.
func NewRationalLinearExtrusionSurface(name string, profile SplineCurve, ratios [3]*big.Rat, v0, v1 float64) (RationalLinearExtrusionSurface, error) {
	s := RationalLinearExtrusionSurface{Name: name, Profile: profile, VStart: v0, VEnd: v1}
	for i, r := range ratios {
		if _, e := extrusionDirectionDecimal(r); e != nil {
			return RationalLinearExtrusionSurface{}, e
		}
		s.ratios[i] = new(big.Rat).Set(r)
	}
	if _, e := s.Surface(); e != nil {
		return RationalLinearExtrusionSurface{}, e
	}
	return s, nil
}
func (s RationalLinearExtrusionSurface) DirectionRatios() (out [3]*big.Rat) {
	for i, r := range s.ratios {
		if r != nil {
			out[i] = new(big.Rat).Set(r)
		}
	}
	return
}

func extrusionDirectionDecimal(r *big.Rat) (string, error) {
	if r == nil || r.Num().BitLen() > 256 || r.Denom().BitLen() > 256 {
		return "", geom.ErrDomain
	}
	d := new(big.Int).Set(r.Denom())
	two, five := big.NewInt(2), big.NewInt(5)
	counts := [2]int{}
	for i, div := range []*big.Int{two, five} {
		for new(big.Int).Mod(d, div).Sign() == 0 {
			d.Quo(d, div)
			counts[i]++
			if counts[i] > 64 {
				return "", geom.ErrUnrepresentable
			}
		}
	}
	if d.Cmp(big.NewInt(1)) != 0 {
		return "", geom.ErrUnrepresentable
	}
	n := max(counts[0], counts[1])
	text := r.FloatString(n)
	if n == 0 {
		text += "."
	} else {
		text = strings.TrimRight(text, "0")
		if strings.HasSuffix(text, ".") {
			text += "0"
		}
	}
	if len(text) > 64 {
		return "", geom.ErrUnrepresentable
	}
	return text, nil
}
func (s RationalLinearExtrusionSurface) unitDirection() ([3]*big.Rat, error) {
	var out [3]*big.Rat
	norm := new(big.Rat)
	for _, r := range s.ratios {
		if _, e := extrusionDirectionDecimal(r); e != nil {
			return out, e
		}
		norm.Add(norm, new(big.Rat).Mul(r, r))
	}
	if norm.Sign() == 0 {
		return out, geom.ErrDomain
	}
	n, d := new(big.Int).Sqrt(norm.Num()), new(big.Int).Sqrt(norm.Denom())
	if new(big.Int).Mul(n, n).Cmp(norm.Num()) != 0 || new(big.Int).Mul(d, d).Cmp(norm.Denom()) != 0 {
		return out, geom.ErrUnrepresentable
	}
	length := new(big.Rat).SetFrac(n, d)
	for i, r := range s.ratios {
		out[i] = new(big.Rat).Quo(r, length)
		if out[i].Num().BitLen() > 1024 || out[i].Denom().BitLen() > 1024 {
			return out, geom.ErrDomain
		}
	}
	return out, nil
}

// Surface constructs an exact represented tensor identity, not just endpoint
// agreement: repeated profile weights make its denominator independent of V,
// and the linear V basis reproduces v. Consequently S(u,v)=C(u)+v*unitDirection
// throughout the full rectangle. Each endpoint pole must fit float64 exactly.
// Regularity, embedding, face trimming and material are not certified.
func (s RationalLinearExtrusionSurface) Surface() (*geom.BSplineSurface, error) {
	if s.Profile.Geometry == nil || !utf8.ValidString(s.Name) || !finiteNumber(s.VStart) || !finiteNumber(s.VEnd) || s.VStart >= s.VEnd {
		return nil, geom.ErrDomain
	}
	axis, e := s.unitDirection()
	if e != nil {
		return nil, e
	}
	c := s.Profile.Geometry
	u0, u1 := c.Domain()
	if !finiteNumber(u0) || !finiteNumber(u1) || u0 >= u1 {
		return nil, geom.ErrDomain
	}
	knots := c.Knots()
	for i := 0; i <= c.Degree(); i++ {
		if knots[i] != u0 || knots[len(knots)-1-i] != u1 {
			return nil, geom.ErrDomain
		}
	}
	if _, _, e = surfaceKnotStrings(knots, c.Degree()); e != nil {
		return nil, e
	}
	poles, weights := c.Poles(), c.Weights()
	net := make([]geom.Vec3, 0, 2*len(poles))
	ws := make([]float64, 0, 2*len(poles))
	for i, p := range poles {
		for _, v := range []float64{s.VStart, s.VEnd} {
			a := [3]float64{p.X, p.Y, p.Z}
			for j := range a {
				q := new(big.Rat).Add(new(big.Rat).SetFloat64(a[j]), new(big.Rat).Mul(new(big.Rat).SetFloat64(v), axis[j]))
				x, exact := q.Float64()
				if !exact || !finiteNumber(x) {
					return nil, geom.ErrUnrepresentable
				}
				a[j] = x
			}
			net = append(net, geom.Vec3{X: a[0], Y: a[1], Z: a[2]})
			ws = append(ws, weights[i])
		}
	}
	return geom.NewBSplineSurface(c.Degree(), 1, len(poles), 2, net, knots, []float64{s.VStart, s.VStart, s.VEnd, s.VEnd}, ws)
}

// DecodeRationalLinearExtrusionSurface decodes one bounded support without
// accepting surrounding face topology or applying model unit conversion.
func DecodeRationalLinearExtrusionSurface(f *File, id int64, maxPoints int) (RationalLinearExtrusionSurface, error) {
	fields, e := decodeLinearExtrusionFields(f, id, maxPoints)
	if e != nil {
		return RationalLinearExtrusionSurface{}, e
	}
	s := fields.surface
	return NewRationalLinearExtrusionSurface(s.Name, s.Profile, fields.ratios, s.VStart, s.VEnd)
}

// ReadRationalLinearExtrusions reads the same strictly owned standalone graph
// as ReadLinearExtrusions, with the additive exact rational-direction subset.
func ReadRationalLinearExtrusions(r io.Reader, l Limits) (RationalLinearExtrusionModel, error) {
	l, e := l.defaults()
	if e != nil {
		return RationalLinearExtrusionModel{}, e
	}
	f, e := Parse(r, l)
	if e != nil {
		return RationalLinearExtrusionModel{}, e
	}
	roots, unit, e := linearExtrusionRoots(f, l)
	if e != nil {
		return RationalLinearExtrusionModel{}, e
	}
	m := RationalLinearExtrusionModel{LengthUnit: unit}
	count := 0
	for _, id := range roots {
		s, e := DecodeRationalLinearExtrusionSurface(f, id, l.MaxVertices-count)
		if e != nil {
			return RationalLinearExtrusionModel{}, e
		}
		count += 2 * len(s.Profile.Geometry.Poles())
		m.Surfaces = append(m.Surfaces, s)
	}
	return m, nil
}

// WriteRationalLinearExtrusions preserves original finite-decimal direction
// ratios and swept profiles. All geometry is validated before caller output.
func WriteRationalLinearExtrusions(w io.Writer, m RationalLinearExtrusionModel) error {
	l, _ := (Limits{}).defaults()
	if w == nil || len(m.Surfaces) == 0 || len(m.Surfaces) > l.MaxFaces || !validLengthUnit(m.LengthUnit) {
		return geom.ErrDomain
	}
	curves := make([]SplineCurve, len(m.Surfaces))
	count := 0
	for i, s := range m.Surfaces {
		if s.Profile.Geometry == nil {
			return geom.ErrDomain
		}
		count += 2 * len(s.Profile.Geometry.Poles())
		if count > l.MaxVertices {
			return geom.ErrDomain
		}
		if _, e := s.Surface(); e != nil {
			return e
		}
		curves[i] = s.Profile
	}
	var raw bytes.Buffer
	if e := WriteCurves(&raw, CurveModel{Curves: curves, LengthUnit: m.LengthUnit}); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), l)
	if e != nil {
		return e
	}
	var roots []int64
	for _, id := range f.Order {
		if _, ok := component(f.Entities[id], "B_SPLINE_CURVE_WITH_KNOTS"); ok {
			roots = append(roots, id)
		}
	}
	if len(roots) != len(curves) {
		return geom.ErrInvalid
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_SET", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		cids := copySplineGeometry(emit, f, roots)
		out := make([]int64, len(cids))
		for i, cid := range cids {
			s := m.Surfaces[i]
			var a [3]string
			for j, r := range s.ratios {
				a[j], _ = extrusionDirectionDecimal(r)
			}
			d := emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", a[0], a[1], a[2]))
			v := emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))
			basis := emit(fmt.Sprintf("SURFACE_OF_LINEAR_EXTRUSION('',#%d,#%d)", cid, v))
			u0, u1 := s.Profile.Geometry.Domain()
			out[i] = emit(fmt.Sprintf("RECTANGULAR_TRIMMED_SURFACE(%s,#%d,%s,%s,%s,%s,.T.,.T.)", quote(s.Name), basis, real(u0), real(u1), real(s.VStart), real(s.VEnd)))
		}
		return out
	})
}
