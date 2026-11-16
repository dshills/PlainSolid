package iges

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type nativeTopologyRecord struct {
	kind, form int
	label      string
	subscript  int
	data       string
	status     *int
}
type topologyAuthor struct {
	records       []nativeTopologyRecord
	geometry      Model
	groupedSolids bool
}

func (a *topologyAuthor) add(kind, form int, data string) int {
	id := 2*len(a.records) + 1
	a.records = append(a.records, nativeTopologyRecord{kind: kind, form: form, data: data})
	return id
}
func topologyNumbers(v ...int) string {
	p := make([]string, len(v))
	for i, n := range v {
		p[i] = strconv.Itoa(n)
	}
	return strings.Join(p, ",") + ";"
}
func (a *topologyAuthor) entity(en Entity) int {
	k, data := encode(en)
	id := a.add(k, en.Form, data)
	a.records[len(a.records)-1].label = en.Label
	a.records[len(a.records)-1].subscript = en.Subscript
	a.geometry.Entities = append(a.geometry.Entities, en)
	return id
}
func (a *topologyAuthor) file(unit float64) ([]byte, error) {
	a.geometry.LengthUnit = unit
	var geom bytes.Buffer
	if e := Write(&geom, a.geometry); e != nil {
		return nil, e
	}
	var globals []string
	scan := bufio.NewScanner(&geom)
	for scan.Scan() {
		l := scan.Text()
		if len(l) == 80 && l[72] == 'G' {
			globals = append(globals, l[:72])
		}
	}
	if e := scan.Err(); e != nil {
		return nil, e
	}
	var b bytes.Buffer
	line := func(d string, s byte, n int) { fmt.Fprintf(&b, "%-72s%c%7d\n", d, s, n) }
	line("PlainSolid exact spline topology", 'S', 1)
	for i, g := range globals {
		line(g, 'G', i+1)
	}
	var params []record
	for i, r := range a.records {
		c := parameterChunks(r.data, 64)
		start := len(params) + 1
		for _, s := range c {
			params = append(params, record{s, 2*i + 1})
		}
		status := 10000
		if r.kind == 504 {
			status = 10001
		}
		if r.kind == 186 {
			status = 0
			if a.groupedSolids {
				status = 20000
			}
		}
		if r.kind == 402 && a.groupedSolids {
			status = 0
		}
		if r.status != nil {
			status = *r.status
		}
		line(fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%8d%08d", r.kind, start, 0, 0, 0, 0, 0, 0, status), 'D', 2*i+1)
		line(fmt.Sprintf("%8d%8d%8d%8d%8d%8d%8d%-8s%8d", r.kind, 0, 0, len(c), r.form, 0, 0, r.label, r.subscript), 'D', 2*i+2)
	}
	for i, p := range params {
		line(fmt.Sprintf("%-64s%8d", p.data, p.owner), 'P', i+1)
	}
	line(fmt.Sprintf("S%7dG%7dD%7dP%7d", 1, len(globals), 2*len(a.records), len(params)), 'T', 1)
	return b.Bytes(), nil
}
