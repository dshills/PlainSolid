package step

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RGB contains STEP COLOUR_RGB components or the exact standard draughting
// palette equivalent in sRGB space, each in [0,1].
// No alpha, texture, material, lighting or per-face override is implied.
type RGB struct{ R, G, B float64 }
type ColoredBody struct {
	Body         Body
	SurfaceColor *RGB
}
type ColoredModel struct {
	Bodies     []ColoredBody
	LengthUnit float64
}

func validRGB(c RGB) bool {
	return finiteOblique(c.R) && finiteOblique(c.G) && finiteOblique(c.B) && c.R >= 0 && c.R <= 1 && c.G >= 0 && c.G <= 1 && c.B >= 0 && c.B <= 1
}

// ReadColoredPlanar imports planar bodies and optional whole-body, both-sided
// surface fill colors. Conflicting, face-level, occurrence and overriding
// styles are rejected rather than silently dropped. Existing Read ignores styles.
func ReadColoredPlanar(r io.Reader, l Limits) (ColoredModel, error) {
	m, e := readFaceColors(r, l, false)
	if e != nil {
		return ColoredModel{}, e
	}
	out := ColoredModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		out.Bodies = append(out.Bodies, ColoredBody{b.Body, b.SurfaceColor})
	}
	return out, nil
}

func readFaceColors(r io.Reader, l Limits, allowFaces bool) (FaceColoredModel, error) {
	fail := func() (FaceColoredModel, error) {
		return FaceColoredModel{}, errors.New("step: unsupported or malformed whole-body surface style")
	}
	l, e := l.defaults()
	if e != nil {
		return FaceColoredModel{}, e
	}
	if r == nil {
		return fail()
	}
	raw, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return FaceColoredModel{}, e
	}
	if int64(len(raw)) > l.MaxBytes {
		return fail()
	}
	f, e := Parse(bytes.NewReader(raw), l)
	if e != nil {
		return FaceColoredModel{}, e
	}
	m, e := Read(bytes.NewReader(raw), l)
	if e != nil {
		return FaceColoredModel{}, e
	}
	ids, contexts, e := colorBodyIDs(f)
	if e != nil || len(ids) != len(m.Bodies) {
		return fail()
	}
	return readStylesForBodies(f, m, ids, contexts, allowFaces)
}
func readStylesForBodies(f *File, m Model, ids []int64, contexts map[int64]int64, allowFaces bool) (FaceColoredModel, error) {
	counts := make([]int, len(m.Bodies))
	for i, b := range m.Bodies {
		counts[i] = len(b.Solid.Faces())
	}
	return readStylesForBodyCounts(f, m, ids, contexts, allowFaces, counts)
}

