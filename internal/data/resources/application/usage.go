package application

type Usage struct {
	QoS       string        `json:"qos"`
	Resources ResourceUsage `json:"resources"`
	Available bool          `json:"available"`
	Timestamp string        `json:"timestamp"`
}

type ResourceUsage struct {
	TotalCPU         string             `json:"totalCpu"`
	TotalMemory      string             `json:"totalMemory"`
	UsagePerInstance []UsagePerInstance `json:"usagePerInstance"`
}

type UsagePerInstance struct {
	Name        string           `json:"name"`
	Containers  []ContainerUsage `json:"containers"`
	TotalCPU    string           `json:"totalCpu"`
	TotalMemory string           `json:"totalMemory"`
}

type ContainerUsage struct {
	Name   string `json:"name"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}
