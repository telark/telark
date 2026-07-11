// Load: baseline at BASELINE_RPS for ~7 min. Ramping-arrival-rate.
// Read-heavy mix with some writes. Threshold breaches are signal, not abort.

import exec from 'k6/execution';
import { request } from '../lib/http.js';
import { checkResponse } from '../lib/checks.js';
import { pathProviders, bodies } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';
import { preAllocatedVUs, maxVUs } from '../lib/vu.js';

const SCENARIO = 'load';
const BASELINE_RPS = parseInt(__ENV.BASELINE_RPS || '500', 10);

const COVERAGE = [
  'R1','R2','R4','R5','R13',
  'R15','R16','R19',
  'R22','R23','R24','R25','R26',
  'R30','R31','R32',
  'R35','R36','R37',
  'R44','R45','R46',
];

// Weights (read-heavy). Sums need not normalize — handled at runtime.
const WEIGHTS = {
  R1: 1, R2: 1,
  R4: 5, R5: 8,
  R13: 2,
  R15: 1, R16: 6, R19: 8,
  R22: 1, R23: 6, R24: 4, R25: 4, R26: 8,
  R30: 1, R31: 6, R32: 6,
  R35: 1, R36: 6, R37: 6,
  R44: 1, R45: 4, R46: 4,
};

const WEIGHT_TABLE = buildWeightTable();

function buildWeightTable() {
  const table = [];
  for (const rid of COVERAGE) {
    const w = WEIGHTS[rid] || 1;
    for (let i = 0; i < w; i++) table.push(rid);
  }
  return table;
}

function pickRoute() {
  return WEIGHT_TABLE[exec.scenario.iterationInTest % WEIGHT_TABLE.length];
}

export const options = {
  scenarios: {
    load: {
      executor: 'ramping-arrival-rate',
      startRate: 0,
      timeUnit: '1s',
      preAllocatedVUs: preAllocatedVUs(BASELINE_RPS),
      maxVUs: maxVUs(BASELINE_RPS),
      stages: [
        { target: BASELINE_RPS, duration: '1m' },
        { target: BASELINE_RPS, duration: '5m' },
        { target: 0,            duration: '1m' },
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
    case 'R1':  return {};
    case 'R2':  return {};
    case 'R4':  return {};
    case 'R5':  return { pathParams: { id: pathProviders.planID() } };
    case 'R13': return {};
    case 'R15': return { body: bodies.appCreate() };
    case 'R16': return {};
    case 'R19': return { pathParams: { name: pathProviders.appName() } };
    case 'R22': return { body: bodies.userCreate() };
    case 'R23': return {};
    case 'R24': return { pathParams: { username: pathProviders.username() } };
    case 'R25': return { pathParams: { email: pathProviders.email() } };
    case 'R26': return { pathParams: { id: pathProviders.userID() } };
    case 'R30': return { body: bodies.groupCreate() };
    case 'R31': return {};
    case 'R32': return { pathParams: { id: pathProviders.groupID() } };
    case 'R35': return { body: bodies.roleCreate() };
    case 'R36': return {};
    case 'R37': return { pathParams: { id: pathProviders.roleID() } };
    case 'R44': return { body: bodies.categoryCreate() };
    case 'R45': return {};
    case 'R46': return { pathParams: { id: pathProviders.categoryID() } };
    default: throw new Error(`no opts for ${rid}`);
  }
}

export default function () {
  fire(pickRoute());
}

export const handleSummary = buildHandleSummary(SCENARIO, BASELINE_RPS, null);
