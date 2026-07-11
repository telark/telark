// Response checks. Tagged identically to http.js.

import { check } from 'k6';

const ACCEPTABLE_4XX = {
  read: [404],
  patch: [404, 409],
  delete: [404, 409],
};

function isOK(res, expectClass) {
  if (res.status >= 200 && res.status < 300) return true;
  const allowed = ACCEPTABLE_4XX[expectClass];
  if (allowed && allowed.indexOf(res.status) !== -1) return true;
  return false;
}

export function checkResponse(res, opts) {
  const { routeID, scenario, expectClass } = opts;
  const tags = { route: routeID, scenario };
  return check(res, {
    'status acceptable': (r) => isOK(r, expectClass),
  }, tags);
}

export function captureJSONField(res, field) {
  try {
    const obj = res.json();
    if (!obj) return null;
    if (obj.data && obj.data[field] !== undefined) return obj.data[field];
    if (obj[field] !== undefined) return obj[field];
    return null;
  } catch (_e) {
    return null;
  }
}