// Callers must independently validate geometry and provide its source face count.
func readStylesForBodyCounts(f *File, m Model, ids []int64, contexts map[int64]int64, allowFaces bool, counts []int) (FaceColoredModel, error) {
	fail := func() (FaceColoredModel, error) {
		return FaceColoredModel{}, errors.New("step: unsupported or malformed surface style")
	}
	out := FaceColoredModel{LengthUnit: m.LengthUnit, Bodies: make([]FaceColoredBody, len(ids))}
	indices := map[int64]int{}
	for i, id := range ids {
		indices[id] = i
		out.Bodies[i].Body = m.Bodies[i]
	}
	faceTargets := map[int64][2]int{}
	if allowFaces {
		for i, id := range ids {
			faces, e := bodyFaceIDs(f, id)
			if e != nil {
				return fail()
			}
			if i >= len(counts) || len(faces) != counts[i] {
				return fail()
			}
			out.Bodies[i].FaceColors = map[int]RGB{}
			for j, fid := range faces {
				if _, ok := faceTargets[fid]; ok {
					return fail()
				}
				faceTargets[fid] = [2]int{i, j}
				contexts[fid] = contexts[id]
			}
		}
	}
	coloredTargets := map[int64]bool{}
	overrides := map[int64]int64{}
	styleTargets := map[int64]int64{}
	seen := map[int64]bool{}
	styled := map[int64]bool{}
	take := func(id int64, kind string, n int) (Record, error) {
		r, e := simple(f, id, kind, n)
		if e == nil {
			seen[id] = true
		}
		return r, e
	}
	one := func(v Value) (int64, error) {
		ids, e := listRefs(v, 1)
		if e != nil || len(ids) != 1 {
			return 0, errors.New("step: expected one style")
		}
		return ids[0], nil
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION" {
			continue
		}
		rep, e := take(id, ent.Records[0].Type, 3)
		if e != nil {
			return fail()
		}
		if _, e = textValue(rep.Args[0]); e != nil {
			return fail()
		}
		ctx, e := ref(rep.Args[2])
		if e != nil {
			return fail()
		}
		items, e := listRefs(rep.Args[1], 1)
		if e != nil {
			return fail()
		}
		for _, item := range items {
			if styled[item] {
				return fail()
			}
			styled[item] = true
			s, e := take(item, "STYLED_ITEM", 3)
			if e != nil && allowFaces {
				s, e = take(item, "OVER_RIDING_STYLED_ITEM", 4)
				if e == nil {
					parent, err := ref(s.Args[3])
					if err != nil {
						return fail()
					}
					overrides[item] = parent
				}
			}
			if e != nil {
				return fail()
			}
			if _, e = textValue(s.Args[0]); e != nil {
				return fail()
			}
			body, e := ref(s.Args[2])
			if e != nil {
				return fail()
			}
			index, ok := indices[body]
			faceIndex := -1
			if !ok {
				if target, found := faceTargets[body]; found {
					index, faceIndex, ok = target[0], target[1], true
				}
			}
			styleTargets[item] = body
			if _, overridden := overrides[item]; overridden && faceIndex < 0 {
				return fail()
			}
			if !ok || coloredTargets[body] || contexts[body] != ctx {
				return fail()
			}
			next, e := one(s.Args[1])
			if e != nil {
				return fail()
			}
			assignment, e := take(next, "PRESENTATION_STYLE_ASSIGNMENT", 1)
			if e != nil {
				return fail()
			}
			next, e = one(assignment.Args[0])
			if e != nil {
				return fail()
			}
			usage, e := take(next, "SURFACE_STYLE_USAGE", 2)
			if e != nil || usage.Args[0].Kind != Enumeration || usage.Args[0].Text != "BOTH" {
				return fail()
			}
			next, e = ref(usage.Args[1])
			if e != nil {
				return fail()
			}
			side, e := take(next, "SURFACE_SIDE_STYLE", 2)
			if e != nil {
				return fail()
			}
			if _, e = textValue(side.Args[0]); e != nil {
				return fail()
			}
			next, e = one(side.Args[1])
			if e != nil {
				return fail()
			}
			fill, e := take(next, "SURFACE_STYLE_FILL_AREA", 1)
			if e != nil {
				return fail()
			}
			next, e = ref(fill.Args[0])
			if e != nil {
				return fail()
			}
			style, e := take(next, "FILL_AREA_STYLE", 2)
			if e != nil {
				return fail()
			}
			if _, e = textValue(style.Args[0]); e != nil {
				return fail()
			}
			next, e = one(style.Args[1])
			if e != nil {
				return fail()
			}
			color, e := take(next, "FILL_AREA_STYLE_COLOUR", 2)
			if e != nil {
				return fail()
			}
			if _, e = textValue(color.Args[0]); e != nil {
				return fail()
			}
			next, e = ref(color.Args[1])
			if e != nil {
				return fail()
			}
			c, e := readSurfaceStyleColour(f, next)
			if e != nil {
				return fail()
			}
			seen[next] = true

			coloredTargets[body] = true
			if faceIndex < 0 {
				out.Bodies[index].SurfaceColor = &c
			} else {
				out.Bodies[index].FaceColors[faceIndex] = c
			}
		}
	}
	for item, parent := range overrides {
		target, ok := faceTargets[styleTargets[item]]
		if !ok || !styled[parent] || styleTargets[parent] != ids[target[0]] {
			return fail()
		}
		if _, nested := overrides[parent]; nested {
			return fail()
		}
		if out.Bodies[target[0]].SurfaceColor == nil {
			return fail()
		}
	}
	for id, ent := range f.Entities {
		for _, r := range ent.Records {
			if (strings.Contains(r.Type, "STYLE") || strings.Contains(r.Type, "STYLED") || strings.Contains(r.Type, "COLOUR") || strings.Contains(r.Type, "INVISIBILITY") || (strings.HasPrefix(r.Type, "PRESENTATION_") || r.Type == "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION")) && !seen[id] {
				return fail()
			}
		}
	}
	return out, nil
}
func colorBodyIDs(f *File) ([]int64, map[int64]int64, error) {
	var ids []int64
	contexts := map[int64]int64{}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 {
			continue
		}
		r := ent.Records[0]
		if r.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" && r.Type != "FACETED_BREP_SHAPE_REPRESENTATION" {
			continue
		}
		if len(r.Args) != 3 {
			return nil, nil, errors.New("step: malformed colored representation")
		}
		ctx, e := ref(r.Args[2])
		if e != nil {
			return nil, nil, e
		}
		items, e := listRefs(r.Args[1], 1)
		if e != nil {
			return nil, nil, e
		}
		for _, bid := range items {
			ent := f.Entities[bid]
			if len(ent.Records) == 1 && ent.Records[0].Type == "AXIS2_PLACEMENT_3D" {
				continue
			}
			ids = append(ids, bid)
			contexts[bid] = ctx
		}
	}
	return ids, contexts, nil
}

// WriteColoredPlanar emits planar advanced BRep geometry with AP214 whole-body
// surface fill styles. Nil colors remain unspecified, including in mixed models.
func WriteColoredPlanar(w io.Writer, m ColoredModel) error {
	full := FaceColoredModel{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		full.Bodies = append(full.Bodies, FaceColoredBody{Body: b.Body, SurfaceColor: b.SurfaceColor})
	}
	return WriteFaceColoredPlanar(w, full)
}

