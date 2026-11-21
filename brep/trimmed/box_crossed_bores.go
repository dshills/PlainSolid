package trimmed

// BoxCrossedBoresResult is a box with two crossing bores of equal radius,
// with the volume the bores removed and the change of surface area.
type BoxCrossedBoresResult struct {
	Solid                           Solid
	RemovedVolume, SurfaceAreaDelta float64
}
