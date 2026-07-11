// Journey: end-to-end flows at 50 RPS for 5 min. Per-step p95 < 500ms.
// Path: constant-arrival-rate (per spec — per-iteration VU model only for smoke + journey).

import exec from 'k6/execution';
import { request } from '../lib/http.js';
import { checkResponse, captureJSONField } from '../lib/checks.js';
import { pathProviders, bodies } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';
import { preAllocatedVUs, maxVUs } from '../lib/vu.js';

const SCENARIO = 'journey';
const JOURNEY_RPS = 50;

// J2 + J3 routes (J1 needs X-User-ID — out of scope).
const COVERAGE = ['R22', 'R30', 'R35', 'R28', 'R26', 'R53', 'R55', 'R57'];

export const options = {
  scenarios: {
    journey: {
      executor: 'constant-arrival-rate',
      rate: JOURNEY_RPS,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: preAllocatedVUs(JOURNEY_RPS),
      maxVUs: maxVUs(JOURNEY_RPS),
      tags: { scenario: SCENARIO },
    },
  },
  thresholds: buildThresholds(SCENARIO, { routes: COVERAGE, abortOnFail: false }),
};

function call(routeID, opts) {
  const res = request({ routeID, scenario: SCENARIO, ...opts });
  checkResponse(res, { routeID, scenario: SCENARIO, expectClass: opts.expectClass || 'read' });
  return res;
}

// J2 — user + group + role assignment.
function journeyUserGroupRole() {
  const userBody = bodies.userCreate();
  const userRes = call('R22', { body: userBody });
  const userID = captureJSONField(userRes, 'id') || userBody.id;

  call('R30', { body: bodies.groupCreate() });
  call('R35', { body: bodies.roleCreate() });
  call('R28', { pathParams: { id: userID }, body: bodies.userPatch(), expectClass: 'patch' });
  call('R26', { pathParams: { id: userID } });
}

// J3 — session lifecycle.
function journeySession() {
  const userID = pathProviders.userID();
  const createBody = bodies.sessionCreate(userID);
  const createRes = call('R53', { pathParams: { userId: userID }, body: createBody });
  const token = captureJSONField(createRes, 'sessionToken') || createBody.sessionToken;
  call('R55', { pathParams: { token } });
  call('R57', { pathParams: { token }, expectClass: 'delete' });
}

const JOURNEYS = [journeyUserGroupRole, journeySession];

export default function () {
  // Round-robin journeys by iteration index.
  const j = JOURNEYS[exec.scenario.iterationInTest % JOURNEYS.length];
  j();
}

export const handleSummary = buildHandleSummary(SCENARIO, JOURNEY_RPS, null);
