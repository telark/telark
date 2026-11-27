package testutil

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/authrequest"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
)

const (
	fakeAPIPrefix     = "/api/v1/"
	fakeUsersPrefix   = "users/"
	fakeUsersPath     = "users"
	CreatedUserID     = "u-created"
	fakePendingPath   = "internal/auth/requests/pending"
	fakeStatusPrefix  = "internal/auth/requests/"
	fakeStatusSuffix  = "/status"
	fakeByEmailPrefix = "internal/users/by-email/"
	fakeRolesPrefix   = "accessroles/"
	fakeGroupsPrefix  = "groups/"
	fakePasskeysPath  = "internal/auth/passkeys"
	fakeNoticesPath   = "internal/notifications"
	fakeConfigPath    = "config"
	fakeBootstrapPath = "bootstrap"
	fakeRequestsPath  = "bootstrap/requests"
	fakeSessionsPath  = "internal/auth/users/"
	fakePathSep       = "/"
	fakeKeyStatus     = "status"
	fakeKeyData       = "data"
	fakeKeyItems      = "items"
	fakeKeyMetadata   = "metadata"
	fakeKeyVersion    = "resourceVersion"
	fakeKeySpec       = "spec"
	fakeKeyInvite     = "invite"
	fakeKeyAccepted   = "inviteAcceptedAt"
	fakeKeyIdentities = "identities"
	fakeConfigVersion = "1"
)

// Seed the exported fields before StubBackend: once it serves, only the locking methods touch them. Credentials are
// the passkeys an account signs with, and a passkey saved for such an account joins them; Passkeys only counts
// placeholders. RequestAnswers answers an auth request route by its last path segment (requests, approve,
// cancel, complete), and a route without one answers 409.
type FakeExporter struct {
	Users          map[string]*userresource.User
	Roles          map[string]*roledata.AccessRole
	Groups         map[string]*groupdata.Group
	Passkeys       map[string]int
	Credentials    map[string][]authdata.Passkey
	Gone           map[string]bool
	Config         telarkconfigresource.TelarkConfig
	Bootstrap      telarkconfigresource.BootstrapStatus
	RequestAnswers map[string]bootstrapeps.RequestResult
	// The objects the request loop lists and reports on: one is listed until a status write ends it.
	AuthRequests []authrequest.AuthRequest
	// Runs on each request route call, under the fake's lock: it must not call the fake's methods.
	OnRequest func(path string)

	mu              sync.Mutex
	down            bool
	passkeyReadDown bool
	noticesDown     bool
	configReads     int
	notices         []notificationsclient.Notification
	requestCalls    []string
	sessionsIssued  int
	sessions        []authdata.Session
	listReads       int
	failedListReads []int
	statusWriteDown bool
	statusWrites    []StatusWrite
}

// One status write the request loop sent, as the exporter stored it.
type StatusWrite struct {
	ID     string
	Status authrequest.Status
}

func (f *FakeExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, fakeAPIPrefix)
	if r.Method == http.MethodGet && path == fakeConfigPath {
		f.configReads++
	}
	switch {
	case r.Method == http.MethodGet && f.down:
		fakeReply(w, http.StatusInternalServerError, nil)
	case path == fakeConfigPath:
		f.serveConfig(w, r)
	case path == fakePasskeysPath:
		f.servePasskeys(w, r)
	case path == fakeNoticesPath:
		f.recordNotice(w, r)
	case strings.HasPrefix(path, fakeByEmailPrefix):
		fakeRecord(w, f.userByEmail(strings.TrimPrefix(path, fakeByEmailPrefix)))
	case strings.HasPrefix(path, fakeUsersPrefix):
		f.serveUser(w, r, strings.TrimPrefix(path, fakeUsersPrefix))
	case strings.HasPrefix(path, fakeRolesPrefix):
		fakeRecord(w, f.Roles[strings.TrimPrefix(path, fakeRolesPrefix)])
	case strings.HasPrefix(path, fakeGroupsPrefix):
		fakeRecord(w, f.Groups[strings.TrimPrefix(path, fakeGroupsPrefix)])
	default:
		f.serveBootstrap(w, r, path)
	}
}

func (f *FakeExporter) serveUser(w http.ResponseWriter, r *http.Request, id string) {
	user, found := f.Users[id]
	switch {
	case f.Gone[id]:
		fakeReply(w, http.StatusGone, nil)
	case !found:
		fakeReply(w, http.StatusNotFound, nil)
	case r.Method == http.MethodPatch:
		patchUser(w, r, user)
	default:
		fakeReply(w, http.StatusOK, user)
	}
}

func (f *FakeExporter) userByEmail(email string) *userresource.User {
	for user := range maps.Values(f.Users) {
		if user.Email == email {
			return user
		}
	}
	return nil
}

