package collision

import (
	"context"
	"math/big"

	"plainsolid/brep"
	"plainsolid/geom"
)

// AssemblyLocationOwner identifies an original component and, on its boundary,
// the first original face and its triangle in the component's stable order.
// An interior owner has Face and Triangle set to -1.
type AssemblyLocationOwner struct{ Solid, Face, Triangle int }

// VertexContactAssemblyLocation classifies the union of the proved independent
// components. Owners is ordered by original component index. Contacts lists
// every proved vertex pair at a shared boundary point.
type VertexContactAssemblyLocation struct {
	Location Location
	Owners   []AssemblyLocationOwner
	Contacts []AssemblyVertexContact
	Work     int
}

type vertexContactLocatedComponent struct {
	loc *PointLocator
	box geom.Box
}

// VertexContactAssemblyLocator is an immutable, reusable exact classifier for
// the bounded planar component contact graph. Its zero value locates the empty
// assembly, and it is safe for concurrent read-only queries.
type VertexContactAssemblyLocator struct {
	components []vertexContactLocatedComponent
	contacts   map[geom.Vec3][]AssemblyVertexContact
	report     VertexContactAssemblyReport
	maxWork    int
}

// NewVertexContactAssemblyLocator proves the same original component contract
// as ValidateVertexContactAssembly and reuses the exact component snapshots for
// subsequent queries. It publishes no locator on incomplete preparation.
func NewVertexContactAssemblyLocator(ctx context.Context, solids []brep.Solid, declarations []AssemblyVertexContact, o VertexContactAssemblyOptions) (*VertexContactAssemblyLocator, error) {
	var prepared []vertexContactComponent
	report, err := prepareVertexContactAssembly(ctx, solids, declarations, o, &prepared)
	if err != nil {
		return nil, err
	}
	l := &VertexContactAssemblyLocator{components: make([]vertexContactLocatedComponent, len(prepared)), contacts: make(map[geom.Vec3][]AssemblyVertexContact), report: report, maxWork: o.MaxQueryWork}
	for i, p := range prepared {
		l.components[i] = vertexContactLocatedComponent{p.loc, p.box}
	}
	for _, contact := range report.Contacts {
		p := prepared[contact.A.Solid].mesh.Vertices[contact.A.Vertex]
		l.contacts[p] = append(l.contacts[p], contact)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l, nil
}

// Report returns a caller-owned copy of the original exact preparation report.
func (l *VertexContactAssemblyLocator) Report() VertexContactAssemblyReport {
	if l == nil || l.report.SignedVolume == nil {
		return VertexContactAssemblyReport{}
	}
	r := l.report
	r.SignedVolume = new(big.Rat).Set(r.SignedVolume)
	r.Contacts = append([]AssemblyVertexContact(nil), r.Contacts...)
	return r
}

// Locate classifies a represented point without a geometric tolerance. The
// assembly query budget counts component bounds and all exact per-component
// work, in addition to each component's individual query limit. A failure
// returns no partial classification or ancestry.
func (l *VertexContactAssemblyLocator) Locate(ctx context.Context, p geom.Vec3) (VertexContactAssemblyLocation, error) {
	var zero VertexContactAssemblyLocation
	if l == nil || ctx == nil || !p.IsFinite() {
		return zero, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	maxWork := l.maxWork
	if maxWork == 0 {
		maxWork = 2000000
	}
	r := VertexContactAssemblyLocation{Location: Outside}
	for i, c := range l.components {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		if r.Work == maxWork {
			return zero, ErrLimit
		}
		r.Work++
		if !c.box.Contains(p) {
			continue
		}
		remaining := maxWork - r.Work
		if remaining <= 0 {
			return zero, ErrLimit
		}
		loc := *c.loc
		loc.options.MaxWork = min(remaining, loc.options.MaxWork)
		result, err := loc.Locate(ctx, p)
		if err != nil {
			return zero, err
		}
		r.Work += result.Work
		if result.Location == Outside {
			continue
		}
		if r.Location == Inside || result.Location == Inside && r.Location != Outside {
			return zero, ErrInvalid
		}
		r.Location = result.Location
		r.Owners = append(r.Owners, AssemblyLocationOwner{i, result.Face, result.Triangle})
	}
	if r.Location == OnBoundary {
		r.Contacts = append([]AssemblyVertexContact(nil), l.contacts[p]...)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return r, nil
}
