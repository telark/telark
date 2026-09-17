package authz

// SessionValidator turns a session token into a user ID. A rejected token is
// answered with ErrNotFound or ErrSessionExpired; any other error is read as
// the backend being unreachable.
type SessionValidator func(token string) (string, error)

// BasicResolver is the no-cache Resolver a service without its own session cache
// would otherwise hand-write: validate the token, then flatten the user's grants
// through CollectGrants. A service that needs caching implements Resolver itself.
type BasicResolver struct {
	source   GrantSource
	validate SessionValidator
	log      Warner
}

func NewBasicResolver(source GrantSource, validate SessionValidator, log Warner) *BasicResolver {
	return &BasicResolver{source: source, validate: validate, log: log}
}

func (r *BasicResolver) UserIDForToken(token string) (string, error) {
	return r.validate(token)
}

func (r *BasicResolver) GrantsForUser(userID string) (Grants, error) {
	return CollectGrants(r.source, r.log, userID)
}
