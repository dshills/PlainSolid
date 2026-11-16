package iges

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

func number(x float64) string {
	s := strconv.FormatFloat(x, 'g', 17, 64)
	if i := strings.IndexByte(s, 'e'); i >= 0 {
		mantissa := s[:i]
		if !strings.Contains(mantissa, ".") {
			mantissa += "."
		}
		return mantissa + "E" + s[i+1:]
	}
	if !strings.Contains(s, ".") {
		s += "."
	}
	return s
}
func hollerith(s string) string { return strconv.Itoa(len(s)) + "H" + s }
func chunks(s string, n int) []string {
	var a []string
	for len(s) > n {
		a = append(a, s[:n])
		s = s[n:]
	}
	if s != "" {
		a = append(a, s)
	}
	return a
}
func flag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
func encode(e Entity) (int, string) {
	var p []string
	kind := 0
	add := func(v ...float64) {
		for _, x := range v {
			p = append(p, number(x))
		}
	}
	switch {
	case e.Polyline != nil:
		kind = 106
		p = append(p, "106", "2", strconv.Itoa(len(e.Polyline.Points)))
		for _, v := range e.Polyline.Points {
			add(v.X, v.Y, v.Z)
		}
	case e.Point != nil:
		kind = 116
		p = append(p, "116")
		add(e.Point.X, e.Point.Y, e.Point.Z)
		p = append(p, "0")
	case e.Line != nil:
		kind = 110
		p = append(p, "110")
		a, b := e.Line.Start, e.Line.End
		add(a.X, a.Y, a.Z, b.X, b.Y, b.Z)
	case e.Arc != nil:
		kind = 100
		p = append(p, "100")
		a := e.Arc
		add(a.Center.Z, a.Center.X, a.Center.Y, a.Start.X, a.Start.Y, a.End.X, a.End.Y)
	case e.Surface != nil:
		kind = 128
		s := e.Surface
		d := s.Geometry.Data()
		p = append(p, "128", strconv.Itoa(d.CountU-1), strconv.Itoa(d.CountV-1), strconv.Itoa(d.DegreeU), strconv.Itoa(d.DegreeV), flag(s.ClosedU), flag(s.ClosedV), flag(s.Polynomial), flag(s.PeriodicU), flag(s.PeriodicV))
		add(d.KnotsU...)
		add(d.KnotsV...)
		for v := 0; v < d.CountV; v++ {
			for u := 0; u < d.CountU; u++ {
				add(d.Weights[u*d.CountV+v])
			}
		}
		for v := 0; v < d.CountV; v++ {
			for u := 0; u < d.CountU; u++ {
				a := d.Poles[u*d.CountV+v]
				add(a.X, a.Y, a.Z)
			}
		}
		add(s.UStart, s.UEnd, s.VStart, s.VEnd)
	case e.Spline != nil:
		kind = 126
		s := e.Spline
		poles := s.Curve.Poles()
		p = append(p, "126", strconv.Itoa(len(poles)-1), strconv.Itoa(s.Curve.Degree()), flag(s.Planar), flag(s.Closed), flag(s.Polynomial), flag(s.Periodic))
		add(s.Curve.Knots()...)
		add(s.Curve.Weights()...)
		for _, v := range poles {
			add(v.X, v.Y, v.Z)
		}
		add(s.Start, s.End, s.Normal.X, s.Normal.Y, s.Normal.Z)
	}
	p = append(p, "0", "0")
	return kind, strings.Join(p, ",") + ";"
}

