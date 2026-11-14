
# ScopeGuard M3 Commit 1 — ReBAC Domain Contract

M3 adds tenant-scoped resource authorization to the existing M2 RBAC.

Commit 1 includes only the domain model, application contract,
pure authorization policy, and unit tests.

No HTTP routes, migrations, or production database readers are added.

## Identity and resource hierarchy

- Subjects are organization membership IDs, not global user IDs.
- Each resource belongs to exactly one organization.
- Resource identity uses an internal UUID and an external key.
- External keys are case-sensitive opaque ASCII identifiers.
- Project is the only root resource type.
- Folder may have project or folder as parent.
- Document may have project or folder as parent.
- Documents cannot have children.
- Parent is immutable in M3.
- Maximum hierarchy depth is 8 edges.
- Each resource has exactly one owner membership.
- Ownership is not stored as a relationship tuple.

## Relationships

Supported relations:

- viewer
- editor
- owner

Only viewer and editor can be granted through relationship mutations.

Owner is derived from Resource.OwnerMembershipID.

## Actions

Supported actions:

- read
- write
- share
- delete
- transfer

## Direct permissions

Viewer:
- read

Editor:
- read
- write

Direct owner:
- read
- write
- share
- delete
- transfer

## Inherited permissions

Inherited viewer:
- read

Inherited editor:
- read
- write

Inherited owner:
- read
- write

Ownership-only actions are not inherited.

## RBAC gates

Every non-admin ReBAC permission requires resource:access.

resource:admin bypasses relationship checks inside its tenant,
but does not bypass active membership or resource existence.

Resource creation additionally requires resource:create.

## Security invariants

1. A subject must be an active, verified organization member.
2. Resource and membership must belong to the selected tenant.
3. Unknown actions and invalid relation facts fail closed.
4. Authorization decisions must use current effective RBAC permissions.
5. Resource ownership cannot be granted through viewer/editor APIs.
6. Resource hierarchy must not contain cycles.
7. Parent relations and tenant identity must be verified in PostgreSQL.
8. Authorization and resource mutations must be transactionally consistent.
9. All authorization writers must follow the tenant locking protocol.

## Persistence follow-ups

NewResource validates local fields and the immediate parent.

RestoreResource validates persisted field values but cannot verify
the entire ancestor graph by itself.

The PostgreSQL repository must verify:

- Existing resource and parent rows
- Same-tenant relationships
- Parent type compatibility
- Maximum hierarchy depth
- Absence of cycles
- Active owner membership
- Atomic ownership transfer

AccessFactsReader must read RBAC permissions and ReBAC facts
using one consistent PostgreSQL snapshot.

No authorization decision cache is introduced in M3.

The permission catalog and default role policies are updated
in M3 Commit 2.
