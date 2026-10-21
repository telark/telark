package role

const (
	ActionViewApplicationsRollbacks       = "viewapplicationsrollbacks"
	ActionViewApplicationsSnapshots       = "viewapplicationssnapshots"
	ActionViewApplicationSnapshotManifest = "viewapplicationsnapshotmanifest"
	ActionEditApplication                 = "editapplication"
	ActionForceApplicationSync            = "forceapplicationsync"
	ActionRollbackApplication             = "rollbackapplication"
	ActionDeleteApplication               = "deleteapplication"
)

const (
	ActionViewGroupsCategories   = "viewgroupscategories"
	ActionViewGroupAttachedRoles = "viewgroupattachedroles"
	ActionCreateGroup            = "creategroup"
	ActionEditGroup              = "editgroup"
	ActionAddGroupCategory       = "addgroupcategory"
	ActionAddUserToGroup         = "addusertogroup"
	ActionRemoveUserFromGroup    = "removeuserfromgroup"
	ActionDeleteGroup            = "deletegroup"
	ActionAttachRoleToGroup      = "attachroletogroup"
	ActionRemoveRoleFromGroup    = "removerolefromgroup"
	ActionEditGroupCategory      = "editgroupcategory"
	ActionDeleteGroupCategory    = "deletegroupcategory"
)

const (
	ActionViewUserAttachedRoles = "viewuserattachedroles"
	ActionCreateUser            = "createuser"
	ActionDeleteUser            = "deleteuser"
	ActionAttachRoleToUser      = "attachroletouser"
	ActionRemoveRoleFromUser    = "removerolefromuser"
	ActionSuspendUser           = "suspenduser"
)

const (
	ActionViewRolesCategories = "viewrolescategories"
	ActionCreateRole          = "createrole"
	ActionEditRole            = "editrole"
	ActionAddRoleCategory     = "addrolecategory"
	ActionDeleteRole          = "deleterole"
	ActionEditRoleCategory    = "editrolecategory"
	ActionDeleteRoleCategory  = "deleterolecategory"
)

const (
	ActionEditDiscoveryConfig = "editdiscoveryconfig"
	ActionEditSnapshotStorage = "editsnapshotstorage"
	ActionControlAIInsights   = "controlainsights"
	ActionEditOIDCConfig      = "editoidcconfig"
)

const (
	ActionAnalyzeInsights = "analyzeinsights"
	ActionTriageInsights  = "triageinsights"
)

const (
	ActionViewProtectionPlanViolations = "viewprotectionplanviolations"
	ActionCreateProtectionPlan         = "createprotectionplan"
	ActionEditProtectionPlan           = "editprotectionplan"
	ActionCancelProtectionPlan         = "cancelprotectionplan"
	ActionDuplicateProtectionPlan      = "duplicateprotectionplan"
	ActionReactivateProtectionPlan     = "reactivateprotectionplan"
	ActionDeleteProtectionPlan         = "deleteprotectionplan"
	ActionApproveProtectionPlan        = "approveprotectionplan"
	ActionViewProtectionPlans          = "viewprotectionplans"
	ActionRejectProtectionPlan         = "rejectprotectionplan"
	ActionGenerateProtectionPlanReport = "generateprotectionplanreport"
	ActionViewProtectionPlanReports    = "viewprotectionplanreports"
	ActionDownloadProtectionPlanReport = "downloadprotectionplanreport"
	ActionAddProtectionPlanCategory    = "addprotectionplancategory"
	ActionEditProtectionPlanCategory   = "editprotectionplancategory"
	ActionDeleteProtectionPlanCategory = "deleteprotectionplancategory"
)

// The actions a role can deny in each scope: a stored rule is the authz RuleKey of the scope and one of them.
var DenyableActions = map[string][]string{
	ScopeApplications: {
		ActionViewApplicationsRollbacks, ActionViewApplicationsSnapshots, ActionViewApplicationSnapshotManifest,
		ActionEditApplication, ActionForceApplicationSync, ActionRollbackApplication, ActionDeleteApplication,
	},
	ScopeGroups: {
		ActionViewGroupsCategories, ActionViewGroupAttachedRoles, ActionCreateGroup, ActionEditGroup,
		ActionAddGroupCategory, ActionAddUserToGroup, ActionRemoveUserFromGroup, ActionDeleteGroup,
		ActionAttachRoleToGroup, ActionRemoveRoleFromGroup, ActionEditGroupCategory, ActionDeleteGroupCategory,
	},
	ScopeUsers: {
		ActionViewUserAttachedRoles, ActionCreateUser, ActionDeleteUser,
		ActionAttachRoleToUser, ActionRemoveRoleFromUser, ActionSuspendUser,
	},
	ScopeRoles: {
		ActionViewRolesCategories, ActionCreateRole, ActionEditRole, ActionAddRoleCategory,
		ActionDeleteRole, ActionEditRoleCategory, ActionDeleteRoleCategory,
	},
	ScopeSettings: {ActionEditDiscoveryConfig, ActionEditSnapshotStorage, ActionControlAIInsights, ActionEditOIDCConfig},
	ScopeInsights: {ActionAnalyzeInsights, ActionTriageInsights},
	ScopeProtectionPlans: {
		ActionViewProtectionPlanViolations, ActionCreateProtectionPlan, ActionEditProtectionPlan,
		ActionCancelProtectionPlan, ActionDuplicateProtectionPlan, ActionReactivateProtectionPlan,
		ActionDeleteProtectionPlan, ActionApproveProtectionPlan, ActionViewProtectionPlans,
		ActionRejectProtectionPlan, ActionGenerateProtectionPlanReport, ActionViewProtectionPlanReports,
		ActionDownloadProtectionPlanReport, ActionAddProtectionPlanCategory, ActionEditProtectionPlanCategory,
		ActionDeleteProtectionPlanCategory,
	},
}
