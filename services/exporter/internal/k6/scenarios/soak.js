// Soak: 30 min @ 500 RPS. Skeleton ONLY — body commented for off-hours run.
// Reads same shape as load.js coverage but skips writes to avoid CRD residue accumulation.

import exec from 'k6/execution';
import { request } from '../lib/http.js';
import { checkResponse } from '../lib/checks.js';
import { pathProviders } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';
import { preAllocatedVUs, maxVUs } from '../lib/vu.js';

const SCENARIO = 'soak';
const SOAK_RPS = parseInt(__ENV.BASELINE_RPS || '500', 10);

const COVERAGE = [
  'R1','R2','R4','R5','R13','R16','R19','R23','R24','R25','R26','R31','R32','R36','R37','R45','R46',
];

const WEIGHT_TABLE = COVERAGE;

function pickRoute() {
  return WEIGHT_TABLE[exec.scenario.iterationInTest % WEIGHT_TABLE.length];
}

export const options = {
  scenarios: {
    // Hourki: enable for off-hours run
    // soak: {
    //   executor: 'constant-arrival-rate',
    //   rate: SOAK_RPS,
    //   timeUnit: '1s',
    //   duration: '30m',
    //   preAllocatedVUs: preAllocatedVUs(SOAK_RPS),
    //   maxVUs: maxVUs(SOAK_RPS),
    //   tags: { scenario: SCENARIO },
    // },
  },
  // Hourki: enable for off-hours run
  // thresholds: buildThresholds(SCENARIO, { routes: COVERAGE, abortOnFail: false }),
};

function fire(routeID) {
  const opts = buildOpts(routeID);
  const res = request({ routeID, scenario: SCENARIO, ...opts });
  checkResponse(res, { routeID, scenario: SCENARIO, expectClass: 'read' });
}

function buildOpts(rid) {
  switch (rid) {
    case 'R1':  return {};
    case 'R2':  return {};
    case 'R4':  return {};
    case 'R5':  return { pathParams: { id: pathProviders.planID() } };
    case 'R13': return {};
    case 'R16': return {};
    case 'R19': return { pathParams: { name: pathProviders.appName() } };
    case 'R23': return {};
    case 'R24': return { pathParams: { username: pathProviders.username() } };
    case 'R25': return { pathParams: { email: pathProviders.email() } };
    case 'R26': return { pathParams: { id: pathProviders.userID() } };
    case 'R31': return {};
    case 'R32': return { pathParams: { id: pathProviders.groupID() } };
    case 'R36': return {};
    case 'R37': return { pathParams: { id: pathProviders.roleID() } };
    case 'R45': return {};
    case 'R46': return { pathParams: { id: pathProviders.categoryID() } };
    default: throw new Error(`no opts for ${rid}`);
  }
}

export default function () {
  fire(pickRoute());
}

// Hourki: enable handleSummary alongside scenario activation
// export const handleSummary = buildHandleSummary(SCENARIO, SOAK_RPS, null);

// Avoid unused-import warnings while skeleton is dormant.
void preAllocatedVUs;
void maxVUs;
void buildThresholds;
void buildHandleSummary;
