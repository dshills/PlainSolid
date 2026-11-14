package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// The two proved original graphs induce a conforming grid on the common
// footprint. Sew only roof and floor tiles, plus walls at the outer perimeter.
// An edge inside this grid joins two tiles directly on each cap. Source
// continuity and exact equality of the pointwise minimum on either side
// give the same geometric edge even when the selected roof changes at a knot.
func intersectPiecewiseRoofBoxes(ctx context.Context, a, b Solid, q *MixedRoofMaterialQuery, d geom.ParameterRectangle, floorIndex int, inset [2][]float64, w *mixedRoofLineWork) (Solid, RoofBoxCommonHistory, error) {
	result, faces, err := sewPiecewiseRoofEnvelope(ctx, a, b, q, d, floorIndex, inset, w)
	if err != nil {
		return Solid{}, RoofBoxCommonHistory{}, err
	}
	h := RoofBoxCommonHistory{ProjectedContact: true, ExactOperations: w.operations}
	for _, face := range faces {
		h.Faces = append(h.Faces, face.source)
	}
	return result, h, nil
}

type roofEnvelopeFace struct {
	source MixedRoofFaceOwner
	side   int // -1 for a roof or floor
}

func sewPiecewiseRoofEnvelope(ctx context.Context, a, b Solid, q *MixedRoofMaterialQuery, d geom.ParameterRectangle, floorIndex int, inset [2][]float64, w *mixedRoofLineWork) (Solid, []roofEnvelopeFace, error) {
	floor := q.box[floorIndex].floor
	cuts := roofEnvelopeKnotCuts(q.box, d)
	for i := range cuts {
		cuts[i] = append(cuts[i], inset[i]...)
	}
	cuts = sortRoofEnvelopeCuts(cuts)
	if len(q.box[0].roofSpans) != 0 || len(q.box[1].roofSpans) != 0 {
		var err error
		cuts, err = refineRoofEnvelopeCuts(ctx, q.box, cuts, w)
		if err != nil {
			return Solid{}, nil, err
		}
	}
	nu, nv := len(cuts[0])-1, len(cuts[1])-1
	// Keep the result within Sew's 128-face limit before spending proof work.
	if 2*(nu*nv+nu+nv) > 128 {
		return Solid{}, nil, geom.ErrUnrepresentable
	}
	var owners []roofEnvelopeFace
	var parts []SewFace
	var positiveCells int
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return Solid{}, nil, err
			}
			cell, xy, roofs, err := roofEnvelopeCell(q.box, cuts, i, j)
			if err != nil {
				return Solid{}, nil, err
			}
			order, err := orderedRoofBoxOrder(ctx, roofs, xy, w)
			if err != nil {
				return Solid{}, nil, err
			}
			owner := 0
			if q.op == parametric.Intersection && order > 0 || q.op == parametric.Union && order < 0 {
				owner = 1
			}
			if q.op == parametric.Union && q.box[0].floor.Cmp(q.box[1].floor) != 0 {
				overlap, err := orderedRoofAgainstFloor(ctx, roofs[floorIndex], xy, q.box[1-floorIndex].floor, w)
				if err != nil {
					return Solid{}, nil, err
				}
				if overlap < 0 {
					return Solid{}, nil, geom.ErrUnrepresentable
				}
			}
			gap, err := orderedRoofAgainstFloor(ctx, roofs[owner], xy, floor, w)
			if err != nil {
				return Solid{}, nil, err
			}
			if gap <= 0 {
				if q.op == parametric.Union {
					return Solid{}, nil, geom.ErrUnrepresentable
				}
				continue
			}
			positiveCells++
			upper, err := orderedRoofRestrictedGraph(ctx, roofs[owner], cell, w)
			if err != nil {
				return Solid{}, nil, err
			}
			if err := certifyRoofStrictlyAboveFloor(ctx, upper, floor, w); err != nil {
				return Solid{}, nil, err
			}
			lower, err := orderedRoofFlatFloor(ctx, cell, xy, floor, w)
			if err != nil {
				return Solid{}, nil, err
			}
			owners = append(owners, roofEnvelopeFace{MixedRoofFaceOwner{owner, q.box[owner].roofFace}, -1}, roofEnvelopeFace{MixedRoofFaceOwner{floorIndex, q.box[floorIndex].floorFace}, -1})
			walls := roofEnvelopeWalls(i, j, nu, nv)
			for k := range 4 {
				if walls&(1<<k) != 0 {
					owners = append(owners, roofEnvelopeFace{roofCommonWallOwner(q.box, d, k), k})
				}
			}
			faces, err := orderedRoofBoxFaces(ctx, upper, lower, cell, walls, w)
			if err != nil {
				return Solid{}, nil, err
			}
			parts = append(parts, faces...)
		}
	}
	if positiveCells == 0 {
		return Solid{}, nil, ctx.Err()
	}
	if positiveCells != nu*nv {
		return Solid{}, nil, geom.ErrUnrepresentable
	}
	result, err := sewOrderedRoofBoxFaces(ctx, parts, max(a.tolerance, b.tolerance), w)
	if err != nil {
		return Solid{}, nil, fmt.Errorf("sew ordered piecewise roof envelope: %w", err)
	}
	return result, owners, nil
}

func roofEnvelopeWalls(i, j, nu, nv int) uint8 {
	var walls uint8
	for axis, outer := range [4]bool{i == 0, i == nu-1, j == 0, j == nv-1} {
		if outer {
			walls |= 1 << axis
		}
	}
	return walls
}

