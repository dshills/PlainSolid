package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type mappedChainClosingSource struct {
	trims    [][]PCurve
	surfaces []geom.Surface
	reversed []bool
}

func (r MappedSplineChainAgreement) mappedChainClosingSource(closing [2][]PCurve, reversed [2]bool, planar []MappedChainPlanarFace, seams [][2]MappedChainClosedUse) (mappedChainClosingSource, error) {
	var zero mappedChainClosingSource
	if len(r.cuts) < 2 || reversed[0] == reversed[1] || len(planar) == 0 || len(planar) > 14 || len(seams) > 64 {
		return zero, geom.ErrInvalid
	}
	n := 2 + len(planar)
	g := mappedChainClosingSource{trims: make([][]PCurve, n), surfaces: make([]geom.Surface, n), reversed: make([]bool, n)}
	for i, loop := range closing {
		if len(loop) < 1 || len(loop) > 16 {
			return zero, geom.ErrInvalid
		}
		g.trims[i], g.surfaces[i], g.reversed[i] = loop, r.supports[i], reversed[i]
	}
	for i, p := range planar {
		if len(p.Outer) < 2 || len(p.Outer) > 16 {
			return zero, geom.ErrInvalid
		}
		g.trims[i+2], g.surfaces[i+2], g.reversed[i+2] = p.Outer, p.Surface, p.Reversed
	}
	return g, nil
}

func (g mappedChainClosingSource) proveSeams(ctx context.Context, seams [][2]MappedChainClosedUse, o edgecheck.Options) error {
	trims := g.trims
	faceCount := len(trims)
	used := make([][]bool, faceCount)
	adjacent := make([][]int, faceCount)
	adjacent[0], adjacent[1] = []int{1}, []int{0}
	for f, loop := range trims {
		used[f] = make([]bool, len(loop))
	}
	for _, seam := range seams {
		a, b := seam[0].Face, seam[1].Face
		if a == b {
			return geom.ErrInvalid
		}
		for _, u := range seam {
			if u.Face < 0 || u.Face >= faceCount || u.Trim < 0 || u.Trim >= len(trims[u.Face]) || used[u.Face][u.Trim] {
				return geom.ErrInvalid
			}
			used[u.Face][u.Trim] = true
		}
		adjacent[a] = append(adjacent[a], b)
		adjacent[b] = append(adjacent[b], a)
	}
	for _, loop := range used {
		for _, paired := range loop {
			if !paired {
				return geom.ErrInvalid
			}
		}
	}
	connected := make([]bool, faceCount)
	connected[0] = true
	queue := []int{0}
	for i := 0; i < len(queue); i++ {
		for _, f := range adjacent[queue[i]] {
			if !connected[f] {
				connected[f] = true
				queue = append(queue, f)
			}
		}
	}
	if len(queue) != faceCount {
		return geom.ErrConvergence
	}

	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	for i, seam := range seams {
		a, b := seam[0], seam[1]
		da, err := polynomialTrimDescriptor(ctx, trims[a.Face][a.Trim], false, limits)
		if err != nil {
			return fmt.Errorf("closing seam %d: %w", i, err)
		}
		db, err := polynomialTrimDescriptor(ctx, trims[b.Face][b.Trim], g.reversed[a.Face] == g.reversed[b.Face], limits)
		if err != nil {
			return fmt.Errorf("closing seam %d: %w", i, err)
		}
		equal, err := compareTrimCompositionsOnPlacedSplineSupports(ctx, g.surfaces[a.Face], da, g.surfaces[b.Face], db, o)
		if err != nil {
			return fmt.Errorf("closing seam %d agreement: %w", i, err)
		}
		if !equal {
			return fmt.Errorf("closing seam %d agreement: %w", i, geom.ErrConvergence)
		}
	}
	return nil
}

// MappedChainShellProofOptions bounds the independent per-seam agreement,
// per-face UV domain and topology, and affine support proofs.
type MappedChainShellProofOptions struct {
	Closing                         edgecheck.Options
	Topology                        trimvalidate.Options
	Affine                          geom.AffineSurfaceChartOptions
	DomainSubdivisions, DomainDepth int
}

// MappedChainShellTopology counts the original face, edge and corner
// identities. The whole mapped spline chain is one original topological edge.
type MappedChainShellTopology struct {
	Faces, Edges, Vertices, EulerCharacteristic int
}

// MappedSplineChainShell retains the original chart regions and closed
// oriented source incidence. Complete source embedding and material admission
// require additional geometric proofs.
type MappedSplineChainShell struct {
	boundary MappedSplineChainAgreement
	closing  [2][]PCurve
	reversed [2]bool
	planar   []MappedChainPlanarFace
	seams    [][2]MappedChainClosedUse
	topology MappedChainShellTopology
}

// Topology returns the exact original shell's certified incidence counts.
func (r MappedSplineChainShell) Topology() MappedChainShellTopology { return r.topology }

