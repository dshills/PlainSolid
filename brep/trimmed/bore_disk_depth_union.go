package trimmed

import (
	"context"
	"math"
	"math/big"

	"plainsolid/geom"
)

// BoreDiskDepthUnion cuts overlapping cylindrical tools from the two caps of
// circular stock. Every complete tool footprint must clear the outside wall.
// The arrangement retains circular walls and planar floors at each axial level.
func BoreDiskDepthUnion(ctx context.Context, frame geom.Frame, outer ExtrusionCircle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) (Solid, LayeredBoreHistory, error) {
	disks, err := diskBoreDepths(frame, outer, through, blind, height)
	if err != nil {
		return Solid{}, LayeredBoreHistory{}, err
	}
	stock, err := ExtrudeDiskWithHoles(ctx, frame, outer, nil, nil, height)
	if err != nil {
		return Solid{}, LayeredBoreHistory{}, err
	}
	tolerance := max(math.Abs(height), outer.Radius) * 1e-11
	return assembleDepthBore(ctx, stock, 0, 1, frame, disks, min(0, height), max(0, height), tolerance)
}

// DiskDepthUnionOverlap measures how much of a contained tool intersects the
// retained stock. Low and high are clipped axial coordinates in the stock
// frame. A finite query may lie fully inside the stock between the two caps.
func DiskDepthUnionOverlap(frame geom.Frame, outer ExtrusionCircle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64, tool ExtrusionCircle, low, high float64) (float64, error) {
	disks, err := diskBoreDepths(frame, outer, through, blind, height)
	if err != nil {
		return 0, err
	}
	stockLow, stockHigh := min(0, height), max(0, height)
	if !finite(low) || !finite(high) || low < stockLow || high > stockHigh || high <= low || !uvFinite(tool.Center) || !finite(tool.Radius) || tool.Radius <= 0 {
		return 0, geom.ErrInvalid
	}
	clear := new(big.Rat).Sub(ratInset(outer.Radius), ratInset(tool.Radius))
	if clear.Sign() <= 0 || extrusionCenterDistanceSquared(outer, tool).Cmp(clear.Mul(clear, clear)) >= 0 {
		return 0, geom.ErrDomain
	}
	tol := max(math.Abs(height), outer.Radius) * 1e-11
	volume := func(disks []depthDisk) (float64, error) {
		_, levels, _, boundaries, err := depthBoreSlabs(disks, stockLow, stockHigh, tol)
		if err != nil {
			return 0, err
		}
		v := 0.
		for i, slab := range boundaries {
			v += slab.area * (levels[i+1] - levels[i])
		}
		return v, nil
	}
	before, err := volume(disks)
	if err != nil {
		return 0, err
	}
	// The arrangement uses only local circles and axial spans for measures.
	// No tool surface or public shape is constructed by this query.
	query := depthDisk{unionDisk{center: tool.Center, radius: tool.Radius}, low, high}
	after, err := volume(append(disks, query))
	if err != nil {
		return 0, err
	}
	delta := after - before
	toolVolume := math.Pi * tool.Radius * tool.Radius * (high - low)
	margin := max(1e-9, max(before, toolVolume)*1e-10)
	if !finite(delta) || !finite(toolVolume) || delta < -margin || delta > toolVolume+margin {
		return 0, geom.ErrUnrepresentable
	}
	return min(toolVolume, max(0, delta)), nil
}

func diskBoreDepths(frame geom.Frame, outer ExtrusionCircle, through []ExtrusionCircle, blind []ExtrusionBlindHole, height float64) ([]depthDisk, error) {
	if len(through)+len(blind) == 0 || len(through)+len(blind) > 32 || frame.ZAxis().Length() == 0 ||
		!finite(height) || height == 0 || !uvFinite(outer.Center) || !finite(outer.Radius) || outer.Radius <= 0 {
		return nil, geom.ErrInvalid
	}
	disks := make([]depthDisk, 0, len(through)+len(blind))
	add := func(circle ExtrusionCircle, low, high float64) error {
		if !uvFinite(circle.Center) || !finite(circle.Radius) || circle.Radius <= 0 {
			return geom.ErrInvalid
		}
		clear := new(big.Rat).Sub(ratInset(outer.Radius), ratInset(circle.Radius))
		if clear.Sign() <= 0 || extrusionCenterDistanceSquared(outer, circle).Cmp(clear.Mul(clear, clear)) >= 0 {
			return geom.ErrDomain
		}
		o := frame.ToWorld(geom.Vec3{X: circle.Center.X, Y: circle.Center.Y})
		f, err := geom.NewFrame(o, frame.XAxis(), frame.ZAxis())
		if err != nil {
			return err
		}
		disks = append(disks, depthDisk{unionDisk{center: circle.Center, radius: circle.Radius, frame: f}, low, high})
		return nil
	}
	for _, hole := range through {
		if err := add(hole, min(0, height), max(0, height)); err != nil {
			return nil, err
		}
	}
	for _, hole := range blind {
		if !finite(hole.Depth) || hole.Depth <= 0 || hole.Depth >= math.Abs(height) {
			return nil, geom.ErrDomain
		}
		start, floor := 0., math.Copysign(hole.Depth, height)
		if hole.OppositeCap {
			start = height
			exact := new(big.Rat).Sub(ratInset(start), ratInset(floor))
			floor = start - floor
			if exact.Cmp(ratInset(floor)) != 0 {
				return nil, geom.ErrUnrepresentable
			}
		}
		if err := add(hole.ExtrusionCircle, min(start, floor), max(start, floor)); err != nil {
			return nil, err
		}
	}
	return disks, nil
}