func roofEnvelopeKnotCuts(boxes [2]*mixedRoofBox, d geom.ParameterRectangle) [2][]float64 {
	cuts := [2][]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	for _, box := range boxes {
		if len(box.roofSpans) == 0 {
			continue
		}
		u, v := box.roof.KnotBreaks()
		for axis, breaks := range [2][]float64{u, v} {
			for _, x := range breaks {
				if x > cuts[axis][0] && x < cuts[axis][1] {
					cuts[axis] = append(cuts[axis], x)
				}
			}
		}
	}
	return sortRoofEnvelopeCuts(cuts)
}

func sortRoofEnvelopeCuts(cuts [2][]float64) [2][]float64 {
	for i := range cuts {
		slices.Sort(cuts[i])
		cuts[i] = slices.Compact(cuts[i])
	}
	return cuts
}

// Discover crossings on each original knot rectangle. Extend each proved
// coordinate cut across the grid so all neighboring cap tiles and perimeter
// walls use the same edges, including when an adjacent cell keeps one roof.
func refineRoofEnvelopeCuts(ctx context.Context, boxes [2]*mixedRoofBox, cuts [2][]float64, w *mixedRoofLineWork) ([2][]float64, error) {
	nu, nv := len(cuts[0])-1, len(cuts[1])-1
	if 2*(nu*nv+nu+nv) > 128 {
		return [2][]float64{}, geom.ErrUnrepresentable
	}
	var additional [2][]float64
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			if err := ctx.Err(); err != nil {
				return [2][]float64{}, err
			}
			_, xy, roofs, err := roofEnvelopeCell(boxes, cuts, i, j)
			if err != nil {
				return [2][]float64{}, err
			}
			if _, err = orderedRoofBoxOrder(ctx, roofs, xy, w); err == nil {
				continue
			} else if err != geom.ErrUnrepresentable {
				return [2][]float64{}, err
			}
			inset, err := orderedRoofCrossingCuts(ctx, roofs, xy, w)
			if err != nil {
				return [2][]float64{}, err
			}
			for axis := range cuts {
				additional[axis] = append(additional[axis], inset[axis]...)
			}
		}
	}
	for axis := range cuts {
		cuts[axis] = append(cuts[axis], additional[axis]...)
	}
	return sortRoofEnvelopeCuts(cuts), nil
}

func roofEnvelopeCell(boxes [2]*mixedRoofBox, cuts [2][]float64, i, j int) (geom.ParameterRectangle, [2]geom.ExactParameterInterval, [2]*mixedRoofBox, error) {
	cell := geom.ParameterRectangle{UMin: cuts[0][i], UMax: cuts[0][i+1], VMin: cuts[1][j], VMax: cuts[1][j+1]}
	xy := [2]geom.ExactParameterInterval{
		{Lower: ratInset(cell.UMin), Upper: ratInset(cell.UMax)},
		{Lower: ratInset(cell.VMin), Upper: ratInset(cell.VMax)},
	}
	roofs := boxes
	for k, box := range boxes {
		if len(box.roofSpans) == 0 {
			continue
		}
		roofs[k] = nil
		for _, span := range box.roofSpans {
			if intersectRectangles(cell, span.domain) == cell {
				roofs[k] = span
				break
			}
		}
		if roofs[k] == nil {
			return cell, xy, roofs, geom.ErrUnrepresentable
		}
	}
	return cell, xy, roofs, nil
}

// A univariate homogeneous height difference makes every sign-changing
// root a complete projected isoparametric crossing. Exact dyadic cuts can
// enter the native topology directly. The actual restricted surfaces and
// their shared model edge are subsequently re-proved by the sewing path.
func orderedRoofCrossingCuts(ctx context.Context, boxes [2]*mixedRoofBox, xy [2]geom.ExactParameterInterval, w *mixedRoofLineWork) ([2][]float64, error) {
	var cuts [2][]float64
	power, err := orderedRoofHeightDifference(boxes, xy, w)
	if err != nil {
		return cuts, err
	}
	polynomial, axis := orderedRoofUnivariate(power)
	if len(polynomial) < 2 {
		return cuts, geom.ErrUnrepresentable
	}
	rootOptions := func() (geom.ExactCurvePlaneEventOptions, error) {
		left, err := roofBoxConstructionOptions(w)
		return geom.ExactCurvePlaneEventOptions{MaxOperations: left.MaxOperations, MaxCoefficientBits: left.MaxCoefficientBits}, err
	}
	options, err := rootOptions()
	if err != nil {
		return cuts, err
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	roots, err := geom.IsolateExactPolynomialRoots(ctx, polynomial, zero, one, options)
	w.operations += roots.Operations
	if err != nil {
		return cuts, err
	}
	width, err := w.rat(new(big.Rat).Sub(xy[axis].Upper, xy[axis].Lower))
	if err != nil {
		return cuts, err
	}
	for _, root := range roots.Roots {
		if root.Multiplicity%2 == 0 {
			continue
		}
		options, err := rootOptions()
		if err != nil {
			return cuts, err
		}
		kind, err := root.Root.Classify(ctx, options)
		w.operations += kind.Operations
		if err != nil {
			return cuts, err
		}
		if kind.Rational == nil {
			return cuts, geom.ErrUnrepresentable
		}
		if kind.Rational.Sign() == 0 || kind.Rational.Cmp(one) == 0 {
			continue
		}
		delta, err := w.rat(new(big.Rat).Mul(kind.Rational, width))
		if err != nil {
			return cuts, err
		}
		point, err := w.rat(delta.Add(delta, xy[axis].Lower))
		if err != nil {
			return cuts, err
		}
		cut, err := geom.RepresentedExactScalar(point)
		if err != nil {
			return cuts, err
		}
		cuts[axis] = append(cuts[axis], cut)
	}
	if len(cuts[axis]) == 0 {
		return cuts, geom.ErrUnrepresentable
	}
	return cuts, nil
}
