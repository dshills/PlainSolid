package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Bounds uses the outward operand enclosures and combines
// them conservatively for the Boolean operation. Empty is true only for
// the proved empty Boolean; its returned box is zero. This need not be the
// tight bounds of the retained face fragments.
func (s AlgebraicShellBoolean) Bounds() (box geom.Box, empty bool, err error) {
	if s.source == nil {
		return box, false, geom.ErrInvalid
	}
	if len(s.faces) == 0 {
		return box, true, nil
	}
	a, b := s.source.bounds[0], s.source.bounds[1]
	switch s.op {
	case parametric.Union:
		if len(s.source.a.faces) == 0 {
			return b, false, nil
		}
		if len(s.source.b.faces) == 0 {
			return a, false, nil
		}
		box, err = a.Union(b)
		return box, false, err
	case parametric.Intersection:
		if box, overlap := a.Intersect(b); overlap {
			return box, false, nil
		}
		return geom.Box{}, false, geom.ErrConvergence
	default:
		return a, false, nil
	}
}

// BoundRetainedGeometry encloses the full original supports of all retained
// result faces, restricted to their original UV trim rectangles, and then
// intersects that enclosure with Bounds. No unretained source face can
// contribute. Branch-trimmed fragment bounds may still be tighter.
func (s AlgebraicShellBoolean) BoundRetainedGeometry(ctx context.Context, o geom.SurfacePatchOptions) (box geom.Box, empty bool, err error) {
	if ctx == nil || s.source == nil {
		return box, false, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return box, false, e
	}
	b, empty, e := s.Bounds()
	if e != nil || empty {
		return box, empty, e
	}
	type key struct{ Operand, Face int }
	seen := map[key]bool{}
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return geom.Box{}, false, e
		}
		k := key{f.Operand, f.Original}
		if seen[k] {
			continue
		}
		q, e := s.OriginalFace(i)
		if e != nil {
			return geom.Box{}, false, e
		}
		x, e := exactFaceSupportPatch(q, o)
		if e != nil {
			return geom.Box{}, false, e
		}
		d, e := faceUVBounds(q)
		if e != nil {
			return geom.Box{}, false, e
		}
		x, e = x.Trim(d)
		if e != nil {
			return geom.Box{}, false, e
		}
		bounds, e := x.Bounds()
		if e != nil {
			return geom.Box{}, false, e
		}
		if len(seen) == 0 {
			box = bounds
		} else {
			box, e = box.Union(bounds)
			if e != nil {
				return geom.Box{}, false, e
			}
		}
		seen[k] = true
	}
	if q, ok := b.Intersect(box); ok {
		return q, false, nil
	}
	return geom.Box{}, false, geom.ErrConvergence
}