// Write emits deterministic 80-column ASCII IGES 5.3 curve files. Geometry is
// not transformed or rescaled; units must be a supported IGES standard unit.
// Optional associativity/property pointers are emitted as zero. Caller owns w.
func Write(w io.Writer, m Model) error { return writeCurvedMode(w, m, false) }
func writeCurvedMode(w io.Writer, m Model, curved bool) error {
	return writeGeometryMode(w, m, curved, false)
}
func writeGeometryMode(w io.Writer, m Model, curved, wires bool) error {
	return writeRevolutionMode(w, m, curved, wires, false)
}
func writeRevolutionMode(w io.Writer, m Model, curved, wires, revolutions bool) error {
	return writeSweptMode(w, m, curved, wires, revolutions, false)
}
func writeSweptMode(w io.Writer, m Model, curved, wires, revolutions, tabulated bool) error {
	if !tabulated && len(m.TabulatedSurfaces) != 0 {
		return errors.New("iges: tabulated surface requires dedicated codec")
	}
	if !revolutions && len(m.Revolutions) != 0 {
		return errors.New("iges: revolution requires dedicated codec")
	}
	if w == nil {
		return errors.New("iges: nil writer")
	}
	for _, c := range m.CurvesOnSurfaces {
		validate := m.ValidateCurveOnSurface
		if curved {
			validate = m.validateCurvedOvalMapping
		}
		if e := validate(c); e != nil {
			return e
		}
	}
	usedComposites := map[int]bool{}
	for _, c := range m.CurvesOnSurfaces {
		usedComposites[c.ParameterCurve] = true
		usedComposites[c.ModelCurve] = true
	}
	for i, e := range m.Entities {
		if e.Composite != nil && !usedComposites[i] && !wires {
			return errors.New("iges: standalone composites unsupported")
		}
	}
	for _, t := range m.TrimmedSurfaces {
		validateTrim := m.ValidateTrimmedSurface
		if curved {
			validateTrim = m.validateCurvedOvalTrim
		}
		if e := validateTrim(t); e != nil {
			return e
		}
	}
	var chosen unit
	for _, u := range units {
		if m.LengthUnit == u.meters {
			chosen = u
			break
		}
	}
	if chosen.flag == 0 {
		return errors.New("iges: unsupported length unit")
	}
	if len(m.Entities) > 4999999 {
		return errors.New("iges: directory sequence limit exceeded")
	}
	for i, e := range m.Entities {
		if e.Surface != nil && (e.Surface.PeriodicU || e.Surface.PeriodicV || e.Surface.periodic != nil) {
			return errors.New("iges: periodic spline surface write unsupported")
		}
		if e.Composite != nil {
			if err := m.validateCompositeMode(i, wires); err != nil {
				return err
			}
		}
		if err := e.validate(); err != nil {
			return err
		}
	}
	global := []string{hollerith(","), hollerith(";"), hollerith("PlainSolid"), hollerith("model.iges"), hollerith("Go"), hollerith("PlainSolid"), "32", "38", "6", "308", "17", hollerith("PlainSolid"), "1.", strconv.Itoa(chosen.flag), hollerith(chosen.name), "1", "1.", hollerith("19700101.000000"), "1.E-7", "0.", "0H", "0H", "11", "0", hollerith("19700101.000000"), "0H"}
	maxCoordinate := 0.
	directoryCount := len(m.Entities) + len(m.CurvesOnSurfaces) + len(m.TrimmedSurfaces)
	for _, e := range m.Entities {
		if e.Transform != nil {
			directoryCount++
		}
		if directoryCount > 4999999 {
			return errors.New("iges: directory sequence limit exceeded")
		}
		if e.Composite != nil {
			continue
		}
		bounds, err := e.bounds()
		if err != nil {
			return err
		}
		for _, x := range []float64{bounds.Min.X, bounds.Min.Y, bounds.Min.Z, bounds.Max.X, bounds.Max.Y, bounds.Max.Z} {
			maxCoordinate = math.Max(maxCoordinate, math.Abs(x))
		}
	}
	global[19] = number(maxCoordinate)
	g := chunks(strings.Join(global, ",")+";", 72)
	d := []string{}
	p := []record{}
	appendEntity := func(kind int, label string, subscript, form, transform int, data string) error {
		lines := parameterChunks(data, 64)
		if len(lines) > 9999999-len(p) {
			return errors.New("iges: parameter sequence limit exceeded")
		}
		status := 0
		if kind == 142 {
			status = 500
		}
		owner := len(d) + 1
		start := len(p) + 1
		for _, line := range lines {
			p = append(p, record{data: line, owner: owner})
		}
		d = append(d, fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%8d%08d", kind, start, 0, 0, 0, 0, transform, 0, status))
		d = append(d, fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%-8s%8d", kind, 0, 0, len(lines), form, 0, 0, label, subscript))
		return nil
	}
	owned := map[int]bool{}
	for _, c := range m.CurvesOnSurfaces {
		owned[c.Surface] = true
		owned[c.ParameterCurve] = true
		owned[c.ModelCurve] = true
	}
	for _, e := range m.Entities {
		if e.Composite != nil {
			for _, idx := range e.Composite.Curves {
				owned[idx] = true
			}
		}
	}
	for _, r := range m.Revolutions {
		owned[r.Axis] = true
		owned[r.Generatrix] = true
	}
	for _, r := range m.TabulatedSurfaces {
		owned[r.Directrix] = true
	}
	entityIDs := make([]int, len(m.Entities))
	nextID := 1
	for i, e := range m.Entities {
		entityIDs[i] = nextID
		nextID += 2
		if e.Transform != nil {
			nextID += 2
		}
	}
	for i, e := range m.Entities {
		entityIDs[i] = len(d) + 1
		kind, data := encode(e)
		if e.Composite != nil {
			values := []string{"102", strconv.Itoa(len(e.Composite.Curves))}
			for _, idx := range e.Composite.Curves {
				values = append(values, strconv.Itoa(entityIDs[idx]))
			}
			kind, data = 102, strings.Join(values, ",")+",0,0;"
		}
		transformID := 0
		if e.Transform != nil {
			transformID = len(d) + 3
		}
		if err := appendEntity(kind, e.Label, e.Subscript, e.Form, transformID, data); err != nil {
			return err
		}
		if owned[i] {
			row := entityIDs[i] - 1
			d[row] = d[row][:64] + "00010000"
		}
		if e.Transform != nil {
			values := []string{"124"}
			for i := 0; i < 3; i++ {
				for j := 0; j < 4; j++ {
					values = append(values, number(e.Transform[i][j]))
				}
			}
			values = append(values, "0", "0")
			if err := appendEntity(124, "", 0, 0, 0, strings.Join(values, ",")+";"); err != nil {
				return err
			}
		}
	}
	for _, r := range m.Revolutions {
		data := fmt.Sprintf("120,%d,%d,%s,%s,0,0;", entityIDs[r.Axis], entityIDs[r.Generatrix], number(r.AngleStart), number(r.AngleEnd))
		if e := appendEntity(120, r.Label, r.Subscript, 0, 0, data); e != nil {
			return e
		}
	}
	for _, r := range m.TabulatedSurfaces {
		data := fmt.Sprintf("122,%d,%s,%s,%s,0,0;", entityIDs[r.Directrix], number(r.End.X), number(r.End.Y), number(r.End.Z))
		if e := appendEntity(122, r.Label, r.Subscript, 0, 0, data); e != nil {
			return e
		}
	}
	ownedContours := map[int]bool{}
	for _, t := range m.TrimmedSurfaces {
		ownedContours[t.Outer] = true
		for _, i := range t.Inner {
			ownedContours[i] = true
		}
	}
	contourIDs := make([]int, len(m.CurvesOnSurfaces))
	for i, c := range m.CurvesOnSurfaces {
		contourIDs[i] = len(d) + 1
		data := fmt.Sprintf("142,%d,%d,%d,%d,%d,0,0;", c.CreationMode, entityIDs[c.Surface], entityIDs[c.ParameterCurve], entityIDs[c.ModelCurve], c.Preference)
		if e := appendEntity(142, c.Label, c.Subscript, 0, 0, data); e != nil {
			return e
		}
		if ownedContours[i] {
			row := contourIDs[i] - 1
			d[row] = d[row][:64] + "00010500"
		}
	}
	for _, t := range m.TrimmedSurfaces {
		values := []string{"144", strconv.Itoa(entityIDs[t.Surface]), "1", strconv.Itoa(len(t.Inner)), strconv.Itoa(contourIDs[t.Outer])}
		for _, i := range t.Inner {
			values = append(values, strconv.Itoa(contourIDs[i]))
		}
		values = append(values, "0", "0")
		if e := appendEntity(144, t.Label, t.Subscript, 0, 0, strings.Join(values, ",")+";"); e != nil {
			return e
		}
	}
	b := bufio.NewWriter(w)
	line := func(s string, section byte, n int) error {
		if len(s) > 72 || n > 9999999 {
			return errors.New("iges: record overflow")
		}
		_, e := fmt.Fprintf(b, "%-72s%c%7d\n", s, section, n)
		return e
	}
	if e := line("PlainSolid independent curve geometry", 'S', 1); e != nil {
		return e
	}
	for i, s := range g {
		if e := line(s, 'G', i+1); e != nil {
			return e
		}
	}
	for i, s := range d {
		if e := line(s, 'D', i+1); e != nil {
			return e
		}
	}
	for i, r := range p {
		if e := line(fmt.Sprintf("%-64s%8d", r.data, r.owner), 'P', i+1); e != nil {
			return e
		}
	}
	term := fmt.Sprintf("S%7dG%7dD%7dP%7d", 1, len(g), len(d), len(p))
	if e := line(term, 'T', 1); e != nil {
		return e
	}
	return b.Flush()
}

// Keep numeric parameter tokens intact: some IGES consumers treat padding at a
// physical record boundary as numeric whitespace even when the next line follows.
func parameterChunks(data string, width int) []string {
	out := []string{}
	for len(data) > width {
		n := strings.LastIndexByte(data[:width], ',') + 1
		if n == 0 {
			n = width
		}
		out = append(out, data[:n])
		data = data[n:]
	}
	return append(out, data)
}
