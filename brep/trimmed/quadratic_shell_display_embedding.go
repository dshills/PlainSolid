package trimmed

import (
	"context"
	"plainsolid/collision"
	"plainsolid/geom"
)

// DisplayEmbeddingStatus distinguishes an unproved result from an exact
// forbidden contact and a complete embedded closed-manifold certificate.
type DisplayEmbeddingStatus uint8

const (
	DisplayEmbeddingUnresolved DisplayEmbeddingStatus = iota
	DisplayEmbeddingCertified
	DisplayEmbeddingContact
)

// ClosedDisplayEmbeddingReport applies only to the current represented mesh,
// not to a smooth source shell or a subsequently edited copy. ContactFaces maps
// collision contacts to current OriginalFace entries (-1 if ancestry missing).
// A positive exact contact remains evidence even when a later work limit makes
// Collision.Complete false. Certified is never returned on any error.
type ClosedDisplayEmbeddingReport struct {
	Status                    DisplayEmbeddingStatus
	ClosedManifold, Connected bool
	Collision                 collision.Report
	ContactFaces              [][2]int
}

// CheckDisplayEmbedding revalidates the caller-owned mesh as one connected
// closed manifold, then classifies all non-incidence triangle intersections
// exactly. Shared indexed edge/vertex contact is expected; coplanar overlap,
// crossings and contact beyond that shared feature invalidate embedding.
// Collision's triangle/pair/contact limits bound work. Cancellation is checked
// before topology, during connectedness, and throughout BVH construction and
// pair traversal. MaxPairs counts BVH node pairs plus leaf triangle candidates.
// Disconnected or nested shell assemblies require their separate assembly API.
func (s CertifiedClosedQuadraticShell) CheckDisplayEmbedding(ctx context.Context, o collision.Options) (ClosedDisplayEmbeddingReport, error) {
	result := ClosedDisplayEmbeddingReport{}
	if ctx == nil || o.MaxPairs < 0 || o.MaxContacts < 0 {
		return result, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return result, e
	}
	m := s.Mesh
	if len(m.Triangles) > collision.MaxTriangles {
		return result, collision.ErrLimit
	}
	if len(m.Vertices) > 3*len(m.Triangles) {
		return result, collision.ErrLimit
	}
	if len(m.Triangles) == 0 {
		return result, geom.ErrInvalid
	}
	if e := m.ValidateSolid(); e != nil {
		return result, e
	}
	vertices := make([]int, len(m.Vertices))
	for i := range vertices {
		vertices[i] = i
	}
	if e := validateQuadraticJunctionFans(m, vertices); e != nil {
		return result, e
	}
	result.ClosedManifold = true
	adjacency := make([][]int, len(m.Vertices))
	for _, t := range m.Triangles {
		for j, a := range t {
			b := t[(j+1)%3]
			adjacency[a] = append(adjacency[a], b)
			adjacency[b] = append(adjacency[b], a)
		}
	}
	visited := make([]bool, len(m.Vertices))
	visited[0] = true
	stack := []int{0}
	count := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return result, e
		}
		a := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		for _, b := range adjacency[a] {
			if !visited[b] {
				visited[b] = true
				stack = append(stack, b)
			}
		}
	}
	if count != len(m.Vertices) {
		return result, geom.ErrConvergence
	}
	result.Connected = true
	if e := ctx.Err(); e != nil {
		return result, e
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, o)
	result.Collision = report
	face := func(triangle int) int {
		if triangle < 0 || triangle >= len(s.OriginalFace) {
			return -1
		}
		return s.OriginalFace[triangle]
	}
	for _, contact := range report.Contacts {
		result.ContactFaces = append(result.ContactFaces, [2]int{face(contact.TriangleA), face(contact.TriangleB)})
	}
	if len(report.Contacts) > 0 {
		result.Status = DisplayEmbeddingContact
	}
	if canceled := ctx.Err(); canceled != nil {
		return result, canceled
	}
	if e != nil {
		return result, e
	}
	if !report.Complete {
		return result, geom.ErrConvergence
	}
	if len(report.Contacts) == 0 {
		result.Status = DisplayEmbeddingCertified
	}
	return result, nil
}
