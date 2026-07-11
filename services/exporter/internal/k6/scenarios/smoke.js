// Smoke: 1 VU, 1 iteration. Every in-scope route hit once with correct wire format.
// abortOnFail: true on smoke thresholds.

import { request } from '../lib/http.js';
import { checkResponse } from '../lib/checks.js';
import { pathProviders, queryProviders, bodies } from '../config/data.js';
import { buildThresholds } from '../config/thresholds.js';
import { buildHandleSummary } from '../lib/summary.js';

const SCENARIO = 'smoke';

export const options = {
  scenarios: {
    smoke: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 1,
      maxDuration: '2m',
      tags: { scenario: SCENARIO },
    },
  },
  thresholds: buildThresholds(SCENARIO, { abortOnFail: true }),
};

function smoke(routeID, opts) {
  const res = request({ routeID, scenario: SCENARIO, ...opts });
  checkResponse(res, { routeID, scenario: SCENARIO, expectClass: opts.expectClass || 'read' });
}

export default function () {
  // ---- probes ----
  smoke('R1', {});
  smoke('R2', {});

  // ---- plans (read-only — R3/R6 require X-User-ID) ----
  smoke('R4', {});
  smoke('R5', { pathParams: { id: pathProviders.planID() } });
  smoke('R7', { pathParams: { id: pathProviders.planID() }, expectClass: 'delete' });

  // ---- global config ----
  smoke('R13', {});
  smoke('R14', { body: bodies.globalconfigPatch(), expectClass: 'patch' });

  // ---- applications ----
  smoke('R15', { body: bodies.appCreate() });
  smoke('R16', {});
  smoke('R17', { pathParams: { name: pathProviders.appName() } });
  smoke('R19', { pathParams: { name: pathProviders.appName() } });
  smoke('R20', { pathParams: { name: pathProviders.appName() }, body: bodies.appPatch(), expectClass: 'patch' });
  smoke('R21', { pathParams: { name: pathProviders.appName() }, expectClass: 'delete' });

  // ---- users ----
  smoke('R22', { body: bodies.userCreate() });
  smoke('R23', {});
  smoke('R24', { pathParams: { username: pathProviders.username() } });
  smoke('R25', { pathParams: { email: pathProviders.email() } });
  smoke('R26', { pathParams: { id: pathProviders.userID() } });
  smoke('R27', { query: queryProviders.identity() });
  smoke('R28', { pathParams: { id: pathProviders.userID() }, body: bodies.userPatch(), expectClass: 'patch' });
  smoke('R29', { pathParams: { id: pathProviders.userID() }, expectClass: 'delete' });

  // ---- groups ----
  smoke('R30', { body: bodies.groupCreate() });
  smoke('R31', {});
  smoke('R32', { pathParams: { id: pathProviders.groupID() } });
  smoke('R33', { pathParams: { id: pathProviders.groupID() }, body: bodies.groupPatch(), expectClass: 'patch' });
  smoke('R34', { pathParams: { id: pathProviders.groupID() }, expectClass: 'delete' });

  // ---- roles ----
  smoke('R35', { body: bodies.roleCreate() });
  smoke('R36', {});
  smoke('R37', { pathParams: { id: pathProviders.roleID() } });
  smoke('R38', { pathParams: { userId: pathProviders.userID() } });
  smoke('R39', { pathParams: { userId: pathProviders.userID() } });
  smoke('R40', { pathParams: { groupId: pathProviders.groupID() } });
  smoke('R41', { pathParams: { groupId: pathProviders.groupID() } });
  smoke('R42', { pathParams: { id: pathProviders.roleID() }, body: bodies.rolePatch(), expectClass: 'patch' });
  smoke('R43', { pathParams: { id: pathProviders.roleID() }, expectClass: 'delete' });

  // ---- categories ----
  smoke('R44', { body: bodies.categoryCreate() });
  smoke('R45', {});
  smoke('R46', { pathParams: { id: pathProviders.categoryID() } });
  smoke('R47', { pathParams: { scope: pathProviders.categoryScope() } });
  smoke('R48', { pathParams: { id: pathProviders.categoryID() }, body: bodies.categoryPatch(), expectClass: 'patch' });
  smoke('R49', { pathParams: { id: pathProviders.categoryID() }, expectClass: 'delete' });

  // ---- sessions ----
  const sUserID = pathProviders.userID();
  smoke('R53', { pathParams: { userId: sUserID }, body: bodies.sessionCreate(sUserID) });
  smoke('R54', { pathParams: { userId: sUserID } });
  smoke('R55', { pathParams: { token: pathProviders.sessionToken() } });
  smoke('R56', { pathParams: { token: pathProviders.sessionToken() }, body: bodies.sessionPatch(), expectClass: 'patch' });
  smoke('R57', { pathParams: { token: pathProviders.sessionToken() }, expectClass: 'delete' });

  // ---- snapshots ----
  smoke('R63', { body: bodies.snapshotCreate() });
  smoke('R66', {});
}

export const handleSummary = buildHandleSummary(SCENARIO, 1, null);
