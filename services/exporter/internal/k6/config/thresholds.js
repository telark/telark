// Per-route + per-scenario thresholds. Tag key in checks/http is {route: <RouteID>, scenario: <name>}.
// p95/p99 budgets sourced from TEST_PLAN.md (Per-route threshold table).

import { ROUTES } from '../lib/routes.js';

// Per-route latency budgets (ms). Sourced from TEST_PLAN.md "Per-route threshold table".
const BUDGETS = {
  // probe
  R1:  { p95: 50,   p99: 100  },
  R2:  { p95: 50,   p99: 100  },
  // list (cached)
  R4:  { p95: 500,  p99: 1000 },
  R16: { p95: 500,  p99: 1000 },
  R23: { p95: 500,  p99: 1000 },
  R31: { p95: 500,  p99: 1000 },
  R36: { p95: 500,  p99: 1000 },
  R39: { p95: 500,  p99: 1000 },
  R41: { p95: 500,  p99: 1000 },
  R45: { p95: 500,  p99: 1000 },
  R54: { p95: 500,  p99: 1000 },
  // get (cached)
  R5:  { p95: 200,  p99: 400  },
  R19: { p95: 200,  p99: 400  },
  R24: { p95: 200,  p99: 400  },
  R25: { p95: 200,  p99: 400  },
  R26: { p95: 200,  p99: 400  },
  R32: { p95: 200,  p99: 400  },
  R37: { p95: 200,  p99: 400  },
  R38: { p95: 200,  p99: 400  },
  R40: { p95: 200,  p99: 400  },
  R46: { p95: 200,  p99: 400  },
  // get (uncached)
  R13: { p95: 400,  p99: 800  },
  R17: { p95: 400,  p99: 800  },
  R18: { p95: 400,  p99: 800  },
  R27: { p95: 400,  p99: 800  },
  R47: { p95: 400,  p99: 800  },
  R55: { p95: 400,  p99: 800  },
  R64: { p95: 400,  p99: 800  },
  R65: { p95: 400,  p99: 800  },
  // filesystem list
  R66: { p95: 1000, p99: 2000 },
  // create
  R15: { p95: 800,  p99: 1500 },
  R22: { p95: 800,  p99: 1500 },
  R30: { p95: 800,  p99: 1500 },
  R35: { p95: 800,  p99: 1500 },
  R44: { p95: 800,  p99: 1500 },
  R53: { p95: 800,  p99: 1500 },
  // create (filesystem)
  R63: { p95: 3000, p99: 6000 },
  // patch
  R14: { p95: 600,  p99: 1200 },
  R20: { p95: 600,  p99: 1200 },
  R28: { p95: 600,  p99: 1200 },
  R33: { p95: 600,  p99: 1200 },
  R42: { p95: 600,  p99: 1200 },
  R48: { p95: 600,  p99: 1200 },
  R56: { p95: 600,  p99: 1200 },
  // delete
  R7:  { p95: 600,  p99: 1200 },
  R21: { p95: 600,  p99: 1200 },
  R29: { p95: 600,  p99: 1200 },
  R34: { p95: 600,  p99: 1200 },
  R43: { p95: 600,  p99: 1200 },
  R49: { p95: 600,  p99: 1200 },
  R57: { p95: 600,  p99: 1200 },
};

// Error-rate budget per scenario.
const ERROR_BUDGET = {
  smoke:   0.0001,  // 0.01% (one bad request per 10k — effectively zero)
  load:    0.005,   // 0.5%
  stress:  0.05,    // 5%
  spike:   0.02,    // 2%
  journey: 0.001,   // 0.1%
  soak:    0.001,   // 0.1%
};

const JOURNEY_STEP_P95 = 500; // ms — stricter per task spec

export function buildThresholds(scenarioName, opts) {
  const abortOnFail = !!(opts && opts.abortOnFail);
  const routeIDs = (opts && opts.routes) || Object.keys(ROUTES);
  const out = {};

  // Smoke validates correctness, not latency. Latency budgets create breach counts
  // but never abort. Only real_failures aggregate aborts smoke.
  const isSmoke = scenarioName === 'smoke';
  const durationAbort = isSmoke ? false : abortOnFail;

  for (const rid of routeIDs) {
    const b = BUDGETS[rid];
    if (!b) continue;
    const tagFilter = `{route:${rid},scenario:${scenarioName}}`;
    const p95 = scenarioName === 'journey' ? Math.min(b.p95, JOURNEY_STEP_P95) : b.p95;
    out[`http_req_duration${tagFilter}`] = [{
      threshold: `p(95)<${p95}`,
      abortOnFail: durationAbort,
    }, {
      threshold: `p(99)<${b.p99}`,
      abortOnFail: false,
    }];
    // Materialize per-route submetrics so summary can read counts + per-route err rate.
    // Thresholds chosen to always hold (purely structural).
    out[`http_reqs${tagFilter}`] = [{ threshold: 'count>=0', abortOnFail: false }];
    out[`real_failures${tagFilter}`] = [{ threshold: 'rate<=1', abortOnFail: false }];
  }

  // Aggregate failure-rate threshold (always abort-aware per scenario opts).
  out[`real_failures{scenario:${scenarioName}}`] = [{
    threshold: `rate<${ERROR_BUDGET[scenarioName] ?? 0.005}`,
    abortOnFail: abortOnFail,
  }];

  return out;
}

export { BUDGETS, ERROR_BUDGET, JOURNEY_STEP_P95 };
