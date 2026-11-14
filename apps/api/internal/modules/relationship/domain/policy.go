package domain

type AccessFacts struct {
	ActiveMember      bool
	ResourceExists    bool
	HasResourceAccess bool
	HasResourceAdmin  bool
	IsDirectOwner     bool
	DirectRelations   []Relation
	AncestorRelations []Relation
}

// EvaluateAccess performs a pure RBAC + ReBAC decision.
// It does not access a database and does not modify state.
func EvaluateAccess(facts AccessFacts, action Action) (bool, error) {
	if !action.Valid() {
		return false, ErrInvalidAction
	}

	//reject malformed relationship facts before allowing access
	for _, relation := range facts.DirectRelations {
		if !relation.Grantable() {
			return false, ErrInvalidRelation
		}
	}

	for _, relation := range facts.AncestorRelations {
		if !relation.Valid() {
			return false, ErrInvalidRelation
		}
	}

	//neither RBAC admin nor ReBAC ownership may bypass
	//active membership and  resource existence
	if !facts.ActiveMember || !facts.ResourceExists {
		return false, nil
	}

	//tenant scope administration override
	if facts.HasResourceAdmin {
		return true, nil
	}

	// all non admin ReBAC access requires this RBAC gate
	if !facts.HasResourceAccess {
		return false, nil
	}

	if facts.IsDirectOwner {
		return true, nil
	}

	for _, relation := range facts.DirectRelations {
		if relationAllows(relation, action, false) {
			return true, nil
		}
	}

	for _, relation := range facts.AncestorRelations {
		if relationAllows(relation, action, true) {
			return true, nil
		}
	}

	return false, nil
}

func relationAllows(relation Relation, action Action, inherited bool) bool {
	if inherited {
		//ancestor ownership is not ownership of descendants
		if action == ActionRead {
			return relation == RelationViewer || relation == RelationEditor || relation == RelationOwner
		}

		return action == ActionWrite && (relation == RelationEditor || relation == RelationOwner)
	}

	if relation == RelationViewer {
		return action == ActionRead
	}

	return relation == RelationEditor && (action == ActionRead || action == ActionWrite)
}
