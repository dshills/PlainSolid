package step

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func surfaceKnotStrings(knots []float64, degree int) (string, string, error) {
	var ks, ms []string
	for i := 0; i < len(knots); {
		j := i + 1
		for j < len(knots) && knots[j] == knots[i] {
			j++
		}
		limit := degree
		if i == 0 || j == len(knots) {
			limit++
		}
		if j-i > limit {
			return "", "", errors.New("step: unsupported surface knot multiplicity")
		}
		ks = append(ks, real(knots[i]))
		ms = append(ms, strconv.Itoa(j-i))
		i = j
	}
	return "(" + strings.Join(ms, ",") + ")", "(" + strings.Join(ks, ",") + ")", nil
}

// WriteSurfaces emits independent full-domain rational spline patches, without
// face trimming or topology, in a bounded surface representation.
func WriteSurfaces(w io.Writer, m SurfaceModel) error {
	if w == nil || len(m.Surfaces) == 0 {
		return errors.New("step: missing surface writer/model")
	}
	for _, s := range m.Surfaces {
		if s.Geometry == nil || !utf8.ValidString(s.Name) {
			return errors.New("step: invalid spline surface")
		}
		a, b, c, d := s.Geometry.Domain()
		if !finiteNumber(a) || !finiteNumber(b) || !finiteNumber(c) || !finiteNumber(d) || a >= b || c >= d {
			return errors.New("step: invalid surface domain")
		}
		data := s.Geometry.Data()
		if _, _, e := surfaceKnotStrings(data.KnotsU, data.DegreeU); e != nil {
			return e
		}
		if _, _, e := surfaceKnotStrings(data.KnotsV, data.DegreeV); e != nil {
			return e
		}
		for _, v := range []string{s.ClosedU, s.ClosedV, s.SelfIntersect} {
			if v != "" && v != "T" && v != "F" && v != "U" {
				return errors.New("step: invalid surface logical flag")
			}
		}
	}
	return writeStandalone(w, m.LengthUnit, "GEOMETRIC_SET", "GEOMETRICALLY_BOUNDED_SURFACE_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		ids := make([]int64, len(m.Surfaces))
		for i, s := range m.Surfaces {
			d := s.Geometry.Data()
			rows, wrows := make([]string, d.CountU), make([]string, d.CountU)
			for u := 0; u < d.CountU; u++ {
				points := make([]int64, d.CountV)
				weights := make([]string, d.CountV)
				for v := 0; v < d.CountV; v++ {
					p := d.Poles[u*d.CountV+v]
					points[v] = emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X), real(p.Y), real(p.Z)))
					weights[v] = real(d.Weights[u*d.CountV+v])
				}
				rows[u] = references(points)
				wrows[u] = "(" + strings.Join(weights, ",") + ")"
			}
			mu, ku, _ := surfaceKnotStrings(d.KnotsU, d.DegreeU)
			mv, kv, _ := surfaceKnotStrings(d.KnotsV, d.DegreeV)
			flags := []string{s.ClosedU, s.ClosedV, s.SelfIntersect}
			for j := range flags {
				if flags[j] == "" {
					flags[j] = "U"
				}
			}
			ids[i] = emit(fmt.Sprintf("(BOUNDED_SURFACE() B_SPLINE_SURFACE(%d,%d,(%s),.UNSPECIFIED.,.%s.,.%s.,.%s.) B_SPLINE_SURFACE_WITH_KNOTS(%s,%s,%s,%s,.UNSPECIFIED.) GEOMETRIC_REPRESENTATION_ITEM() RATIONAL_B_SPLINE_SURFACE((%s)) REPRESENTATION_ITEM(%s) SURFACE())", d.DegreeU, d.DegreeV, strings.Join(rows, ","), flags[0], flags[1], flags[2], mu, mv, ku, kv, strings.Join(wrows, ","), quote(s.Name)))
		}
		return ids
	})
}
