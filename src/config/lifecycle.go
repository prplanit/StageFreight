package config

// LifecycleConfig defines the repository lifecycle mode.
type LifecycleConfig struct {
	Mode string `yaml:"mode"` // image | gitops | governance
}

// GovernanceConfig declares governance clusters for the control repo.
// Only valid when lifecycle.mode is "governance".
// Assets (CI skeletons, settings files, etc.) are declared inside each
// cluster's stagefreight config as assets: entries — no separate skeleton construct.
type GovernanceConfig struct {
	Clusters []GovernanceCluster `yaml:"clusters"`
}

// GovernanceCluster assigns lifecycle doctrine to a group of repos.
type GovernanceCluster struct {
	ID           string                   `yaml:"id"`
	Targets      GovernanceClusterTargets `yaml:"targets"`
	StageFreight map[string]any           `yaml:"stagefreight"`
}

// GovernanceClusterTargets identifies which repos belong to this cluster.
type GovernanceClusterTargets struct {
	Repos []string `yaml:"repos"`
}
