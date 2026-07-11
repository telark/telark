// Seed data + payload generators. All env-tunable. SharedArray for memory efficiency.

import { SharedArray } from 'k6/data';
import exec from 'k6/execution';

export const SEED_POOL_SIZE = parseInt(__ENV.SEED_POOL_SIZE || '1000', 10);
export const BASE_URL = __ENV.BASE_URL || 'http://telark-exporter-service.telark.svc.cluster.local:8080';

const PREFIX = __ENV.SEED_PREFIX || 'k6';

function isoNow() {
  return new Date().toISOString();
}

function isoOffset(ms) {
  return new Date(Date.now() + ms).toISOString();
}

function uuid() {
  // RFC4122 v4-ish; not cryptographic, fine for IDs.
  const rnd = () => Math.floor(Math.random() * 0x10000).toString(16).padStart(4, '0');
  return `${rnd()}${rnd()}-${rnd()}-${rnd()}-${rnd()}-${rnd()}${rnd()}${rnd()}`;
}

function seedID(kind, idx) {
  return `${PREFIX}-${kind}-${idx}`;
}

// Pre-seeded ID pools. Read-only across VUs. Used for path-param read routes.
export const seedPlanIDs       = new SharedArray('seedPlanIDs',       () => buildSeed('plan'));
export const seedUserIDs       = new SharedArray('seedUserIDs',       () => buildSeed('user'));
export const seedUsernames     = new SharedArray('seedUsernames',     () => buildSeed('uname'));
export const seedEmails        = new SharedArray('seedEmails',        () => buildSeedEmails());
export const seedGroupIDs      = new SharedArray('seedGroupIDs',      () => buildSeed('group'));
export const seedRoleIDs       = new SharedArray('seedRoleIDs',       () => buildSeed('role'));
export const seedCategoryIDs   = new SharedArray('seedCategoryIDs',   () => buildSeed('cat'));
export const seedAppNames      = new SharedArray('seedAppNames',      () => buildSeed('app'));
export const seedSessionTokens = new SharedArray('seedSessionTokens', () => buildSeed('sess'));
export const seedSnapshotIDs   = new SharedArray('seedSnapshotIDs',   () => buildSeed('snap'));

function buildSeed(kind) {
  const out = new Array(SEED_POOL_SIZE);
  for (let i = 0; i < SEED_POOL_SIZE; i++) out[i] = seedID(kind, i);
  return out;
}

function buildSeedEmails() {
  const out = new Array(SEED_POOL_SIZE);
  for (let i = 0; i < SEED_POOL_SIZE; i++) out[i] = `${PREFIX}-${i}@k6.test`;
  return out;
}

function pick(arr) {
  const i = exec.scenario.iterationInTest % arr.length;
  return arr[i];
}

// ---- Path-param providers ----

export const pathProviders = {
  planID:       () => pick(seedPlanIDs),
  userID:       () => pick(seedUserIDs),
  username:     () => pick(seedUsernames),
  email:        () => pick(seedEmails),
  groupID:      () => pick(seedGroupIDs),
  roleID:       () => pick(seedRoleIDs),
  categoryID:   () => pick(seedCategoryIDs),
  appName:      () => pick(seedAppNames),
  sessionToken: () => pick(seedSessionTokens),
  snapshotID:   () => pick(seedSnapshotIDs),
  categoryScope: () => 'global',
  rollbackID:   () => 'rb-k6-0', // synthetic — R18 will 404 unless real seed supplied
};

// ---- Query providers ----

export const queryProviders = {
  // R27 — synthetic identity (will 404; counted as business 4xx)
  identity:     () => ({ provider: 'k6', issuer: 'k6', subject: `k6-${exec.vu.idInTest}` }),
  // R64, R65
  snapshotMeta: () => ({ scope: 'apps', namespace: 'default', generation: '1' }),
};

// ---- Body generators ----
// One per write route. Each returns a JSON-serializable object.

