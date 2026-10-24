package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	lg        = constants.GetLogger(constants.PrefixBootstrap)
	errPruned = errors.New(string(constants.ErrBootstrapStatusPruned))
	// Exported for the handler: five writers that kept winning is an answer (409), not a fault.
	ErrContended = errors.New(string(constants.ErrBootstrapWriteContended))
)

func Status() (*telarkconfig.BootstrapStatus, error) {
	status, _, err := read()
	return status, err
}

// The bool says whether the Admins on ALL hear of the requests and read them.
func View() (*telarkconfig.BootstrapStatus, bool, error) {
	config, err := get()
	if err != nil {
		return nil, false, err
	}
	status, err := decode(config)
	return status, alertsOn(config), err
}

// A failed read keeps the default, so a request still reaches the Admins on ALL.
func NotifyAdmins() bool {
	config, err := get()
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapNoticeFailed), err))
		return true
	}
	return alertsOn(config)
}

// A config stored before the switch existed has no value, which reads as the default: on.
func alertsOn(config *unstructured.Unstructured) bool {
	on, found, err := unstructured.NestedBool(config.Object, constants.SpecField, telarkconfig.FieldBreakGlassAlerts,
		telarkconfig.FieldNotifyAdmins)
	return on || !found || err != nil
}

// The resourceVersion returned is the one a write must carry.
func read() (*telarkconfig.BootstrapStatus, string, error) {
	config, err := get()
	if err != nil {
		return nil, constants.EmptyString, err
	}
	status, err := decode(config)
	if err != nil {
		return nil, constants.EmptyString, err
	}
	return status, config.GetResourceVersion(), nil
}

func get() (*unstructured.Unstructured, error) {
	result := api.GetCustomResourceByName(constants.TelarkConfigResourceName, metadata.TelarkConfigMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrBootstrapStatusRead), result.Error)
	}
	config, ok := result.Data.(*unstructured.Unstructured)
	if !ok || config == nil {
		return nil, fmt.Errorf(string(constants.ErrBootstrapStatusRead), constants.ErrInvalidResourceTypeReturned)
	}
	return config, nil
}

func decode(config *unstructured.Unstructured) (*telarkconfig.BootstrapStatus, error) {
	status := &telarkconfig.BootstrapStatus{}
	stored, found, err := unstructured.NestedMap(config.Object, constants.FieldStatus, telarkconfig.FieldBootstrap)
	if err == nil && found {
		var raw []byte
		if raw, err = json.Marshal(stored); err == nil {
			err = json.Unmarshal(raw, status)
		}
	}
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
	}
	return status, nil
}

// The resourceVersion makes the patch a compare-and-swap, sent once: a conflict means another writer won, and update
// decides again from what it wrote. The API server answers 200 even when it drops a field its stored CRD lacks, so the
// status that came back must be the one written.
func write(status *telarkconfig.BootstrapStatus, resourceVersion string) error {
	result := api.PatchCustomResourceStatusOnce(metadata.TelarkConfigMetadata, constants.TelarkConfigResourceName, map[string]any{
		constants.MetadataField: map[string]any{constants.ResourceVersionField: resourceVersion},
		constants.FieldStatus:   map[string]any{telarkconfig.FieldBootstrap: status},
	})
	if k8serrors.IsConflict(result.Error) {
		return ErrContended
	}
	if result.Error != nil {
		return fmt.Errorf(string(constants.ErrBootstrapStatusWrite), result.Error)
	}
	config, ok := result.Data.(*unstructured.Unstructured)
	if !ok || config == nil {
		return fmt.Errorf(string(constants.ErrBootstrapStatusWrite), constants.ErrInvalidResourceTypeReturned)
	}
	stored, err := decode(config)
	if err != nil {
		return err
	}
	wrote, err := json.Marshal(status)
	if err != nil {
		return err
	}
	kept, err := json.Marshal(stored)
	if err != nil || !bytes.Equal(wrote, kept) {
		return errors.Join(err, errPruned)
	}
	return nil
}

// decide changes the status it is given and says whether to write it. After a lost race it runs again on what
// the winner wrote, so it decides from that status alone.
func update(decide func(*telarkconfig.BootstrapStatus) (bool, error)) (*telarkconfig.BootstrapStatus, error) {
	for range constants.BootstrapWriteAttempts {
		status, version, err := read()
		if err != nil {
			return nil, err
		}
		changed, err := decide(status)
		if err != nil || !changed {
			return status, err
		}
		if err = write(status, version); !errors.Is(err, ErrContended) {
			return status, err
		}
	}
	return nil, ErrContended
}
