package protection

import (
	"context"
	"os"
	"slices"
	"strconv"

	"github.com/redis/go-redis/v9"
	discoveryauthz "github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/clients"
	dconfig "github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	kcorek8s "github.com/telark/kcore/k8sclient"
	kcorecore "github.com/telark/kcore/resources/core"
	xwareredis "github.com/telark/x-ware/redis/stream"
	"k8s.io/client-go/kubernetes"
)

func BuildService(kubeClient *kubernetes.Clientset, rdb *redis.Client, logger Logger) (*Service, error) {
	dyn, err := kcorek8s.InitDynamicClient()
	if err != nil {
		return nil, err
	}
	mapper := kcorek8s.NewDeferredRESTMapper(kubeClient)
	applier := protpolicies.NewApplier(dyn, mapper)
	resolver := applications.NewRedisResolver(rdb, applications.NewClusterClaimReader(dyn))
	exporter := clients.NewProtectionPlanClient()

	// Own client budget: the checkpoint burst on leadership must never starve the applier.
	qps, burst := dconfig.RollbackK8sClientRateLimit()
	reportsDyn, err := kcorek8s.NewDynamicClientWithRateLimit(qps, burst)
	if err != nil {
		return nil, err
	}
	checkpointEvery, _ := dconfig.ReportCheckpointInterval()
	gen := reports.NewGenerator(
		clients.NewReportClient(), reportsDyn, resolver, rdb, logger, reportMaxViolations(), checkpointEvery,
	)
	notifier := &ApprovalNotifier{
		ListUsers:   clients.NewAuthzClient().GetAllUsers,
		Grants:      discoveryauthz.NewResolver().GrantsForUser,
		Emit:        clients.NewNotificationClient().Emit,
		Requirement: discoveryauthz.ApprovePlanRequirement(),
		Logger:      logger,
	}
	svc := NewService(applier, resolver, exporter, dyn, ListClusterNamespaces, gen, logger, notifier)
	svc.environments = clients.NewCategoryClient().PlanEnvironmentIDs
	if rdb != nil {
		svc.names = NewNameLocks(xwareredis.NewLockClient(rdb))
	}
	return svc, nil
}

func reportMaxViolations() int {
	n, err := strconv.Atoi(os.Getenv(constants.EnvReportMaxViolations))
	if err != nil || n <= constants.DefaultInitValue {
		return constants.DefaultReportMaxViolations
	}
	return n
}

func ListClusterNamespaces(context.Context) ([]string, error) {
	list, err := kcorecore.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	out := make([]string, constants.DefaultInitValue, len(list))
	for i := range list {
		out = append(out, list[i].Name)
	}
	slices.Sort(out)
	return out, nil
}
