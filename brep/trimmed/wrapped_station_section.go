//go:build research

package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// WrappedStationSectionLoop describes an exact rational section and its
// original wall owners. At an interior station both incident wall faces are
// reported. Its exact rational curve retains one original chart's U basis.
type WrappedStationSectionLoop struct {
	Section geom.WrappedFrustumSection
	Faces   []int
	Hole    bool
}

// WrappedStationSection is a horizontal cross-section of certified
// material. Cap is -1 for an interior section, or the coincident cap face.
type WrappedStationSection struct {
	Outer WrappedStationSectionLoop
	Inner *WrappedStationSectionLoop
	Cap   int
}

// SectionWrappedCircularStationLoft extracts the closed horizontal section
// at a represented height and retains both owners at an exact crease rim.
func (s Solid) SectionWrappedCircularStationLoft(ctx context.Context, height float64,
	o geom.SurfacePatchOptions) (WrappedStationSection, error) {
	r := WrappedStationSection{Cap: -1}
	if !finite(height) {
		return r, geom.ErrInvalid
	}
	p, e := s.ValidateWrappedCircularStationLoft(ctx, o)
	if e != nil {
		return r, e
	}
	r.Outer, e = s.sectionWrappedStationWalls(ctx, p, p.History.WallFaces, ratInset(height), o)
	if e != nil {
		return WrappedStationSection{Cap: -1}, e
	}
	r.Cap = sectionStationCap(r.Outer.Section.Cap, p.History.CapFaces)
	return r, nil
}

// SectionWrappedRationalStationLoftRational extracts the original exact
// weighted section at a rational height, including nonbinary heights.
// An interior station retains both original wall owners and no cap.
func (s Solid) SectionWrappedRationalStationLoftRational(ctx context.Context, height *big.Rat,
	o geom.SurfacePatchOptions) (WrappedStationSection, error) {
	empty := WrappedStationSection{Cap: -1}
	if err := checkStationSectionHeight(ctx, height, o); err != nil {
		return empty, err
	}
	height = new(big.Rat).Set(height)
	p, _, err := s.validateWrappedProfileStationLoft(ctx, o, false)
	if err != nil {
		return empty, err
	}
	section, err := s.sectionWrappedStationWalls(ctx, p, p.History.WallFaces, height, o)
	if err != nil {
		return empty, err
	}
	return WrappedStationSection{Outer: section, Cap: sectionStationCap(section.Section.Cap, p.History.CapFaces)}, nil
}

// SectionWrappedCircularStationBore extracts both outer and inner exact
// section curves, with independent crease ancestry on each wall family.
func (s Solid) SectionWrappedCircularStationBore(ctx context.Context, height float64,
	o geom.SurfacePatchOptions) (WrappedStationSection, error) {
	if !finite(height) {
		return WrappedStationSection{Cap: -1}, geom.ErrInvalid
	}
	return s.sectionWrappedStationBore(ctx, ratInset(height), o, true)
}

// SectionWrappedRationalStationBoreRational returns both original weighted
// section loops. Inner is the hole, not a second solid. A terminal section
// names the original annular cap, and internal station rims name both walls.
func (s Solid) SectionWrappedRationalStationBoreRational(ctx context.Context, height *big.Rat,
	o geom.SurfacePatchOptions) (WrappedStationSection, error) {
	if err := checkStationSectionHeight(ctx, height, o); err != nil {
		return WrappedStationSection{Cap: -1}, err
	}
	return s.sectionWrappedStationBore(ctx, new(big.Rat).Set(height), o, false)
}

func checkStationSectionHeight(ctx context.Context, z *big.Rat, o geom.SurfacePatchOptions) error {
	if ctx == nil || z == nil {
		return geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if bits < 64 || bits > 65536 {
		return geom.ErrInvalid
	}
	if z.Num().BitLen() > bits || z.Denom().BitLen() > bits {
		return geom.ErrConvergence
	}
	return nil
}

func (s Solid) sectionWrappedStationBore(ctx context.Context, z *big.Rat,
	o geom.SurfacePatchOptions, circular bool) (WrappedStationSection, error) {
	r := WrappedStationSection{Cap: -1}
	p, e := s.validateWrappedProfileStationBore(ctx, o, circular)
	if e != nil {
		return r, e
	}
	r.Outer, e = s.sectionWrappedStationWalls(ctx, p.Outer, p.OuterWalls, z, o)
	if e != nil {
		return WrappedStationSection{Cap: -1}, e
	}
	q, e := s.sectionWrappedStationWalls(ctx, p.Inner, p.InnerWalls, z, o)
	if e != nil {
		return WrappedStationSection{Cap: -1}, e
	}
	q.Hole = true
	r.Inner = &q
	r.Cap = sectionStationCap(r.Outer.Section.Cap, p.Caps)
	return r, nil
}

func sectionStationCap(cap geom.CylinderBoundaryPart, faces [2]int) int {
	switch cap {
	case geom.CylinderLowerCap:
		return faces[0]
	case geom.CylinderUpperCap:
		return faces[1]
	}
	return -1
}

func (s Solid) sectionWrappedStationWalls(ctx context.Context, p WrappedStationLoftProof, ids []int, z *big.Rat,
	o geom.SurfacePatchOptions) (WrappedStationSectionLoop, error) {
	var empty WrappedStationSectionLoop
	j := sort.Search(len(p.Heights), func(i int) bool { return p.Heights[i].Cmp(z) >= 0 })
	if j == len(p.Heights) || j == 0 && p.Heights[0].Cmp(z) != 0 {
		return empty, geom.ErrDomain
	}
	i := max(j-1, 0)
	w, ok := s.faces[ids[i]].Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return empty, geom.ErrConvergence
	}
	q, e := geom.SectionWrappedFrustumRational(ctx, w, z, o)
	if e != nil {
		return empty, e
	}
	faces := []int{ids[i]}
	if j > 0 && j < len(ids) && p.Heights[j].Cmp(z) == 0 {
		faces = append(faces, ids[j])
		sort.Ints(faces)
		q.Cap = 0 // the band's upper disk is not part of the loft boundary
	}
	return WrappedStationSectionLoop{Section: q, Faces: faces}, nil
}
