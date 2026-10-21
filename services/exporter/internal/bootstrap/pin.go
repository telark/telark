package bootstrap

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The first passkey of a bootstrap account pins it: a chart change or a break-glass run then moves the marker only
// through a completed replacement. Of two racing first enrollments, one pins and the other loses the marker. A pin
// that didn't persist leaves break-glass refusing to act until the exporter's next start pins the account.
func PinAtFirstPasskey(optimizer *performance.Optimizer, userID string) {
	user, err := loadUser(userID)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrBootstrapPinFailed), userID, err))
		return
	}
	if user == nil || !user.Bootstrap {
		return
	}
	if err := pin(user.ID, user.Email); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrBootstrapPinFailed), userID, err))
		return
	}
	if err := settleMarkers(optimizer, userID); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapMarkersFailed), err))
	}
}

// Whether a create or patch body sets the marker; null decodes to false, like an absent one.
func Marks(body map[string]any) bool {
	marked, isBool := body[constants.FieldBootstrap].(bool)
	return isBool && marked
}

// After a write that marked userID: before any pin it keeps the marker and every other account loses it, so a
// second break-glass run fixes a mistyped email. Once pinned, only the pinned account keeps it. An account marked
// with a passkey already has its first one, so it is pinned now.
// A failed passkey lookup can't fail the write, done by then: it is logged, the marker still moves, and the account
// stays unpinned, so break-glass refuses it until the exporter's next start pins it.
func MarkerSet(optimizer *performance.Optimizer, userID string) {
	passkeys, err := passkeyutils.FindPasskeysByUserID(userID)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrBootstrapPinFailed), userID, err))
	}
	if err == nil && len(passkeys) > constants.DefaultInitValue {
		PinAtFirstPasskey(optimizer, userID)
		return
	}
	if err := settleMarkers(optimizer, userID); err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapMarkersFailed), err))
	}
}

func pin(userID, email string) error {
	wrote := false
	_, err := update(func(status *telarkconfig.BootstrapStatus) (bool, error) {
		wrote = !status.Pinned()
		if wrote {
			at := time.Now().UTC()
			status.UserID, status.Email, status.PinnedAt = userID, email, stamp(at)
			// A request a reset left open belongs to the earlier pin: the new passkey answers it, as a sign-in would.
			if newest := status.Newest(); newest != nil && newest.Open() {
				end(newest, telarkconfig.BootstrapReasonSignedIn, at)
			}
		}
		return wrote, nil
	})
	if err == nil && wrote {
		lg.Info(fmt.Sprintf(string(constants.InfBootstrapPinned), userID))
	}
	return err
}

func settleMarkers(optimizer *performance.Optimizer, candidate string) error {
	status, err := Status()
	if err != nil {
		return err
	}
	if status.Pinned() {
		candidate = status.UserID
	}
	users, err := listUsers()
	if err != nil {
		return err
	}
	errs := make([]error, constants.DefaultInitValue, len(users))
	for _, user := range users {
		if user.Bootstrap && user.ID != candidate {
			errs = append(errs, setMarker(optimizer, user.ID, false))
		}
	}
	return errors.Join(errs...)
}

// At each start: with no pin, the enrolled bootstrap account is pinned (BOOTSTRAP_ADMIN's first, else the oldest),
// and every other account loses the marker but keeps its roles. With none enrolled, nothing is pinned. The Redis
// response cache outlives a restart, so the marker writes invalidate it as at runtime.
func PinEnrolled(optimizer *performance.Optimizer) error {
	status, err := Status()
	if err != nil {
		return err
	}
	if !status.Pinned() {
		chosen, err := enrolledBootstrap()
		if err != nil || chosen == nil {
			return err
		}
		if err := pin(chosen.ID, chosen.Email); err != nil {
			return err
		}
	}
	return settleMarkers(optimizer, constants.EmptyString)
}

func enrolledBootstrap() (*userdata.User, error) {
	items, err := listUserItems()
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(items, func(a, b unstructured.Unstructured) int {
		return a.GetCreationTimestamp().Compare(b.GetCreationTimestamp().Time)
	})
	var chosen *userdata.User
	for i := range items {
		user, err := userutils.ExtractUserFromUnstructured(&items[i])
		if err != nil || user == nil || !user.Bootstrap {
			continue
		}
		passkeys, err := passkeyutils.FindPasskeysByUserID(user.ID)
		if err != nil {
			return nil, err
		}
		if len(passkeys) == constants.DefaultInitValue {
			continue
		}
		if userutils.NormalizeEmail(user.Email) == envmanager.GetBootstrapAdmin() {
			return user, nil
		}
		if chosen == nil {
			chosen = user
		}
	}
	return chosen, nil
}

// Written to the CR directly: the guard refuses every caller a marker that differs from the pin.
func setMarker(optimizer *performance.Optimizer, userID string, marked bool) error {
	lock := concurrency.GetLock(userID)
	lock.Lock()
	result := api.PatchCustomResource(metadata.UserMetadata, userID,
		map[string]any{constants.SpecField: map[string]any{constants.FieldBootstrap: marked}})
	lock.Unlock()
	userutils.InvalidateUserCaches(optimizer, userID)
	if result.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrBootstrapMarkerWrite), userID, result.Error)
	}
	return nil
}

func loadUser(userID string) (*userdata.User, error) {
	result := api.GetCustomResourceByName(userID, metadata.UserMetadata)
	if k8serrors.IsNotFound(result.Error) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	item, ok := result.Data.(*unstructured.Unstructured)
	if !ok || item == nil {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}
	return userutils.ExtractUserFromUnstructured(item)
}

func listUsers() ([]*userdata.User, error) {
	items, err := listUserItems()
	if err != nil {
		return nil, err
	}
	users := make([]*userdata.User, constants.DefaultInitValue, len(items))
	for i := range items {
		if user, err := userutils.ExtractUserFromUnstructured(&items[i]); err == nil && user != nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func listUserItems() ([]unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.UserMetadata)
	if result.Error != nil {
		return nil, result.Error
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}
	return list.Items, nil
}
