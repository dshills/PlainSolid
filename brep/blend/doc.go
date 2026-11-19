// Package blend treats straight edges of a solid bounded by planes and
// circular cylinders, as specified in specs/general-edge-treatment.
//
// Input domain. An edge is named by its two ends and must resolve to one
// chain of straight edges of the solid, each between the same two planar
// faces, which must not be parallel. Anything else is refused with an
// error: an edge with a cylinder on one side, a seam between coplanar
// faces, a reference that covers part of an edge, or one that no edge
// joins.
//
// The package holds no state and does not modify a solid it is given.
package blend
