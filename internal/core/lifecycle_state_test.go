package core

import "testing"

func TestResolveStatePriority(t *testing.T) {
	failure := &LastFailureContext{Message: "boom"}
	cases := []struct {
		name      string
		installed bool
		running   bool
		failure   *LastFailureContext
		want      CoreState
	}{
		{
			name:      "not_installed_has_highest_priority",
			installed: false,
			running:   true,
			failure:   failure,
			want:      CoreStateNotInstalled,
		},
		{
			name:      "running_overrides_error",
			installed: true,
			running:   true,
			failure:   failure,
			want:      CoreStateRunning,
		},
		{
			name:      "error_when_not_running_with_failure",
			installed: true,
			running:   false,
			failure:   failure,
			want:      CoreStateError,
		},
		{
			name:      "stopped_when_installed_without_failure",
			installed: true,
			running:   false,
			failure:   nil,
			want:      CoreStateStopped,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := resolveState(tc.installed, tc.running, tc.failure)
			if got != tc.want {
				t.Fatalf("resolveState() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestActionMatrix(t *testing.T) {
	cases := []struct {
		state CoreState
		want  []string
	}{
		{state: CoreStateRunning, want: []string{"stop", "restart"}},
		{state: CoreStateStopped, want: []string{"start"}},
		{state: CoreStateNotInstalled, want: []string{"install"}},
		{state: CoreStateError, want: []string{"retry_start", "view_logs"}},
	}

	for _, tc := range cases {
		got := ActionMatrix(tc.state)
		if len(got) != len(tc.want) {
			t.Fatalf("ActionMatrix(%q) len = %d, want %d", tc.state, len(got), len(tc.want))
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ActionMatrix(%q)[%d] = %q, want %q", tc.state, i, got[i], tc.want[i])
			}
		}
	}
}

func TestLastFailureContextSnapshotIsCopied(t *testing.T) {
	clearLastFailure()
	t.Cleanup(clearLastFailure)

	ctx := newFailureContext("failed to start", "start", "unit-test")
	setLastFailure(ctx)

	got := getLastFailure()
	if got == nil {
		t.Fatal("getLastFailure() = nil, want non-nil")
	}
	if got.Message != "failed to start" {
		t.Fatalf("getLastFailure().Message = %q, want %q", got.Message, "failed to start")
	}

	got.Message = "mutated"
	again := getLastFailure()
	if again == nil {
		t.Fatal("getLastFailure() second call = nil, want non-nil")
	}
	if again.Message != "failed to start" {
		t.Fatalf("internal failure context should be immutable snapshot, got %q", again.Message)
	}
}
