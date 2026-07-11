// Stress: climb to find ceiling. No sleep. ramping-arrival-rate.
// Stages locked per task spec.

import exec from 'k6/execution';
import { request } from '../lib/http.js';
import { checkResponse } from '../lib/checks.js';
import { pathProviders, bodies } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';
import { preAllocatedVUs, maxVUs } from '../lib/vu.js';

const SCENARIO = 'stress';
const MAX_RPS = parseInt(__ENV.MAX_RPS || '10000', 10);

// Stress focuses on highest-throughput read routes + write routes that exercise k8sCreateSem.
const COVERAGE = [
  'R4','R5','R16','R19','R23','R26','R31','R36','R45',
  'R15','R22','R30','R35','R44',
];

const WEIGHTS = {
  R4: 6, R5: 6, R16: 6, R19: 8, R23: 6, R26: 8, R31: 6, R36: 6, R45: 4,
  // Writes: lower weight; service-side semaphore caps real throughput.
  R15: 1, R22: 1, R30: 1, R35: 1, R44: 1,
};

const WEIGHT_TABLE = (() => {
  const t = [];
  for (const r of COVERAGE) for (let i = 0; i < (WEIGHTS[r] || 1); i++) t.push(r);
  return t;
})();

function pickRoute() {
  return WEIGHT_TABLE[exec.scenario.iterationInTest % WEIGHT_TABLE.length];
}

export const options = {
  scenarios: {
    stress: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: preAllocatedVUs(MAX_RPS),
      maxVUs: maxVUs(MAX_RPS),
      stages: [
        { target: 500,     duration: '1m' },
        { target: 500,     duration: '1m' },
        { target: 2000,    duration: '2m' },
        { target: 5000,    duration: '2m' },
        { target: MAX_RPS, duration: '2m' },
        { target: 0,       duration: '2m' },
      ],
      tags: { scenario: SCENARIO },
    },
  },
  thresholds: buildThresholds(SCENARIO, { routes: COVERAGE, abortOnFail: false }),
};

function fire(routeID) {
  const opts = buildOpts(routeID);
  const res = request({ routeID, scenario: SCENARIO, ...opts });
  checkResponse(res, { routeID, scenario: SCENARIO, expectClass: opts.expectClass || 'read' });
}

function buildOpts(rid) {
  switch (rid) {
    case 'R4':  return {};
    case 'R5':  return { pathParams: { id: pathProviders.planID() } };
    case 'R16': return {};
    case 'R19': return { pathParams: { name: pathProviders.appName() } };
    case 'R23': return {};
    case 'R26': return { pathParams: { id: pathProviders.userID() } };
    case 'R31': return {};
    case 'R36': return {};
    case 'R45': return {};
    case 'R15': return { body: bodies.appCreate() };
    case 'R22': return { body: bodies.userCreate() };
    case 'R30': return { body: bodies.groupCreate() };
    case 'R35': return { body: bodies.roleCreate() };
    case 'R44': return { body: bodies.categoryCreate() };
    default: throw new Error(`no opts for ${rid}`);
  }
}

export default function () {
  fire(pickRoute());
}

export const handleSummary = buildHandleSummary(SCENARIO, MAX_RPS, MAX_RPS);
