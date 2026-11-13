package step

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// WriteCurves emits independent explicit-knot rational spline geometry in an
// AP203-style bounded wireframe representation. It preserves knot parameters;
// no trimming, wire connectivity, periodic wrapping or topology is inferred.
func WriteCurves(w io.Writer, m CurveModel) error {
	if w == nil || len(m.Curves) == 0 {
		return errors.New("step: missing curve writer/model")
	}
	for _, c := range m.Curves {
		if c.Geometry == nil || !utf8.ValidString(c.Name) {
			return errors.New("step: invalid spline")
		}
		a, b := c.Geometry.Domain()
		if !finiteNumber(a) || !finiteNumber(b) || a >= b {
			return errors.New("step: invalid spline domain")
		}
		knots := c.Geometry.Knots()
		for i := 0; i < len(knots); {
			j := i + 1
			for j < len(knots) && knots[j] == knots[i] {
				j++
			}
			limit := c.Geometry.Degree()
			if i == 0 || j == len(knots) {
				limit++
			}
			if j-i > limit {
				return errors.New("step: unsupported spline knot multiplicity")
			}
			i = j
		}
		for _, v := range []string{c.Closed, c.SelfIntersect} {
			if v != "" && v != "T" && v != "F" && v != "U" {
				return errors.New("step: invalid spline logical flag")
			}
		}
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_CURVE_SET", "GEOMETRICALLY_BOUNDED_WIREFRAME_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := make([]int64, len(m.Curves))
		for i, c := range m.Curves {
			points := make([]int64, len(c.Geometry.Poles()))
			for j, p := range c.Geometry.Poles() {
				points[j] = emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
			}
			var distinct []float64
			var mult []int
			for _, k := range c.Geometry.Knots() {
				if len(distinct) == 0 || distinct[len(distinct)-1] != k {
					distinct = append(distinct, k)
					mult = append(mult, 1)
				} else {
					mult[len(mult)-1]++
				}
			}
			ks, ms, ws := make([]string, len(distinct)), make([]string, len(mult)), make([]string, len(points))
			for j, k := range distinct {
				ks[j] = real(k)
				ms[j] = strconv.Itoa(mult[j])
			}
			for j, v := range c.Geometry.Weights() {
				ws[j] = real(v)
			}
			closed, self := c.Closed, c.SelfIntersect
			if closed == "" {
				closed = "U"
			}
			if self == "" {
				self = "U"
			}
			ids[i] = emit(fmt.Sprintf("(BOUNDED_CURVE() B_SPLINE_CURVE(%d,%s,.UNSPECIFIED.,.%s.,.%s.) B_SPLINE_CURVE_WITH_KNOTS((%s),(%s),.UNSPECIFIED.) CURVE() GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_CURVE((%s)) REPRESENTATION_ITEM(%s))", c.Geometry.Degree(), references(points), closed, self, strings.Join(ms, ","), strings.Join(ks, ","), strings.Join(ws, ","), quote(c.Name)))
		}
		return ids
	})
}
