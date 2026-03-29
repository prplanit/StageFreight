package docker

import "context"

// InventorySource resolves candidate hosts from an external source.
// Adapter boundary: no source-specific concepts (Ansible, etc.) leak past this.
type InventorySource interface {
	Name() string
	Resolve(ctx context.Context, selector TargetSelector) ([]HostTarget, error)
}

// SecretsProvider handles encryption/decryption for stack secret files.
// SOPS today, Vault/Infisical later.
type SecretsProvider interface {
	Name() string
	Decrypt(ctx context.Context, path string) ([]byte, error)
	Encrypt(ctx context.Context, path string, data []byte) error
	IsEncrypted(path string) bool
}

// HostTransport executes typed stack actions on a target host.
// Transport compiles the intent to whatever execution form it needs.
// It does NOT know compose lifecycle semantics — it executes steps.
type HostTransport interface {
	ExecuteAction(ctx context.Context, action StackAction) (ExecResult, error)
	Close() error
}
