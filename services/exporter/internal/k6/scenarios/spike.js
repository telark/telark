// Spike: burst + recovery. Reads + one write. No sleep.

import exec from 'k6/execution';
import { request } from '../lib/http.js';
import { checkResponse } from '../lib/checks.js';
import { pathProviders, bodies } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';
import { preAllocatedVUs, maxVUs } from '../lib/vu.js';

const SCENARIO = 'spike';
const SPIKE_PEAK_RPS = 5000;

const COVERAGE = ['R4', 'R16', 'R23', 'R26', 'R31', 'R15'];

const WEIGHTS = {
  R4: 4, R16: 6, R23: 6, R26: 6, R31: 4,
  R15: 1, // single write present to detect write-path recovery
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
    spike: {
      executor: 'ramping-arrival-rate',
      startRate: 500,
      timeUnit: '1s',
      preAllocatedVUs: preAllocatedVUs(SPIKE_PEAK_RPS),
      maxVUs: maxVUs(SPIKE_PEAK_RPS),
      stages: [
        { target: 500,            duration: '1m'  }, // warm
        { target: SPIKE_PEAK_RPS, duration: '10s' }, // burst up
        { target: SPIKE_PEAK_RPS, duration: '1m'  }, // hold peak
        { target: 500,            duration: '10s' }, // drop
        { target: 500,            duration: '2m'  }, // recovery
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
    case 'R16': return {};
    case 'R23': return {};
    case 'R26': return { pathParams: { id: pathProviders.userID() } };
    case 'R31': return {};
    case 'R15': return { body: bodies.appCreate() };
    default: throw new Error(`no opts for ${rid}`);
  }
}

export default function () {
  fire(pickRoute());
}

export const handleSummary = buildHandleSummary(SCENARIO, SPIKE_PEAK_RPS, null);
