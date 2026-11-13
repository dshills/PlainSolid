package collision

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"sort"
)

// ShellInfo reports the validated shell nesting tree. Parent=-1 identifies an
// outer root. Even depths bound material; odd depths bound cavities. Triangle
// indices refer to the original solid tessellation. SignedVolume is exact for
// the represented tessellation. Shell order follows first source triangle.
type ShellInfo struct {
	Parent, Depth int
	Triangles     []int
	SignedVolume  *big.Rat
}

// Shells returns caller-owned diagnostics from the same exact validation that
// prepared the locator, without rerunning collision or nesting predicates.
func (p *PointLocator) Shells() []ShellInfo {
	if p == nil {
		return nil
	}
	out := make([]ShellInfo, len(p.shells))
	for i, s := range p.shells {
		out[i] = s
		out[i].Triangles = append([]int(nil), s.Triangles...)
		sort.Ints(out[i].Triangles)
		out[i].SignedVolume = new(big.Rat).Set(s.SignedVolume)
	}
	return out
}

type AssemblyOptions struct {
	MaxSolids  int
	Validation LocationOptions
}
type AssemblyFace struct{ Solid, Face int }
type AssemblyShell struct {
	ShellInfo
	Faces []AssemblyFace
}
type AssemblyReport struct {
	Shells       []AssemblyShell
	SignedVolume *big.Rat
}

// ValidateAssembly treats all supplied boundaries as one material domain,
// without welding or unioning overlaps. Crossing, coincident or touching shells
// fail; nested cavities must already have reversed orientation. Inputs use one
// coordinate system. Empty solids are allowed. Default MaxSolids is 256; the
// total triangle cap and all geometry work follow Validation options.
func ValidateAssembly(ctx context.Context, solids []brep.Solid, o AssemblyOptions) (AssemblyReport, error) {
	empty := AssemblyReport{}
	if ctx == nil || o.MaxSolids < 0 {
		return empty, ErrInvalid
	}
	if o.MaxSolids == 0 {
		o.MaxSolids = 256
	}
	if len(solids) > o.MaxSolids {
		return empty, ErrLimit
	}
	opts, e := locationDefaults(o.Validation)
	if e != nil {
		return empty, e
	}
	count := 0
	for _, s := range solids {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		count += len(s.Tessellate().Triangles)
		if count > opts.MaxTriangles {
			return empty, ErrLimit
		}
	}
	var vertices []geom.Vec3
	var edges []brep.Edge
	var faces []brep.Face
	var ancestry []AssemblyFace
	for i, s := range solids {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		vo, eo := len(vertices), len(edges)
		vertices = append(vertices, s.Vertices()...)
		for _, edge := range s.Edges() {
			edges = append(edges, brep.Edge{Start: vo + edge.Start, End: vo + edge.End})
		}
		for j, f := range s.Faces() {
			for k := range f.Boundary {
				f.Boundary[k].Edge += eo
			}
			for h := range f.Holes {
				for k := range f.Holes[h] {
					f.Holes[h][k].Edge += eo
				}
			}
			faces = append(faces, f)
			ancestry = append(ancestry, AssemblyFace{i, j})
		}
	}
	combined, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, ErrInvalid
	}
	locator, e := NewPointLocator(ctx, combined, opts)
	if e != nil {
		return empty, e
	}
	mapping := combined.TriangleFaces()
	report := AssemblyReport{SignedVolume: new(big.Rat)}
	for _, shell := range locator.Shells() {
		record := AssemblyShell{ShellInfo: shell}
		indices := map[int]bool{}
		for _, triangle := range shell.Triangles {
			indices[mapping[triangle]] = true
		}
		var ids []int
		for id := range indices {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			record.Faces = append(record.Faces, ancestry[id])
		}
		report.Shells = append(report.Shells, record)
		report.SignedVolume.Add(report.SignedVolume, shell.SignedVolume)
	}
	return report, nil
}
