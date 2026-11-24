package constants

const (
	RootShort = "Check and manage a Telark installation"
	RootLong  = "telark works with a Telark installation through the Kubernetes API server, as the user in your kubeconfig, " +
		"so the cluster's RBAC and audit log cover every action."
	VersionUse   = "version"
	VersionShort = "Show the CLI and server versions"
)

const (
	ErrorFormat        = "Error: %v\n"
	LineFormat         = "%s %s\n"
	ErrUnknownFormat   = "must be table, json or yaml"
	ErrNoCluster       = "no cluster selected: pass --kubeconfig or --context, or set KUBECONFIG"
	ErrListDeployments = "list the Telark deployments in namespace %q: %w"
	ErrNoDeployments   = "no Telark deployments in namespace %q: pass the namespace Telark runs in with -n"
	WarnVersionSkew    = "The CLI (%s) and the server (%s) are from different releases: use the CLI that matches the server"
)

// Both row formats put the value in column 15, so the client and server parts line up.
const (
	TableClient     = "Client:"
	TableServer     = "Server:"
	TableServices   = "  Services:"
	TableRowFormat  = "  %-12s %s\n"
	TableServiceRow = "    %-10s %s\n"
	TableVersion    = "Version:"
	TableGitCommit  = "Git commit:"
	TableBuildDate  = "Build date:"
	TableGoVersion  = "Go version:"
	TableNamespace  = "Namespace:"
)
