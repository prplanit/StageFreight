package gitops

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// FluxReconcileResult reports the outcome of reconciling one kustomization.
type FluxReconcileResult struct {
	Kustomization string
	Namespace     string
	Attempted     bool
	Success       bool
	Duration      time.Duration
	Ready         bool
	Message       string
}

// Reconcile executes flux reconcile on the given kustomizations in order.
// If dryRun is true, no commands are executed.
func Reconcile(keys []KustomizationKey, dryRun bool) []FluxReconcileResult {
	var results []FluxReconcileResult

	for _, k := range keys {
		res := FluxReconcileResult{
			Kustomization: k.Name,
			Namespace:     k.Namespace,
			Attempted:     true,
		}

		if dryRun {
			res.Success = true
			res.Message = "dry-run"
			results = append(results, res)
			continue
		}

		start := time.Now()

		// Reconcile source first
		srcCmd := exec.Command("flux", "reconcile", "source", "git", "flux-system", "-n", k.Namespace)
		if out, err := srcCmd.CombinedOutput(); err != nil {
			res.Duration = time.Since(start)
			res.Success = false
			res.Message = fmt.Sprintf("source reconcile failed: %s", strings.TrimSpace(string(out)))
			results = append(results, res)
			continue
		}

		// Reconcile kustomization
		cmd := exec.Command("flux", "reconcile", "kustomization", k.Name, "-n", k.Namespace)
		out, err := cmd.CombinedOutput()
		res.Duration = time.Since(start)

		if err != nil {
			res.Success = false
			res.Message = strings.TrimSpace(string(out))
			results = append(results, res)
			continue
		}

		res.Success = true
		res.Ready = true
		res.Message = strings.TrimSpace(string(out))
		results = append(results, res)
	}

	return results
}
