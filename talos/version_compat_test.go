package talos

import (
	"testing"

	"github.com/blang/semver/v4"
)

func TestSupportsLifecycleUpgrade(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
		wantErr bool
	}{
		{
			name:    "supports new API - 1.14.0",
			version: "1.14.0",
			want:    true,
			wantErr: false,
		},
		{
			name:    "supports new API - v1.14.0 with v prefix",
			version: "v1.14.0",
			want:    true,
			wantErr: false,
		},
		{
			name:    "supports new API - 1.15.3",
			version: "1.15.3",
			want:    true,
			wantErr: false,
		},
		{
			name:    "supports new API - 1.99.99 (high minor version)",
			version: "1.99.99",
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not support - 1.13.0",
			version: "1.13.0",
			want:    false,
			wantErr: false,
		},
		{
			name:    "does not support - 1.12.5",
			version: "1.12.5",
			want:    false,
			wantErr: false,
		},
		{
			name:    "does not support - 2.0.0 (upper bound exclusive)",
			version: "2.0.0",
			want:    false,
			wantErr: false,
		},
		{
			name:    "invalid version",
			version: "invalid",
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SupportsLifecycleUpgrade(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("SupportsLifecycleUpgrade() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SupportsLifecycleUpgrade() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLifecycleAPIRange(t *testing.T) {
	v114, err := semver.Parse("1.14.0")
	if err != nil {
		t.Fatal(err)
	}

	if !lifecycleAPIRange(v114) {
		t.Error("1.14.0 should be in range")
	}

	v113, err := semver.Parse("1.13.0")
	if err != nil {
		t.Fatal(err)
	}

	if lifecycleAPIRange(v113) {
		t.Error("1.13.0 should not be in range")
	}

	v200, err := semver.Parse("2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	if lifecycleAPIRange(v200) {
		t.Error("2.0.0 should not be in range (upper bound is exclusive)")
	}

	v199, err := semver.Parse("1.99.99")
	if err != nil {
		t.Fatal(err)
	}

	if !lifecycleAPIRange(v199) {
		t.Error("1.99.99 should be in range")
	}
}