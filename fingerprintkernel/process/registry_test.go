package process

import "testing"

func TestRegistryKeepsReplacementOnLateExit(t *testing.T) {
	var reg Registry[string, int]
	reg.Set("profile", 11)
	old := reg.Snapshot()
	reg.Set("profile", 12)
	if current, removed := reg.RemoveIf("profile", func(pid int) bool { return pid == 11 }); removed || current != 12 {
		t.Fatalf("late exit removed replacement: current=%d removed=%v", current, removed)
	}
	if old["profile"] != 11 || reg.Len() != 1 {
		t.Fatal("unstable registry snapshot")
	}
	if current, removed := reg.RemoveIf("profile", nil); !removed || current != 12 || reg.Len() != 0 {
		t.Fatal("stop did not remove active instance")
	}
}
