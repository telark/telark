package coordination

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func AdvertiseReplica(ctx context.Context, rdb *redis.Client, replicaID, addr string) {
	if rdb == nil {
		return
	}
	pipe := rdb.Pipeline()
	pipe.Set(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaHB,
		time.Now().UTC().String(), xwareredis.ReplicaHeartbeatTTL)
	// A replica ID is a pod name, which nothing resolves; peers dial this instead.
	if addr != constants.EmptyString {
		pipe.Set(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaAddr,
			addr, xwareredis.ReplicaHeartbeatTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func ReplicaAddress(ctx context.Context, rdb *redis.Client, replicaID string) string {
	if rdb == nil {
		return constants.EmptyString
	}
	addr, _ := rdb.Get(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaAddr).Result()
	return addr
}

// Redis is untrusted, so the address it holds is used only when the Kubernetes API confirms it
// belongs to the leader pod, and that pod carries this replica's own component label.
func VerifiedReplicaAddress(
	ctx context.Context,
	rdb *redis.Client,
	pods corev1client.PodInterface,
	selfID, leaderID string,
) (string, error) {
	addr := ReplicaAddress(ctx, rdb, leaderID)
	if addr == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrForceSyncLeaderNotAvailable))
	}
	if net.ParseIP(addr) == nil {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrLeaderAddressInvalid), leaderID, addr)
	}
	self, err := pods.Get(ctx, selfID, metav1.GetOptions{})
	if err != nil {
		return constants.EmptyString, err
	}
	leader, err := pods.Get(ctx, leaderID, metav1.GetOptions{})
	if err != nil {
		return constants.EmptyString, err
	}
	component := self.Labels[constants.LabelComponent]
	if component == constants.EmptyString || leader.Labels[constants.LabelComponent] != component ||
		leader.Status.PodIP != addr {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrLeaderPodUnverified), leaderID)
	}
	return addr, nil
}
