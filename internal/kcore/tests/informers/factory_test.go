package informers

import (
	"testing"
	"time"

	"github.com/telark/kcore/informers/factory"
)

func TestCreateInformerFactory_NilClient(t *testing.T) {
	inf, err := factory.CreateInformerFactory(nil, 30*time.Second)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}

func TestCreateServiceInformer_NilFactory(t *testing.T) {
	inf, err := factory.CreateServiceInformer(nil)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}

func TestCreateNamespaceInformer_NilFactory(t *testing.T) {
	inf, err := factory.CreateNamespaceInformer(nil)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}

func TestCreateDeploymentInformer_NilFactory(t *testing.T) {
	inf, err := factory.CreateDeploymentInformer(nil)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}

func TestCreateStatefulSetInformer_NilFactory(t *testing.T) {
	inf, err := factory.CreateStatefulSetInformer(nil)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}

func TestCreateDaemonSetInformer_NilFactory(t *testing.T) {
	inf, err := factory.CreateDaemonSetInformer(nil)
	if err == nil || inf != nil {
		t.Error(ExpectedErrorForNilFactory)
	}
}