export const bodies = {
  // R14 globalconfig patch — partial spec map
  globalconfigPatch: () => ({
    userSettings: {
      theme: 'dark',
      density: 'compact',
      uiViewSize: 'md',
      fetchIntervalSeconds: 60,
    },
  }),

  // R15 app create — minimum Application
  appCreate: () => {
    const id = uuid();
    return {
      name: `${PREFIX}-app-${id}`,
      displayName: `${PREFIX} app`,
      namespaces: { primary: 'default' },
      managed: { by: 'k6' },
      createdAt: isoNow(),
      lastUpdated: isoNow(),
      resourceCount: 0,
      health: { phase: 'healthy' },
      resourceSummary: { total: 0 },
      ingressRules: [],
      snapshots: [],
      metrics: {},
      history: { generation: 0, changeLog: [] },
    };
  },

  // R20 app patch — partial map
  appPatch: () => ({ displayName: `${PREFIX} app updated` }),

  // R22 user create — UserAsResource
  userCreate: () => {
    const id = uuid();
    return {
      id: `${PREFIX}-user-${id}`,
      username: `${PREFIX}-${id}`,
      fullname: 'k6 user',
      email: `${id}@k6.test`,
      assignedRolesIDs: [],
      assignedGroupsIDs: [],
      status: { phase: 'active' },
      creationDate: isoNow(),
    };
  },

  // R28 user patch — partial map
  userPatch: () => ({ fullname: 'k6 user updated' }),

  // R30 group create — GroupAsResource
  groupCreate: () => {
    const id = uuid();
    return {
      id: `${PREFIX}-group-${id}`,
      name: `${PREFIX}-group-${id}`,
      description: '',
      categoryID: '',
      assignedUsersIDs: [],
      assignedRolesIDs: [],
      creationDate: isoNow(),
    };
  },

  // R33 group patch
  groupPatch: () => ({ description: 'k6 group updated' }),

  // R35 role create — RoleAsResource. RoleStatus.Phase capitalized "Active".
  roleCreate: () => {
    const id = uuid();
    return {
      id: `${PREFIX}-role-${id}`,
      name: `${PREFIX}-role-${id}`,
      description: '',
      version: '1.0',
      type: 'custom',
      priority: 1,
      categoryID: '',
      scopesAndPermissions: [],
      status: { phase: 'Active' },
      creationDate: isoNow(),
    };
  },

  // R42 role patch
  rolePatch: () => ({ description: 'k6 role updated' }),

  // R44 category create — Category
  categoryCreate: () => {
    const id = uuid();
    return {
      id: `${PREFIX}-cat-${id}`,
      name: `${PREFIX}-cat-${id}`,
      scope: 'global',
      type: 'custom',
      creationDate: isoNow(),
    };
  },

  // R48 category patch
  categoryPatch: () => ({ name: `${PREFIX}-cat-renamed` }),

  // R53 session create — UserSession. userId substituted from path.
  sessionCreate: (userId) => ({
    userId: userId,
    sessionToken: `sess-${uuid()}`,
    createdTimestamp: isoNow(),
    expiresTimestamp: isoOffset(60 * 60 * 1000),
    deviceMetadata: {},
  }),

  // R56 session patch
  sessionPatch: () => ({ expiresTimestamp: isoOffset(2 * 60 * 60 * 1000) }),

  // R63 snapshot create — CreateSnapshotPayload. Manifest is `any` — ConfigMap stub.
  snapshotCreate: () => {
    const id = uuid();
    return {
      id: `${PREFIX}-snap-${id}`,
      scope: 'apps',
      namespace: 'default',
      generation: 1,
      manifest: {
        apiVersion: 'v1',
        kind: 'ConfigMap',
        metadata: { name: 'k6-stub', namespace: 'default' },
        data: { key: 'value' },
      },
    };
  },
};

export const helpers = { uuid, isoNow, isoOffset, pick };
