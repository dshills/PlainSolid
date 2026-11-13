package offset

import (
	"context"
	"errors"
	"fmt"
	"plainsolid/brep"
	"plainsolid/csg/prism"
	"plainsolid/geom"
	"plainsolid/solid"
	"sort"
)

// FromSolid promotes an outward orthogonal BRep whose occupied XY columns all
// continuously span identical minimum/maximum Z to rectangles, then calls Compute.
// Variable-height solids, rotated prisms, and triangulated faces with diagonal
// edges are outside this promotion path. Input cell conversion is separately
// bounded by MaxCells/MaxWork and cannot be interrupted internally.
func FromSolid(ctx context.Context, s brep.Solid, distance float64, o Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Result{}, e
	}
	if o.MaxCells < 0 || o.MaxWork < 0 || o.MaxSegments < 0 || o.MaxSegments > 2048 || !finite(distance) || !finite(o.MaxCoordinateError) || o.MaxCoordinateError < 0 {
		return Result{}, ErrInvalid
	}
	if o.MaxCells == 0 {
		o.MaxCells = 262144
	}
	if o.MaxWork == 0 {
		o.MaxWork = 20000000
	}
	cells, e := solid.FromOrthogonal(s, solid.Options{MaxCells: o.MaxCells, MaxWork: o.MaxWork})
	if e != nil {
		if errors.Is(e, solid.ErrLimit) {
			return Result{}, ErrLimit
		}
		return Result{}, fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	v := cells.Cells()
	if len(v) == 0 {
		return Compute(ctx, nil, 0, 1, distance, o)
	}
	bounds, e := cells.Bounds()
	if e != nil {
		return Result{}, e
	}
	lo, hi := bounds.Min.Z, bounds.Max.Z
	columns := map[[4]float64][]geom.Box{}
	var keys [][4]float64
	for _, c := range v {
		if e := ctx.Err(); e != nil {
			return Result{}, e
		}
		key := [4]float64{c.Min.X, c.Min.Y, c.Max.X, c.Max.Y}
		if _, ok := columns[key]; !ok {
			keys = append(keys, key)
		}
		columns[key] = append(columns[key], c)
	}
	sort.Slice(keys, func(i, j int) bool {
		for k := 0; k < 4; k++ {
			if keys[i][k] != keys[j][k] {
				return keys[i][k] < keys[j][k]
			}
		}
		return false
	})
	limit := o.MaxSegments
	if limit == 0 {
		limit = 256
	}
	if len(keys) > limit/4 {
		return Result{}, ErrLimit
	}
	var profiles []prism.Profile
	for _, key := range keys {
		if e := ctx.Err(); e != nil {
			return Result{}, e
		}
		column := columns[key]
		sort.Slice(column, func(i, j int) bool { return column[i].Min.Z < column[j].Min.Z })
		end := lo
		for _, c := range column {
			if c.Min.Z != end {
				return Result{}, fmt.Errorf("%w: variable extrusion height", ErrInvalid)
			}
			end = c.Max.Z
		}
		if end != hi {
			return Result{}, fmt.Errorf("%w: variable extrusion height", ErrInvalid)
		}
		profiles = append(profiles, prism.Profile{Outer: []geom.Vec2{{X: key[0], Y: key[1]}, {X: key[2], Y: key[1]}, {X: key[2], Y: key[3]}, {X: key[0], Y: key[3]}}})
	}
	return Compute(ctx, profiles, lo, hi, distance, o)
}
