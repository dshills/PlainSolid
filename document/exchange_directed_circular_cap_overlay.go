package document

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteCircularCapOverlaySTEP exports a named occurrence of a complete cap
// join as one analytic solid. The exact composition of proper signed-axis
// feature and assembly placements supplies the STEP frame.
func (s State) WriteCircularCapOverlaySTEP(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	a, b, pose, name, err := s.circularJoinExportRecipe(ctx, nodeID, DirectedCircularCapOverlayFeature)
	if err != nil {
		return err
	}
	frame, err := geom.NewFrame(pose.Apply(geom.Vec3{}), pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if err != nil {
		return err
	}
	return step.WriteExactCircularSweepCapOverlay(ctx, w, step.ExactCircularSweepCapOverlay{
		Name: name, Frame: frame, Sweeps: [2]trimmed.ExactCircularSweep{a, b}, LengthUnit: s.LengthUnit,
	})
}

// WriteCircularCapOverlayIGES exports a named occurrence of a complete cap
// join after applying an exact placement that preserves positive Z. Both
// original source partitions are regenerated before forming the shared rim.
func (s State) WriteCircularCapOverlayIGES(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	a, b, pose, _, err := s.circularJoinExportRecipe(ctx, nodeID, DirectedCircularCapOverlayFeature)
	if err != nil {
		return err
	}
	if err = placeCircularJoinProfilesForIGES(pose, &a, &b); err != nil {
		return err
	}
	return iges.WriteExactCircularSweepCapOverlay(ctx, w, iges.ExactCircularSweepCapOverlay{
		Sweeps: [2]trimmed.ExactCircularSweep{a, b}, Unit: s.LengthUnit,
	})
}
