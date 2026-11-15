package collision

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// AssemblyVertex names an original model vertex in a separate component.
type AssemblyVertex struct{ Solid, Vertex int }

// AssemblyVertexContact permits one exact point intersection. The original
// component vertex IDs remain distinct; neither shell is welded to the other.
type AssemblyVertexContact struct{ A, B AssemblyVertex }

type VertexContactAssemblyOptions struct {
	// Zero selects 256 solids, 1024 declarations, and one million cross
	// broad-phase comparisons over the complete assembly.
	MaxSolids, MaxDeclaredContacts, MaxCrossPairs int
	// MaxQueryWork bounds each prepared assembly lookup across all possible
	// components. Zero selects two million units.
	MaxQueryWork int
	// Validation bounds each component's exact embedding and location
	// queries. Its triangle limit also applies to the whole assembly.
	Validation LocationOptions
}

// VertexContactAssemblyReport contains the proved, canonicalized contact
// declarations and the exact sum of the independent component volumes.
type VertexContactAssemblyReport struct {
	Contacts                 []AssemblyVertexContact
	SignedVolume             *big.Rat
	BroadPairs, ContactPairs int // component and cross-triangle broad phase; accepted triangle contacts
}

type vertexContactComponent struct {
	mesh mesh.Mesh
	box  geom.Box
	loc  *PointLocator
}

type vertexContactPair struct {
	claims map[[2]int]int
	used   [2]map[int]bool
}

// ValidateVertexContactAssembly proves an opt-in planar component contact
// graph. Every component must be one outward, embedded shell. Cross-component
// intersections must be exactly the declared points and both incident
// triangles must use their declared original vertices. Edge/face contact,
// hidden containment and undeclared or unrealized contacts are rejected.
// One noncontact vertex per component and potentially overlapping mate must
// be available to prove exterior membership. No partial report is published.
func ValidateVertexContactAssembly(ctx context.Context, solids []brep.Solid, declarations []AssemblyVertexContact, o VertexContactAssemblyOptions) (VertexContactAssemblyReport, error) {
	return prepareVertexContactAssembly(ctx, solids, declarations, o, nil)
}

