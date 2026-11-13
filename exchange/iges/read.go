package iges

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"plainsolid/geom"
)

type record struct {
	data  string
	owner int
}

func field(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, e := strconv.Atoi(s)
	if e != nil {
		return 0, errors.New("iges: invalid directory integer")
	}
	return n, nil
}

// Read accepts fixed 80-column ASCII records in S/G/D/P/T order. Directory
// pointers, parameter back-pointers, record counts and termination counts must
// agree. Unsupported entities, transformations and non-unit model scales fail.
func Read(r io.Reader, l Limits) (Model, error) { return readCurvedMode(r, l, false) }
func readCurvedMode(r io.Reader, l Limits, curved bool) (Model, error) {
	l, e := l.defaults()
	if e != nil {
		return Model{}, e
	}
	if r == nil {
		return Model{}, errors.New("iges: nil reader")
	}
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return Model{}, e
	}
	if int64(len(b)) > l.MaxBytes {
		return Model{}, errors.New("iges: byte limit exceeded")
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
			return Model{}, errors.New("iges: record limit exceeded")
		}
		line := scan.Text()
		if len(line) != 80 {
			return Model{}, errors.New("iges: expected 80-column ASCII records")
		}
		for _, c := range []byte(line) {
			if c < 32 || c > 126 {
				return Model{}, errors.New("iges: non-ASCII record")
			}
		}
		section := line[72]
		rank, ok := order[section]
		if !ok || rank < stage {
			return Model{}, errors.New("iges: invalid section order")
		}
		stage = rank
		n, e := field(line[73:])
		if e != nil || n != len(sections[section])+1 {
			return Model{}, errors.New("iges: invalid section sequence")
		}
		rec := record{data: line[:72]}
		if section == 'P' {
			rec.data = line[:64]
			rec.owner, e = field(line[64:72])
			if e != nil || rec.owner < 1 || rec.owner%2 != 1 {
				return Model{}, errors.New("iges: invalid parameter back-pointer")
			}
		}
		sections[section] = append(sections[section], rec)
	}
	if e = scan.Err(); e != nil {
		return Model{}, e
	}
	if len(sections['S']) < 1 || len(sections['G']) < 1 || len(sections['T']) != 1 || len(sections['D'])%2 != 0 {
		return Model{}, errors.New("iges: missing/malformed sections")
	}
	if len(sections['D'])/2 > l.MaxEntities {
		return Model{}, errors.New("iges: entity limit exceeded")
	}
	term := sections['T'][0].data
	for i, section := range []byte{'S', 'G', 'D', 'P'} {
		n, e := field(term[i*8+1 : i*8+8])
		if term[i*8] != section || e != nil || n != len(sections[section]) {
			return Model{}, errors.New("iges: termination counts disagree")
		}
	}
	if strings.TrimSpace(term[32:]) != "" {
		return Model{}, errors.New("iges: invalid termination padding")
	}
	var globals strings.Builder
	for _, rec := range sections['G'] {
		globals.WriteString(rec.data)
	}
	budget := l.MaxValues
	g, sep, end, e := globalParameters(globals.String(), &budget)
	if e != nil {
		return Model{}, e
	}
	unit, e := lengthUnit(g)
	if e != nil {
		return Model{}, e
	}
	model := Model{LengthUnit: unit}
	used := make([]bool, len(sections['P']))
	controlPoints := 0
	transforms := map[int]geom.Affine{}
	references := []int{}
	entityIndex := map[int]int{}
	subordinate := map[int]bool{}
	type pendingCurve struct {
		refs  [3]int
		value CurveOnSurface
	}
	var pending []pendingCurve
	curveIDs := []int{}
	type pendingTrim struct {
		surface, outer int
		inner          []int
		value          TrimmedSurface
	}
	var trims []pendingTrim
	composites := map[int][]int{}
	for i := 0; i < len(sections['D']); i += 2 {
		a, c := sections['D'][i].data, sections['D'][i+1].data
		var first, second [9]int
		for j := 0; j < 9; j++ {
			first[j], e = field(a[j*8 : j*8+8])
			if e != nil {
				return Model{}, e
			}
			if j != 7 {
				second[j], e = field(c[j*8 : j*8+8])
				if e != nil {
					return Model{}, e
				}
			}
		}
		kind := first[0]
		if kind != 102 && kind != 100 && kind != 110 && kind != 126 && kind != 124 && kind != 128 && kind != 142 && kind != 144 {
			return Model{}, fmt.Errorf("iges: unsupported entity type %d", kind)
		}
		if second[0] != kind {
			return Model{}, errors.New("iges: directory type mismatch")
		}
		for _, j := range []int{2, 5, 7} {
			if first[j] != 0 {
				return Model{}, errors.New("iges: directory structure/view/status unsupported")
			}
		}
		if first[8] != 0 && first[8] != 10000 && !(kind == 142 && (first[8] == 500 || first[8] == 10500)) {
			return Model{}, errors.New("iges: unsupported directory status")
		}
		if first[8] == 10000 || first[8] == 10500 {
			subordinate[i+1] = true
		}
		if first[3] < 0 || first[4] < 0 || second[1] < 0 || second[2] < 0 || second[5] != 0 || second[6] != 0 {
			return Model{}, errors.New("iges: directory pointer/style unsupported")
		}
		start, lines := first[1], second[3]
		if start < 1 || lines < 1 || start > len(used) || lines > len(used)-(start-1) {
			return Model{}, errors.New("iges: parameter record range out of bounds")
		}
		var params strings.Builder
		for j := start - 1; j < start-1+lines; j++ {
			if used[j] || sections['P'][j].owner != i+1 {
				return Model{}, errors.New("iges: overlapping or inconsistent parameter pointers")
			}
			used[j] = true
			params.WriteString(sections['P'][j].data)
		}
		values, e := parseParameters(params.String(), sep, end, &budget)
		if e != nil {
			return Model{}, e
		}
		if kind == 124 {
			if first[6] != 0 || second[4] != 0 {
				return Model{}, errors.New("iges: nested/nonzero-form matrix unsupported")
			}
			matrix, e := decodeMatrix(values)
			if e != nil {
				return Model{}, e
			}
			transforms[i+1] = matrix
			continue
		}
		if kind == 144 {
			if first[6] != 0 || second[4] != 0 || first[8] != 0 || len(values) < 5 {
				return Model{}, errors.New("iges: invalid trimmed surface directory")
			}
			var ints [5]int
			for j := range ints {
				v, e := intValue(values[j])
				if e != nil {
					return Model{}, e
				}
				ints[j] = v
			}
			n := ints[3]
			if ints[0] != 144 || ints[2] != 1 || n < 0 || n > len(values)-5 {
				return Model{}, errors.New("iges: invalid trim contours")
			}
			inner := make([]int, n)
			for j := range inner {
				v, e := intValue(values[5+j])
				if e != nil {
					return Model{}, e
				}
				inner[j] = v
			}
			if e := noPointers(values[5+n:]); e != nil {
				return Model{}, e
			}
			trims = append(trims, pendingTrim{ints[1], ints[4], inner, TrimmedSurface{Label: strings.TrimSpace(c[56:64]), Subscript: second[8]}})
			continue
		}
		if kind == 142 {
			if first[6] != 0 || second[4] != 0 || (first[8] != 0 && first[8] != 500 && first[8] != 10500) || len(values) < 6 {
				return Model{}, errors.New("iges: invalid curve-on-surface directory")
			}
			var ints [6]int
			for j := range ints {
				v, e := intValue(values[j])
				if e != nil {
					return Model{}, e
				}
				ints[j] = v
			}
			if ints[0] != 142 {
				return Model{}, errors.New("iges: type142 parameter mismatch")
			}
			if e := noPointers(values[6:]); e != nil {
				return Model{}, e
			}
			curveIDs = append(curveIDs, i+1)
			pending = append(pending, pendingCurve{[3]int{ints[2], ints[3], ints[4]}, CurveOnSurface{CreationMode: ints[1], Preference: ints[5], Label: strings.TrimSpace(c[56:64]), Subscript: second[8]}})
			continue
		}
		if kind == 102 {
			if first[6] != 0 || second[4] != 0 || len(values) < 6 {
				return Model{}, errors.New("iges: invalid composite directory")
			}
			typ, er := intValue(values[0])
			n, er2 := intValue(values[1])
			if er != nil || er2 != nil || typ != 102 || n < 4 || n > MaxOrthogonalVertices || n%2 != 0 || len(values) < 2+n {
				return Model{}, errors.New("iges: unsupported composite edge count")
			}
			refs := make([]int, n)
			for j := range refs {
				refs[j], e = intValue(values[j+2])
				if e != nil {
					return Model{}, e
				}
			}
			if e := noPointers(values[2+n:]); e != nil {
				return Model{}, e
			}
			entityIndex[i+1] = len(model.Entities)
			composites[len(model.Entities)] = refs
			model.Entities = append(model.Entities, Entity{Composite: &CompositeCurve{Curves: make([]int, n)}, Label: strings.TrimSpace(c[56:64]), Subscript: second[8]})
			references = append(references, 0)
			continue
		}
		entityIndex[i+1] = len(model.Entities)
		entity, e := decode(kind, values, l.MaxControlPoints-controlPoints)
		if e != nil {
			return Model{}, e
		}
		entity.Label = strings.TrimSpace(c[56:64])
		entity.Subscript = second[8]
		entity.Form = second[4]
		if e = entity.validate(); e != nil {
			return Model{}, e
		}
		if entity.Surface != nil {
			controlPoints += len(entity.Surface.Geometry.Data().Poles)
		}
		if entity.Spline != nil {
			controlPoints += len(entity.Spline.Curve.Poles())
		}
		model.Entities = append(model.Entities, entity)
		references = append(references, first[6])
	}
	for _, ok := range used {
		if !ok {
			return Model{}, errors.New("iges: unclaimed parameter record")
		}
	}
	usedTransforms := map[int]bool{}
	for i, id := range references {
		if id == 0 {
			continue
		}
		matrix, ok := transforms[id]
		if !ok {
			return Model{}, errors.New("iges: missing transformation matrix")
		}
		model.Entities[i].Transform = &matrix
		usedTransforms[id] = true
		if e := model.Entities[i].validate(); e != nil {
			return Model{}, e
		}
	}
	for id := range transforms {
		if !usedTransforms[id] {
			return Model{}, errors.New("iges: unreferenced transformation matrix")
		}
	}
	owned := map[int]bool{}
	for idx, refs := range composites {
		for j, id := range refs {
			k, ok := entityIndex[id]
			if !ok || model.Entities[k].Composite != nil {
				return Model{}, errors.New("iges: missing or nested composite reference")
			}
			model.Entities[idx].Composite.Curves[j] = k
			owned[id] = true
		}
		if e := model.validateComposite(idx); e != nil {
			return Model{}, e
		}
	}

	for _, p := range pending {
		refs := [3]int{}
		for j, id := range p.refs {
			idx, ok := entityIndex[id]
			if !ok {
				return Model{}, errors.New("iges: missing or invalid curve-on-surface reference")
			}
			refs[j] = idx
			owned[id] = true
		}
		c := p.value
		c.Surface = refs[0]
		c.ParameterCurve = refs[1]
		c.ModelCurve = refs[2]
		validate := model.ValidateCurveOnSurface
		if curved {
			validate = model.validateCurvedOvalMapping
		}
		if e := validate(c); e != nil {
			return Model{}, e
		}
		model.CurvesOnSurfaces = append(model.CurvesOnSurfaces, c)
	}
	for id, idx := range entityIndex {
		if model.Entities[idx].Composite != nil && !owned[id] {
			return Model{}, errors.New("iges: standalone composites unsupported")
		}
	}
	curveIndex := map[int]int{}
	for i, id := range curveIDs {
		curveIndex[id] = i
	}
	for _, p := range trims {
		s, ok := entityIndex[p.surface]
		if !ok {
			return Model{}, errors.New("iges: invalid trim surface reference")
		}
		outer, ok := curveIndex[p.outer]
		if !ok {
			return Model{}, errors.New("iges: invalid outer contour reference")
		}
		t := p.value
		t.Surface = s
		t.Outer = outer
		owned[p.surface] = true
		owned[p.outer] = true
		for _, id := range p.inner {
			idx, ok := curveIndex[id]
			if !ok {
				return Model{}, errors.New("iges: invalid inner contour reference")
			}
			t.Inner = append(t.Inner, idx)
			owned[id] = true
		}
		validateTrim := model.ValidateTrimmedSurface
		if curved {
			validateTrim = model.validateCurvedOvalTrim
		}
		if e := validateTrim(t); e != nil {
			return Model{}, e
		}
		model.TrimmedSurfaces = append(model.TrimmedSurfaces, t)
	}
	for id := range subordinate {
		if !owned[id] {
			return Model{}, errors.New("iges: orphan subordinate entity")
		}
	}
	return model, nil
}
func decode(kind int, p []parameter, maxPoints int) (Entity, error) {
	if len(p) == 0 {
		return Entity{}, errors.New("iges: empty parameters")
	}
	k, e := intValue(p[0])
	if e != nil || k != kind {
		return Entity{}, errors.New("iges: parameter entity type mismatch")
	}
	numbers := func(start, n int) ([]float64, error) {
		if start < 0 || n < 0 || start > len(p) || n > len(p)-start {
			return nil, errors.New("iges: truncated geometry")
		}
		v := make([]float64, n)
		for i := range v {
			v[i], e = realValue(p[start+i])
			if e != nil {
				return nil, e
			}
		}
		return v, nil
	}
	switch kind {
	case 110:
		v, e := numbers(1, 6)
		if e != nil {
			return Entity{}, e
		}
		if e = noPointers(p[7:]); e != nil {
			return Entity{}, e
		}
		return Entity{Line: &Line{Start: geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, End: geom.Vec3{X: v[3], Y: v[4], Z: v[5]}}}, nil
	case 100:
		v, e := numbers(1, 7)
		if e != nil {
			return Entity{}, e
		}
		if e = noPointers(p[8:]); e != nil {
			return Entity{}, e
		}
		return Entity{Arc: &Arc{Center: geom.Vec3{X: v[1], Y: v[2], Z: v[0]}, Start: geom.Vec2{X: v[3], Y: v[4]}, End: geom.Vec2{X: v[5], Y: v[6]}}}, nil
	case 128:
		return decodeSurface(p, maxPoints)
	case 126:
		if len(p) < 7 {
			return Entity{}, errors.New("iges: truncated spline")
		}
		upper, e := intValue(p[1])
		if e != nil || upper < 0 || upper >= maxPoints {
			return Entity{}, errors.New("iges: control-point limit/count invalid")
		}
		n := upper + 1
		degree, e := intValue(p[2])
		if e != nil || degree < 1 || degree >= n {
			return Entity{}, errors.New("iges: invalid spline degree")
		}
		if n > (len(p)-13-degree)/5 {
			return Entity{}, errors.New("iges: truncated spline")
		}
		flags := [4]bool{}
		for i := range flags {
			v, e := intValue(p[3+i])
			if e != nil || (v != 0 && v != 1) {
				return Entity{}, errors.New("iges: invalid spline flag")
			}
			flags[i] = v == 1
		}
		knotCount := n + degree + 1
		knots, e := numbers(7, knotCount)
		if e != nil {
			return Entity{}, e
		}
		weights, e := numbers(7+knotCount, n)
		if e != nil {
			return Entity{}, e
		}
		coords, e := numbers(7+knotCount+n, 3*n)
		if e != nil {
			return Entity{}, e
		}
		poles := make([]geom.Vec3, n)
		for i := range poles {
			poles[i] = geom.Vec3{X: coords[3*i], Y: coords[3*i+1], Z: coords[3*i+2]}
		}
		last := 7 + knotCount + 4*n
		tail, e := numbers(last, 5)
		if e != nil {
			return Entity{}, e
		}
		if e = noPointers(p[last+5:]); e != nil {
			return Entity{}, e
		}
		curve, e := geom.NewBSpline(degree, poles, knots, weights)
		if e != nil {
			return Entity{}, e
		}
		return Entity{sourceWeightsRounded: !exactWeightNormalization(weights), Spline: &Spline{Curve: curve, Start: tail[0], End: tail[1], Planar: flags[0], Closed: flags[1], Polynomial: flags[2], Periodic: flags[3], Normal: geom.Vec3{X: tail[2], Y: tail[3], Z: tail[4]}}}, nil
	}
	return Entity{}, errors.New("iges: unsupported geometry")
}

func decodeMatrix(p []parameter) (geom.Affine, error) {
	if len(p) < 13 {
		return geom.Affine{}, errors.New("iges: truncated transformation")
	}
	kind, e := intValue(p[0])
	if e != nil || kind != 124 && kind != 128 && kind != 142 && kind != 144 {
		return geom.Affine{}, errors.New("iges: invalid transformation type")
	}
	var a geom.Affine
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			a[i][j], e = realValue(p[1+i*4+j])
			if e != nil {
				return a, e
			}
		}
	}
	if e = noPointers(p[13:]); e != nil {
		return a, e
	}
	if _, e = a.Inverse(); e != nil {
		return a, e
	}
	return a, nil
}
