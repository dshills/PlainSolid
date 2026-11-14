package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// WrappedCylinderTubeHistory maps canonical source faces and edges to the
// retained tube. Both cap rows on Inner identify the same annular cap as
// their Outer counterpart. The inner lateral face is reversed.
type WrappedCylinderTubeHistory struct {
	OuterFaces, InnerFaces [3]int
	OuterEdges, InnerEdges [9]int
}

// NewWrappedCylinderTube constructs a through-bore between two concentric,
// homothetic canonical rational cylinder supports on the same height
// interval. Their normalized profiles must coincide exactly, or both must
// be proven circles. The inner profile is strictly smaller. The result has
// both original lateral supports, two annular cap faces and two exact seams.
func NewWrappedCylinderTube(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedCylinderTubeHistory, error) {
	return newWrappedCylinderBore(ctx, outer, inner, tolerance, o, true)
}

// NewWrappedCylinderBore subtracts an inner coaxial extrusion whose exact
// height interval is contained in the outer interval. A common endpoint forms
// an open annular cap; a strict endpoint retains an inward bore floor. Thus
// the result may be a through-bore, a blind bore or a closed internal cavity.
// Both supports remain exact. The input profiles must be strictly concentric
// and homothetic, or proven circles with the same center.
func NewWrappedCylinderBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedCylinderTubeHistory, error) {
	return newWrappedCylinderBore(ctx, outer, inner, tolerance, o, false)
}

func newWrappedCylinderBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions, requireThrough bool) (Solid, WrappedCylinderTubeHistory, error) {
	var history WrappedCylinderTubeHistory
	a, pa, e := NewWrappedCylinder(ctx, outer, tolerance, o)
	if e != nil {
		return Solid{}, history, e
	}
	b, pb, e := NewWrappedCylinder(ctx, inner, tolerance, o)
	if e != nil {
		return Solid{}, history, e
	}
	lowOpen, highOpen := pa.LowerZ.Cmp(pb.LowerZ) == 0, pa.UpperZ.Cmp(pb.UpperZ) == 0
	if !sameCylinderAxis(pa, pb) || pa.Radius.Cmp(pb.Radius) <= 0 || pa.LowerZ.Cmp(pb.LowerZ) > 0 ||
		pa.UpperZ.Cmp(pb.UpperZ) < 0 || requireThrough && (!lowOpen || !highOpen) {
		return Solid{}, history, geom.ErrConvergence
	}
	same, e := sameNormalizedCylinderProfile(ctx, outer, inner, pa, pb, o)
	if e != nil || !same {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, history, e
	}
	return assembleWrappedFrustumBore(ctx, a, b, lowOpen, highOpen, tolerance, o)
}

// NewWrappedCircularFrustumBore subtracts an exactly contained coaxial
// circular frustum. Its radii must be strictly smaller than the outer radii
// everywhere in the inner height interval. A shared cap plane opens the
// corresponding end; strict heights keep the inner cap as a bore floor.
// Both original lateral supports and the original retained rims are kept.
func NewWrappedCircularFrustumBore(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedCylinderTubeHistory, error) {
	var empty WrappedCylinderTubeHistory
	stock, a, e := NewWrappedFrustum(ctx, outer, tolerance, o)
	if e != nil {
		return Solid{}, empty, e
	}
	tool, b, e := NewWrappedFrustum(ctx, inner, tolerance, o)
	if e != nil {
		return Solid{}, empty, e
	}
	if !strictWrappedCircularFrustumBore(a, b) {
		return Solid{}, empty, geom.ErrConvergence
	}
	return assembleWrappedFrustumBore(ctx, stock, tool, a.LowerZ.Cmp(b.LowerZ) == 0, a.UpperZ.Cmp(b.UpperZ) == 0, tolerance, o)
}

func strictWrappedCircularFrustumBore(a, b geom.WrappedFrustumEmbedding) bool {
	if !a.Circular || !b.Circular || a.CenterX.Cmp(b.CenterX) != 0 || a.CenterY.Cmp(b.CenterY) != 0 ||
		a.LowerZ.Cmp(b.LowerZ) > 0 || a.UpperZ.Cmp(b.UpperZ) < 0 {
		return false
	}
	span := new(big.Rat).Sub(a.UpperZ, a.LowerZ)
	slope := new(big.Rat).Quo(new(big.Rat).Sub(a.UpperRadius, a.LowerRadius), span)
	for i, z := range [2]*big.Rat{b.LowerZ, b.UpperZ} {
		r := new(big.Rat).Add(a.LowerRadius, new(big.Rat).Mul(slope, new(big.Rat).Sub(z, a.LowerZ)))
		if r.Cmp([2]*big.Rat{b.LowerRadius, b.UpperRadius}[i]) <= 0 {
			return false
		}
	}
	// The radial clearance is affine in height. Strict separation at
	// both endpoints certifies it throughout the tool interval.
	return true
}

