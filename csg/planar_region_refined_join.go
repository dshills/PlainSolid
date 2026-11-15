package csg

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
)

// PlanarRegionRefinementOptions bounds seam refinement and exact contact.
// The default limits are one million point/edge checks and 100,000 splits.
type PlanarRegionRefinementOptions struct {
	Contact                      collision.PlanarFaceContactOptions
	MaxStationTests, MaxStations int
}

// PlanarRegionResultEdge retains an exact parent interval and its oriented
// result edge. Result.Edge is -1 for pieces removed with the interface.
type PlanarRegionResultEdge struct {
	Result brep.EdgeUse
	Range  [2]*big.Rat
}

// PlanarRegionRefinedOwner maps the original vertices, faces, and ordered
// edge pieces that remain after conforming the two original regions.
type PlanarRegionRefinedOwner struct {
	Vertices, Faces []int
	Edges           [][]PlanarRegionResultEdge
}

// PlanarRegionRefinedJoinHistory maps original topology independently for
// the two inputs. Contact edge/vertex IDs refer to their conformed boundary
// copies; contact face IDs retain the original source indices.
type PlanarRegionRefinedJoinHistory struct {
	Owners  [2]PlanarRegionRefinedOwner
	Contact collision.PlanarRegionContact
}

// JoinRefinedPlanarRegions joins complete contacting original planar regions
// after reconciling different straight edge subdivisions on their common
// exterior seam. Only original vertices from the two bodies can be inserted.
// Both original neighborhoods keep their oriented face identities.
func JoinRefinedPlanarRegions(ctx context.Context, a, b brep.Solid, faces [2][]int, o PlanarRegionRefinementOptions) (brep.Solid, PlanarRegionRefinedJoinHistory, error) {
	var zero PlanarRegionRefinedJoinHistory
	if ctx == nil || o.MaxStationTests < 0 || o.MaxStations < 0 {
		return brep.Solid{}, zero, collision.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return brep.Solid{}, zero, err
	}
	maxChecks := o.MaxStationTests
	if maxChecks == 0 {
		maxChecks = collision.DefaultMaxPairs
	}
	maxSplits := o.MaxStations
	if maxSplits == 0 {
		maxSplits = 100000
	}
	src := [2]brep.Solid{a, b}
	edges := [2][]brep.Edge{a.Edges(), b.Edges()}
	points := [2][]geom.Vec3{a.Vertices(), b.Vertices()}
	var seams, stations [2][]int
	for i, s := range src {
		var err error
		seams[i], stations[i], err = planarRegionSeamStations(ctx, s.Faces(), edges[i], len(points[i]), faces[i])
		if err != nil {
			return brep.Solid{}, zero, err
		}
	}
	var copied [2]brep.Solid
	var splitHistory [2]brep.PlanarEdgeSplitHistory
	one := big.NewRat(1, 1)
	for i, s := range src {
		var splits []brep.PlanarEdgeSplit
		for _, id := range seams[i] {
			split := brep.PlanarEdgeSplit{Edge: id}
			for _, j := range stations[1-i] {
				if err := ctx.Err(); err != nil {
					return brep.Solid{}, zero, err
				}
				if maxChecks == 0 {
					return brep.Solid{}, zero, collision.ErrLimit
				}
				maxChecks--
				p := points[1-i][j]
				t, err := s.PlanarEdgeParameter(id, p)
				if errors.Is(err, geom.ErrDomain) {
					continue
				}
				if err != nil {
					return brep.Solid{}, zero, err
				}
				if t.Sign() <= 0 || t.Cmp(one) >= 0 {
					continue
				}
				if maxSplits == 0 {
					return brep.Solid{}, zero, collision.ErrLimit
				}
				maxSplits--
				split.Points = append(split.Points, p)
			}
			if len(split.Points) > 0 {
				splits = append(splits, split)
			}
		}
		var err error
		limit := o.MaxStations
		if limit == 0 {
			limit = 100000
		}
		copied[i], splitHistory[i], err = brep.SplitPlanarEdges(ctx, s, splits, limit)
		if err != nil {
			return brep.Solid{}, zero, err
		}
	}
	out, h, err := JoinPlanarRegions(ctx, copied[0], copied[1], faces, o.Contact)
	if err != nil {
		return brep.Solid{}, zero, err
	}
	result := PlanarRegionRefinedJoinHistory{Contact: h.Contact}
	for i, old := range splitHistory {
		owner := &result.Owners[i]
		owner.Vertices = append([]int(nil), h.Owners[i].Vertices[:len(points[i])]...)
		owner.Faces = h.Owners[i].Faces
		owner.Edges = make([][]PlanarRegionResultEdge, len(old.Children))
		for j, cs := range old.Children {
			for _, p := range cs {
				owner.Edges[j] = append(owner.Edges[j], PlanarRegionResultEdge{Result: h.Owners[i].Edges[p.Edge], Range: p.Range})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return brep.Solid{}, zero, err
	}
	return out, result, nil
}

func planarRegionSeamStations(ctx context.Context, faces []brep.Face, edges []brep.Edge, nv int, ids []int) (seams, stations []int, err error) {
	if len(ids) == 0 || len(ids) > len(faces) {
		return nil, nil, collision.ErrInvalid
	}
	count := make([]uint8, len(edges))
	seen := make([]bool, len(faces))
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if id < 0 || id >= len(faces) || seen[id] {
			return nil, nil, fmt.Errorf("%w: invalid original region face", collision.ErrInvalid)
		}
		seen[id] = true
		f := faces[id]
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				count[u.Edge]++
			}
		}
	}
	vertex := make([]bool, nv)
	for i, c := range count {
		if c == 1 {
			seams = append(seams, i)
			v := edges[i]
			vertex[v.Start], vertex[v.End] = true, true
		}
	}
	for i, on := range vertex {
		if on {
			stations = append(stations, i)
		}
	}
	if len(seams) == 0 {
		return nil, nil, fmt.Errorf("%w: original contact region has no exterior seam", collision.ErrInvalid)
	}
	return seams, stations, nil
}
