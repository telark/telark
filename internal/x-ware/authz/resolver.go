package authz

// A rejected token answers ErrNotFound, ErrGone or ErrSessionExpired; any other
// error is read as the backend being unreachable.
type SessionValidator func(token string) (string, error)

// The no-cache Resolver; a service that needs caching implements Resolver itself.
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
