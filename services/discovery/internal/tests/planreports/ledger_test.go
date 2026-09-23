package planreports

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	restmapper "github.com/telark/rest/mappers"
)

const (
	longMessageRunes = 2000
	worstCaseRunes   = 512
	fieldChars       = 64
	ledgerBodyLimit  = 32 << 20
	previousRun      = "old"
	lockValue        = "someone-else"
)

func TestMergeFencesRowsToRunWindowDropsPreviousRunEvents(t *testing.T) {
	live := []planseps.ProtectionPlanViolation{
		row("previous", stamp(startedAt, -afterFive), "before start"),
		row("current", stamp(startedAt, afterFive), "after start"),
	}

	rows, truncated := reports.Merge(nil, live, activePlan(), maxRows)

	testutil.Equal(t, "rows", len(rows), one)
	testutil.Equal(t, "kept uid", rows[zero].EventUID, "current")
	testutil.Equal(t, nameTruncated, truncated, false)
}

func TestMergeDedupsByEventUIDCapsMessagesUsesPage(t *testing.T) {
	existing := []planseps.ProtectionPlanViolation{row(uidOne, stamp(startedAt, afterTwo), "old copy")}
	live := []planseps.ProtectionPlanViolation{
		row(uidOne, stamp(startedAt, afterTwo), strings.Repeat("x", longMessageRunes)),
		row("u2", stamp(startedAt, afterOne), "second"),
	}

	rows, truncated := reports.Merge(existing, live, activePlan(), maxRows)
	testutil.Equal(t, "deduped", len(rows), two)
	testutil.Equal(t, "newest first", rows[zero].EventUID, uidOne)
	capped := reports.MaxMessageRunes + utf8.RuneCountInString(reports.MessageEllipsis)
	testutil.Equal(t, "capped runes", utf8.RuneCountInString(rows[zero].Message), capped)
	testutil.Equal(t, "ellipsis", strings.HasSuffix(rows[zero].Message, reports.MessageEllipsis), true)
	testutil.Equal(t, "not truncated", truncated, false)

	page, truncated := reports.Merge(existing, live, activePlan(), one)
	testutil.Equal(t, "page", len(page), one)
	testutil.Equal(t, nameTruncated, truncated, true)
}

func TestLoadLedgerStartsEmptyOnRunMismatch(t *testing.T) {
	store := &fakeStore{stored: storedLedger(t, previousRun, row(uidOne, stamp(startedAt, afterOne), "stale"))}
	gen, _ := newGen(store, fakeDyn(), nil)

	ledger, err := gen.Checkpoint(context.Background(), activePlan())
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	testutil.Equal(t, "run", ledger.Run, runKey)
	testutil.Equal(t, "rows", len(ledger.Violations), zero)
	testutil.Equal(t, "plan", ledger.PlanID, planID)
	puts, _, _ := store.snapshot()
	testutil.Equal(t, "persisted", len(puts), one)
	testutil.Equal(t, "persisted run", decodeLedger(t, puts[zero]).Run, runKey)
}

func TestPersistSkipsWriteWhenLockHeldAndUsesUUIDValue(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	key := constants.KeyPrefixReportLedgerLock + planID
	if err := mr.Set(key, lockValue); err != nil {
		t.Fatalf("preset lock: %v", err)
	}
	store := &fakeStore{}
	gen, logger := newGen(store, fakeDyn(), rdb)

	if _, err := gen.Checkpoint(context.Background(), activePlan()); err != nil {
		t.Fatalf("checkpoint under lock: %v", err)
	}
	puts, _, _ := store.snapshot()
	testutil.Equal(t, "writes while locked", len(puts), zero)
	testutil.Equal(t, "locked log", logger.has(fmt.Sprintf(reports.LogLedgerLocked, planID)), true)

	mr.Del(key)
	var values []string
	store.onPut = func() {
		held, err := mr.Get(key)
		if err != nil {
			t.Errorf("lock value: %v", err)
		}
		values = append(values, held)
	}
	for range two {
		if _, err := gen.Checkpoint(context.Background(), activePlan()); err != nil {
			t.Fatalf("checkpoint: %v", err)
		}
	}
	testutil.Equal(t, "lock values", len(values), two)
	for _, v := range values {
		if _, err := uuid.Parse(v); err != nil {
			t.Fatalf("lock value %q is not a uuid: %v", v, err)
		}
	}
	testutil.Equal(t, "distinct values", values[zero] != values[one], true)
	testutil.Equal(t, "lock released", mr.Exists(key), false)
}

func TestLedgerJSONRoundTripsThroughRealMapperAndKeysAreCamelCase(t *testing.T) {
	ledger := reports.PlanReportLedger{
		PlanID:           planID,
		Run:              runKey,
		UpdatedAt:        startedAt,
		RenderedPolicies: []string{planPolicy},
		Checkpoints:      []reports.LedgerCheckpoint{{At: startedAt, Health: plans.HealthHealthy, ViolationsSeen: one}},
		Violations:       []planseps.ProtectionPlanViolation{row(uidOne, startedAt, "m")},
		Truncated:        true,
	}
	raw := ledgerBytes(t, ledger)

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"planId", "run", "updatedAt", "renderedPolicies", "checkpoints", "violations", nameTruncated} {
		if _, ok := top[key]; !ok {
			t.Fatalf("missing top-level key %q in %s", key, raw)
		}
	}
	var checkpoints []map[string]json.RawMessage
	if err := json.Unmarshal(top["checkpoints"], &checkpoints); err != nil {
		t.Fatalf("decode checkpoints: %v", err)
	}
	for _, key := range []string{"at", "health", "violationsSeen"} {
		if _, ok := checkpoints[zero][key]; !ok {
			t.Fatalf("missing checkpoint key %q", key)
		}
	}

	mapped, err := restmapper.MapToJSONPayload(ledger)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if _, bad := mapped[""]; bad {
		t.Fatalf("mapper produced an empty key: %v", mapped)
	}
	testutil.Equal(t, "round trip", decodeLedger(t, raw).Violations[zero].EventUID, uidOne)
}

func worstCaseRows(n int) []planseps.ProtectionPlanViolation {
	message := strings.Repeat(`<&"`, worstCaseRunes/3)
	field := strings.Repeat("f", fieldChars)
	rows := make([]planseps.ProtectionPlanViolation, zero, n)
	for i := range n {
		rows = append(rows, planseps.ProtectionPlanViolation{
			Policy:    field,
			Rule:      field,
			Namespace: field,
			Resource:  planseps.ProtectionPlanResource{Kind: field, Name: field, Namespace: field},
			Result:    "fail",
			Message:   message,
			Timestamp: stamp(startedAt, time.Duration(i)*time.Second),
			EventUID:  uuid.NewString(),
		})
	}
	return rows
}

func TestLedgerMaxRowsWorstCaseUnderBodyLimit(t *testing.T) {
	raw := ledgerBytes(t, reports.PlanReportLedger{PlanID: planID, Run: runKey, Violations: worstCaseRows(maxRows)})

	testutil.Equal(t, "under limit", len(raw) < ledgerBodyLimit, true)
}

func TestRunKeyRequiresStartedAtAndErrorsOnGarbage(t *testing.T) {
	plan := activePlan()
	key, err := reports.RunKey(plan)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "key", key, runKey)

	plan.StartedAt = nil
	if _, err := reports.RunKey(plan); err == nil {
		t.Fatal("nil StartedAt: expected an error")
	}
	plan.StartedAt = ptr("yesterday")
	if _, err := reports.RunKey(plan); err == nil {
		t.Fatal("garbage StartedAt: expected an error")
	}
}
