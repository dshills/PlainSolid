package heal

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

// ShellOrientation records the verified embedding of a sewn shell. Parent is
// an index in Report.Shells, or -1 for an outer root. SignedVolume is exact for
// the welded geometry, after orientation. Reversed refers to the whole-shell
// reversal after fixing neighboring face winding. Faces are input indices.
type ShellOrientation struct {
	Parent, Depth int
	Faces         []int
	SignedVolume  *big.Rat
	Reversed      bool
}

func orientNestedShells(ctx context.Context, solid brep.Solid, vertices []geom.Vec3, edges []brep.Edge, faces []brep.Face, flips []int, options collision.LocationOptions) (brep.Solid, []ShellOrientation, error) {
	shells, err := collision.InspectShellNesting(ctx, solid, options)
	if err != nil {
		return brep.Solid{}, nil, planarValidationError(err)
	}
	mapping := solid.TriangleFaces()
	report := make([]ShellOrientation, len(shells))
	changed := false
	faceSeen := make([]bool, len(faces))
	for i, shell := range shells {
		if err := ctx.Err(); err != nil {
			return brep.Solid{}, nil, err
		}
		record := ShellOrientation{Parent: shell.Parent, Depth: shell.Depth, SignedVolume: shell.SignedVolume}
		record.Reversed = (record.SignedVolume.Sign() < 0) != (record.Depth%2 == 1)
		for _, triangle := range shell.Triangles {
			face := mapping[triangle]
			if faceSeen[face] {
				continue
			}
			faceSeen[face] = true
			record.Faces = append(record.Faces, face)
			if record.Reversed {
				reverseFace(&faces[face])
				flips[face] = -flips[face]
			}
		}
		sort.Ints(record.Faces)
		if record.Reversed {
			changed = true
			record.SignedVolume.Neg(record.SignedVolume)
		}
		report[i] = record
	}
	if changed {
		solid, err = brep.NewSolid(vertices, edges, faces)
		if err != nil {
			return brep.Solid{}, nil, fmt.Errorf("%w: oriented boundary: %v", ErrInvalid, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return brep.Solid{}, nil, err
	}
	return solid, report, nil
}

func planarValidationError(err error) error {
	switch {
	case errors.Is(err, collision.ErrInvalid):
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	case errors.Is(err, collision.ErrLimit):
		return fmt.Errorf("%w: %w", ErrLimit, err)
	default:
		return err
	}
}
