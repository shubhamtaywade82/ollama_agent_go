package governance

// Permission is a fine-grained capability.
type Permission string

const (
	PermRead    Permission = "read"
	PermWrite   Permission = "write"
	PermExecute Permission = "execute"
	PermApprove Permission = "approve"
	PermAdmin   Permission = "admin"
)

// Authorizer decides whether an identity may perform an action on a resource.
type Authorizer interface {
	Can(identity Identity, perm Permission, resource string) bool
}

// RBACAuthorizer implements role-based access control.
// Built-in roles: "viewer" (read), "operator" (read+write+execute), "admin" (all).
type RBACAuthorizer struct {
	rolePermissions map[string][]Permission
}

// NewRBACAuthorizer returns an RBACAuthorizer with the built-in role set.
func NewRBACAuthorizer() *RBACAuthorizer {
	return &RBACAuthorizer{
		rolePermissions: map[string][]Permission{
			"viewer":   {PermRead},
			"operator": {PermRead, PermWrite, PermExecute},
			"admin":    {PermRead, PermWrite, PermExecute, PermApprove, PermAdmin},
		},
	}
}

// Can returns true if any of the identity's roles grants perm.
// resource is accepted for forward-compatibility but is not used in this implementation.
func (a *RBACAuthorizer) Can(identity Identity, perm Permission, _ string) bool {
	for _, role := range identity.Roles {
		for _, p := range a.rolePermissions[role] {
			if p == perm {
				return true
			}
		}
	}
	return false
}

// AllowAll is an Authorizer that grants every request (used when authz is disabled).
type AllowAll struct{}

func (AllowAll) Can(_ Identity, _ Permission, _ string) bool { return true }
