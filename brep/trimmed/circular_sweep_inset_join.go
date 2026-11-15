package trimmed

import "context"

// CircularSweepInset retains both inputs' original edge and vertex order.
// GuestEdgeOffset and GuestVertexOffset locate guest topology in Solid.
// HostFaces maps each original host face to all its surviving fragments;
// GuestFaces maps each original guest face, or -1 for its consumed cap.
type CircularSweepInset struct {
	Solid                              Solid
	HostFaces                          [][]int
	GuestFaces                         []int
	GuestEdgeOffset, GuestVertexOffset int
	HostCap, GuestCap                  int
}

// JoinExactCircularSweepInset forms the exact union of two positive
// extrusions at a shared cap elevation. The guest's outer wire must lie
// strictly inside the host material, clear of host holes. Guest holes expose
// new faces on the original host cap support. A direction-independent exact
// trim proof certifies the complete interface before the union is published.
func JoinExactCircularSweepInset(ctx context.Context, host, guest ExactCircularSweep, o ExactCircularLoopOptions) (CircularSweepInset, error) {
	group, err := JoinExactCircularSweepInsets(ctx, host, []ExactCircularSweep{guest}, o)
	if err != nil {
		return CircularSweepInset{}, err
	}
	g := group.Guests[0]
	return CircularSweepInset{
		Solid: group.Solid, HostFaces: group.HostFaces, GuestFaces: g.Faces,
		GuestEdgeOffset: g.EdgeOffset, GuestVertexOffset: g.VertexOffset,
		HostCap: g.HostCap, GuestCap: g.GuestCap,
	}, nil
}
