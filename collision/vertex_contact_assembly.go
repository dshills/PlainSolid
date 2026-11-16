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
	// Zero selects 256 solids, 1024 declared or discovered point contacts, and one million cross
	// broad-phase comparisons over the complete assembly.
	MaxSolids, MaxDeclaredContacts, MaxCrossPairs int
	// MaxQueryWork bounds each prepared assembly lookup across all possible
	// components. Zero selects two million units.
	MaxQueryWork int
	// Validation bounds each component's exact embedding and location
	// queries. Its triangle limit also applies to the whole assembly.
	Validation LocationOptions
	// AllowCavities permits one exterior and directly enclosed void shells
	// per component. Each shell gets an independent containment witness.
	AllowCavities bool
	// CrossBVH selects a 3D hierarchy for cross-component contacts.
	// MaxCrossPairs then counts visited tree pairs, including leaves.
	CrossBVH bool
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
// graph. Every component must be outward and embedded; by default it has
// exactly one shell. Cross-component
// intersections must be exactly the declared points and both incident
// triangles must use their declared original vertices. Edge/face contact,
// hidden containment and undeclared or unrealized contacts are rejected.
// One noncontact vertex per shell and potentially overlapping mate must be
// available to prove exterior membership. No partial report is published.
func ValidateVertexContactAssembly(ctx context.Context, solids []brep.Solid, declarations []AssemblyVertexContact, o VertexContactAssemblyOptions) (VertexContactAssemblyReport, error) {
	return prepareVertexContactAssembly(ctx, solids, declarations, o, false, nil)
}

// DiscoverVertexContactAssembly explicitly admits all proved original vertex
// contacts between independently embedded, disjoint-interior planar solids.
// It discovers a complete ledger using the same predicates and limits as
// ValidateVertexContactAssembly. No other kind of contact is admitted.
func DiscoverVertexContactAssembly(ctx context.Context, solids []brep.Solid, o VertexContactAssemblyOptions) (VertexContactAssemblyReport, error) {
	return prepareVertexContactAssembly(ctx, solids, nil, o, true, nil)
}

// ValidateMeshVertexContactAssembly verifies the exact declared vertex contacts
// of independently indexed display meshes. It applies the same embedding,
// orientation, containment and contact checks as ValidateVertexContactAssembly.
func ValidateMeshVertexContactAssembly(ctx context.Context, components []mesh.Mesh, declarations []AssemblyVertexContact,
	o VertexContactAssemblyOptions) (VertexContactAssemblyReport, error) {
	return prepareVertexContactMeshes(ctx, len(components), func(i int) (mesh.Mesh, []int) {
		return components[i], nil
	}, declarations, o, false, nil)
}

func prepareVertexContactAssembly(ctx context.Context, solids []brep.Solid, declarations []AssemblyVertexContact, o VertexContactAssemblyOptions, discover bool, prepared *[]vertexContactComponent) (VertexContactAssemblyReport, error) {
	return prepareVertexContactMeshes(ctx, len(solids), func(i int) (mesh.Mesh, []int) {
		return solids[i].Tessellate(), solids[i].TriangleFaces()
	}, declarations, o, discover, prepared)
}