// The exporter merges a phase-less status, so only the invite fields and the sign-in stamp it carries change.
func patchUser(w http.ResponseWriter, r *http.Request, user *userresource.User) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fakeReply(w, http.StatusBadRequest, nil)
		return
	}
	status, isMap := body[fakeKeyStatus].(map[string]any)
	if invite, present := status[fakeKeyInvite]; isMap && present {
		raw, err := json.Marshal(invite)
		user.Status.Invite = nil
		if err == nil {
			err = json.Unmarshal(raw, &user.Status.Invite)
		}
		if err != nil {
			fakeReply(w, http.StatusBadRequest, nil)
			return
		}
	}
	if accepted, isString := status[fakeKeyAccepted].(string); isString {
		user.Status.InviteAcceptedAt = accepted
	}
	if stamp, isString := status[constants.UserStatusFieldLastLoginAt].(string); isString {
		user.Status.LastLoginAt = &stamp
	}
	if !fakeField(body, fakeKeyIdentities, &user.Identities) || !fakeField(body, constants.SpecFieldRoleRefs, &user.RoleRefs) ||
		!fakeField(body, constants.UserFieldBootstrap, &user.Bootstrap) {
		fakeReply(w, http.StatusBadRequest, nil)
		return
	}
	fakeReply(w, http.StatusOK, user)
}

// A field the patch doesn't carry stays as it is; false when its value doesn't fit.
func fakeField[T any](body map[string]any, key string, target *T) bool {
	value, present := body[key]
	if !present {
		return true
	}
	var decoded T
	raw, err := json.Marshal(value)
	if err != nil || json.Unmarshal(raw, &decoded) != nil {
		return false
	}
	*target = decoded
	return true
}

func (f *FakeExporter) createUser(w http.ResponseWriter, r *http.Request) {
	var user userresource.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		fakeReply(w, http.StatusBadRequest, nil)
		return
	}
	user.ID = CreatedUserID
	if f.Users == nil {
		f.Users = map[string]*userresource.User{}
	}
	f.Users[user.ID] = &user
	fakeReply(w, http.StatusCreated, user)
}

func (f *FakeExporter) servePasskeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(constants.HeaderUserID)
	if r.Method == http.MethodPost {
		if f.Passkeys == nil {
			f.Passkeys = map[string]int{}
		}
		f.Passkeys[userID]++
		var saved authdata.Passkey
		if _, signs := f.Credentials[userID]; signs && json.NewDecoder(r.Body).Decode(&saved) == nil {
			f.Credentials[userID] = append(f.Credentials[userID], saved)
		}
		fakeReply(w, http.StatusOK, nil)
		return
	}
	if f.passkeyReadDown {
		f.passkeyReadDown = false
		fakeReply(w, http.StatusInternalServerError, nil)
		return
	}
	if credentials, signs := f.Credentials[userID]; signs {
		fakeReply(w, http.StatusOK, map[string]any{fakeKeyItems: credentials})
		return
	}
	items := make([]authdata.Passkey, f.Passkeys[userID])
	for i := range items {
		items[i] = authdata.Passkey{UserID: userID, CredentialID: strconv.Itoa(i)}
	}
	fakeReply(w, http.StatusOK, map[string]any{fakeKeyItems: items})
}

// The auth request state, its request routes, the AuthRequest objects, a created account and the sessions a sign-in
// creates; any other path is unknown.
func (f *FakeExporter) serveBootstrap(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case path == fakeUsersPath && r.Method == http.MethodPost:
		f.createUser(w, r)
	case path == fakePendingPath:
		f.listAuthRequests(w)
	case strings.HasPrefix(path, fakeStatusPrefix) && strings.HasSuffix(path, fakeStatusSuffix):
		f.writeStatus(w, r, strings.TrimSuffix(strings.TrimPrefix(path, fakeStatusPrefix), fakeStatusSuffix))
	case path == fakeBootstrapPath:
		fakeReply(w, http.StatusOK, f.Bootstrap)
	case strings.HasPrefix(path, fakeRequestsPath):
		f.answerRequest(w, r, path)
	case strings.HasPrefix(path, fakeSessionsPath):
		f.sessionsIssued++
		var session authdata.Session
		if json.NewDecoder(r.Body).Decode(&session) == nil {
			f.sessions = append(f.sessions, session)
		}
		fakeReply(w, http.StatusOK, nil)
	default:
		fakeReply(w, http.StatusNotFound, nil)
	}
}

// Records each call as its path and body, the way the request routes answer.
func (f *FakeExporter) answerRequest(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fakeReply(w, http.StatusBadRequest, nil)
		return
	}
	f.requestCalls = append(f.requestCalls, strings.TrimSpace(path+constants.SpaceSeparator+string(body)))
	if f.OnRequest != nil {
		f.OnRequest(path)
	}
	answer, found := f.RequestAnswers[path[strings.LastIndex(path, fakePathSep)+len(fakePathSep):]]
	if !found {
		fakeReply(w, http.StatusConflict, nil)
		return
	}
	fakeReply(w, http.StatusOK, answer)
}