func prepareVertexContactAssembly(ctx context.Context, solids []brep.Solid, declarations []AssemblyVertexContact, o VertexContactAssemblyOptions, prepared *[]vertexContactComponent) (VertexContactAssemblyReport, error) {
	var zero VertexContactAssemblyReport
	if ctx == nil || o.MaxSolids < 0 || o.MaxDeclaredContacts < 0 || o.MaxCrossPairs < 0 || o.MaxQueryWork < 0 {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if o.MaxSolids == 0 {
		o.MaxSolids = 256
	}
	if o.MaxDeclaredContacts == 0 {
		o.MaxDeclaredContacts = 1024
	}
	if o.MaxCrossPairs == 0 {
		o.MaxCrossPairs = DefaultMaxPairs
	}
	if len(solids) > o.MaxSolids || len(declarations) > o.MaxDeclaredContacts {
		return zero, ErrLimit
	}
	validation, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	report := VertexContactAssemblyReport{SignedVolume: new(big.Rat), Contacts: append([]AssemblyVertexContact(nil), declarations...)}
	components := make([]vertexContactComponent, len(solids))
	triangles := 0
	for i, solid := range solids {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		m := solid.Tessellate()
		triangles += len(m.Triangles)
		if triangles > validation.MaxTriangles {
			return zero, ErrLimit
		}
		loc, err := NewMeshPointLocator(ctx, m, solid.TriangleFaces(), validation)
		if err != nil {
			return zero, fmt.Errorf("assembly component %d: %w", i, err)
		}
		if len(loc.shells) != 1 {
			return zero, fmt.Errorf("%w: assembly component %d must contain exactly one shell", ErrInvalid, i)
		}
		box, err := geom.Bounds(m.Vertices)
		if err != nil {
			return zero, fmt.Errorf("%w: assembly component %d bounds", ErrInvalid, i)
		}
		components[i] = vertexContactComponent{m, box, loc}
		report.SignedVolume.Add(report.SignedVolume, loc.shells[0].SignedVolume)
	}
	for i, c := range report.Contacts {
		if c.A.Solid == c.B.Solid {
			return zero, ErrInvalid
		}
		for _, v := range [2]AssemblyVertex{c.A, c.B} {
			if v.Solid < 0 || v.Solid >= len(components) || v.Vertex < 0 || v.Vertex >= len(components[v.Solid].mesh.Vertices) {
				return zero, ErrInvalid
			}
		}
		if components[c.A.Solid].mesh.Vertices[c.A.Vertex] != components[c.B.Solid].mesh.Vertices[c.B.Vertex] {
			return zero, fmt.Errorf("%w: declared component vertices do not coincide exactly", ErrInvalid)
		}
		if c.A.Solid > c.B.Solid {
			report.Contacts[i] = AssemblyVertexContact{c.B, c.A}
		}
	}
	sort.Slice(report.Contacts, func(i, j int) bool {
		a, b := report.Contacts[i], report.Contacts[j]
		if a.A.Solid != b.A.Solid {
			return a.A.Solid < b.A.Solid
		}
		if a.B.Solid != b.B.Solid {
			return a.B.Solid < b.B.Solid
		}
		if a.A.Vertex != b.A.Vertex {
			return a.A.Vertex < b.A.Vertex
		}
		return a.B.Vertex < b.B.Vertex
	})
	pairs := map[[2]int]*vertexContactPair{}
	for i, c := range report.Contacts {
		k := [2]int{c.A.Solid, c.B.Solid}
		p := pairs[k]
		if p == nil {
			p = &vertexContactPair{make(map[[2]int]int), [2]map[int]bool{make(map[int]bool), make(map[int]bool)}}
			pairs[k] = p
		}
		if p.used[0][c.A.Vertex] || p.used[1][c.B.Vertex] {
			return zero, fmt.Errorf("%w: duplicate vertex contact for one pair", ErrInvalid)
		}
		p.claims[[2]int{c.A.Vertex, c.B.Vertex}] = i
		p.used[0][c.A.Vertex], p.used[1][c.B.Vertex] = true, true
	}
	seen := make([]bool, len(report.Contacts))
	for i, a := range components {
		for j := i + 1; j < len(components); j++ {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			if report.BroadPairs >= o.MaxCrossPairs {
				return zero, ErrLimit
			}
			report.BroadPairs++
			b := components[j]
			if !closedBoxesMeet(a.box, b.box) {
				continue
			}
			p := pairs[[2]int{i, j}]
			remaining := o.MaxCrossPairs - report.BroadPairs
			if remaining <= 0 {
				return zero, ErrLimit
			}
			cross, err := sweepIntersections(ctx, a.mesh, b.mesh, false, Options{MaxPairs: remaining, MaxContacts: remaining}, func(ta, tb int, k Kind, points []point) error {
				if p != nil && k == Touching && len(points) == 1 {
					for _, va := range a.mesh.Triangles[ta] {
						for _, vb := range b.mesh.Triangles[tb] {
							if id, ok := p.claims[[2]int{va, vb}]; ok && same(points[0], exact(a.mesh.Vertices[va])) {
								seen[id] = true
								report.ContactPairs++
								return nil
							}
						}
					}
				}
				return fmt.Errorf("%w: undeclared or nonvertex contact between components %d and %d", ErrInvalid, i, j)
			})
			if err != nil {
				return zero, err
			}
			if !cross.Complete {
				return zero, ErrLimit
			}
			report.BroadPairs += cross.BroadPhasePairs
			var skip [2]map[int]bool
			if p != nil {
				skip = p.used
			}
			for k, c := range [2]vertexContactComponent{a, b} {
				other := [2]vertexContactComponent{b, a}[k]
				if err := validateComponentOutside(ctx, c, other, skip[k]); err != nil {
					return zero, fmt.Errorf("components %d and %d: %w", i, j, err)
				}
			}
		}
	}
	for _, hit := range seen {
		if !hit {
			return zero, fmt.Errorf("%w: vertex contact was not realized", ErrInvalid)
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if prepared != nil {
		*prepared = components
	}
	return report, nil
}

func validateComponentOutside(ctx context.Context, component, other vertexContactComponent, skip map[int]bool) error {
	for i, p := range component.mesh.Vertices {
		if skip[i] {
			continue
		}
		if !other.box.Contains(p) {
			return nil
		}
		r, err := other.loc.Locate(ctx, p)
		if err != nil {
			return err
		}
		if r.Location != Outside {
			return fmt.Errorf("%w: component interiors are not disjoint", ErrInvalid)
		}
		return nil
	}
	return fmt.Errorf("%w: no noncontact component vertex for containment proof", ErrInvalid)
}

func closedBoxesMeet(a, b geom.Box) bool {
	return a.Min.X <= b.Max.X && b.Min.X <= a.Max.X && a.Min.Y <= b.Max.Y && b.Min.Y <= a.Max.Y && a.Min.Z <= b.Max.Z && b.Min.Z <= a.Max.Z
}
