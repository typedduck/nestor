package executor_test

import (
	"runtime"
	"testing"

	"github.com/typedduck/nestor/agent/executor"
)

func TestDetectSystem_BasicFields(t *testing.T) {
	fr := executor.NewMockFileSystem()
	cl := executor.NewMockCommandRunner()
	info := executor.DetectSystem(fr, cl)

	if info.OS != runtime.GOOS {
		t.Fatalf("expected OS=%s, got %s", runtime.GOOS, info.OS)
	}
	if info.Architecture != runtime.GOARCH {
		t.Fatalf("expected Arch=%s, got %s", runtime.GOARCH, info.Architecture)
	}
}

func TestDetectSystem_NilDefaults(t *testing.T) {
	// Passing nil should not panic; it falls back to real OS implementations
	info := executor.DetectSystem(nil, nil)
	if info.OS != runtime.GOOS {
		t.Fatalf("expected OS=%s, got %s", runtime.GOOS, info.OS)
	}
}

func TestDetectDistribution_Ubuntu(t *testing.T) {
	fr := executor.NewMockFileSystem()
	fr.AddFile("/etc/os-release",
		[]byte("NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"), 0755)

	distro := executor.DetectDistribution(fr)
	if distro != "ubuntu" {
		t.Fatalf("expected ubuntu, got %s", distro)
	}
}

func TestDetectDistribution_QuotedID(t *testing.T) {
	fr := executor.NewMockFileSystem()
	fr.AddFile("/etc/os-release", []byte("ID=\"centos\"\n"), 0755)

	distro := executor.DetectDistribution(fr)
	if distro != "centos" {
		t.Fatalf("expected centos, got %s", distro)
	}
}

func TestDetectDistribution_FallbackDebian(t *testing.T) {
	fr := executor.NewMockFileSystem()
	// No os-release, but debian_version exists
	fr.AddFile("/etc/debian_version", []byte("11.0"), 0755)

	distro := executor.DetectDistribution(fr)
	if distro != "debian" {
		t.Fatalf("expected debian, got %s", distro)
	}
}

func TestDetectDistribution_FallbackRedhat(t *testing.T) {
	fr := executor.NewMockFileSystem()
	fr.AddFile("/etc/redhat-release", []byte("CentOS Linux release 7.9"), 0755)

	distro := executor.DetectDistribution(fr)
	if distro != "redhat" {
		t.Fatalf("expected redhat, got %s", distro)
	}
}

func TestDetectDistribution_Unknown(t *testing.T) {
	fr := executor.NewMockFileSystem()
	distro := executor.DetectDistribution(fr)
	if distro != "unknown" {
		t.Fatalf("expected unknown, got %s", distro)
	}
}

func TestDetectPackageManager_Apt(t *testing.T) {
	cl := executor.NewMockCommandRunner()
	cl.SetLookPath("apt-get", "/usr/bin/apt-get")

	pm := executor.DetectPackageManager(cl)
	if pm != "apt" {
		t.Fatalf("expected apt, got %s", pm)
	}
}

func TestDetectPackageManager_Dnf(t *testing.T) {
	cl := executor.NewMockCommandRunner()
	cl.SetLookPath("dnf", "/usr/bin/dnf")

	pm := executor.DetectPackageManager(cl)
	if pm != "dnf" {
		t.Fatalf("expected dnf, got %s", pm)
	}
}

func TestDetectPackageManager_Priority(t *testing.T) {
	cl := executor.NewMockCommandRunner()
	// Both yum and dnf available; apt-get takes priority
	cl.SetLookPath("apt-get", "/usr/bin/apt-get")
	cl.SetLookPath("yum", "/usr/bin/yum")
	cl.SetLookPath("dnf", "/usr/bin/dnf")

	pm := executor.DetectPackageManager(cl)
	if pm != "apt" {
		t.Fatalf("expected apt (highest priority), got %s", pm)
	}
}

func TestDetectPackageManager_Unknown(t *testing.T) {
	cl := executor.NewMockCommandRunner()
	pm := executor.DetectPackageManager(cl)
	if pm != "unknown" {
		t.Fatalf("expected unknown, got %s", pm)
	}
}

func TestDetectInitSystem_Systemd(t *testing.T) {
	fr := executor.NewMockFileSystem()
	cl := executor.NewMockCommandRunner()
	cl.SetLookPath("systemctl", "/usr/bin/systemctl")

	init := executor.DetectInitSystem(fr, cl)
	if init != "systemd" {
		t.Fatalf("expected systemd, got %s", init)
	}
}

func TestDetectInitSystem_SysVInit(t *testing.T) {
	fr := executor.NewMockFileSystem()
	fr.AddFile("/etc/init.d", []byte{}, 0755)
	cl := executor.NewMockCommandRunner()

	init := executor.DetectInitSystem(fr, cl)
	if init != "sysvinit" {
		t.Fatalf("expected sysvinit, got %s", init)
	}
}

func TestDetectInitSystem_OpenRC(t *testing.T) {
	fr := executor.NewMockFileSystem()
	cl := executor.NewMockCommandRunner()
	cl.SetLookPath("rc-service", "/sbin/rc-service")

	init := executor.DetectInitSystem(fr, cl)
	if init != "openrc" {
		t.Fatalf("expected openrc, got %s", init)
	}
}

func TestDetectInitSystem_Unknown(t *testing.T) {
	fr := executor.NewMockFileSystem()
	cl := executor.NewMockCommandRunner()
	init := executor.DetectInitSystem(fr, cl)
	if init != "unknown" {
		t.Fatalf("expected unknown, got %s", init)
	}
}

func TestGetFileInfo_Exists(t *testing.T) {
	fr := executor.NewMockFileSystem()
	fr.AddFile("/etc/test.conf", []byte("hello"), 0755)

	info, err := executor.GetFileInfo(fr, "/etc/test.conf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Exists {
		t.Fatal("expected file to exist")
	}
	if info.Size != 5 {
		t.Fatalf("expected size=5, got %d", info.Size)
	}
}

func TestGetFileInfo_NotExists(t *testing.T) {
	fr := executor.NewMockFileSystem()
	info, err := executor.GetFileInfo(fr, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Exists {
		t.Fatal("expected file to not exist")
	}
}
