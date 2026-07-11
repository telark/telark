// HTTP wrapper. Every request tagged {route, scenario}. Business-4xx classifier
// pulled from TEST_PLAN.md (Business 4xx exclusion table).

import http from 'k6/http';
import { Rate } from 'k6/metrics';
import { BASE_URL } from '../config/data.js';
import { ROUTES, buildPath } from './routes.js';

export const realFailures = new Rate('real_failures');

// Business 4xx exclusion — codes that DO NOT count as failures.
// Maps route ID -> set of HTTP codes excluded.
const BUSINESS_EXCLUSIONS = {
  R5:  [404], R7:  [404], R17: [404], R18: [404], R19: [404],
  R24: [404], R25: [404], R26: [404], R27: [404], R32: [404],
  R37: [404], R38: [404], R40: [404], R46: [404], R55: [404],
  R64: [404], R65: [404],
};

// Stress-only tolerated codes (409 conflict on patches, 503 on saturated creates).
const STRESS_TOLERATED = {
  R14: [409], R20: [409], R28: [409], R33: [409], R42: [409], R48: [409], R56: [409],
  R15: [503], R22: [503], R30: [503], R35: [503], R44: [503], R53: [503], R63: [503],
};

function isExcluded(routeID, status, scenario) {
  const exact = BUSINESS_EXCLUSIONS[routeID];
  if (exact && exact.indexOf(status) !== -1) return true;
  if (scenario === 'stress') {
    const tol = STRESS_TOLERATED[routeID];
    if (tol && tol.indexOf(status) !== -1) return true;
  }
  return false;
}

function jsonHeaders(extra) {
  const h = { 'Content-Type': 'application/json', 'Accept': 'application/json' };
  if (extra) for (const k of Object.keys(extra)) h[k] = extra[k];
  return h;
}

export function request(opts) {
  const { routeID, pathParams, query, body, headers, scenario } = opts;
  const route = ROUTES[routeID];
  if (!route) throw new Error(`unknown route id: ${routeID}`);

  let url = `${BASE_URL}${buildPath(route, pathParams)}`;
  if (query) {
    const qs = Object.keys(query)
      .filter((k) => query[k] !== undefined && query[k] !== null && query[k] !== '')
      .map((k) => `${encodeURIComponent(k)}=${encodeURIComponent(query[k])}`)
      .join('&');
    if (qs) url += `?${qs}`;
  }

  const tags = { route: routeID, scenario };
  const params = { tags, headers: jsonHeaders(headers) };

  let res;
  switch (route.method) {
    case 'GET':    res = http.get(url, params); break;
    case 'DELETE': res = http.del(url, body ? JSON.stringify(body) : null, params); break;
    case 'POST':   res = http.post(url, body ? JSON.stringify(body) : null, params); break;
    case 'PATCH':  res = http.patch(url, body ? JSON.stringify(body) : null, params); break;
    case 'PUT':    res = http.put(url, body ? JSON.stringify(body) : null, params); break;
    default:       throw new Error(`unsupported method: ${route.method}`);
  }

  const failed = res.status >= 400 && !isExcluded(routeID, res.status, scenario);
  realFailures.add(failed, tags);
  return res;
}
