package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// CombineOpenPlaneSplineShells discovers complete regular open branches for
// selected face pairs with at least one affine planar spline support. It retains
// exact original-domain terminal ownership and passes all sections to the native
// shell Boolean verifier. Omitted contacts, ambiguous corner terminals, closed
// branches and unsupported material arrangements fail without a partial solid.
// Pairs are input face IDs; no trace or caller-authored endpoint proof is trusted.
func CombineOpenPlaneSplineShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, pairs [][2]int,
	discovery intersect.SplinePlanePairDiscoveryOptions, o SplineShellOpenOptions) (Solid, SplineShellOpenHistory, error) {
	fail := func(e error) (Solid, SplineShellOpenHistory, error) { return Solid{}, SplineShellOpenHistory{}, e }
	if ctx == nil || len(pairs) == 0 || len(pairs) > 64 || op > parametric.Difference {
		return fail(geom.ErrInvalid)
	}
	seen := map[[2]int]bool{}
	var sections []SplineShellSection
	for _, pair := range pairs {
		if e := ctx.Err(); e != nil {
			return fail(e)
		}
		if pair[0] < 0 || pair[0] >= len(a.faces) || pair[1] < 0 || pair[1] >= len(b.faces) || seen[pair] {
			return fail(geom.ErrInvalid)
		}
		seen[pair] = true
		var patches [2]intersect.Patch
		for i, f := range []Face{a.faces[pair[0]], b.faces[pair[1]]} {
			d, e := faceUVBounds(f)
			if e != nil {
				return fail(e)
			}
			p, e := intersectSplineFace(f, d)
			if e != nil {
				return fail(e)
			}
			patches[i] = p
		}
		proof, e := intersect.CertifyOpenSplinePlanePair(ctx, patches[0], patches[1], intersect.OverlappingGraphTraceOptions{Discovery: discovery, Trace: o.Trace})
		if e != nil {
			return fail(e)
		}
		for index, branch := range proof.Trace.BranchGraph.Branches {
			if branch.Closed || len(branch.Cells) == 0 || len(sections) >= 64 {
				return fail(intersect.ErrUnresolved)
			}
			s := SplineShellSection{FaceA: pair[0], FaceB: pair[1], Branch: index}
			if index == 0 {
				s.TubeCells = proof.Trace.Seeds
			}
			ends := [2]*SplineShellOpenEndpoint{&s.Start, &s.End}
			for end := 0; end < 2; end++ {
				cell := branch.Cells[0]
				if end == 1 {
					cell = branch.Cells[len(branch.Cells)-1]
				}
				upper := (end == 1) != cell.Reversed
				matches := 0
				for _, term := range proof.Terminals {
					if term.Discovery.Attempt != cell.Cell || term.Discovery.Upper != upper {
						continue
					}
					matches++
					operand, side := -1, -1
					for i, mask := range term.OriginalSides {
						if mask == 0 {
							continue
						}
						if mask&(mask-1) != 0 || operand != -1 {
							return fail(intersect.ErrUnresolved)
						}
						operand = i
						for j := 0; j < 4; j++ {
							if mask == 1<<j {
								side = j
							}
						}
					}
					if operand < 0 {
						return fail(intersect.ErrUnresolved)
					}
					var hint [4]*big.Rat
					for j, q := range term.Discovery.Parameters {
						hint[j] = new(big.Rat).Set(q)
					}
					*ends[end] = SplineShellOpenEndpoint{Operand: operand, Edge: SplineTubeEndpoint{Side: geom.SurfaceEdge(side), Hint: hint}}
				}
				if matches != 1 {
					return fail(intersect.ErrUnresolved)
				}
			}
			sections = append(sections, s)
		}
	}
	solid, history, e := CombineSectionedSplineShells(ctx, a, b, op, sections, o)
	if e != nil {
		return fail(e)
	}
	return solid, history, nil
}