func prepareVertexContactMeshes(ctx context.Context, count int, source func(int) (mesh.Mesh, []int), declarations []AssemblyVertexContact,
	o VertexContactAssemblyOptions, discover bool, prepared *[]vertexContactComponent) (VertexContactAssemblyReport, error) {
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
	if count > o.MaxSolids || len(declarations) > o.MaxDeclaredContacts {
		return zero, ErrLimit
	}
	validation, err := locationDefaults(o.Validation)
	if err != nil {
		return zero, err
	}
	report := VertexContactAssemblyReport{SignedVolume: new(big.Rat), Contacts: append([]AssemblyVertexContact(nil), declarations...)}
	components := make([]vertexContactComponent, count)
	triangles := 0
	for i := range count {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		m, faces := source(i)
		triangles += len(m.Triangles)
		if triangles > validation.MaxTriangles {
			return zero, ErrLimit
		}
		loc, err := NewMeshPointLocator(ctx, m, faces, validation)
		if err != nil {
			return zero, fmt.Errorf("assembly component %d: %w", i, err)
		}
		if len(loc.shells) != 1 {
			if !o.AllowCavities {
				return zero, fmt.Errorf("%w: assembly component %d must contain exactly one shell", ErrInvalid, i)
			}
			if !oneExteriorWithCavities(loc.shells) {
				return zero, fmt.Errorf("%w: assembly component %d must have one exterior and only directly enclosed cavities", ErrInvalid, i)
			}
		}
		box, err := geom.Bounds(m.Vertices)
		if err != nil {
			return zero, fmt.Errorf("%w: assembly component %d bounds", ErrInvalid, i)
		}
		components[i] = vertexContactComponent{m, box, loc}
		for _, shell := range loc.shells {
			report.SignedVolume.Add(report.SignedVolume, shell.SignedVolume)
		}
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
	sortAssemblyVertexContacts(report.Contacts)
	pairs := map[[2]int]*vertexContactPair{}
	for i, c := range report.Contacts {
		k := [2]int{c.A.Solid, c.B.Solid}
		p := pairs[k]
		if p == nil {
			p = newVertexContactPair()
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
			visit := func(ta, tb int, k Kind, points []point) error {
				if (p != nil || discover) && k == Touching && len(points) == 1 {
					if p == nil {
						p = newVertexContactPair()
					}
					for _, va := range a.mesh.Triangles[ta] {
						for _, vb := range b.mesh.Triangles[tb] {
							key := [2]int{va, vb}
							id, known := p.claims[key]
							if !known && (!discover || a.mesh.Vertices[va] != b.mesh.Vertices[vb]) {
								continue
							}
							if !same(points[0], exact(a.mesh.Vertices[va])) {
								continue
							}
							if !known {
								if p.used[0][va] || p.used[1][vb] {
									return ErrInvalid
								}
								if len(report.Contacts) >= o.MaxDeclaredContacts {
									return ErrLimit
								}
								id = len(report.Contacts)
								p.claims[key], p.used[0][va], p.used[1][vb] = id, true, true
								report.Contacts = append(report.Contacts, AssemblyVertexContact{AssemblyVertex{i, va}, AssemblyVertex{j, vb}})
								seen = append(seen, false)
							}
							seen[id] = true
							report.ContactPairs++
							return nil
						}
					}
				}
				return fmt.Errorf("%w: undeclared or nonvertex contact between components %d and %d", ErrInvalid, i, j)
			}
			options := Options{MaxPairs: remaining, MaxContacts: remaining}
			var cross Report
			if o.CrossBVH {
				cross, err = vertexContactCrossBVH(ctx, a.mesh, b.mesh, options, visit)
			} else {
				cross, err = sweepIntersections(ctx, a.mesh, b.mesh, false, options, visit)
			}
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
	if discover {
		sortAssemblyVertexContacts(report.Contacts)
	}
	if prepared != nil {
		*prepared = components
	}
	return report, nil
}

func newVertexContactPair() *vertexContactPair {
	return &vertexContactPair{make(map[[2]int]int), [2]map[int]bool{make(map[int]bool), make(map[int]bool)}}
}

func sortAssemblyVertexContacts(contacts []AssemblyVertexContact) {
	sort.Slice(contacts, func(i, j int) bool {
		a, b := contacts[i], contacts[j]
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
}

func oneExteriorWithCavities(shells []ShellInfo) bool {
	outer := -1
	for i, s := range shells {
		if s.Depth == 0 {
			if outer >= 0 {
				return false
			}
			outer = i
		}
	}
	if outer < 0 {
		return false
	}
	for i, s := range shells {
		if i != outer && (s.Depth != 1 || s.Parent != outer) {
			return false
		}
	}
	return true
}

func validateComponentOutside(ctx context.Context, component, other vertexContactComponent, skip map[int]bool) error {
	if len(component.loc.shells) > 1 {
		// Boundary intersection has already been excluded except for
		// proved isolated vertex contacts. The other part has constant
		// material membership on each connected shell minus those points.
		// Disjoint shell components cannot share a single witness.
		for _, shell := range component.loc.shells {
			found := false
		shellWitness:
			for _, ti := range shell.Triangles {
				for _, vi := range component.mesh.Triangles[ti] {
					if skip[vi] {
						continue
					}
					if err := validateAssemblyPointOutside(ctx, component.mesh.Vertices[vi], other); err != nil {
						return err
					}
					found = true
					break shellWitness
				}
			}
			if !found {
				return fmt.Errorf("%w: no noncontact shell vertex for containment proof", ErrInvalid)
			}
		}
		return nil
	}
	for i, p := range component.mesh.Vertices {
		if skip[i] {
			continue
		}
		return validateAssemblyPointOutside(ctx, p, other)
	}
	return fmt.Errorf("%w: no noncontact component vertex for containment proof", ErrInvalid)
}

func validateAssemblyPointOutside(ctx context.Context, p geom.Vec3, other vertexContactComponent) error {
	if err := ctx.Err(); err != nil {
		return err
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

func closedBoxesMeet(a, b geom.Box) bool {
	return a.Min.X <= b.Max.X && b.Min.X <= a.Max.X && a.Min.Y <= b.Max.Y && b.Min.Y <= a.Max.Y && a.Min.Z <= b.Max.Z && b.Min.Z <= a.Max.Z
}
