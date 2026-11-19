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
// A treatment is a chamfer or a fillet of one size, applied to one or
// more resolved edges. Its tool for an edge is a prism over the edge: a
// triangle for a chamfer, and for a fillet a kite less a cylinder that is
// tangent to both faces. The tools of convex edges are cut from the body
// and those of concave edges united with it, with boolean.Combine, in at
// most 2n + 1 calls for n edges. The result is returned only when it
// passes the checks of the specification: the tool ends on faces of the
// body, nothing else meets or touches the new faces, the volume that
// changed is the volume of the tool, and the body is not divided. An
// edge that ends at a wall or on an oblique face, a treatment that
// reaches another feature, and anything boolean.Combine refuses are
// refused with an error.
//
// The package holds no state and does not modify a solid it is given.
package blend
