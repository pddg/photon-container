package e2e_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setup(t *testing.T, ns string) (string, string) {
	t.Helper()
	kubectl(t, "create", "ns", ns)
	t.Cleanup(func() {
		kubectl(t, "delete", "ns", ns)
	})

	manifests := getOutput(t, "kustomize", "build", "testdata/manifests")
	apply := kubectlCmd(t, "apply", "-n", ns, "-f", "-")
	apply.Stdin = strings.NewReader(manifests)
	runCommand(t, apply)

	// Statefulset does not have any conditions, so we need to wait for the pod to be created
	require.Eventually(t, func() bool {
		cmd := kubectlCmd(t, "get", "pod", "-n", ns, "-l", "app.kubernetes.io/name=photon")
		if err := cmd.Run(); err != nil {
			t.Logf("Error getting pod status: %v", err)
			return false
		}
		t.Logf("Pod created")
		return true
	}, 1*time.Minute, 1*time.Second)

	// Wait for the photon pod to be ready
	kubectl(t, "wait", "--for=condition=Ready", "pod", "-n", ns, "-l", "app.kubernetes.io/name=photon")

	photonAgentUrl := fmt.Sprintf("http://api.%s.svc.cluster.local:8080/", ns)
	photonUrl := fmt.Sprintf("http://api.%s.svc.cluster.local:80/", ns)
	return photonUrl, photonAgentUrl
}
