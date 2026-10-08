package testutil

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	authdata "github.com/telark/telark/internal/data/auth"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/auth/internal/constants"
)

const (
	fakeAPIPrefix     = "/api/v1/"
	fakeUsersPrefix   = "users/"
	fakeByEmailPrefix = "internal/users/by-email/"
	fakeRolesPrefix   = "accessroles/"
	fakeGroupsPrefix  = "groups/"
	fakePasskeysPath  = "internal/auth/passkeys"
	fakeNoticesPath   = "internal/notifications"
	fakeConfigPath    = "config"
	fakeKeyStatus     = "status"
	fakeKeyData       = "data"
	fakeKeyItems      = "items"
	fakeKeyMetadata   = "metadata"
	fakeKeyVersion    = "resourceVersion"
	fakeKeySpec       = "spec"
	fakeKeyInvite     = "invite"
	fakeKeyAccepted   = "inviteAcceptedAt"
	fakeConfigVersion = "1"
)

// Seed the exported fields before StubBackend: once it serves, only the locking methods touch them.
type FakeExporter struct {
	Users    map[string]*userresource.User
	Roles    map[string]*roledata.AccessRole
	Groups   map[string]*groupdata.Group
	Passkeys map[string]int
	Gone     map[string]bool
	Config   telarkconfigresource.TelarkConfig

	mu          sync.Mutex
	down        bool
	noticesDown bool
	configReads int
	notices     []notificationsclient.Notification
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
		fakeReply(w, http.StatusNotFound, nil)
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

// The exporter merges a phase-less status, so only the invite fields it carries change.
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
	fakeReply(w, http.StatusOK, user)
}

func (f *FakeExporter) servePasskeys(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(constants.HeaderUserID)
	if r.Method == http.MethodPost {
		if f.Passkeys == nil {
			f.Passkeys = map[string]int{}
		}
		f.Passkeys[userID]++
		fakeReply(w, http.StatusOK, nil)
		return
	}
	items := make([]authdata.Passkey, f.Passkeys[userID])
	for i := range items {
		items[i] = authdata.Passkey{UserID: userID, CredentialID: strconv.Itoa(i)}
	}
	fakeReply(w, http.StatusOK, map[string]any{fakeKeyItems: items})
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

// Every read then fails (500), as when the exporter is unreachable.
func (f *FakeExporter) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
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
