// Single source of truth for route paths.
// Paths are derived from internal/routes/base.go + the rest module's endpoints/**/def.go.
// V1 prefix from internal/rest/base/def.go (V1="api/v1") combined in router/def.go:34.

const V1 = '/api/v1';

export const ROUTES = {
  R1:  { method: 'GET',    path: `${V1}/status/live`,                                                    name: 'status_live'           },
  R2:  { method: 'GET',    path: `${V1}/status/ready`,                                                   name: 'status_ready'          },
  R4:  { method: 'GET',    path: `${V1}/protectionplans`,                                                name: 'plans_list'            },
  R5:  { method: 'GET',    path: `${V1}/protectionplans/{id}`,                                           name: 'plans_get'             },
  R7:  { method: 'DELETE', path: `${V1}/protectionplans/{id}`,                                           name: 'plans_delete'          },
  R13: { method: 'GET',    path: `${V1}/config`,                                                         name: 'config_get'            },
  R14: { method: 'PATCH',  path: `${V1}/config`,                                                         name: 'config_patch'          },
  R15: { method: 'POST',   path: `${V1}/applications`,                                                   name: 'app_create'            },
  R16: { method: 'GET',    path: `${V1}/applications`,                                                   name: 'app_list'              },
  R17: { method: 'GET',    path: `${V1}/applications/{name}/rollbacks`,                                  name: 'app_rollbacks'         },
  R18: { method: 'GET',    path: `${V1}/applications/{name}/rollbacks/{rollbackId}`,                     name: 'app_rollback_by_id'    },
  R19: { method: 'GET',    path: `${V1}/applications/{name}`,                                            name: 'app_get'               },
  R20: { method: 'PATCH',  path: `${V1}/applications/{name}`,                                            name: 'app_patch'             },
  R21: { method: 'DELETE', path: `${V1}/applications/{name}`,                                            name: 'app_delete'            },
  R22: { method: 'POST',   path: `${V1}/users`,                                                          name: 'user_create'           },
  R23: { method: 'GET',    path: `${V1}/users`,                                                          name: 'user_list'             },
  R24: { method: 'GET',    path: `${V1}/internal/users/by-username/{username}`,                          name: 'user_by_username'      },
  R25: { method: 'GET',    path: `${V1}/internal/users/by-email/{email}`,                                name: 'user_by_email'         },
  R26: { method: 'GET',    path: `${V1}/users/{id}`,                                                     name: 'user_by_id'            },
  R27: { method: 'GET',    path: `${V1}/internal/users/by-identity`,                                     name: 'user_by_identity'      },
  R28: { method: 'PATCH',  path: `${V1}/users/{id}`,                                                     name: 'user_patch'            },
  R29: { method: 'DELETE', path: `${V1}/users/{id}`,                                                     name: 'user_delete'           },
  R30: { method: 'POST',   path: `${V1}/groups`,                                                         name: 'group_create'          },
  R31: { method: 'GET',    path: `${V1}/groups`,                                                         name: 'group_list'            },
  R32: { method: 'GET',    path: `${V1}/groups/{id}`,                                                    name: 'group_get'             },
  R33: { method: 'PATCH',  path: `${V1}/groups/{id}`,                                                    name: 'group_patch'           },
  R34: { method: 'DELETE', path: `${V1}/groups/{id}`,                                                    name: 'group_delete'          },
  R35: { method: 'POST',   path: `${V1}/accessroles`,                                                    name: 'role_create'           },
  R36: { method: 'GET',    path: `${V1}/accessroles`,                                                    name: 'role_list'             },
  R37: { method: 'GET',    path: `${V1}/accessroles/{id}`,                                               name: 'role_get'              },
  R42: { method: 'PATCH',  path: `${V1}/accessroles/{id}`,                                               name: 'role_patch'            },
  R43: { method: 'DELETE', path: `${V1}/accessroles/{id}`,                                               name: 'role_delete'           },
  R44: { method: 'POST',   path: `${V1}/categories`,                                                     name: 'category_create'       },
  R45: { method: 'GET',    path: `${V1}/categories`,                                                     name: 'category_list'         },
  R46: { method: 'GET',    path: `${V1}/categories/{id}`,                                                name: 'category_get'          },
  R47: { method: 'GET',    path: `${V1}/categories?scope={scope}`,                                       name: 'category_by_scope'     },
  R48: { method: 'PATCH',  path: `${V1}/categories/{id}`,                                                name: 'category_patch'        },
  R49: { method: 'DELETE', path: `${V1}/categories/{id}`,                                                name: 'category_delete'       },
  R53: { method: 'POST',   path: `${V1}/internal/auth/users/{userId}/sessions`,                          name: 'session_create'        },
  R54: { method: 'GET',    path: `${V1}/auth/sessions?user={userId}`,                                    name: 'session_list'          },
  R55: { method: 'GET',    path: `${V1}/auth/sessions/self`,                                             name: 'session_self'          },
  R56: { method: 'PATCH',  path: `${V1}/auth/sessions/self`,                                             name: 'session_patch'         },
  R57: { method: 'DELETE', path: `${V1}/auth/sessions/self`,                                             name: 'session_delete'        },
  R63: { method: 'POST',   path: `${V1}/internal/snapshots`,                                             name: 'snapshot_create'       },
  R64: { method: 'GET',    path: `${V1}/snapshots/{id}`,                                                 name: 'snapshot_get'          },
  R65: { method: 'GET',    path: `${V1}/snapshots/{id}/manifest`,                                        name: 'snapshot_manifest'     },
  R66: { method: 'GET',    path: `${V1}/snapshots`,                                                      name: 'snapshot_infos'        },
};

export function buildPath(route, params) {
  let p = route.path;
  if (params) {
    for (const key of Object.keys(params)) {
      p = p.replace(`{${key}}`, encodeURIComponent(params[key]));
    }
  }
  return p;
}

export function allRouteIDs() {
  return Object.keys(ROUTES);
}
