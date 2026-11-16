package trimmed

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// Tessellate builds a separate display mesh for each native Cut component.
// It retains native vertex indices and checks sampled deflection and normal
// angles. MaxTriangles bounds the complete display and the contact proof is
// subject to the collision package's exact validation limits. The display
// components meet only at the source-declared pairs in Contacts.
func (r *ClippedPolynomialCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	if r == nil || r.native == nil {
		return nil, geom.ErrInvalid
	}
	return tessellateClippedCut(ctx, r.native.axis, r.native.material, r.shells, r.native.volumes, r.native.orders == ([2]int{1, 1}), nil, o)
}

// Tessellate displays the three-crossing Cut as separately indexed, outward
// meshes. Each declared native contact remains an independent vertex in
// each incident shell. The caller's triangle budget covers all components.
func (r *ClippedPolynomialThreeTransverseCut) Tessellate(ctx context.Context, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	if r == nil || r.cut == nil {
		return nil, geom.ErrInvalid
	}
	return tessellateClippedCut(ctx, r.axis, r.cut.material, r.cut.shells, r.cut.volumes, true, nil, o)
}

// TessellateWithVolumeTolerance also requires each display component's exact
// mesh volume to be within relativeVolumeTolerance of its independently
// integrated source volume. The tolerance is a positive finite fraction.
// If the bound cannot be reached within the tessellation limits, it returns
// parametric.ErrBudget and no meshes. The exact assembly contact check still applies.
func (r *ClippedPolynomialThreeTransverseCut) TessellateWithVolumeTolerance(ctx context.Context, o parametric.TessellationOptions,
	relativeVolumeTolerance float64) ([]mesh.Mesh, error) {
	if r == nil || r.cut == nil || !finite(relativeVolumeTolerance) || relativeVolumeTolerance <= 0 {
		return nil, geom.ErrInvalid
	}
	return tessellateClippedCut(ctx, r.axis, r.cut.material, r.cut.shells, r.cut.volumes, false, ratInset(relativeVolumeTolerance), o)
}

func tessellateClippedCut(ctx context.Context, axis int, material polynomialNodeCutMaterial, shells []clippedPolynomialCutShellLedger,
	volumes []*big.Rat, preferStripVolume bool, relativeVolumeTolerance *big.Rat, o parametric.TessellationOptions) ([]mesh.Mesh, error) {
	o, e := normalizeTrimmedTessellationOptions(ctx, o)
	if e != nil {
		return nil, e
	}
	if o.AnalyticBound {
		return nil, parametric.ErrDeflectionUnsupported
	}
	parts := material.components
	displays := make([]mesh.Mesh, len(parts))
	maximum := min(o.MaxTriangles, collision.MaxTriangles)
	remaining := maximum
	for i, part := range parts {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if remaining == 0 {
			return nil, parametric.ErrBudget
		}
		options := o
		options.MaxTriangles = remaining
		part = part.withTessellationTrimMemo(&tessellationTrimMemo{limit: tessellationTrimMemoLimit})
		var accept func(mesh.Mesh) bool
		if relativeVolumeTolerance != nil {
			// Work in six times volume, the direct sum of oriented tetrahedra.
			volume := new(big.Rat).Mul(volumes[i], big.NewRat(6, 1))
			tolerance := new(big.Rat).Mul(volume, relativeVolumeTolerance)
			accept = func(m mesh.Mesh) bool {
				return clippedCutDisplayMatchesVolume(ctx, m, volume, tolerance)
			}
		}
		display, e := part.tessellateClippedPolynomialCandidate(ctx, axis, options, accept)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e != nil && accept != nil {
			return nil, fmt.Errorf("trimmed: clipped Cut display component %d volume tolerance: %w", i, e)
		}
		if e != nil {
			return nil, fmt.Errorf("trimmed: clipped Cut display component %d: %w", i, e)
		}
		displays[i] = display
		remaining -= len(display.Triangles)
	}

	declarations := make([]collision.AssemblyVertexContact, 0, len(material.contacts))
	for _, contact := range material.contacts {
		declarations = append(declarations, collision.AssemblyVertexContact{
			A: collision.AssemblyVertex{Solid: contact.Components[0], Vertex: contact.Vertices[0]},
			B: collision.AssemblyVertex{Solid: contact.Components[1], Vertex: contact.Vertices[1]},
		})
	}
	validation := collision.VertexContactAssemblyOptions{Validation: collision.LocationOptions{MaxTriangles: maximum}}
	if _, e = collision.ValidateMeshVertexContactAssembly(ctx, displays, declarations, validation); e != nil {
		return nil, fmt.Errorf("trimmed: clipped Cut display contacts: %w", e)
	}
	if !preferStripVolume {
		return displays, nil
	}
	preferred, e := preferTransverseStripDisplays(ctx, axis, parts, shells, volumes, o, displays, remaining)
	if e != nil {
		return nil, e
	}
	if preferred != nil {
		if _, e := collision.ValidateMeshVertexContactAssembly(ctx, preferred, declarations, validation); e == nil {
			return preferred, nil
		}
		if e := ctx.Err(); e != nil {
			return nil, e
		}
	}
	return displays, nil
}

