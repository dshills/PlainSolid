package document

import (
	"context"
	"fmt"
	"io"
	"math/big"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

// MixedConicalAP242DAGDocumentLimits bounds strict source decoding and
// the total number of expanded document nodes, including groups and root.
// Zero MaxNodes selects the strict hierarchy's maximum of 2,049 nodes.
type MixedConicalAP242DAGDocumentLimits struct {
	Assembly step.MixedConicalAP242DAGLimits
	MaxNodes int
}

const mixedConicalDAGDocumentNodeCap = 1 + 8*256 // strict source path and expanded leaf maximums

// ReadMixedConicalAP242DAGSTEP materializes a strictly proved bounded STEP
// DAG as an editable document. A shared subassembly has one occurrence
// node per expanded path; its local material assets are shared. Source
// labels keep the exact original placement, independently of later edits.
func ReadMixedConicalAP242DAGSTEP(ctx context.Context, r io.Reader, l MixedConicalAP242DAGDocumentLimits, geometry geom.SurfacePatchOptions, historyLimit int) (*Document, error) {
	if ctx == nil || r == nil || historyLimit < 0 {
		return nil, geom.ErrInvalid
	}
	if l.MaxNodes < 0 || l.MaxNodes > mixedConicalDAGDocumentNodeCap {
		return nil, geom.ErrInvalid
	}
	a, err := step.ReadMixedConicalAP242DAG(ctx, r, l.Assembly, geometry)
	if err != nil {
		return nil, err
	}
	return mixedConicalDAGDocument(ctx, a, l.MaxNodes, historyLimit)
}

func mixedConicalDAGDocument(ctx context.Context, a step.MixedConicalAP242DAG, maxNodes, historyLimit int) (*Document, error) {
	if maxNodes == 0 {
		maxNodes = mixedConicalDAGDocumentNodeCap
	}
	b, err := newMixedConicalDocumentBuilder(ctx, a.Assembly, a.Parts)
	if err != nil {
		return nil, err
	}
	root := a.Assembly.Roots[0]
	rootKey := mixedConicalProductKey(root)
	children := make(map[int64][]step.ExactNAUOOccurrence, len(a.Assembly.Products))
	for _, rel := range a.Occurrences {
		children[rel.ParentID] = append(children[rel.ParentID], rel)
	}
	witnesses := make(map[string]step.RationalHierarchyInstance, len(a.Instances))
	for _, leaf := range a.Instances {
		key := rootKey
		for _, id := range leaf.Path {
			key = mixedConicalOccurrenceKey(key, id)
		}
		if _, repeated := witnesses[key]; repeated {
			return nil, geom.ErrUnrepresentable
		}
		witnesses[key] = leaf
	}
	b.addRoot(root, rootKey)
	var walk func(int64, string, [3][4]*big.Rat) error
	walk = func(product int64, parent string, world [3][4]*big.Rat) error {
		for _, rel := range children[product] {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(b.state.Nodes) == maxNodes {
				return fmt.Errorf("document: mixed conical DAG exceeds the node limit: %w", geom.ErrConvergence)
			}
			key := mixedConicalOccurrenceKey(parent, rel.NAUOID)
			pose := rel.Matrix()
			if parent != rootKey {
				pose = composeConicalDocumentPose(world, pose)
			}
			asset := ""
			if _, material := a.Parts[rel.ChildID]; material {
				leaf, ok := witnesses[key]
				if !ok || leaf.ProductID != rel.ChildID || mixedConicalPoseText(leaf.Matrix()) != mixedConicalPoseText(pose) {
					return geom.ErrUnrepresentable
				}
				delete(witnesses, key)
				asset = mixedConicalProductKey(rel.ChildID)
			}
			b.addOccurrence(rel, key, parent, asset, pose)
			if err := walk(rel.ChildID, key, pose); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, rootKey, [3][4]*big.Rat{}); err != nil {
		return nil, err
	}
	if len(witnesses) != 0 {
		return nil, geom.ErrUnrepresentable
	}
	return New(b.state, historyLimit)
}

func composeConicalDocumentPose(a, b [3][4]*big.Rat) (out [3][4]*big.Rat) {
	for i := range out {
		for j := range out[i] {
			q := new(big.Rat)
			for k := range 3 {
				q.Add(q, new(big.Rat).Mul(a[i][k], b[k][j]))
			}
			if j == 3 {
				q.Add(q, a[i][3])
			}
			out[i][j] = q
		}
	}
	return out
}
