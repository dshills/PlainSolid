package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// WrappedCylinderBoreFilletProof describes two independent cylinder axes and
// the exact rolling radii, cap stations, and face owners of a rounded bore.
// A sharp end has radius zero and blend face -1.
type WrappedCylinderBoreFilletProof struct {
	Stock, Hole  geom.WrappedCylinderEmbedding
	Fillets      [2]*big.Rat
	Outer, Bore  int
	Caps, Blends [2]int
	Complete     bool
}

// ValidateWrappedCylinderBoreRimFillet authenticates the whole rounded bore
// on an existing solid, including after native replay. It proves exact
// toroidal patches, cap containment, and both owners of every model edge.
func (s Solid) ValidateWrappedCylinderBoreRimFillet(ctx context.Context, o geom.SurfacePatchOptions) (WrappedCylinderBoreFilletProof, error) {
	if ctx == nil {
		return WrappedCylinderBoreFilletProof{}, geom.ErrInvalid
	}
	t := s.Topology()
	if len(t.Shells) != 1 || t.EulerCharacteristic != 0 || t.InnerWires != 2 {
		return WrappedCylinderBoreFilletProof{}, geom.ErrConvergence
	}
	p, err := certifyWrappedCylinderBoreFilletFaces(ctx, s.faces, o)
	if err != nil {
		return WrappedCylinderBoreFilletProof{}, err
	}
	agreement, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if err != nil {
		return WrappedCylinderBoreFilletProof{}, err
	}
	if !agreement.Complete {
		return WrappedCylinderBoreFilletProof{}, geom.ErrConvergence
	}
	for _, use := range agreement.Uses {
		if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
			return WrappedCylinderBoreFilletProof{}, geom.ErrConvergence
		}
	}
	p.Complete = true
	return p, nil
}

func certifyWrappedCylinderBoreFilletFaces(ctx context.Context, faces []Face, o geom.SurfacePatchOptions) (WrappedCylinderBoreFilletProof, error) {
	fail := func(err error) (WrappedCylinderBoreFilletProof, error) { return WrappedCylinderBoreFilletProof{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(faces) < 5 || len(faces) > 6 {
		return fail(geom.ErrConvergence)
	}
	p := WrappedCylinderBoreFilletProof{Outer: -1, Bore: -1, Caps: [2]int{-1, -1}, Blends: [2]int{-1, -1}}
	var caps [2]circularSeedCap
	for id, f := range faces {
		switch w := f.Surface.(type) {
		case geom.PlaneSurface:
			end := 1
			if f.Reversed {
				end = 0
			}
			if p.Caps[end] >= 0 {
				return fail(geom.ErrConvergence)
			}
			cap, ok, err := certifyCircularAnnularSeedCap(ctx, f)
			if err != nil {
				return fail(err)
			}
			if !ok {
				return fail(geom.ErrConvergence)
			}
			p.Caps[end], caps[end] = id, cap
		case *geom.WrappedSurfacePatch:
			if _, full := straightFullWrappedRectangle(w, f); !full || len(f.Outer) != 10 || !singleWrappedBoreSeam(f) {
				return fail(geom.ErrConvergence)
			}
			cylinder, err := geom.CertifyWrappedCylinderEmbedding(ctx, w, o)
			if err == nil && cylinder.Circular {
				if f.Reversed && p.Bore < 0 {
					p.Bore, p.Hole = id, cylinder
				} else if !f.Reversed && p.Outer < 0 {
					p.Outer, p.Stock = id, cylinder
				} else {
					return fail(geom.ErrConvergence)
				}
				continue
			}
			if err != nil && !errors.Is(err, geom.ErrConvergence) {
				return fail(err)
			}
			end, ok := circularBoreBlendEnd(w, f)
			if !ok || p.Blends[end] >= 0 {
				return fail(geom.ErrConvergence)
			}
			p.Blends[end] = id
		default:
			return fail(geom.ErrConvergence)
		}
	}
	if p.Outer < 0 || p.Bore < 0 || p.Caps[0] < 0 || p.Caps[1] < 0 || p.Blends == [2]int{-1, -1} {
		return fail(geom.ErrConvergence)
	}
	z := [2]*big.Rat{p.Hole.LowerZ, p.Hole.UpperZ}
	for end, outerZ := range [2]*big.Rat{p.Stock.LowerZ, p.Stock.UpperZ} {
		cap := caps[end]
		if cap.height.Cmp(outerZ) != 0 || cap.radii[0].Cmp(p.Stock.Radius) != 0 ||
			cap.centers[0][0].Cmp(p.Stock.CenterX) != 0 || cap.centers[0][1].Cmp(p.Stock.CenterY) != 0 ||
			cap.centers[1][0].Cmp(p.Hole.CenterX) != 0 || cap.centers[1][1].Cmp(p.Hole.CenterY) != 0 {
			return fail(geom.ErrConvergence)
		}
		radius := new(big.Rat).Sub(z[end], outerZ)
		if end == 1 {
			radius.Neg(radius)
		}
		if radius.Sign() < 0 || new(big.Rat).Add(p.Hole.Radius, radius).Cmp(cap.radii[1]) != 0 ||
			(radius.Sign() == 0) != (p.Blends[end] < 0) {
			return fail(geom.ErrConvergence)
		}
		p.Fillets[end] = radius
		if radius.Sign() == 0 {
			continue
		}
		want, err := wrappedCylinderRimBlendPatch(ctx, [3]*big.Rat{p.Hole.CenterX, p.Hole.CenterY, z[end]}, cap.radii[1], radius, end, true, o)
		if err != nil {
			return fail(err)
		}
		have := faces[p.Blends[end]].Surface.(*geom.WrappedSurfacePatch)
		same, err := geom.CompareExactSurfaces(ctx, have.FundamentalPatch(), want, geom.ExactSurfaceComparisonOptions{
			MaxCells: 4, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
		})
		if err != nil {
			return fail(err)
		}
		if !same.Complete || !same.Equal {
			return fail(geom.ErrConvergence)
		}
	}
	// The original constant-radius stock has the same verified cap radius
	// throughout. Each proved quarter-circle stays within the radius of its
	// opening; both larger opening disks already passed exact containment.
	return p, nil
}

func circularBoreBlendEnd(w *geom.WrappedSurfacePatch, f Face) (int, bool) {
	u, v := w.FundamentalPatch().ControlShape()
	d := w.Domain()
	if u != 9 || v != 3 || f.Reversed || w.SeamCertificate().PositionMatched != [2]bool{true, false} ||
		d.UMin != 0 || d.UMax != 4 || d.VMax != d.VMin+1 || (d.VMin != 1 && d.VMin != 2) {
		return 0, false
	}
	return 2 - int(d.VMin), true
}

func (s Solid) hasCircularBoreRimBlend() bool {
	if len(s.faces) < 5 || len(s.faces) > 6 {
		return false
	}
	for _, f := range s.faces {
		if w, ok := f.Surface.(*geom.WrappedSurfacePatch); ok {
			if _, blend := circularBoreBlendEnd(w, f); blend {
				return true
			}
		}
	}
	return false
}

func singleWrappedBoreSeam(f Face) bool {
	seam, count := -1, 0
	for _, u := range f.Outer {
		if u.UVStart.X != u.UVEnd.X {
			continue
		}
		if count == 0 {
			seam = u.Edge
		}
		if u.Edge != seam {
			return false
		}
		count++
	}
	return count == 2
}
