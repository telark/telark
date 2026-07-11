// handleSummary entry point. Writes JSON to k6/results/<scenario>-<UTC>.json
// + stdout table sorted by descending p95. Flags LOAD GENERATOR SATURATED
// and CEILING NOT REACHED.

const RESULTS_DIR = __ENV.RESULTS_DIR || 'k6/results';

function fmt(n) {
  if (n === null || n === undefined || isNaN(n)) return '-';
  if (n < 10) return n.toFixed(2);
  if (n < 100) return n.toFixed(1);
  return Math.round(n).toString();
}

function pad(s, n, right) {
  s = String(s);
  if (s.length >= n) return s;
  const padding = ' '.repeat(n - s.length);
  return right ? s + padding : padding + s;
}

// Parse metric name + tag filter into {base, tags{}}
function parseMetric(name) {
  const idx = name.indexOf('{');
  if (idx === -1) return { base: name, tags: {} };
  const base = name.slice(0, idx);
  const tagBlob = name.slice(idx + 1, -1);
  const tags = {};
  for (const part of tagBlob.split(',')) {
    const eq = part.indexOf(':');
    if (eq !== -1) tags[part.slice(0, eq).trim()] = part.slice(eq + 1).trim();
  }
  return { base, tags };
}

function extractDurations(data) {
  const out = {};
  for (const name of Object.keys(data.metrics)) {
    const { base, tags } = parseMetric(name);
    if (base !== 'http_req_duration' || !tags.route || !tags.scenario) continue;
    const m = data.metrics[name];
    const vals = m.values || {};
    out[tags.route] = out[tags.route] || {};
    out[tags.route][tags.scenario] = {
      p50: vals['p(50)'],
      p95: vals['p(95)'],
      p99: vals['p(99)'],
      max: vals.max,
      avg: vals.avg,
      count: vals.count,
    };
  }
  // Backfill reqs count from http_reqs submetric (more reliable than trend.count).
  for (const name of Object.keys(data.metrics)) {
    const { base, tags } = parseMetric(name);
    if (base !== 'http_reqs' || !tags.route || !tags.scenario) continue;
    if (!out[tags.route] || !out[tags.route][tags.scenario]) continue;
    const reqs = data.metrics[name].values?.count;
    if (reqs !== undefined) out[tags.route][tags.scenario].count = reqs;
  }
  return out;
}

function extractErrors(data) {
  const out = {};
  for (const name of Object.keys(data.metrics)) {
    const { base, tags } = parseMetric(name);
    if (base !== 'real_failures' || !tags.scenario) continue;
    const m = data.metrics[name];
    const route = tags.route || '_aggregate';
    out[route] = out[route] || {};
    out[route][tags.scenario] = {
      rate: m.values?.rate,
      fails: m.values?.fails ?? 0,
      passes: m.values?.passes ?? 0,
    };
  }
  return out;
}

function detectSaturation(data, targetRps) {
  const totalReqs = data.metrics.http_reqs?.values?.count ?? 0;
  const duration = data.state?.testRunDurationMs ?? 0;
  if (!duration) return { saturated: false, achievedRps: 0 };
  const achievedRps = (totalReqs * 1000) / duration;
  const saturated = targetRps > 0 && achievedRps < targetRps * 0.95;
  return { saturated, achievedRps };
}

function findBreachedThresholds(data) {
  const breaches = [];
  for (const name of Object.keys(data.metrics)) {
    const m = data.metrics[name];
    if (!m.thresholds) continue;
    for (const tname of Object.keys(m.thresholds)) {
      if (m.thresholds[tname].ok === false) {
        breaches.push({ metric: name, threshold: tname });
      }
    }
  }
  return breaches;
}

function collectFailedRoutes(errors, scenario) {
  const out = [];
  for (const route of Object.keys(errors)) {
    if (route === '_aggregate') continue;
    const e = errors[route]?.[scenario];
    if (e && (e.fails ?? 0) > 0) out.push(`${route}(${e.fails})`);
  }
  return out;
}

function renderTable(durations, errors) {
  const rows = [];
  for (const route of Object.keys(durations)) {
    for (const scenario of Object.keys(durations[route])) {
      const d = durations[route][scenario];
      const e = errors[route]?.[scenario] || { rate: 0, fails: 0 };
      rows.push({
        route, scenario,
        count: d.count,
        p50: d.p50, p95: d.p95, p99: d.p99, max: d.max,
        errRate: e.rate, errCount: e.fails,
      });
    }
  }
  rows.sort((a, b) => (b.p95 ?? 0) - (a.p95 ?? 0));

  const header = `${pad('route', 8, true)} ${pad('scenario', 10, true)} ${pad('reqs', 7)} ${pad('p50', 8)} ${pad('p95', 8)} ${pad('p99', 8)} ${pad('max', 8)} ${pad('err%', 7)} ${pad('errs', 5)}`;
  const lines = [header, '-'.repeat(header.length)];
  for (const r of rows) {
    lines.push(
      `${pad(r.route, 8, true)} ${pad(r.scenario, 10, true)} ${pad(fmt(r.count), 7)} ${pad(fmt(r.p50), 8)} ${pad(fmt(r.p95), 8)} ${pad(fmt(r.p99), 8)} ${pad(fmt(r.max), 8)} ${pad(fmt((r.errRate ?? 0) * 100), 7)} ${pad(fmt(r.errCount), 5)}`,
    );
  }
  return lines.join('\n');
}

export function buildHandleSummary(scenarioName, targetRps, ceilingMaxRps) {
  return function handleSummary(data) {
    const durations = extractDurations(data);
    const errors = extractErrors(data);
    const { saturated, achievedRps } = detectSaturation(data, targetRps);
    const breaches = findBreachedThresholds(data);

    const now = new Date().toISOString().replace(/[:.]/g, '-');
    const jsonPath = `${RESULTS_DIR}/${scenarioName}-${now}.json`;

    const ceilingReached = breaches.length > 0 || saturated;
    const ceilingMessage = (scenarioName === 'stress' && ceilingMaxRps && !ceilingReached)
      ? `CEILING NOT REACHED — raise MAX_RPS above ${ceilingMaxRps} and rerun`
      : null;

    const summary = {
      scenario: scenarioName,
      targetRps,
      achievedRps,
      loadGeneratorSaturated: saturated,
      ceilingReached,
      ceilingMessage,
      breaches,
      routes: durations,
      errors,
      raw: {
        totalReqs: data.metrics.http_reqs?.values?.count,
        durationMs: data.state?.testRunDurationMs,
      },
    };

    const table = renderTable(durations, errors);
    const failedRoutes = collectFailedRoutes(errors, scenarioName);
    const stdoutLines = [
      `\n=== k6 summary: ${scenarioName} ===`,
      `target RPS: ${targetRps}   achieved RPS: ${fmt(achievedRps)}`,
      saturated ? '*** LOAD GENERATOR SATURATED ***' : '',
      ceilingMessage ? `*** ${ceilingMessage} ***` : '',
      breaches.length ? `*** BREACHED THRESHOLDS: ${breaches.length} ***` : 'thresholds: all green',
      failedRoutes.length ? `*** FAILED ROUTES: ${failedRoutes.join(', ')} ***` : '',
      '',
      table,
      '',
      `JSON written: ${jsonPath}`,
      '',
    ].filter((l) => l !== '').join('\n');

    return {
      stdout: stdoutLines,
      [jsonPath]: JSON.stringify(summary, null, 2),
    };
  };
}
