package domain

import (
	"errors"
	"testing"
)

func TestEvaluateAccessMatrix(t *testing.T) {
	base := AccessFacts{
		ActiveMember:      true,
		ResourceExists:    true,
		HasResourceAccess: true,
	}

	tests := []struct {
		name   string
		facts  AccessFacts
		action Action
		want   bool
	}{
		{
			"no tuple",
			base,
			ActionRead,
			false,
		},
		{
			"viewer read",
			withDirect(base, RelationViewer),
			ActionRead,
			true,
		},
		{
			"viewer write",
			withDirect(base, RelationViewer),
			ActionWrite,
			false,
		},
		{
			"editor read",
			withDirect(base, RelationEditor),
			ActionRead,
			true,
		},
		{
			"editor write",
			withDirect(base, RelationEditor),
			ActionWrite,
			true,
		},
		{
			"editor cannot share",
			withDirect(base, RelationEditor),
			ActionShare,
			false,
		},
		{
			"editor cannot delete",
			withDirect(base, RelationEditor),
			ActionDelete,
			false,
		},
		{
			"editor cannot transfer",
			withDirect(base, RelationEditor),
			ActionTransfer,
			false,
		},
		{
			"ancestor viewer read",
			withAncestor(base, RelationViewer),
			ActionRead,
			true,
		},
		{
			"ancestor viewer write",
			withAncestor(base, RelationViewer),
			ActionWrite,
			false,
		},
		{
			"ancestor editor write",
			withAncestor(base, RelationEditor),
			ActionWrite,
			true,
		},
		{
			"ancestor owner write",
			withAncestor(base, RelationOwner),
			ActionWrite,
			true,
		},
		{
			"ancestor owner no share",
			withAncestor(base, RelationOwner),
			ActionShare,
			false,
		},
		{
			"ancestor owner no delete",
			withAncestor(base, RelationOwner),
			ActionDelete,
			false,
		},
		{
			"ancestor owner no transfer",
			withAncestor(base, RelationOwner),
			ActionTransfer,
			false,
		},
		{
			"direct viewer plus inherited editor",
			AccessFacts{
				ActiveMember:      true,
				ResourceExists:    true,
				HasResourceAccess: true,
				DirectRelations: []Relation{
					RelationViewer,
				},
				AncestorRelations: []Relation{
					RelationEditor,
				},
			},
			ActionWrite,
			true,
		},
		{
			"direct owner may transfer",
			AccessFacts{
				ActiveMember:      true,
				ResourceExists:    true,
				HasResourceAccess: true,
				IsDirectOwner:     true,
			},
			ActionTransfer,
			true,
		},
		{
			"direct owner may delete",
			AccessFacts{
				ActiveMember:      true,
				ResourceExists:    true,
				HasResourceAccess: true,
				IsDirectOwner:     true,
			},
			ActionDelete,
			true,
		},
		{
			"owner needs baseline access",
			AccessFacts{
				ActiveMember:   true,
				ResourceExists: true,
				IsDirectOwner:  true,
			},
			ActionRead,
			false,
		},
		{
			"admin bypass",
			AccessFacts{
				ActiveMember:     true,
				ResourceExists:   true,
				HasResourceAdmin: true,
			},
			ActionTransfer,
			true,
		},
		{
			"admin cannot bypass inactive member",
			AccessFacts{
				ResourceExists:   true,
				HasResourceAdmin: true,
			},
			ActionRead,
			false,
		},
		{
			"admin cannot bypass missing resource",
			AccessFacts{
				ActiveMember:     true,
				HasResourceAdmin: true,
			},
			ActionRead,
			false,
		},
		{
			"inactive viewer",
			AccessFacts{
				ResourceExists:    true,
				HasResourceAccess: true,
				DirectRelations: []Relation{
					RelationViewer,
				},
			},
			ActionRead,
			false,
		},
		{
			"viewer without RBAC gate",
			AccessFacts{
				ActiveMember:   true,
				ResourceExists: true,
				DirectRelations: []Relation{
					RelationViewer,
				},
			},
			ActionRead,
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateAccess(
				tc.facts,
				tc.action,
			)

			if err != nil || got != tc.want {
				t.Fatalf(
					"allowed=%v err=%v; want %v",
					got, err, tc.want,
				)
			}
		})
	}
}

func withDirect(
	f AccessFacts,
	r Relation,
) AccessFacts {
	f.DirectRelations = []Relation{r}
	return f
}

func withAncestor(
	f AccessFacts,
	r Relation,
) AccessFacts {
	f.AncestorRelations = []Relation{r}
	return f
}

func TestEvaluateAccessInvalidFactsFailClosed(
	t *testing.T,
) {
	base := AccessFacts{
		ActiveMember:      true,
		ResourceExists:    true,
		HasResourceAccess: true,
	}

	tests := []struct {
		name   string
		facts  AccessFacts
		action Action
		want   error
	}{
		{
			"invalid action",
			base,
			"grant",
			ErrInvalidAction,
		},
		{
			"forged owner tuple",
			withDirect(base, RelationOwner),
			ActionRead,
			ErrInvalidRelation,
		},
		{
			"invalid direct relation",
			withDirect(base, "admin"),
			ActionRead,
			ErrInvalidRelation,
		},
		{
			"invalid ancestor relation",
			withAncestor(base, "admin"),
			ActionWrite,
			ErrInvalidRelation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EvaluateAccess(
				tc.facts,
				tc.action,
			)

			if got || !errors.Is(err, tc.want) {
				t.Fatalf(
					"allowed=%v err=%v; want %v",
					got, err, tc.want,
				)
			}
		})
	}
}