// WriteFaceColoredPlanar emits body defaults and deterministic per-face overrides.
func WriteFaceColoredPlanar(w io.Writer, m FaceColoredModel) error {
	if w == nil {
		return errors.New("step: nil color writer")
	}
	geometry := Model{LengthUnit: m.LengthUnit}
	for _, b := range m.Bodies {
		if b.SurfaceColor != nil && !validRGB(*b.SurfaceColor) {
			return errors.New("step: invalid RGB components")
		}
		for index, c := range b.FaceColors {
			if index < 0 || index >= len(b.Body.Solid.Faces()) || !validRGB(c) {
				return errors.New("step: invalid face color or index")
			}
		}
		geometry.Bodies = append(geometry.Bodies, b.Body)
	}
	var raw bytes.Buffer
	if e := WriteAdvanced(&raw, geometry); e != nil {
		return e
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		return e
	}
	ids, contexts, e := colorBodyIDs(f)
	if e != nil || len(ids) != len(m.Bodies) {
		return errors.New("step: colored body graph mismatch")
	}
	var targets []styleTarget
	for i, b := range m.Bodies {
		if b.SurfaceColor != nil {
			targets = append(targets, styleTarget{ids[i], contexts[ids[i]], *b.SurfaceColor})
		}
		if len(b.FaceColors) > 0 {
			faces, e := bodyFaceIDs(f, ids[i])
			if e != nil {
				return e
			}
			for index, fid := range faces {
				if c, ok := b.FaceColors[index]; ok {
					targets = append(targets, styleTarget{fid, contexts[ids[i]], c})
				}
			}
		}
	}
	return appendSurfaceStyles(w, raw.Bytes(), f, targets)
}

type styleTarget struct {
	id, ctx int64
	color   RGB
}

func appendSurfaceStyles(w io.Writer, raw []byte, f *File, targets []styleTarget) error {
	var extra strings.Builder
	next := int64(1)
	for id := range f.Entities {
		if id >= next {
			next = id + 1
		}
	}
	emit := func(s string) int64 { id := next; next++; fmt.Fprintf(&extra, "#%d=%s;\n", id, s); return id }
	for _, target := range targets {
		c := target.color
		color := emit(fmt.Sprintf("COLOUR_RGB('',%s,%s,%s)", real(c.R), real(c.G), real(c.B)))
		fillColor := emit(fmt.Sprintf("FILL_AREA_STYLE_COLOUR('',#%d)", color))
		fill := emit(fmt.Sprintf("FILL_AREA_STYLE('',(#%d))", fillColor))
		surface := emit(fmt.Sprintf("SURFACE_STYLE_FILL_AREA(#%d)", fill))
		side := emit(fmt.Sprintf("SURFACE_SIDE_STYLE('',(#%d))", surface))
		usage := emit(fmt.Sprintf("SURFACE_STYLE_USAGE(.BOTH.,#%d)", side))
		assignment := emit(fmt.Sprintf("PRESENTATION_STYLE_ASSIGNMENT((#%d))", usage))
		styled := emit(fmt.Sprintf("STYLED_ITEM('color',(#%d),#%d)", assignment, target.id))
		emit(fmt.Sprintf("MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION('',(#%d),#%d)", styled, target.ctx))
	}
	s := string(raw)
	pos := strings.LastIndex(s, "ENDSEC;")
	if pos < 0 {
		return errors.New("step: missing DATA terminator")
	}
	s = s[:pos] + extra.String() + s[pos:]
	s = strings.ReplaceAll(s, "CONFIG_CONTROL_DESIGN", "AUTOMOTIVE_DESIGN")
	s = strings.ReplaceAll(s, "'config_control_design',1994", "'automotive_design',2000")
	n, e := io.WriteString(w, s)
	if e == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return e
}

// The standard draughting palette has exact RGB corner values. Names are
// case-sensitive; unknown names and other predefined-colour subtypes are not
// interpreted as vendor aliases.
func readSurfaceStyleColour(f *File, id int64) (RGB, error) {
	fail := func() (RGB, error) { return RGB{}, errors.New("step: unsupported surface colour") }
	if r, e := simple(f, id, "DRAUGHTING_PRE_DEFINED_COLOUR", 1); e == nil {
		name, e := textValue(r.Args[0])
		if e != nil {
			return fail()
		}
		palette := map[string]RGB{"black": {0, 0, 0}, "white": {1, 1, 1}, "red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0}, "magenta": {1, 0, 1}, "cyan": {0, 1, 1}}
		c, ok := palette[name]
		if !ok {
			return fail()
		}
		return c, nil
	}
	r, e := simple(f, id, "COLOUR_RGB", 4)
	if e != nil {
		return fail()
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return fail()
	}
	c := RGB{}
	values := []*float64{&c.R, &c.G, &c.B}
	for i, p := range values {
		*p, e = numeric(r.Args[i+1])
		if e != nil {
			return fail()
		}
	}
	if !validRGB(c) {
		return fail()
	}
	return c, nil
}
