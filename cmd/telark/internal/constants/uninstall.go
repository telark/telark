package constants

import "time"

const (
	UninstallUse   = "uninstall"
	UninstallShort = "Uninstall Telark with Helm and clear its cleanup finalizers"
	UninstallLong  = "telark uninstall removes the Helm release telark, then the cleanup finalizers that only auth clears " +
		"from users, groups and access roles, so a later delete of the CRDs or the namespace doesn't hang. " +
		"The CRDs, the custom resources, the Secrets and the volumes are kept."
	FlagYes               = "yes"
	FlagYesShort          = "y"
	FlagYesUsage          = "Don't ask for confirmation"
	FlagUninstallDryRun   = "Show what would be removed without changing anything"
	FlagUninstallWait     = "Wait until every object of the release is deleted"
	FlagTimeout           = "timeout"
	FlagTimeoutUsage      = "How long to wait for Helm and for auth's pods to stop"
	AuthComponent         = "auth-service"
	FinalizerPollInterval = time.Second
	AnswerYes             = "yes"
	AnswerY               = "y"
	LineEnd               = '\n'
	KeyMetadata           = "metadata"
	KeyFinalizers         = "finalizers"
)

const (
	MsgNotInstalled       = "Telark isn't installed in namespace %q: no release and no cleanup finalizer.\n"
	MsgUninstallRelease   = "The release telark (%s) in namespace %q will be uninstalled.\n"
	MsgUninstallNoRelease = "No release telark in namespace %q: only the cleanup finalizers are removed.\n"
	MsgUninstallHeld      = "%d users, groups and access roles hold a cleanup finalizer, which will be removed.\n"
	MsgUninstallKeeps     = "The CRDs, the custom resources, the Secrets and the volumes are kept.\n"
	MsgDryRunUninstall    = "Dry run: nothing changed.\n"
	MsgUninstallConfirm   = "Proceed? [y/N]: "
	MsgUninstalled        = "Uninstalled the release telark.\n"
	MsgFinalizersRemoved  = "Cleanup finalizers: %d removed, %d failed.\n"
	MsgUninstallNext      = "To delete Telark's data too, see \"Full teardown\" in docs/INSTALL.md.\n"
	WarnAuthStillRuns     = "auth's pods didn't stop in time (%v): auth can restore the finalizers, so run telark uninstall again " +
		"if a delete hangs\n"
	WarnFinalizerList   = "Couldn't list %s: %v\n"
	WarnFinalizerRemove = "Couldn't remove the cleanup finalizer of %s/%s: %v\n"
	ErrUninstall        = "uninstall: %w"
	ErrUninstallNotSure = "uninstall canceled: answer y, or pass --yes"
	ErrFinalizersLeft   = "%d cleanup finalizers are left: clear them with the kubectl command under \"Uninstall\" in docs/INSTALL.md"
)
