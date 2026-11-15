package document

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/csg"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

// PlanarEdgeSplitRecipe names one edge by its two incident box face roles.
// Parameters are canonical positive rational fractions strictly inside the
// original directed edge. Both role order and station order are immaterial.
type PlanarEdgeSplitRecipe struct {
	Roles      [2]string
	Parameters []string
}

func (st State) evaluatePlanarEdgeSplit(ctx context.Context, s brep.Solid, f Feature, wantFaces, wantEdges bool) (brep.Solid, poly.TopologyHistory, error) {
	var zero poly.TopologyHistory
	r := f.PlanarEdgeSplit
	if r == nil || len(r.Parameters) == 0 || len(r.Parameters) > 64 {
		return brep.Solid{}, zero, geom.ErrInvalid
	}
	var ids [2]int
	for i, role := range r.Roles {
		id, err := st.ResolveBoxFace(FeatureFace{Feature: f.Input, Role: role})
		if err != nil {
			return brep.Solid{}, zero, err
		}
		ids[i] = id
	}
	faces := s.Faces()
	if ids[0] == ids[1] || ids[0] >= len(faces) || ids[1] >= len(faces) {
		return brep.Solid{}, zero, geom.ErrInvalid
	}
	edge := -1
	for _, a := range faces[ids[0]].Boundary {
		for _, b := range faces[ids[1]].Boundary {
			if a.Edge == b.Edge {
				if edge >= 0 {
					return brep.Solid{}, zero, fmt.Errorf("%w: roles name a divided edge", geom.ErrInvalid)
				}
				edge = a.Edge
			}
		}
	}
	if edge < 0 {
		return brep.Solid{}, zero, geom.ErrInvalid
	}
	e, v := s.Edges(), s.Vertices()
	from, to := v[e[edge].Start], v[e[edge].End]
	var stations []geom.Vec3
	one := big.NewRat(1, 1)
	for _, text := range r.Parameters {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, zero, err
		}
		if len(text) > 128 {
			return brep.Solid{}, zero, geom.ErrInvalid
		}
		t, ok := new(big.Rat).SetString(text)
		if !ok || t.Sign() <= 0 || t.Cmp(one) >= 0 || t.RatString() != text || t.Num().BitLen() > 256 || t.Denom().BitLen() > 256 {
			return brep.Solid{}, zero, fmt.Errorf("%w: edge split uses canonical fractions strictly inside the edge", geom.ErrInvalid)
		}
		interpolate := func(a, b float64) (float64, bool) {
			x, y := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)
			y.Sub(y, x)
			y.Mul(y, t)
			y.Add(y, x)
			return y.Float64()
		}
		x, xOK := interpolate(from.X, to.X)
		y, yOK := interpolate(from.Y, to.Y)
		z, zOK := interpolate(from.Z, to.Z)
		p := geom.Vec3{X: x, Y: y, Z: z}
		if !xOK || !yOK || !zOK || !p.IsFinite() {
			return brep.Solid{}, zero, fmt.Errorf("%w: exact authored edge station cannot be represented", geom.ErrUnrepresentable)
		}
		stations = append(stations, p)
	}
	out, h, err := brep.SplitPlanarEdges(ctx, s, []brep.PlanarEdgeSplit{{Edge: edge, Points: stations}}, 64)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	if !wantFaces && !wantEdges {
		return out, zero, nil
	}
	var owners [2][]int
	owners[0] = make([]int, len(faces))
	for i := range owners[0] {
		owners[0][i] = i
	}
	var ancestry [2][][]csg.PlanarRegionResultEdge
	if wantEdges {
		ancestry[0] = make([][]csg.PlanarRegionResultEdge, len(h.Children))
		for i, children := range h.Children {
			for _, c := range children {
				ancestry[0][i] = append(ancestry[0][i], csg.PlanarRegionResultEdge{Result: brep.EdgeUse{Edge: c.Edge}, Range: c.Range})
			}
		}
	}
	return topologyFromPlanarOwnership(ctx, out, owners, ancestry, wantFaces, wantEdges)
}
