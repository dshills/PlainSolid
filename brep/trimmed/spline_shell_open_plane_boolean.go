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
// A terminal on one side of both operands retains OtherSide; downstream
// assembly independently verifies both edge anchors and local continuation.
// Affine/affine pairs additionally use exact affine chart intersection when
// graph projection discovery cannot express their oblique parameterization.
// Pairs are candidate input face IDs. A failed discovery can be discarded
// only after independent exact pair separation proves no contact. Tangent or
// coincident candidates are never discarded as disjoint. No caller-authored
// trace or endpoint proof is trusted.
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
			if !isUnresolvedOpenSpline(e) {
				return fail(e)
			}
			// First attempt an independent exact exclusion. An unresolved
			// exclusion is not evidence of disjointness; only an exact affine
			// intersection fallback can recover that case here.
			var exact [2]*geom.SurfacePatch
			for i, p := range patches {
				surface, ok := p.Surface.(*geom.BSplineSurface)
				if !ok {
					return fail(geom.ErrInvalid)
				}
				exact[i], e = geom.NewSurfacePatch(surface, o.Trace.Seeds.Patches)
				if e != nil {
					return fail(e)
				}
				exact[i], e = exact[i].Trim(geom.ParameterRectangle{UMin: p.Min.X, UMax: p.Max.X, VMin: p.Min.Y, VMax: p.Max.Y})
				if e != nil {
					return fail(e)
				}
			}
			separation := proveSplinePatchPairDisjoint(ctx, exact[0], exact[1], o.Shells.Separation)
			if separation == nil {
				continue
			}
			if !isUnresolvedOpenSpline(separation) {
				return fail(separation)
			}
			var charts [2]*geom.AffineSurfaceChart
			for i := range exact {
				charts[i], e = geom.NewAffineSurfaceChart(ctx, exact[i], geom.AffineSurfaceChartOptions{MaxOperations: o.Trace.Coincidence.MaxOperations, MaxCoefficientBits: o.Trace.Coincidence.MaxCoefficientBits})
				if e != nil {
					return fail(e)
				}
			}
			section, e := proposeAffineSplineSection(ctx, pair, patches, exact, charts, o)
			if e != nil {
				return fail(e)
			}
			if len(sections) >= 64 {
				return fail(intersect.ErrUnresolved)
			}
			sections = append(sections, section)
			continue
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
					var hint [4]*big.Rat
					for j, q := range term.Discovery.Parameters {
						hint[j] = new(big.Rat).Set(q)
					}
					endpoint, e := openPlaneTerminalEndpoint(patches[0], patches[1], hint)
					if e != nil {
						return fail(e)
					}
					*ends[end] = endpoint
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
