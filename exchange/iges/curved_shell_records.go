package iges

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type topologyRecord struct {
	id, kind  int
	transform int
	label     string
	subscript int
	values    []parameter
}
type topologySource struct {
	unit     float64
	bytes    []byte
	entities []topologyRecord
}

func readTopologySource(r io.Reader, l Limits, lensMode ...bool) (topologySource, error) {
	return readSupportRecords(r, l, len(lensMode) == 1 && lensMode[0], false)
}

func readSupportRecords(r io.Reader, l Limits, lens, cylinders bool) (topologySource, error) {
	return readAnalyticSupportRecords(r, l, lens, cylinders, false)
}
func readAnalyticSupportRecords(r io.Reader, l Limits, lens, cylinders, planes bool) (topologySource, error) {
	return readElementarySupportRecords(r, l, lens, cylinders, planes, false)
}
func readElementarySupportRecords(r io.Reader, l Limits, lens, cylinders, planes, spheres bool) (topologySource, error) {
	return readRingSupportRecords(r, l, lens, cylinders, planes, spheres, false)
}
func readRingSupportRecords(r io.Reader, l Limits, lens, cylinders, planes, spheres, tori bool) (topologySource, error) {
	return readConeSupportRecords(r, l, lens, cylinders, planes, spheres, tori, false)
}
func readConeSupportRecords(r io.Reader, l Limits, lens, cylinders, planes, spheres, tori, cones bool) (topologySource, error) {
	return readPlacedTopologyRecords(r, l, lens, cylinders, planes, spheres, tori, cones, false)
}
func readPlacedTopologyRecords(r io.Reader, l Limits, lens, cylinders, planes, spheres, tori, cones, placed bool, circularMode ...bool) (topologySource, error) {
	l, e := l.defaults()
	if e != nil {
		return topologySource{}, e
	}
	if r == nil {
		return topologySource{}, errors.New("iges: nil reader")
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return topologySource{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return topologySource{}, errors.New("iges: byte limit exceeded")
	}
	sections := map[byte][]record{}
	order := map[byte]int{'S': 0, 'G': 1, 'D': 2, 'P': 3, 'T': 4}
	stage := -1
	count := 0
	scan := bufio.NewScanner(bytes.NewReader(b))
	scan.Buffer(make([]byte, 256), 1024)
	for scan.Scan() {
		count++
		if count > l.MaxRecords {
			return topologySource{}, errors.New("iges: record limit exceeded")
		}
		line := scan.Text()
		if len(line) != 80 {
			return topologySource{}, errors.New("iges: expected 80-column ASCII records")
		}
		for _, c := range []byte(line) {
			if c < 32 || c > 126 {
				return topologySource{}, errors.New("iges: non-ASCII record")
			}
		}
		section := line[72]
		rank, ok := order[section]
		if !ok || rank < stage {
			return topologySource{}, errors.New("iges: invalid section order")
		}
		stage = rank
		n, e := field(line[73:])
		if e != nil || n != len(sections[section])+1 {
			return topologySource{}, errors.New("iges: invalid section sequence")
		}
		rec := record{data: line[:72]}
		if section == 'P' {
			rec.data = line[:64]
			rec.owner, e = field(line[64:72])
			if e != nil || rec.owner < 1 || rec.owner%2 != 1 {
				return topologySource{}, errors.New("iges: invalid parameter back-pointer")
			}
		}
		sections[section] = append(sections[section], rec)
	}
	if e = scan.Err(); e != nil {
		return topologySource{}, e
	}
	if len(sections['S']) < 1 || len(sections['G']) < 1 || len(sections['T']) != 1 || len(sections['D'])%2 != 0 {
		return topologySource{}, errors.New("iges: missing/malformed sections")
	}
	if len(sections['D'])/2 > l.MaxEntities {
		return topologySource{}, errors.New("iges: entity limit exceeded")
	}
	term := sections['T'][0].data
	for i, section := range []byte{'S', 'G', 'D', 'P'} {
		n, e := field(term[i*8+1 : i*8+8])
		if term[i*8] != section || e != nil || n != len(sections[section]) {
			return topologySource{}, errors.New("iges: termination counts disagree")
		}
	}
	if strings.TrimSpace(term[32:]) != "" {
		return topologySource{}, errors.New("iges: invalid termination padding")
	}
	var globals strings.Builder
	for _, rec := range sections['G'] {
		globals.WriteString(rec.data)
	}
	budget := l.MaxValues
	g, sep, end, e := globalParameters(globals.String(), &budget)
	if e != nil {
		return topologySource{}, e
	}
	unit, e := lengthUnit(g)
	if e != nil {
		return topologySource{}, e
	}
	doc := topologySource{unit: unit, bytes: append([]byte(nil), b...)}
	used := make([]bool, len(sections['P']))
	for i := 0; i < len(sections['D']); i += 2 {
		a, c := sections['D'][i].data, sections['D'][i+1].data
		var first, second [9]int
		for j := 0; j < 9; j++ {
			first[j], e = field(a[j*8 : j*8+8])
			if e != nil {
				return topologySource{}, e
			}
			if j != 7 {
				second[j], e = field(c[j*8 : j*8+8])
				if e != nil {
					return topologySource{}, e
				}
			}
		}
		kind := first[0]
		// The second private flag is reserved for the strict model-only type194
		// material codec. The distinct three-flag mode admits only full ellipse104
		// and its owned124; existing caller modes retain their entity gates.
		placedCone := len(circularMode) == 3 && circularMode[0] && circularMode[1] && circularMode[2] && lens && placed && !cones && !planes && !cylinders && !spheres && !tori
		conicalMaterial := len(circularMode) == 2 && circularMode[0] && circularMode[1] && lens && !placed && !cones && !planes && !cylinders && !spheres && !tori || placedCone
		ellipsePrism := len(circularMode) == 7 && !circularMode[0] && !circularMode[1] && circularMode[2] && circularMode[3] && circularMode[4] && circularMode[5] && circularMode[6]
		ellipseDisk := len(circularMode) == 6 && !circularMode[0] && !circularMode[1] && circularMode[2] && circularMode[3] && circularMode[4] && circularMode[5]
		ellipseTabulated := len(circularMode) == 5 && !circularMode[0] && !circularMode[1] && circularMode[2] && circularMode[3] && circularMode[4]
		ellipseRing := ellipseTabulated || len(circularMode) == 4 && !circularMode[0] && !circularMode[1] && circularMode[2] && circularMode[3]
		ellipseConic := ellipsePrism || ellipseDisk || ellipseRing || len(circularMode) == 3 && !circularMode[0] && !circularMode[1] && circularMode[2]
		circular := len(circularMode) == 1 && circularMode[0] || conicalMaterial
		supported := (conicalMaterial && kind == 194) || (circular && (kind == 100 || kind == 110 || kind == 116 || kind == 123 || kind == 190 || kind == 192)) || (placed && kind == 124) || kind == 126 || kind == 128 || kind == 502 || kind == 504 || kind == 508 || kind == 510 || kind == 514 || (lens && kind == 186)
		if cylinders || planes || spheres || tori || cones {
			supported = kind == 116 || kind == 123 || (cylinders && kind == 192) || ((planes || ellipseDisk || ellipsePrism) && kind == 190) || (spheres && kind == 196) || (tori && kind == 198) || (cones && kind == 194)
		}
		if ellipseConic {
			supported = kind == 104 || kind == 124 || ellipseRing && kind == 102 || ellipseTabulated && kind == 122 || ellipseDisk && (kind == 144 || kind == 142 || kind == 190 || kind == 116 || kind == 123) || ellipsePrism && (kind == 110 || kind == 116 || kind == 123 || kind == 122 || kind == 190 || kind == 186 || kind == 502 || kind == 504 || kind == 508 || kind == 510 || kind == 514)
		}
		if !supported {
			return topologySource{}, fmt.Errorf("iges: unsupported topology entity %d", kind)
		}

		if second[0] != kind {
			return topologySource{}, errors.New("iges: directory type mismatch")
		}
		for _, j := range []int{2, 5, 7} {
			if first[j] != 0 {
				return topologySource{}, errors.New("iges: directory structure/view/status unsupported")
			}
		}
		if !(ellipseDisk && kind == 142 && first[8] == 10500) && first[8] != 0 && first[8] != 10000 && !(kind == 504 && first[8] == 10001) && !((cylinders || planes || spheres || tori || cones || ellipseDisk) && ((kind == 116 && first[8] == 10400) || (kind == 123 && first[8] == 10200))) {
			return topologySource{}, errors.New("iges: unsupported directory status")
		}
		if first[8] == 10000 || first[8] == 10500 {
			// owned topology geometry
		}
		if first[3] < 0 || first[4] < 0 || second[1] < 0 || second[2] < 0 || second[5] != 0 || second[6] != 0 {
			return topologySource{}, errors.New("iges: directory pointer/style unsupported")
		}
		start, lines := first[1], second[3]
		if start < 1 || lines < 1 || start > len(used) || lines > len(used)-(start-1) {
			return topologySource{}, errors.New("iges: parameter record range out of bounds")
		}
		var params strings.Builder
		for j := start - 1; j < start-1+lines; j++ {
			if used[j] || sections['P'][j].owner != i+1 {
				return topologySource{}, errors.New("iges: overlapping or inconsistent parameter pointers")
			}
			used[j] = true
			params.WriteString(sections['P'][j].data)
		}
		values, e := parseParameters(params.String(), sep, end, &budget)
		if e != nil {
			return topologySource{}, e
		}
		if (first[6] != 0 && !(placed && (placedCone && kind == 194 || ellipseConic && kind == 104 || ellipsePrism && (kind == 110 || kind == 190) || kind == 126 || kind == 128 || circular && (kind == 100 || kind == 110 || kind == 190 || kind == 192)))) || second[8] < 0 || second[8] > 99999999 || (kind >= 500 && kind != 514 && second[4] != 1) || (kind == 514 && ((!lens && second[4] != 2) || (lens && second[4] != 1))) || (kind < 500 && second[4] != 0 && !((ellipseConic && kind == 104 || cylinders && kind == 192 || (planes || ellipseDisk || ellipsePrism) && kind == 190 || spheres && kind == 196 || tori && kind == 198 || cones && kind == 194 || conicalMaterial && kind == 194 || circular && (kind == 190 || kind == 192)) && second[4] == 1)) || ((ellipseConic && kind == 104 || cylinders && kind == 192 || (planes || ellipseDisk || ellipsePrism) && kind == 190 || spheres && kind == 196 || tori && kind == 198 || cones && kind == 194 || conicalMaterial && kind == 194 || circular && (kind == 190 || kind == 192)) && second[4] != 1) {
			return topologySource{}, errors.New("iges: unsupported topology placement/form")
		}
		doc.entities = append(doc.entities, topologyRecord{id: i + 1, kind: kind, transform: first[6], values: values, label: strings.TrimSpace(c[56:64]), subscript: second[8]})
	}
	for _, u := range used {
		if !u {
			return topologySource{}, errors.New("iges: unclaimed parameter record")
		}
	}
	return doc, nil
}
