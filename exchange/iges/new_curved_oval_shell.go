package iges

import (
	"bufio"
	"bytes"
	"fmt"
	"plainsolid/geom"
	"strconv"
	"strings"
)

// NewCurvedOvalShell constructs a real open IGES shell from a certified annulus
// and its fill disk, retaining both original support charts. Four exact
// quarters are emitted per contour; inner model edges and vertices are shared.
// Quarter coefficients and endpoints must be losslessly float-representable.
// As with CurvedOvalSheet, pairwise face-interior embedding is not certified.
func NewCurvedOvalShell(sheet CurvedOvalSheet) (CurvedOvalShell, error) {
	proved, e := CertifyCurvedOvalSheet(sheet.Model)
	if e != nil {
		return CurvedOvalShell{}, e
	}
	m := proved.Model
	at, dt := m.TrimmedSurfaces[proved.Annulus], m.TrimmedSurfaces[proved.Disk]
	contours := []CurveOnSurface{m.CurvesOnSurfaces[at.Outer], m.CurvesOnSurfaces[at.Inner[0]], m.CurvesOnSurfaces[dt.Outer]}
	type authored struct {
		kind, form int
		label      string
		subscript  int
		data       string
	}
	var records []authored
	add := func(kind, form int, data string) int {
		id := 2*len(records) + 1
		records = append(records, authored{kind: kind, form: form, data: data})
		return id
	}
	entity := func(en Entity) int {
		k, d := encode(en)
		id := add(k, en.Form, d)
		records[len(records)-1].label = en.Label
		records[len(records)-1].subscript = en.Subscript
		return id
	}
	surfaces := [2]int{entity(m.Entities[at.Surface]), entity(m.Entities[dt.Surface])}
	var modelIDs [2][4]int
	var uvIDs [3][4]int
	var vertices []geom.Vec3
	split := func(s *Spline, q int) (*geom.BSpline, error) {
		x, e := geom.NewExactSplineCurve(s.Curve, curvedOvalOptions())
		if e != nil {
			return nil, e
		}
		x, e = x.Restrict(float64(q), float64(q+1), curvedOvalOptions())
		if e != nil {
			return nil, e
		}
		x, e = x.Reparameterize(0, 1, false, curvedOvalOptions())
		if e != nil {
			return nil, e
		}
		return x.ToBSpline()
	}
	for loop := 0; loop < 2; loop++ {
		for q := 0; q < 4; q++ {
			curve, e := split(m.Entities[contours[loop].ModelCurve].Spline, q)
			if e != nil {
				return CurvedOvalShell{}, e
			}
			x, e := geom.NewExactSplineCurve(curve, curvedOvalOptions())
			if e != nil {
				return CurvedOvalShell{}, e
			}
			p, e := x.ExactPoint(0)
			if e != nil {
				return CurvedOvalShell{}, e
			}
			vertices = append(vertices, p)
			modelIDs[loop][q] = entity(Entity{Spline: &Spline{Curve: curve, Start: 0, End: 1}})
		}
	}
	for loop := 0; loop < 3; loop++ {
		for q := 0; q < 4; q++ {
			curve, e := split(m.Entities[contours[loop].ParameterCurve].Spline, q)
			if e != nil {
				return CurvedOvalShell{}, e
			}
			uvIDs[loop][q] = entity(Entity{Spline: &Spline{Curve: curve, Start: 0, End: 1, Planar: true, Normal: geom.Vec3{Z: 1}}})
		}
	}
	values := []string{"502", "8"}
	for _, p := range vertices {
		values = append(values, number(p.X), number(p.Y), number(p.Z))
	}
	vertexID := add(502, 1, strings.Join(values, ",")+";")
	ints := func(v ...int) string {
		p := make([]string, len(v))
		for i, n := range v {
			p[i] = strconv.Itoa(n)
		}
		return strings.Join(p, ",") + ";"
	}
	edges := []int{504, 8}
	for loop := 0; loop < 2; loop++ {
		for q := 0; q < 4; q++ {
			edges = append(edges, modelIDs[loop][q], vertexID, 4*loop+q+1, vertexID, 4*loop+(q+1)%4+1)
		}
	}
	edgeID := add(504, 1, ints(edges...))
	var loops [3]int
	for loop := 0; loop < 3; loop++ {
		v := []int{508, 4}
		for q := 0; q < 4; q++ {
			index, forward := 4*loop+q+1, 1
			if loop == 2 {
				index, forward = 8-q, 0
			}
			v = append(v, 0, edgeID, index, forward, 1, 0, uvIDs[loop][q])
		}
		loops[loop] = add(508, 1, ints(v...))
	}
	a := add(510, 1, ints(510, surfaces[0], 2, 1, loops[0], loops[1]))
	records[len(records)-1].label = at.Label
	records[len(records)-1].subscript = at.Subscript
	d := add(510, 1, ints(510, surfaces[1], 1, 1, loops[2]))
	records[len(records)-1].label = dt.Label
	records[len(records)-1].subscript = dt.Subscript
	add(514, 2, ints(514, 2, a, 1, d, 1))
	// Reuse the existing independently validated global-unit encoder.
	var geometry bytes.Buffer
	if e = WriteCurvedOvalSheet(&geometry, proved); e != nil {
		return CurvedOvalShell{}, e
	}
	var globals []string
	scan := bufio.NewScanner(bytes.NewReader(geometry.Bytes()))
	for scan.Scan() {
		line := scan.Text()
		if len(line) == 80 && line[72] == 'G' {
			globals = append(globals, line[:72])
		}
	}
	if e = scan.Err(); e != nil {
		return CurvedOvalShell{}, e
	}
	var out bytes.Buffer
	line := func(data string, section byte, n int) { fmt.Fprintf(&out, "%-72s%c%7d\n", data, section, n) }
	line("PlainSolid exact curved open shell", 'S', 1)
	for i, g := range globals {
		line(g, 'G', i+1)
	}
	var parameters []record
	for i, r := range records {
		chunks := parameterChunks(r.data, 64)
		start := len(parameters) + 1
		for _, c := range chunks {
			parameters = append(parameters, record{data: c, owner: 2*i + 1})
		}
		status := 10000
		if r.kind == 504 {
			status = 10001
		}
		if r.kind == 514 {
			status = 0
		}
		line(fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%8d%08d", r.kind, start, 0, 0, 0, 0, 0, 0, status), 'D', 2*i+1)
		line(fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%-8s%8d", r.kind, 0, 0, len(chunks), r.form, 0, 0, r.label, r.subscript), 'D', 2*i+2)
	}
	for i, p := range parameters {
		line(fmt.Sprintf("%-64s%8d", p.data, p.owner), 'P', i+1)
	}
	line(fmt.Sprintf("S%7dG%7dD%7dP%7d", 1, len(globals), 2*len(records), len(parameters)), 'T', 1)
	return ReadCurvedOvalShell(bytes.NewReader(out.Bytes()), Limits{})
}
