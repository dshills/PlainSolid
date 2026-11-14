package fillet

import (
	"context"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// RoundedCylinderBoreCut describes one removal. Through cuts ignore neither
// depth nor entry: Depth must be zero and FromUpper false unless Blind is true.
type RoundedCylinderBoreCut struct {
	Center    geom.Vec2
	Radius    float64
	Blind     bool
	Depth     float64
	FromUpper bool
}

// RoundedCylinderBoreCutHistory maps one authored cutter to its generated
// wall, optional floor and lower/upper rims. Face IDs refer to the final solid.
type RoundedCylinderBoreCutHistory struct {
	BoreFace, FloorFace int
	BoreRims            [2][]int
	FloorZ              *big.Rat
}

type WrappedRoundedCylinderBoreSet struct {
	Solid                    trimmed.Solid
	SourceFaces              [][]int
	SourceCaps, SourceBlends [2]int
	Cuts                     []RoundedCylinderBoreCutHistory
}

// BoreWrappedRoundedCylinderSet simultaneously removes 1..16 cutters from one
// validated rim-filleted cylinder. Every cutter independently passes the single
// stock proof. Whenever their CLOSED axial intervals overlap, projected disks
// must be strictly disjoint by exact squared distance. Thus cap/floor contact,
// tangency and overlapping removals are rejected; axially separated blind bores
// may share a center. This is a conservative disjoint-cut operation, not a union
// of overlapping tools. All original topology IDs and surface objects survive.
//
// Every generated component is proved against the original stock, never a
// progressively modified input. The merged cap holes receive independent exact
// trim validation; final topology and model-edge agreement are revalidated.
// Limits apply per at-most-16 cutter proof and final bounded trim/edge proof.
// Pair checks number at most120. Any failure returns no partially cut stock.
func BoreWrappedRoundedCylinderSet(ctx context.Context, source trimmed.Solid, cuts []RoundedCylinderBoreCut, o geom.SurfacePatchOptions) (WrappedRoundedCylinderBoreSet, error) {
	fail := func(e error) (WrappedRoundedCylinderBoreSet, error) { return WrappedRoundedCylinderBoreSet{}, e }
	if ctx == nil || len(cuts) == 0 {
		return fail(geom.ErrInvalid)
	}
	if len(cuts) > 16 {
		return fail(geom.ErrConvergence)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	p, e := source.ValidateWrappedCylinderRimFillet(ctx, o)
	if e != nil {
		return fail(e)
	}
	donors := make([]WrappedRoundedCylinderBore, len(cuts))
	intervals := make([][2]*big.Rat, len(cuts))
	for i, c := range cuts {
		if !c.Blind && (c.Depth != 0 || c.FromUpper) {
			return fail(geom.ErrInvalid)
		}
		if c.Blind {
			donors[i], e = BlindBoreWrappedRoundedCylinder(ctx, source, c.Center, c.Radius, c.Depth, c.FromUpper, o)
		} else {
			donors[i], e = BoreWrappedRoundedCylinder(ctx, source, c.Center, c.Radius, o)
		}
		if e != nil {
			return fail(e)
		}
		intervals[i] = [2]*big.Rat{p.LowerZ, p.UpperZ}
		if c.Blind {
			if c.FromUpper {
				intervals[i][0] = donors[i].FloorZ
			} else {
				intervals[i][1] = donors[i].FloorZ
			}
		}
		for j := 0; j < i; j++ {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if intervals[i][1].Cmp(intervals[j][0]) < 0 || intervals[j][1].Cmp(intervals[i][0]) < 0 {
				continue
			}
			dx := new(big.Rat).Sub(new(big.Rat).SetFloat64(c.Center.X), new(big.Rat).SetFloat64(cuts[j].Center.X))
			dy := new(big.Rat).Sub(new(big.Rat).SetFloat64(c.Center.Y), new(big.Rat).SetFloat64(cuts[j].Center.Y))
			sum := new(big.Rat).Add(new(big.Rat).SetFloat64(c.Radius), new(big.Rat).SetFloat64(cuts[j].Radius))
			if new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy)).Cmp(new(big.Rat).Mul(sum, sum)) <= 0 {
				return fail(geom.ErrDomain)
			}
		}
	}
	vertices, edges, faces := source.Vertices(), source.Edges(), source.Faces()
	nv, ne, nf := len(vertices), len(edges), len(faces)
	result := WrappedRoundedCylinderBoreSet{SourceCaps: p.Caps, SourceBlends: p.Blends, Cuts: make([]RoundedCylinderBoreCutHistory, len(cuts)), SourceFaces: make([][]int, nf)}
	for i := 0; i < nf; i++ {
		result.SourceFaces[i] = []int{i}
	}
	for i, donor := range donors {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		dv, de, df := donor.Solid.Vertices(), donor.Solid.Edges(), donor.Solid.Faces()
		vm, em, fm := len(vertices)-nv, len(edges)-ne, len(faces)-nf
		mapV := func(id int) int {
			if id < nv {
				return id
			}
			return id + vm
		}
		mapE := func(id int) int {
			if id < ne {
				return id
			}
			return id + em
		}
		remap := func(loop []trimmed.Use) []trimmed.Use {
			q := append([]trimmed.Use(nil), loop...)
			for j := range q {
				q[j].Edge = mapE(q[j].Edge)
			}
			return q
		}
		vertices = append(vertices, dv[nv:]...)
		for _, edge := range de[ne:] {
			edge.Start = mapV(edge.Start)
			edge.End = mapV(edge.End)
			edges = append(edges, edge)
		}
		for _, cap := range p.Caps {
			for _, hole := range df[cap].Holes {
				faces[cap].Holes = append(faces[cap].Holes, remap(hole))
			}
		}
		for _, face := range df[nf:] {
			face.Outer = remap(face.Outer)
			for j := range face.Holes {
				face.Holes[j] = remap(face.Holes[j])
			}
			faces = append(faces, face)
			result.SourceFaces = append(result.SourceFaces, nil)
		}
		h := RoundedCylinderBoreCutHistory{BoreFace: donor.BoreFace + fm, FloorFace: -1}
		if donor.FloorFace >= 0 {
			h.FloorFace = donor.FloorFace + fm
			h.FloorZ = new(big.Rat).Set(donor.FloorZ)
		}
		for end, ids := range donor.BoreRims {
			for _, id := range ids {
				h.BoreRims[end] = append(h.BoreRims[end], mapE(id))
			}
		}
		result.Cuts[i] = h
	}
	for _, cap := range p.Caps {
		proof, e := trimmed.ValidateFaceTrimLoops(ctx, faces[cap], trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	result.Solid, e = trimmed.NewSolidContext(ctx, vertices, edges, faces, source.Tolerance())
	if e != nil {
		return fail(e)
	}
	agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil {
		return fail(e)
	}
	if !agreement.Complete {
		return fail(geom.ErrConvergence)
	}
	return result, nil
}
