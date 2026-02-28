package executor

import (
	"fmt"
	"log"

	agentexec "github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/handlers"
	"github.com/typedduck/nestor/playbook"
)

// Local executes a playbook from dir on the local machine.
// dir must contain playbook.yaml; upload files are resolved relative to dir.
// Bypasses packaging, signing, and SSH — no root check is enforced,
// but operations requiring elevated privileges will fail naturally.
func Local(pb *playbook.Playbook, dir string, dryRun bool) error {
	epb := &agentexec.Playbook{
		Playbook:    *pb,
		ExtractPath: dir,
	}

	sysInfo := agentexec.DetectSystem(nil, nil)
	log.Printf("[INFO ] local system: OS=%s, PackageManager=%s, InitSystem=%s",
		sysInfo.OS, sysInfo.PackageManager, sysInfo.InitSystem)

	engine := agentexec.New(epb, sysInfo, "", agentexec.OSFileSystem{}, agentexec.OSCommandRunner{})
	engine.SetDryRun(dryRun)
	handlers.RegisterLocal(engine)

	result, err := engine.Execute()
	if err != nil {
		return fmt.Errorf("execution error: %w", err)
	}

	log.Printf("[INFO ] status: %s — total: %d, success: %d, failed: %d, changed: %d",
		result.Status,
		result.Summary.Total, result.Summary.Success,
		result.Summary.Failed, result.Summary.Changed)

	if result.Status == "failed" {
		return fmt.Errorf("playbook %q failed (%d/%d actions failed)",
			pb.Name, result.Summary.Failed, result.Summary.Total)
	}
	return nil
}
