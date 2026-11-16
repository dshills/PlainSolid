package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BuildProjectiveSplineRoofOverlapCommon intersects two original
// projective graph solids above the same planar base. Only their exact
// rectangular projected overlap is material. Inside that footprint the
// heights must have a strict order, coincide, or admit one complete open
// analytic contour. An empty overlap has no regularized material.
//
// The original support evaluators are retained. Walls along a new
// clipping edge have Source.OriginalEdge and OriginalUse equal to -1.
func BuildProjectiveSplineRoofOverlapCommon(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r ProjectiveRoofSolid, err error) {
	p, err := proveProjectiveRoofFootprints(ctx, a, b, axes, baseHeight, o)
	if err != nil {
		return r, err
	}
	if p.overlap.NoArea {
		return emptyProjectiveRoofBoolean(), nil
	}
	faces, ledger, err := p.clip()
	if err != nil {
		return r, err
	}
	q, err := buildProjectiveSplineRoofBooleanOnEqualFootprints(ctx, faces[0], faces[1], axes, baseHeight, parametric.Intersection, o)
	if err != nil {
		return r, err
	}
	if err := restoreProjectiveRoofCoedges(ctx, &q, ledger); err != nil {
		return r, err
	}
	return q, nil
}

type projectiveRoofFootprintProof struct {
	owners   [2]Face
	supports [2]*geom.SurfacePatch
	overlap  geom.ProjectiveSurfaceGraphOverlap
}

func proveProjectiveRoofFootprints(ctx context.Context, a, b Face, axes [2]int, baseHeight float64,
	o ProjectiveRoofSolidOptions) (r projectiveRoofFootprintProof, err error) {
	if ctx == nil || !finite(baseHeight) || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 ||
		axes[0] == axes[1] || o.Sew.RepairOrientation {
		return r, geom.ErrInvalid
	}
	height := 3 - axes[0] - axes[1]
	owners := [2]Face{a, b}
	var supports [2]*geom.SurfacePatch
	for i, f := range owners {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		if len(f.Holes) != 0 {
			return r, geom.ErrConvergence
		}
		s, err := projectiveRoofSupportAboveBase(f, height, baseHeight, o.Build)
		if err != nil {
			return r, err
		}
		supports[i] = s
	}
	proj, err := geom.CertifyProjectiveSurfaceGraphOverlap(ctx, supports[0], supports[1], axes, o.Envelope.Imprint.Contacts.Patches)
	if err != nil && (!errors.Is(err, geom.ErrConvergence) || proj.Areas[1].V.Upper == nil) {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	parity := 1
	if (axes[0]+1)%3 != axes[1] {
		parity = -1
	}
	for i, f := range owners {
		sign := proj.ProjectionOrientation[i]
		if f.Reversed {
			sign = -sign
		}
		if sign != parity {
			return r, geom.ErrConvergence
		}
	}
	return projectiveRoofFootprintProof{owners, supports, proj}, nil
}

func (p projectiveRoofFootprintProof) clip() ([2]Face, [2][4]ProjectiveSplineImprintUse, error) {
	var faces [2]Face
	var ledger [2][4]ProjectiveSplineImprintUse
	for i := range p.owners {
		f, record, err := projectiveClipGraphFace(p.owners[i], p.supports[i].Domain(), p.overlap.Areas[i])
		if err != nil {
			return [2]Face{}, [2][4]ProjectiveSplineImprintUse{}, err
		}
		faces[i], ledger[i] = f, record
	}
	return faces, ledger, nil
}

func restoreProjectiveRoofCoedges(ctx context.Context, q *ProjectiveRoofSolid, ledger [2][4]ProjectiveSplineImprintUse) error {
	restore := func(owner int, clip ProjectiveSplineImprintUse) (ProjectiveSplineImprintUse, error) {
		if owner < 0 || owner >= len(ledger) || clip.OriginalUse < 0 || clip.OriginalUse >= 4 ||
			clip.OriginalRange[0] == nil || clip.OriginalRange[1] == nil {
			return ProjectiveSplineImprintUse{}, geom.ErrConvergence
		}
		old := ledger[owner][clip.OriginalUse]
		if old.OriginalUse < 0 {
			old.OriginalRange = clip.OriginalRange
		} else {
			w := new(big.Rat).Sub(old.OriginalRange[1], old.OriginalRange[0])
			x := old.OriginalRange[0]
			for j, t := range clip.OriginalRange {
				old.OriginalRange[j] = new(big.Rat).Add(x, new(big.Rat).Mul(w, t))
			}
		}
		return old, nil
	}
	for i, wall := range q.Walls {
		updated, err := restore(wall.Owner, wall.Source)
		if err != nil {
			return err
		}
		q.Walls[i].Source = updated
	}
	for i, floor := range q.FloorSources {
		updated, err := restore(1, floor)
		if err != nil {
			return err
		}
		q.FloorSources[i] = updated
	}
	return ctx.Err()
}

func projectiveClipGraphFace(f Face, domain geom.ParameterRectangle, wanted geom.ExactSurfaceRectangle) (
	out Face, ledger [4]ProjectiveSplineImprintUse, err error) {
	limits := [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
	var clips [2][2]float64
	for i, w := range [2]geom.ExactParameterInterval{wanted.U, wanted.V} {
		for j, q := range [2]*big.Rat{w.Lower, w.Upper} {
			x, err := geom.RepresentedExactScalar(q)
			if err != nil {
				return out, ledger, err
			}
			clips[i][j] = x
		}
		if clips[i][0] < limits[i][0] || clips[i][1] > limits[i][1] || clips[i][0] >= clips[i][1] {
			return out, ledger, geom.ErrConvergence
		}
	}
	out = Face{Surface: f.Surface, Reversed: f.Reversed}
	for i, u := range f.Outer {
		p := [2][2]float64{{u.UVStart.X, u.UVStart.Y}, {u.UVEnd.X, u.UVEnd.Y}}
		var t [2][2]float64
		for j, at := range p {
			for j2, x := range at {
				switch x {
				case limits[j2][0]:
					t[j][j2] = clips[j2][0]
				case limits[j2][1]:
					t[j][j2] = clips[j2][1]
				default:
					return Face{}, [4]ProjectiveSplineImprintUse{}, geom.ErrConvergence
				}
			}
		}
		fixed := 0
		if p[0][fixed] != p[1][fixed] {
			fixed = 1
		}
		if t[0][fixed] != p[0][fixed] {
			ledger[i] = ProjectiveSplineImprintUse{OriginalUse: -1, OriginalEdge: -1}
		} else {
			free := 1 - fixed
			x := new(big.Rat).SetFloat64(p[0][free])
			w := new(big.Rat).Sub(new(big.Rat).SetFloat64(p[1][free]), x)
			s := ProjectiveSplineImprintUse{OriginalUse: i, OriginalEdge: u.Edge}
			for j := range t {
				s.OriginalRange[j] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(t[j][free]), x), w)
			}
			ledger[i] = s
		}
		out.Outer = append(out.Outer, Use{Edge: i, UVStart: geom.Vec2{X: t[0][0], Y: t[0][1]}, UVEnd: geom.Vec2{X: t[1][0], Y: t[1][1]}})
	}
	return out, ledger, nil
}