// CertifyMappedSplineChainShell proves the original UV disks, affine closing
// supports, continuous agreement of every oriented coedge pair, and closure
// and connectivity of their source topology. Corner equivalence is inferred
// only from matching ends of proved paired coedges. This proof uses no mesh
// samples, display tolerance, or stored intersection flags.
func CertifyMappedSplineChainShell(ctx context.Context, boundary MappedSplineChainAgreement, closing [2][]PCurve, reversed [2]bool, planar []MappedChainPlanarFace, seams [][2]MappedChainClosedUse, o MappedChainShellProofOptions) (MappedSplineChainShell, error) {
	var zero MappedSplineChainShell
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	source, err := boundary.mappedChainClosingSource(closing, reversed, planar, seams)
	if err != nil {
		return zero, err
	}
	if err := source.proveSeams(ctx, seams, o.Closing); err != nil {
		return zero, err
	}

	limits := geom.SurfacePatchOptions{MaxOperations: o.Closing.MaxWork, MaxCoefficientBits: o.Closing.MaxBits}
	eval := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Closing.MaxWork, MaxCoefficientBits: o.Closing.MaxBits}
	points := make([][][3]string, len(source.trims))
	for fi, trim := range source.trims {
		patch, err := exactSplineSupportPatch(source.surfaces[fi], limits)
		if err != nil {
			return zero, fmt.Errorf("source face %d support: %w", fi, err)
		}
		outer := trim
		corners := trim
		if fi < 2 {
			outer, err = boundary.mappedChainOwnerOuter(fi, trim)
			if err != nil {
				return zero, fmt.Errorf("source face %d chain: %w", fi, err)
			}
			corners = append([]PCurve{outer[0]}, trim...)
		} else if _, err := geom.NewAffineSurfaceChart(ctx, patch, o.Affine); err != nil {
			return zero, fmt.Errorf("source face %d affine support: %w", fi, err)
		}
		if _, _, err := proveMixedExactTrimLoops(ctx, patch.Domain(), [][]PCurve{outer}, o.DomainSubdivisions, o.DomainDepth, o.Topology); err != nil {
			return zero, fmt.Errorf("source face %d outer loop: %w", fi, err)
		}
		for _, c := range corners {
			uv, err := mixedExactEvaluate(c, new(big.Rat))
			if err != nil {
				return zero, fmt.Errorf("source face %d corner: %w", fi, err)
			}
			xyz, err := patch.EvaluateRational(ctx, uv[0], uv[1], eval)
			if err != nil {
				return zero, fmt.Errorf("source face %d corner: %w", fi, err)
			}
			points[fi] = append(points[fi], polygonPointKey(xyz.Point))
		}
	}
	topology, err := certifyMappedChainSourceCorners(points, source.reversed, seams)
	if err != nil {
		return zero, fmt.Errorf("mapped chain source corners: %w", err)
	}
	planar = slices.Clone(planar)
	for i := range planar {
		planar[i].Outer = slices.Clone(planar[i].Outer)
		planar[i].Mesh = MixedExactAffineMeshOptions{}
	}
	for i := range closing {
		closing[i] = slices.Clone(closing[i])
	}
	return MappedSplineChainShell{boundary: boundary, closing: closing, reversed: reversed, planar: planar, seams: slices.Clone(seams), topology: topology}, nil
}

func certifyMappedChainSourceCorners(points [][][3]string, reversed []bool, seams [][2]MappedChainClosedUse) (MappedChainShellTopology, error) {
	var zero MappedChainShellTopology
	offsets := make([]int, len(points)+1)
	for i, p := range points {
		offsets[i+1] = offsets[i] + len(p)
	}
	weld := newExactVertexWeld(offsets[len(points)])
	join := func(fa, ia, fb, ib int) error {
		if points[fa][ia] != points[fb][ib] {
			return geom.ErrConvergence
		}
		weld.join(offsets[fa]+ia, offsets[fb]+ib)
		return nil
	}
	for _, corner := range []int{0, 1} {
		if err := join(0, corner, 1, corner); err != nil {
			return zero, err
		}
	}
	for _, seam := range seams {
		var ends [2][2]int
		for i, u := range seam {
			if u.Face < 2 {
				u.Trim++
			}
			ends[i] = [2]int{u.Trim, (u.Trim + 1) % len(points[u.Face])}
		}
		a, b := seam[0].Face, seam[1].Face
		if reversed[a] == reversed[b] {
			ends[1][0], ends[1][1] = ends[1][1], ends[1][0]
		}
		for i, j := range ends[0] {
			if err := join(a, j, b, ends[1][i]); err != nil {
				return zero, err
			}
		}
	}
	// Each face corner belongs to exactly two proved endpoint joins. The
	// weld classes are therefore the connected degree-two source vertex links.
	// Distinct vertex links cannot claim the same exact model point.
	class := make(map[[3]string]int)
	for i, p := range points {
		face := make(map[int]bool)
		for j, point := range p {
			id := weld.root(offsets[i] + j)
			previous, ok := class[point]
			if face[id] || (ok && previous != id) {
				return zero, geom.ErrConvergence
			}
			class[point] = id
			face[id] = true
		}
	}
	faces, edges, vertices := len(points), len(seams)+1, len(class)
	return MappedChainShellTopology{Faces: faces, Edges: edges, Vertices: vertices, EulerCharacteristic: vertices - edges + faces}, nil
}

// TessellateClosedCertified uses the original source recipe to construct a
// separately certified closed display. A nil planarMesh uses default options
// for every closing face. Individual planar budgets are temporary.
func (r MappedSplineChainShell) TessellateClosedCertified(ctx context.Context, planarMesh []MixedExactAffineMeshOptions, o MappedChainClosedMeshOptions) (CertifiedMappedChainClosedMesh, error) {
	planar := r.planar
	if planarMesh != nil {
		if len(planarMesh) != len(planar) {
			return CertifiedMappedChainClosedMesh{}, geom.ErrInvalid
		}
		planar = slices.Clone(planar)
		for i, options := range planarMesh {
			planar[i].Mesh = options
		}
	}
	return r.boundary.TessellateClosedCertified(ctx, r.closing, r.reversed, planar, r.seams, o)
}
