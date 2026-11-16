package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaCirclePocketDiskCutHistory records the original portion of a
// positive shell. Region 2 carries an inner rim and region 6 carries the
// separate disk's outer rim. Other regions have -1 in the rim entries.
// Rim entries are ordered counterclockwise starting at physical +X.
type ParabolaCirclePocketDiskCutHistory struct {
	ParabolaCircleFourCrossingsCutHistory
	DiskEdges    [4]int
	DiskVertices [4]int
}

// ParabolaCirclePocketDiskCut holds the original positive difference
// shells in region order. Region 6, when present, is an isolated shell.
// The four original crossing contacts are between other components.
type ParabolaCirclePocketDiskCut struct {
	components []Solid
	history    []ParabolaCirclePocketDiskCutHistory
	contacts   [4]ParabolaTwoCrossingCirclesCutContact
	sources    [3]*geom.BSplineSurface
	circles    [2]int
	diskVolume *big.Rat
	axis       int
	tolerance  float64
}

func (r *ParabolaCirclePocketDiskCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

func (r *ParabolaCirclePocketDiskCut) History() []ParabolaCirclePocketDiskCutHistory {
	if r == nil {
		return nil
	}
	history := slices.Clone(r.history)
	for i := range history {
		history[i].CircleEdges = slices.Clone(history[i].CircleEdges)
		history[i].WallFaces = slices.Clone(history[i].WallFaces)
		history[i].DomainSides = slices.Clone(history[i].DomainSides)
	}
	return history
}

func (r *ParabolaCirclePocketDiskCut) Contacts() (out [4]ParabolaTwoCrossingCirclesCutContact) {
	if r != nil {
		for i, c := range r.contacts {
			out[i] = cloneCrossingCircleCutContact(c)
		}
	}
	return out
}

// DiskSignedVolumePiCoefficient returns the exact signed integral of
// A.Z-B.Z over the original small disk, divided by π. Its sign records
// whether this ordered difference retains the separate disk.
func (r *ParabolaCirclePocketDiskCut) DiskSignedVolumePiCoefficient() *big.Rat {
	if r == nil || r.diskVolume == nil {
		return nil
	}
	return new(big.Rat).Set(r.diskVolume)
}

// CutParabolaCirclePocketDiskSplineGraphs subtracts B from A. Each strict
// positive region is assembled on both original roofs. The annular
// pocket and the disjoint disk use the original shared circular wire.
func CutParabolaCirclePocketDiskSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCirclePocketDiskCut, error) {
	src := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCirclePocketDiskPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: original pocket disk Cut partition: %w", err)
	}
	out := &ParabolaCirclePocketDiskCut{sources: src, axis: axis, tolerance: tolerance,
		circles: [2]int{p.proof.CrossingCircle, p.proof.PocketCircle}, diskVolume: p.proof.DiskSignedVolumePiCoefficient}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	roofs := [2]*geom.BSplineSurface{a, b}
	buildHole := func(v []geom.Vec3, edges []parametric.Edge, faces []Face, t float64) (Solid, error) {
		return newMixedCirclesHoledCutSolid(ctx, v, edges, faces, t)
	}
	regionComponent := [7]int{-1, -1, -1, -1, -1, -1, -1}
	var binding crossingCircleCutBinding
	for region, reg := range p.regions {
		if reg.sign == -1 {
			continue
		}
		if reg.sign != 1 {
			return nil, geom.ErrConvergence
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, assembly, err := assembleGraphRegionDifferenceWithHoleBuilder(a, b, reg, tolerance, buildHole, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: original pocket disk Cut region %d: %w", region, err)
		}
		h, edges, err := bindPocketDiskCutComponent(ctx, part, assembly, p, region, roofs, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: original pocket disk Cut region %d ownership: %w", region, err)
		}
		regionComponent[region] = len(out.components)
		out.components = append(out.components, part)
		out.history = append(out.history, h)
		binding.regions = append(binding.regions, region)
		binding.assembly = append(binding.assembly, assembly)
		binding.edges = append(binding.edges, edges)
	}
	want := []int{1, 2, 3, 4}
	if out.diskVolume.Sign() > 0 {
		want = []int{0, 5, 6}
	}
	if !slices.Equal(binding.regions, want) {
		return nil, geom.ErrConvergence
	}
	binding.material.components = out.components
	out.contacts, err = bindParabolaCircleFourCrossingCutContacts(p.crossing, binding, regionComponent[:])
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindPocketDiskCutComponent(ctx context.Context, part Solid, assembly OpenSectionHistory,
	p parabolaCirclePocketDiskPartition, region int, roofs [2]*geom.BSplineSurface, opts geom.GraphPointContactOptions,
) (ParabolaCirclePocketDiskCutHistory, []int, error) {
	h := ParabolaCirclePocketDiskCutHistory{DiskEdges: [4]int{-1, -1, -1, -1}, DiskVertices: [4]int{-1, -1, -1, -1}}
	reg := p.regions[region]
	pieces := p.crossing.pieces
	if region == 6 {
		pieces = reg.loop
	}
	edges, err := bindCrossingCircleCutComponentWithHole(ctx, part, assembly, pieces, region, reg,
		p.proof.RegionWitnesses[region], roofs, opts, region == 2)
	if err != nil {
		return h, nil, err
	}
	roofFaces := [2]int{assembly.SplineFace, assembly.PlaneFace}
	if region == 2 || region == 6 {
		if err := bindPocketDiskCutRim(part, assembly, p, region, roofs[0], &h); err != nil {
			return h, nil, err
		}
	}
	if region == 6 {
		edges = make([]int, len(p.crossing.pieces))
		for i := range edges {
			edges[i] = -1
		}
	}
	h.ParabolaCircleFourCrossingsCutHistory = ParabolaCircleFourCrossingsCutHistory{
		Region: region, RoofFaces: roofFaces, ParabolaEdges: [fourCrossingParabolaPieces]int(edges[:fourCrossingParabolaPieces]),
		CircleEdges: edges[fourCrossingParabolaPieces:], WallFaces: assembly.WallFaces, DomainSides: assembly.DomainSides,
	}
	return h, edges, nil
}

func bindPocketDiskCutRim(part Solid, assembly OpenSectionHistory, p parabolaCirclePocketDiskPartition,
	region int, roof *geom.BSplineSurface, h *ParabolaCirclePocketDiskCutHistory) error {
	ring := p.regions[6].loop
	edges, err := bindParabolaCircleCrossingEdges(part, assembly.ContourEdges, ring)
	if err != nil {
		return err
	}
	h.DiskEdges = [4]int(edges)
	h.DiskVertices, err = bindOriginalSeparatedCircleRing(part, edges, ring, p.proof.PocketExtrema)
	if err != nil {
		return err
	}
	a, b := part.faces[assembly.SplineFace], part.faces[assembly.PlaneFace]
	loopA, loopB := a.Outer, b.Outer
	if region == 2 {
		loopA, loopB = a.Holes[0], b.Holes[0]
	}
	for _, id := range edges {
		// The bottom face is reversed, giving the same local pcurve
		// direction opposite material incidence at each original edge.
		i := slices.IndexFunc(loopA, func(use Use) bool { return use.Edge == id })
		j := slices.IndexFunc(loopB, func(use Use) bool { return use.Edge == id })
		model, ok := part.edges[id].Curve.(surfaceTrimCurve)
		if i < 0 || j < 0 || loopA[i].Reversed != loopB[j].Reversed || !ok || model.surface != roof {
			return geom.ErrConvergence
		}
	}
	return nil
}
