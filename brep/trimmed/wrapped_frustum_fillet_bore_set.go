package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// FilletedFrustumBoreCut describes an axial cutter. Zero Depth means through;
// positive Depth is a blind entry from the selected original cap.
type FilletedFrustumBoreCut struct {
	Center        geom.Vec2
	Radius, Depth float64
	FromUpper     bool
}
type FilletedFrustumBoreCutHistory struct {
	BoreFace, FloorFace int
	FloorZ              *big.Rat
	BoreRims            [2][]int
}
type WrappedFilletedFrustumBoreSet struct {
	Solid       Solid
	Fillets     WrappedFrustumDoubleFilletHistory
	Cuts        []FilletedFrustumBoreCutHistory
	SourceFaces [][]int
}

// FilletAndBoreWrappedFrustumSet applies 1..16 independently proved cuts to
// one original filleted stock. Each blind floor lies strictly within the
// retained cone band. Closed axial overlaps require strict disk separation;
// disjoint axial intervals permit opposite blind entries on the same axis.
// Every donor uses the supplied proof limits independently (at most 16 times),
// and the final bounded cap/edge proof must succeed. Failure is atomic.
func FilletAndBoreWrappedFrustumSet(ctx context.Context, source Solid, lower, upper float64, cuts []FilletedFrustumBoreCut, o geom.SurfacePatchOptions) (WrappedFilletedFrustumBoreSet, error) {
	fail := func(e error) (WrappedFilletedFrustumBoreSet, error) { return WrappedFilletedFrustumBoreSet{}, e }
	if ctx == nil || len(cuts) == 0 || len(cuts) > 16 {
		return fail(geom.ErrInvalid)
	}
	_, p, e := source.ValidateCanonicalWrappedFrustum(ctx, o)
	if e != nil {
		return fail(e)
	}
	stock, fh, e := FilletWrappedFrustumRims(ctx, source, lower, upper, o)
	if e != nil {
		return fail(e)
	}
	donors := make([]Solid, len(cuts))
	histories := make([]WrappedFilletedFrustumBoreHistory, len(cuts))
	intervals := make([][2]*big.Rat, len(cuts))
	through := 0
	for i, c := range cuts {
		donors[i], histories[i], e = filletAndBoreWrappedFrustum(ctx, source, lower, upper, c.Center, c.Radius, c.Depth, c.FromUpper, o)
		if e != nil {
			return fail(e)
		}
		intervals[i] = [2]*big.Rat{p.LowerZ, p.UpperZ}
		if c.Depth == 0 {
			through++
		} else if c.FromUpper {
			intervals[i][0] = histories[i].FloorZ
		} else {
			intervals[i][1] = histories[i].FloorZ
		}
	}
	for i, a := range cuts {
		for j := 0; j < i; j++ {
			if intervals[i][1].Cmp(intervals[j][0]) < 0 || intervals[j][1].Cmp(intervals[i][0]) < 0 {
				continue
			}
			b := cuts[j]
			dx := new(big.Rat).Sub(new(big.Rat).SetFloat64(a.Center.X), new(big.Rat).SetFloat64(b.Center.X))
			dy := new(big.Rat).Sub(new(big.Rat).SetFloat64(a.Center.Y), new(big.Rat).SetFloat64(b.Center.Y))
			d := new(big.Rat).Add(new(big.Rat).Mul(dx, dx), new(big.Rat).Mul(dy, dy))
			r := new(big.Rat).Add(new(big.Rat).SetFloat64(a.Radius), new(big.Rat).SetFloat64(b.Radius))
			if d.Cmp(new(big.Rat).Mul(r, r)) <= 0 {
				return fail(geom.ErrDomain)
			}
		}
	}
	vertices, edges, faces := stock.Vertices(), stock.Edges(), stock.Faces()
	nv, ne, nf := len(vertices), len(edges), len(faces)
	result := WrappedFilletedFrustumBoreSet{Fillets: fh, Cuts: make([]FilletedFrustumBoreCutHistory, len(cuts)), SourceFaces: make([][]int, nf)}
	for i := 0; i < nf; i++ {
		result.SourceFaces[i] = []int{i}
	}
	for i, donor := range donors {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		dv, de, df := donor.Vertices(), donor.Edges(), donor.Faces()
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
		remap := func(loop []Use) []Use {
			q := append([]Use(nil), loop...)
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
		for _, cap := range fh.Caps {
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
		h := FilletedFrustumBoreCutHistory{BoreFace: histories[i].BoreFace + fm, FloorFace: -1}
		if histories[i].FloorFace >= 0 {
			h.FloorFace = histories[i].FloorFace + fm
			h.FloorZ = new(big.Rat).Set(histories[i].FloorZ)
		}
		for end, ids := range histories[i].BoreRims {
			for _, id := range ids {
				h.BoreRims[end] = append(h.BoreRims[end], mapE(id))
			}
		}
		result.Cuts[i] = h
	}
	for _, cap := range fh.Caps {
		proof, e := ValidateFaceTrimLoops(ctx, faces[cap], trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if proof.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	result.Solid, e = NewSolidContext(ctx, vertices, edges, faces, source.Tolerance())
	if e != nil {
		return fail(e)
	}
	agreement, e := result.Solid.ValidateEdgeAgreement(ctx, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	if e != nil {
		return fail(e)
	}
	if !agreement.Complete {
		return fail(geom.ErrConvergence)
	}
	if result.Solid.Topology().EulerCharacteristic != 2-2*through || len(result.Solid.Topology().Shells) != 1 {
		return fail(geom.ErrDegenerate)
	}
	for _, u := range agreement.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			return fail(geom.ErrConvergence)
		}
	}
	return result, nil
}
