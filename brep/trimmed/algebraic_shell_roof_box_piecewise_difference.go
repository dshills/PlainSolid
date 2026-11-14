package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
)

type roofCutBands uint8

const (
	roofCutUnchanged roofCutBands = 1 << iota
	roofCutBelow
	roofCutAbove
)

// A full-footprint cutter can split two complete vertical bands. Each band
// must span every original knot cell with strictly positive thickness.
// Joining only a subset would require constructing its terminating edge.
func subtractPiecewiseOrderedRoofBoxes(ctx context.Context, stock, cutter Solid, boxes [2]*mixedRoofBox, w *mixedRoofLineWork) (Solid, RoofBoxDifferenceHistory, error) {
	var zero RoofBoxDifferenceHistory
	a, b := boxes[0], boxes[1]
	cuts := roofEnvelopeKnotCuts(boxes, a.domain)
	nu, nv := len(cuts[0])-1, len(cuts[1])-1
	const maxBandFaces = 128
	if 2*(nu*nv+nu+nv) > maxBandFaces {
		return Solid{}, zero, geom.ErrUnrepresentable
	}
	var mode roofCutBands
	var parts [2][]SewFace
	var histories [2]RoofBoxDifferenceHistory
	for i := range nu {
		for j := range nv {
			if err := ctx.Err(); err != nil {
				return Solid{}, zero, err
			}
			cell, xy, roofs, err := roofEnvelopeCell(boxes, cuts, i, j)
			if err != nil {
				return Solid{}, zero, err
			}
			cellMode, err := certifyPiecewiseRoofCutCell(ctx, boxes, roofs, xy, w)
			if err != nil {
				return Solid{}, zero, err
			}
			if i == 0 && j == 0 {
				mode = cellMode
			} else if mode != cellMode {
				return Solid{}, zero, geom.ErrUnrepresentable
			}
			walls := roofEnvelopeWalls(i, j, nu, nv)
			if mode&roofCutBelow != 0 {
				top, err := orderedRoofFlatFloor(ctx, cell, xy, b.floor, w)
				if err != nil {
					return Solid{}, zero, err
				}
				bottom, err := orderedRoofFlatFloor(ctx, cell, xy, a.floor, w)
				if err != nil {
					return Solid{}, zero, err
				}
				faces, err := orderedRoofBoxFaces(ctx, top, bottom, cell, walls, w)
				if err != nil {
					return Solid{}, zero, err
				}
				parts[0] = append(parts[0], faces...)
				appendRoofCutCellHistory(&histories[0], MixedRoofFaceOwner{1, b.floorFace}, MixedRoofFaceOwner{0, a.floorFace}, a.sideFaces, walls)
			}
			if mode&roofCutAbove != 0 {
				top, err := orderedRoofRestrictedGraph(ctx, roofs[0], cell, w)
				if err != nil {
					return Solid{}, zero, err
				}
				bottom, err := orderedRoofRestrictedGraph(ctx, roofs[1], cell, w)
				if err != nil {
					return Solid{}, zero, err
				}
				faces, err := orderedRoofBoxFaces(ctx, top, bottom, cell, walls, w)
				if err != nil {
					return Solid{}, zero, err
				}
				parts[1] = append(parts[1], faces...)
				appendRoofCutCellHistory(&histories[1], MixedRoofFaceOwner{0, a.roofFace}, MixedRoofFaceOwner{1, b.roofFace}, a.sideFaces, walls)
			}
		}
	}
	if mode == roofCutUnchanged {
		return unchangedOrderedRoofBox(ctx, stock, w)
	}
	var shells []PlacedSolid
	var history RoofBoxDifferenceHistory
	for i, band := range parts {
		if len(band) == 0 {
			continue
		}
		shell, err := sewOrderedRoofBoxFaces(ctx, band, max(stock.tolerance, cutter.tolerance), w)
		if err != nil {
			return Solid{}, zero, fmt.Errorf("ordered piecewise roof cut band: %w", err)
		}
		shells = append(shells, PlacedSolid{shell, geom.Identity()})
		history.Faces = append(history.Faces, histories[i].Faces...)
		history.Reversed = append(history.Reversed, histories[i].Reversed...)
	}
	history.ExactOperations = w.operations
	switch len(shells) {
	case 0:
		return Solid{}, history, ctx.Err()
	case 1:
		return shells[0].Solid, history, nil
	}
	// The original cutter has positive thickness throughout the footprint,
	// and separates the two independently validated shells.
	out, err := AssembleShells(ctx, shells, 2*maxBandFaces)
	if err != nil {
		return Solid{}, zero, err
	}
	return out, history, nil
}

func certifyPiecewiseRoofCutCell(ctx context.Context, boxes, roofs [2]*mixedRoofBox, xy [2]geom.ExactParameterInterval, w *mixedRoofLineWork) (roofCutBands, error) {
	var mode roofCutBands
	switch boxes[1].floor.Cmp(boxes[0].floor) {
	case 1:
		sign, strict, err := orderedRoofAgainstFloorDetail(ctx, roofs[0], xy, boxes[1].floor, w)
		if err != nil {
			return 0, err
		}
		if sign <= 0 {
			return roofCutUnchanged, nil
		}
		if !strict {
			return 0, geom.ErrUnrepresentable
		}
		mode = roofCutBelow
	default:
		sign, strict, err := orderedRoofAgainstFloorDetail(ctx, roofs[1], xy, boxes[0].floor, w)
		if err != nil {
			return 0, err
		}
		if sign <= 0 {
			return roofCutUnchanged, nil
		}
		if !strict {
			return 0, geom.ErrUnrepresentable
		}
	}
	cache := make(roofOrderCache)
	difference, err := cache.order(roofs, xy, w)
	if err != nil {
		return 0, err
	}
	sign, strict, err := orderedRoofPowerSign(ctx, cache[roofs].power, difference, w)
	if err != nil || sign <= 0 {
		return mode, err
	}
	if !strict {
		return 0, geom.ErrUnrepresentable
	}
	return mode | roofCutAbove, nil
}

func appendRoofCutCellHistory(h *RoofBoxDifferenceHistory, top, bottom MixedRoofFaceOwner, stockSides [4]int, walls uint8) {
	h.Faces = append(h.Faces, top, bottom)
	h.Reversed = append(h.Reversed, top.Operand == 1, bottom.Operand == 1)
	for side, face := range stockSides {
		if walls&(1<<side) != 0 {
			h.Faces = append(h.Faces, MixedRoofFaceOwner{0, face})
			h.Reversed = append(h.Reversed, false)
		}
	}
}
