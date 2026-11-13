package step

import (
	"errors"
	"io"
)

// FaceColoredBody stores a body-wide default and optional face overrides.
// Face indices refer to Body.Solid.Faces(), preserving STEP CLOSED_SHELL order.
type FaceColoredBody struct {
	Body         Body
	SurfaceColor *RGB
	FaceColors   map[int]RGB
}
type FaceColoredModel struct {
	Bodies     []FaceColoredBody
	LengthUnit float64
}

// ReadFaceColoredPlanar reads both-sided RGB fills on whole planar bodies and
// individual ADVANCED_FACE items. Shared-face ancestry and duplicate overrides
// are rejected. An absent override inherits SurfaceColor, which may be nil.
func ReadFaceColoredPlanar(r io.Reader, l Limits) (FaceColoredModel, error) {
	return readFaceColors(r, l, true)
}
func bodyFaceIDs(f *File, id int64) ([]int64, error) {
	b, e := simple(f, id, "MANIFOLD_SOLID_BREP", 2)
	var voids []int64
	if e != nil {
		b, e = simple(f, id, "BREP_WITH_VOIDS", 3)
		if e != nil {
			return nil, errors.New("step: face colors require explicit advanced bodies")
		}
		refs, err := listRefs(b.Args[2], 1)
		if err != nil {
			return nil, err
		}
		for _, v := range refs {
			r, err := simple(f, v, "ORIENTED_CLOSED_SHELL", 4)
			if err != nil {
				return nil, err
			}
			sense, err := boolean(r.Args[3])
			if err != nil || sense || r.Args[1].Kind != Derived {
				return nil, errors.New("step: invalid styled void shell")
			}
			shell, err := ref(r.Args[2])
			if err != nil {
				return nil, err
			}
			voids = append(voids, shell)
		}
	}
	shell, e := ref(b.Args[1])
	if e != nil {
		return nil, e
	}
	s, e := simple(f, shell, "CLOSED_SHELL", 2)
	if e != nil {
		return nil, e
	}
	ids, e := listRefs(s.Args[1], 1)
	if e != nil {
		return nil, e
	}
	for _, v := range voids {
		inner, err := simple(f, v, "CLOSED_SHELL", 2)
		if err != nil {
			return nil, err
		}
		faces, err := listRefs(inner.Args[1], 1)
		if err != nil {
			return nil, err
		}
		ids = append(ids, faces...)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, errors.New("step: duplicate styled face")
		}
		seen[id] = true
		if _, e := simple(f, id, "ADVANCED_FACE", 4); e != nil {
			return nil, e
		}
	}
	return ids, nil
}