// Volume accuracy compares the exact dyadic vertices of the candidate to
// the exact source integral. Thin overlap shells can lose material digits in
// a floating-point sum even when their individual triangles are well shaped.
func clippedCutDisplayMatchesVolume(ctx context.Context, m mesh.Mesh, expected, tolerance *big.Rat) bool {
	origin := exactBoundPoint(m.Vertices[0])
	points := make([][3]*big.Rat, len(m.Vertices))
	for i, p := range m.Vertices {
		if ctx.Err() != nil {
			return false
		}
		points[i] = ddSub(exactBoundPoint(p), origin)
	}
	sum := new(big.Rat)
	for _, triangle := range m.Triangles {
		if ctx.Err() != nil {
			return false
		}
		a, b, c := points[triangle[0]], points[triangle[1]], points[triangle[2]]
		sum.Add(sum, ddDot(a, ddCross(b, c)))
	}
	return sum.Sign() > 0 && sum.Abs(sum.Sub(sum, expected)).Cmp(tolerance) <= 0
}

// Prefer closer agreement with the exact source volume after the requested
// geometric preview has qualified. A transverse component whose floor steps
// from the base to the cutter can use spare capacity for another subdivision.
func preferTransverseStripDisplays(ctx context.Context, axis int, parts []Solid, shells []clippedPolynomialCutShellLedger,
	volumes []*big.Rat, o parametric.TessellationOptions, displays []mesh.Mesh, remaining int) ([]mesh.Mesh, error) {
	if remaining == 0 {
		return nil, nil
	}
	const relativeVolumePreference = .003
	var preferred []mesh.Mesh
	for i, display := range displays {
		chambers := shells[i].Chambers
		if len(chambers) < 2 || chambers[0].Tile != 0 || chambers[len(chambers)-1].Tile == 0 {
			continue
		}
		volume, _ := volumes[i].Float64()
		if volume <= 0 || !finite(volume) {
			continue
		}
		withinPreference := func(m mesh.Mesh) bool {
			return math.Abs(m.SignedVolume()-volume) <= relativeVolumePreference*volume
		}
		if withinPreference(display) {
			continue
		}
		options := o
		options.MaxTriangles = remaining + len(display.Triangles)
		part := parts[i].withTessellationTrimMemo(&tessellationTrimMemo{limit: tessellationTrimMemoLimit})
		candidate, e := part.tessellateClippedPolynomialCandidate(ctx, axis, options, withinPreference)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e != nil {
			continue
		}
		if preferred == nil {
			preferred = slices.Clone(displays)
		}
		preferred[i] = candidate
		remaining += len(display.Triangles) - len(candidate.Triangles)
	}
	return preferred, nil
}
