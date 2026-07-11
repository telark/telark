// Single source of truth for route paths.
// Paths are derived from exporter-service/routes/base.go + internal/rest/endpoints/**/def.go.
// V1 prefix from internal/rest/base/def.go (V1="api/v1") combined in router/def.go:34.

const V1 = '/api/v1';

export const ROUTES = {
  R1:  { method: 'GET',    path: `${V1}/status/live`,                                                    name: 'status_live'           },
  R2:  { method: 'GET',    path: `${V1}/status/ready`,                                                   name: 'status_ready'          },
  R4:  { method: 'GET',    path: `${V1}/plans/protection/get`,                                           name: 'plans_list'            },
  R5:  { method: 'GET',    path: `${V1}/plans/protection/{id}/get`,                                      name: 'plans_get'             },
  R7:  { method: 'DELETE', path: `${V1}/plans/protection/{id}/delete`,                                   name: 'plans_delete'          },
  R13: { method: 'GET',    path: `${V1}/resources/globalconfig/get`,                                     name: 'globalconfig_get'      },
  R14: { method: 'PATCH',  path: `${V1}/resources/globalconfig/patch`,                                   name: 'globalconfig_patch'    },
  R15: { method: 'POST',   path: `${V1}/resources/applications/create`,                                  name: 'app_create'            },
  R16: { method: 'GET',    path: `${V1}/resources/applications/get`,                                     name: 'app_list'              },
  R17: { method: 'GET',    path: `${V1}/resources/applications/{name}/rollbacks/get`,                    name: 'app_rollbacks'         },
  R18: { method: 'GET',    path: `${V1}/resources/applications/{name}/rollbacks/{rollbackId}/get`,       name: 'app_rollback_by_id'    },
  R19: { method: 'GET',    path: `${V1}/resources/applications/{name}/get`,                              name: 'app_get'               },
  R20: { method: 'PATCH',  path: `${V1}/resources/applications/{name}/patch`,                            name: 'app_patch'             },
  R21: { method: 'DELETE', path: `${V1}/resources/applications/{name}/delete`,                           name: 'app_delete'            },
  R22: { method: 'POST',   path: `${V1}/resources/users/create`,                                         name: 'user_create'           },
  R23: { method: 'GET',    path: `${V1}/resources/users/get`,                                            name: 'user_list'             },
  R24: { method: 'GET',    path: `${V1}/resources/users/findbyusername/{username}/get`,                  name: 'user_by_username'      },
  R25: { method: 'GET',    path: `${V1}/resources/users/findbyemail/{email}/get`,                        name: 'user_by_email'         },
  R26: { method: 'GET',    path: `${V1}/resources/users/findbyid/{id}/get`,                              name: 'user_by_id'            },
  R27: { method: 'GET',    path: `${V1}/resources/users/findbyidentity/get`,                             name: 'user_by_identity'      },
  R28: { method: 'PATCH',  path: `${V1}/resources/users/{id}/patch`,                                     name: 'user_patch'            },
  R29: { method: 'DELETE', path: `${V1}/resources/users/{id}/delete`,                                    name: 'user_delete'           },
  R30: { method: 'POST',   path: `${V1}/resources/groups/create`,                                        name: 'group_create'          },
  R31: { method: 'GET',    path: `${V1}/resources/groups/get`,                                           name: 'group_list'            },
  R32: { method: 'GET',    path: `${V1}/resources/groups/{id}/get`,                                      name: 'group_get'             },
  R33: { method: 'PATCH',  path: `${V1}/resources/groups/{id}/patch`,                                    name: 'group_patch'           },
  R34: { method: 'DELETE', path: `${V1}/resources/groups/{id}/delete`,                                   name: 'group_delete'          },
  R35: { method: 'POST',   path: `${V1}/resources/roles/create`,                                         name: 'role_create'           },
  R36: { method: 'GET',    path: `${V1}/resources/roles/get`,                                            name: 'role_list'             },
  R37: { method: 'GET',    path: `${V1}/resources/roles/{id}/get`,                                       name: 'role_get'              },
  R38: { method: 'GET',    path: `${V1}/resources/roles/findbyuserid/{userId}/get`,                      name: 'role_by_userid'        },
  R39: { method: 'GET',    path: `${V1}/resources/roles/findbyuserid/{userId}/list`,                     name: 'roles_by_userid'       },
  R40: { method: 'GET',    path: `${V1}/resources/roles/findbygroupid/{groupId}/get`,                    name: 'role_by_groupid'       },
  R41: { method: 'GET',    path: `${V1}/resources/roles/findbygroupid/{groupId}/list`,                   name: 'roles_by_groupid'      },
  R42: { method: 'PATCH',  path: `${V1}/resources/roles/{id}/patch`,                                     name: 'role_patch'            },
  R43: { method: 'DELETE', path: `${V1}/resources/roles/{id}/delete`,                                    name: 'role_delete'           },
  R44: { method: 'POST',   path: `${V1}/classification/categories/create`,                               name: 'category_create'       },
  R45: { method: 'GET',    path: `${V1}/classification/categories/get`,                                  name: 'category_list'         },
  R46: { method: 'GET',    path: `${V1}/classification/categories/{id}/get`,                             name: 'category_get'          },
  R47: { method: 'GET',    path: `${V1}/classification/categories/scope/{scope}/get`,                    name: 'category_by_scope'     },
  R48: { method: 'PATCH',  path: `${V1}/classification/categories/{id}/patch`,                           name: 'category_patch'        },
  R49: { method: 'DELETE', path: `${V1}/classification/categories/{id}/delete`,                          name: 'category_delete'       },
  R53: { method: 'POST',   path: `${V1}/auth/sessions/{userId}/create`,                                  name: 'session_create'        },
  R54: { method: 'GET',    path: `${V1}/auth/sessions/{userId}/get`,                                     name: 'session_list'          },
  R55: { method: 'GET',    path: `${V1}/auth/sessions/tokens/{token}/get`,                               name: 'session_by_token'      },
  R56: { method: 'PATCH',  path: `${V1}/auth/sessions/tokens/{token}/patch`,                             name: 'session_patch'         },
  R57: { method: 'DELETE', path: `${V1}/auth/sessions/tokens/{token}/delete`,                            name: 'session_delete'        },
  R63: { method: 'POST',   path: `${V1}/snapshots/create`,                                               name: 'snapshot_create'       },
  R64: { method: 'GET',    path: `${V1}/snapshots/{id}/get`,                                             name: 'snapshot_get'          },
  R65: { method: 'GET',    path: `${V1}/snapshots/{id}/manifest`,                                        name: 'snapshot_manifest'     },
  R66: { method: 'GET',    path: `${V1}/snapshots/infos`,                                                name: 'snapshot_infos'        },
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
