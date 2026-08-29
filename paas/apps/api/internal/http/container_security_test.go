package http

import (
	"strings"
	"testing"
)

func TestSecurityProfileAdminPrivileged(t *testing.T) {
	resMap := map[string]any{
		"privileged": true,
		"memory":     "1g",
		"cpus":       "2.0",
	}

	// 1. Admin user with privileged=true
	adminProf := BuildSecurityProfile(resMap, true)
	if !adminProf.IsAdmin {
		t.Errorf("expected IsAdmin to be true")
	}
	if !adminProf.Privileged {
		t.Errorf("expected Privileged to be true for admin")
	}
	if !adminProf.AllowRoot {
		t.Errorf("expected AllowRoot to be true for admin")
	}

	adminArgs := strings.Join(ContainerSecurityArgs(adminProf), " ")
	if !strings.Contains(adminArgs, "--privileged") {
		t.Errorf("expected --privileged in adminArgs, got: %s", adminArgs)
	}
	if !strings.Contains(adminArgs, "io.paas.security=privileged") {
		t.Errorf("expected io.paas.security=privileged in adminArgs, got: %s", adminArgs)
	}
	if strings.Contains(adminArgs, "no-new-privileges") {
		t.Errorf("admin privileged container should not have no-new-privileges flag")
	}
	if strings.Contains(adminArgs, "--cap-drop") {
		t.Errorf("admin privileged container should not drop capabilities")
	}

	// 2. Non-admin user with privileged=true in config
	restrictedProf := BuildSecurityProfile(resMap, false)
	if restrictedProf.IsAdmin {
		t.Errorf("expected IsAdmin to be false for standard user")
	}
	if restrictedProf.Privileged {
		t.Errorf("expected Privileged to be false for standard user even if requested")
	}
	if restrictedProf.AllowRoot {
		t.Errorf("expected AllowRoot to be false for standard user")
	}

	restrictedArgs := strings.Join(ContainerSecurityArgs(restrictedProf), " ")
	if strings.Contains(restrictedArgs, "--privileged") {
		t.Errorf("standard user must NOT have --privileged flag")
	}
	if !strings.Contains(restrictedArgs, "no-new-privileges:true") {
		t.Errorf("expected no-new-privileges in restrictedArgs, got: %s", restrictedArgs)
	}
	if !strings.Contains(restrictedArgs, "--cap-drop ALL") {
		t.Errorf("expected --cap-drop ALL in restrictedArgs, got: %s", restrictedArgs)
	}
	if !strings.Contains(restrictedArgs, "io.paas.security=restricted") {
		t.Errorf("expected io.paas.security=restricted in restrictedArgs, got: %s", restrictedArgs)
	}
	if !strings.Contains(restrictedArgs, "--pids-limit") {
		t.Errorf("expected --pids-limit in restrictedArgs, got: %s", restrictedArgs)
	}
}

func TestScanDockerfileForDangersAdminVsUser(t *testing.T) {
	dangerousDF := `FROM ubuntu:latest
RUN curl -fsSL https://get.docker.com | sh
VOLUME /var/run/docker.sock:/var/run/docker.sock
CMD ["--privileged"]
`

	// 1. Non-admin user
	_, errorsUser := ScanDockerfileForDangers(dangerousDF, false)
	if len(errorsUser) == 0 {
		t.Errorf("expected blocking errors for standard user with dangerous Dockerfile")
	}
	hasRestricted := false
	for _, e := range errorsUser {
		if strings.Contains(e, "RESTRICTED") {
			hasRestricted = true
			break
		}
	}
	if !hasRestricted {
		t.Errorf("expected RESTRICTED error message for standard user, got: %v", errorsUser)
	}

	// 2. Admin user
	warningsAdmin, errorsAdmin := ScanDockerfileForDangers(dangerousDF, true)
	if len(errorsAdmin) > 0 {
		t.Errorf("expected 0 blocking errors for admin user, got: %v", errorsAdmin)
	}
	hasAdminOverride := false
	for _, w := range warningsAdmin {
		if strings.Contains(w, "ADMIN_OVERRIDE") {
			hasAdminOverride = true
			break
		}
	}
	if !hasAdminOverride {
		t.Errorf("expected ADMIN_OVERRIDE warning for admin user, got: %v", warningsAdmin)
	}
}