func assembleWrappedFrustumBore(ctx context.Context, a, b Solid, lowOpen, highOpen bool, tolerance float64,
	o geom.SurfacePatchOptions) (Solid, WrappedCylinderTubeHistory, error) {
	var history WrappedCylinderTubeHistory
	vertices := append(a.Vertices(), b.vertices...)
	edges := a.Edges()
	for _, edge := range b.edges {
		edge.Start += len(a.vertices)
		edge.End += len(a.vertices)
		edges = append(edges, edge)
	}
	faces := a.Faces()
	history.OuterFaces, history.InnerFaces = [3]int{0, 1, 2}, [3]int{3, -1, -1}
	lateral := cloneFace(b.faces[0])
	lateral.Reversed = true
	for i := range lateral.Outer {
		lateral.Outer[i].Edge += len(a.edges)
	}
	faces = append(faces, lateral)
	openings := 0
	for i, open := range [2]bool{lowOpen, highOpen} {
		i++
		if !open {
			cap := cloneFace(b.faces[i])
			cap.Reversed = !cap.Reversed
			for j := range cap.Outer {
				cap.Outer[j].Edge += len(a.edges)
			}
			history.InnerFaces[i] = len(faces)
			faces = append(faces, cap)
			continue
		}
		openings++
		history.InnerFaces[i] = i
		hole := make([]Use, 4)
		for j, u := range b.faces[i].Outer {
			hole[3-j] = Use{Edge: u.Edge + len(a.edges), Reversed: !u.Reversed,
				UVStart: u.UVEnd, UVEnd: u.UVStart, Curve: reversePCurve(u.Curve)}
		}
		faces[i].Holes = [][]Use{hole}
		p, e := ValidateFaceTrimLoops(ctx, faces[i], trimvalidate.Options{})
		if e != nil || p.Status != trimvalidate.Certified {
			if e == nil {
				e = geom.ErrConvergence
			}
			return Solid{}, history, e
		}
	}
	result, e := NewSolidContext(ctx, vertices, edges, faces, tolerance)
	if e != nil {
		return Solid{}, history, e
	}
	proof, e := result.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil || !proof.Complete {
		if e == nil {
			e = geom.ErrConvergence
		}
		return Solid{}, history, e
	}
	wantShells := 1
	if openings == 0 {
		wantShells = 2
	}
	if t := result.Topology(); t.EulerCharacteristic != 4-2*openings || len(t.Shells) != wantShells || t.InnerWires != openings {
		return Solid{}, history, geom.ErrDegenerate
	}
	for i := range 9 {
		history.OuterEdges[i], history.InnerEdges[i] = i, i+9
	}
	return result, history, nil
}

func sameCylinderAxis(a, b geom.WrappedCylinderEmbedding) bool {
	return a.CenterX.Cmp(b.CenterX) == 0 && a.CenterY.Cmp(b.CenterY) == 0
}

func sameCylinderAxisAndHeight(a, b geom.WrappedCylinderEmbedding) bool {
	return sameCylinderAxis(a, b) && a.LowerZ.Cmp(b.LowerZ) == 0 && a.UpperZ.Cmp(b.UpperZ) == 0
}

func sameNormalizedCylinderProfile(ctx context.Context, a, b *geom.WrappedSurfacePatch, pa, pb geom.WrappedCylinderEmbedding, o geom.SurfacePatchOptions) (bool, error) {
	if pa.Circular && pb.Circular {
		return true, nil
	}
	// The canonical axes and center already agree. Compare cross-scaled
	// profiles using exact represented radii without dividing into a
	// potentially unrepresentable scale.
	support := [2]*geom.WrappedSurfacePatch{a, b}
	scales := [2]*big.Rat{pb.Radius, pa.Radius}
	var curves [2]*geom.ExactSplineCurve
	// Translate by the common center before scaling. Translation after
	// scaling is performed separately so no rational product is rounded.
	cx, e := geom.RepresentedExactScalar(pa.CenterX)
	if e != nil {
		return false, e
	}
	cy, e := geom.RepresentedExactScalar(pa.CenterY)
	if e != nil {
		return false, e
	}
	center := geom.Affine{{1, 0, 0, -cx}, {0, 1, 0, -cy}, {0, 0, 0, 0}}
	for i, w := range support {
		s, e := geom.RepresentedExactScalar(scales[i])
		if e != nil {
			return false, e
		}
		c, e := w.FundamentalPatch().IsoCurve(1, w.Domain().VMin, o)
		if e == nil {
			c, e = c.Transform(center, o)
		}
		if e == nil {
			c, e = c.Transform(geom.Affine{{s, 0, 0, 0}, {0, s, 0, 0}, {0, 0, 1, 0}}, o)
		}
		if e != nil {
			return false, e
		}
		curves[i] = c
	}
	p, e := geom.CompareExactCurvesContext(ctx, curves[0], curves[1], geom.ExactCurveComparisonOptions{
		NormalizeDomains: true, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	return p.Complete && p.Equal, e
}

// LocateWrappedCylinderTube classifies a point against a proved concentric
// through-bore. In the common cap planes, only their annular area is material.
// The caller must pass the supports used to construct the tube.
func LocateWrappedCylinderTube(ctx context.Context, outer, inner *geom.WrappedSurfacePatch, point geom.Vec3, o geom.SurfacePatchOptions) (parametric.Location, error) {
	return locateWrappedCylinderBore(ctx, outer, inner, point, o, true)
}
