package document

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// WriteCircularMultipleInsetsSTEP exports an editable named multi-inset
// occurrence with its exact composed proper signed-coordinate frame.
func (s State) WriteCircularMultipleInsetsSTEP(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	src, pose, name, err := s.circularJoinExportMembers(ctx, nodeID, DirectedCircularMultipleInsetJoinFeature)
	if err != nil {
		return err
	}
	frame, err := geom.NewFrame(pose.Apply(geom.Vec3{}), pose.ApplyVector(geom.Vec3{X: 1}), pose.ApplyVector(geom.Vec3{Z: 1}))
	if err != nil {
		return err
	}
	return step.WriteExactCircularSweepInsets(ctx, w, step.ExactCircularSweepInsets{
		Name: name, Frame: frame, Host: src[0], Guests: src[1:], LengthUnit: s.LengthUnit,
	})
}

// WriteCircularMultipleInsetsIGES exports a named multi-inset occurrence
// after exact composition of a placement that preserves positive Z.
func (s State) WriteCircularMultipleInsetsIGES(ctx context.Context, w io.Writer, nodeID string) error {
	if ctx == nil || w == nil {
		return geom.ErrInvalid
	}
	src, pose, _, err := s.circularJoinExportMembers(ctx, nodeID, DirectedCircularMultipleInsetJoinFeature)
	if err != nil {
		return err
	}
	members := make([]*trimmed.ExactCircularSweep, len(src))
	for i := range src {
		members[i] = &src[i]
	}
	if err = placeCircularJoinProfilesForIGES(pose, members...); err != nil {
		return err
	}
	return iges.WriteExactCircularSweepInsets(ctx, w, iges.ExactCircularSweepInsets{Host: src[0], Guests: src[1:], Unit: s.LengthUnit})
}