// Oldest first, and only the phase of each status, as the exporter lists them.
func (f *FakeExporter) listAuthRequests(w http.ResponseWriter) {
	f.listReads++
	if slices.Contains(f.failedListReads, f.listReads) {
		fakeReply(w, http.StatusInternalServerError, nil)
		return
	}
	pending := slices.DeleteFunc(slices.Clone(f.AuthRequests), func(request authrequest.AuthRequest) bool {
		return request.Phase != constants.EmptyString && request.Phase != authrequest.PhasePending && !request.InFlight()
	})
	slices.SortStableFunc(pending, func(a, b authrequest.AuthRequest) int { return strings.Compare(a.CreatedAt, b.CreatedAt) })
	fakeReply(w, http.StatusOK, map[string]any{fakeKeyItems: pending})
}

func (f *FakeExporter) writeStatus(w http.ResponseWriter, r *http.Request, id string) {
	var status authrequest.Status
	index := slices.IndexFunc(f.AuthRequests, func(request authrequest.AuthRequest) bool { return request.ID == id })
	switch {
	case json.NewDecoder(r.Body).Decode(&status) != nil:
		fakeReply(w, http.StatusBadRequest, nil)
	case f.statusWriteDown:
		f.statusWriteDown = false
		fakeReply(w, http.StatusInternalServerError, nil)
	case index < constants.DefaultInitValue:
		fakeReply(w, http.StatusNotFound, nil)
	default:
		f.statusWrites = append(f.statusWrites, StatusWrite{ID: id, Status: status})
		f.AuthRequests[index].Phase = status.Phase
		fakeReply(w, http.StatusOK, nil)
	}
}

// Answers like GET config: the spec fields plus the resourceVersion a patch replays.
func (f *FakeExporter) serveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPatch {
		var body map[string]map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fakeReply(w, http.StatusBadRequest, nil)
			return
		}
		spec, err := json.Marshal(body[fakeKeySpec])
		if err == nil {
			err = json.Unmarshal(spec, &f.Config)
		}
		if err != nil {
			fakeReply(w, http.StatusBadRequest, nil)
			return
		}
	}
	var view map[string]any
	raw, err := json.Marshal(f.Config)
	if err == nil {
		err = json.Unmarshal(raw, &view)
	}
	if err != nil {
		fakeReply(w, http.StatusInternalServerError, nil)
		return
	}
	view[fakeKeyMetadata] = map[string]any{fakeKeyVersion: fakeConfigVersion}
	fakeReply(w, http.StatusOK, view)
}

func (f *FakeExporter) recordNotice(w http.ResponseWriter, r *http.Request) {
	if f.noticesDown {
		fakeReply(w, http.StatusInternalServerError, nil)
		return
	}
	var notice notificationsclient.Notification
	if err := json.NewDecoder(r.Body).Decode(&notice); err != nil {
		fakeReply(w, http.StatusBadRequest, nil)
		return
	}
	f.notices = append(f.notices, notice)
	fakeReply(w, http.StatusOK, notice)
}

func (f *FakeExporter) SetConfig(cfg telarkconfigresource.TelarkConfig) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Config = cfg
}

// The next status reads answer status, as when a request ends while an auth request runs.
func (f *FakeExporter) SetBootstrap(status telarkconfigresource.BootstrapStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Bootstrap = status
}

// Every read then fails (500), as when the exporter is unreachable.
func (f *FakeExporter) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// The next passkey list read fails (500), then reads work again.
func (f *FakeExporter) FailNextPasskeyRead() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passkeyReadDown = true
}

// The pending list reads with these ordinals, counted from 1, fail (500).
func (f *FakeExporter) FailAuthRequestLists(reads ...int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failedListReads = reads
}

// The next status write fails (500) and stores nothing, then writes work again.
func (f *FakeExporter) FailNextStatusWrite() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusWriteDown = true
}

func (f *FakeExporter) AuthRequestListReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listReads
}

// The status writes that reached the objects, in order.
func (f *FakeExporter) StatusWrites() []StatusWrite {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.statusWrites)
}

// The user then answers 410, as a record only the cleanup finalizer still holds.
func (f *FakeExporter) SetGone(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Gone == nil {
		f.Gone = map[string]bool{}
	}
	f.Gone[id] = true
}

func (f *FakeExporter) RemoveUser(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Users, id)
}

func (f *FakeExporter) SetNoticesDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.noticesDown = down
}

func (f *FakeExporter) SavedConfig() telarkconfigresource.TelarkConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Config
}

func (f *FakeExporter) ConfigReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.configReads
}

func (f *FakeExporter) Notices() []notificationsclient.Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.notices)
}

func (f *FakeExporter) RequestCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requestCalls)
}

func (f *FakeExporter) SessionsIssued() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sessionsIssued
}

// The sessions sign-ins created, as the exporter received them.
func (f *FakeExporter) Sessions() []authdata.Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sessions)
}

func (f *FakeExporter) PasskeyCount(userID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Passkeys[userID]
}

func (f *FakeExporter) User(id string) userresource.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.Users[id]
}

func fakeRecord[T any](w http.ResponseWriter, record *T) {
	if record == nil {
		fakeReply(w, http.StatusNotFound, nil)
		return
	}
	fakeReply(w, http.StatusOK, record)
}

func fakeReply(w http.ResponseWriter, status int, data any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{fakeKeyStatus: status, fakeKeyData: data})
}
