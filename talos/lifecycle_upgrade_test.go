package talos

import "testing"

func TestParseContainerdInstance(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{"system namespace", "system", "system"},
		{"cri namespace", "cri", "cri"},
		{"inmem namespace", "inmem", "inmem"},
		{"unknown namespace defaults to system", "unknown", "system"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseContainerdInstance(tt.namespace)
			if got != tt.want {
				t.Errorf("ParseContainerdInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpgradeViaLifecycleService(t *testing.T) {
	t.Skip("Requires Talos client - implement with mock")
}

func TestUpgradeInternal(t *testing.T) {
	t.Skip("Requires Talos client and gRPC streaming - implement with mock")
}

func TestImagePullInternal(t *testing.T) {
	t.Skip("Requires Talos client - implement with mock")
}
