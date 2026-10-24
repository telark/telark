package webhooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/logger"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	xstream "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/notifier/internal/constants"
)

// consumer names this replica in every target's consumer group; the pod name keeps it unique.
func NewService(rdb *redis.Client, path, consumer string, reload, poll, retryScan time.Duration) *Service {
	return &Service{
		rdb:       rdb,
		stream:    xstream.NewStreamClient(rdb),
		consumer:  consumer,
		path:      path,
		reload:    reload,
		poll:      poll,
		retryScan: retryScan,
		log:       logger.GetLogger(constants.PrefixWebhooks),
		loops:     map[string]*loop{},
	}
}

// Re-reads the targets file every reload interval (the kubelet refreshes a mounted Secret on its
// own, so no restart is needed) and keeps one delivery loop per target until ctx ends.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.reload)
	defer ticker.Stop()
	var seen []byte
	for {
		raw, err := s.read()
		switch {
		case err != nil:
			s.log.Error(fmt.Sprintf(constants.ErrWebhookTargetsRead, err))
		case seen == nil || !bytes.Equal(raw, seen):
			// A refused file is logged once, and the targets already running keep going.
			if targets, err := ParseTargets(raw); err != nil {
				s.log.Error(fmt.Sprintf(constants.ErrWebhookTargetsRefused, err))
			} else {
				s.apply(ctx, targets)
			}
			seen = raw
		default:
		}
		select {
		case <-ctx.Done():
			s.stopAll()
			return
		case <-ticker.C:
		}
	}
}

// No mounted file means no targets.
func (s *Service) read() ([]byte, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return []byte(constants.WebhookNoTargets), nil
	}
	return raw, err
}

func (s *Service) apply(ctx context.Context, targets []Target) {
	wanted := map[string]Target{}
	names := make([]string, constants.DefaultInitValue, len(targets))
	for _, target := range targets {
		wanted[target.Config.Name] = target
		names = append(names, target.Config.Name)
	}
	for name, running := range s.loops {
		if target, ok := wanted[name]; !ok || !reflect.DeepEqual(target.Config, running.target.Config) {
			running.stop()
			delete(s.loops, name)
		}
	}
	s.destroyRemovedGroups(ctx, wanted)
	for _, target := range targets {
		if _, running := s.loops[target.Config.Name]; !running {
			s.start(ctx, target)
		}
	}
	s.log.Info(fmt.Sprintf(constants.InfoWebhookTargetsLoaded, len(targets), strings.Join(names, constants.WebhookNameJoin)))
}

// A new target's group starts after the newest event, so it gets no backlog; an edited
// target keeps its group, and with it the deliveries still owed.
func (s *Service) start(ctx context.Context, target Target) {
	group := constants.WebhookGroupPrefix + target.Config.Name
	if err := s.stream.EnsureConsumerGroupFromNow(ctx, notifclient.EventsStream, group); err != nil {
		s.log.Warn(fmt.Sprintf(constants.ErrWebhookGroup, target.Config.Name, err))
	}
	loopCtx, cancel := context.WithCancel(ctx)
	running := &loop{target: target, client: newClient(target.Config.AllowPrivate), cancel: cancel, done: make(chan struct{})}
	worker := &deliverer{Service: s, target: target, client: running.client, group: group}
	go func() {
		defer close(running.done)
		worker.run(loopCtx)
	}()
	s.loops[target.Config.Name] = running
}

// Every replica does this on reload, so a group survives no longer than its target.
func (s *Service) destroyRemovedGroups(ctx context.Context, wanted map[string]Target) {
	groups, err := s.rdb.XInfoGroups(ctx, notifclient.EventsStream).Result()
	if err != nil {
		if !strings.Contains(err.Error(), constants.WebhookNoSuchKeyError) {
			s.log.Warn(fmt.Sprintf(constants.ErrWebhookGroup, constants.WebhookStatusNone, err))
		}
		return
	}
	for _, group := range groups {
		name, ours := strings.CutPrefix(group.Name, constants.WebhookGroupPrefix)
		if _, kept := wanted[name]; !ours || kept {
			continue
		}
		if err := s.rdb.XGroupDestroy(ctx, notifclient.EventsStream, group.Name).Err(); err != nil {
			s.log.Warn(fmt.Sprintf(constants.ErrWebhookGroup, name, err))
		}
	}
}

func (s *Service) stopAll() {
	for name, running := range s.loops {
		running.stop()
		delete(s.loops, name)
	}
}

// An edited or removed target leaves no keep-alive connection behind.
func (l *loop) stop() {
	l.cancel()
	<-l.done
	l.client.CloseIdleConnections()
}
