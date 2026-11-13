package step

import (
	"bytes"
	"io"
	"plainsolid/geom"
)

// LensStyle contains a product-local body fill and upper/lower face overrides.
// It excludes occurrence overrides, transparency, textures and material lighting.
type LensStyle struct {
	SurfaceColor *RGB
	FaceColors   [2]*RGB
}

func hasLensStyle(s LensStyle) bool {
	return s.SurfaceColor != nil || s.FaceColors[0] != nil || s.FaceColors[1] != nil
}
func readLensStyles(f *File, bodies map[int64]SplineLensBody, unit float64) (map[int64]LensStyle, error) {
	ids, contexts, e := colorBodyIDs(f)
	if e != nil {
		return nil, e
	}
	if len(ids) != len(bodies) {
		return nil, geom.ErrInvalid
	}
	counts := make([]int, len(ids))
	metadata := Model{LengthUnit: unit, Bodies: make([]Body, len(ids))}
	for i, id := range ids {
		if _, ok := bodies[id]; !ok {
			return nil, geom.ErrInvalid
		}
		counts[i] = 2
	}
	styles, e := readStylesForBodyCounts(f, metadata, ids, contexts, true, counts)
	if e != nil {
		return nil, e
	}
	out := map[int64]LensStyle{}
	for i, id := range ids {
		faces, e := bodyFaceIDs(f, id)
		if e != nil || len(faces) != 2 {
			return nil, geom.ErrInvalid
		}
		style := LensStyle{SurfaceColor: styles.Bodies[i].SurfaceColor}
		for j, fid := range faces {
			if c, ok := styles.Bodies[i].FaceColors[j]; ok {
				role := -1
				for k, want := range bodies[id].SourceFaceIDs {
					if fid == want {
						role = k
					}
				}
				if role < 0 {
					return nil, geom.ErrInvalid
				}
				color := c
				style.FaceColors[role] = &color
			}
		}
		out[id] = style
	}
	return out, nil
}
func appendLensStyles(w io.Writer, raw []byte, f *File, bodies map[int64]SplineLensBody, styles map[int64]LensStyle) error {
	_, contexts, e := colorBodyIDs(f)
	if e != nil {
		return e
	}
	targets := []styleTarget{}
	for _, id := range f.Order {
		style, ok := styles[id]
		if !ok {
			continue
		}
		body, ok := bodies[id]
		if !ok {
			return geom.ErrInvalid
		}
		if style.SurfaceColor != nil {
			if !validRGB(*style.SurfaceColor) {
				return geom.ErrInvalid
			}
			targets = append(targets, styleTarget{id, contexts[id], *style.SurfaceColor})
		}
		for role, c := range style.FaceColors {
			if c != nil {
				if !validRGB(*c) {
					return geom.ErrInvalid
				}
				targets = append(targets, styleTarget{body.SourceFaceIDs[role], contexts[id], *c})
			}
		}
	}
	if len(bodies) != len(styles) {
		return geom.ErrInvalid
	}
	var staged bytes.Buffer
	if e = appendSurfaceStyles(&staged, raw, f, targets); e != nil {
		return e
	}
	parsed, e := Parse(bytes.NewReader(staged.Bytes()), Limits{})
	if e != nil {
		return e
	}
	if _, e = readLensStyles(parsed, bodies, 0); e != nil {
		return e
	}
	n, e := w.Write(staged.Bytes())
	if e == nil && n != staged.Len() {
		e = io.ErrShortWrite
	}
	return e
}
