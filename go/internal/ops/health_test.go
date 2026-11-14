package ops

import "testing"

func TestHealthTransitionOnlyOnChange(t *testing.T) {
	a := map[string]any{"xray": true}
	if HealthChanged(nil, a) {
		t.Fatal("first snapshot is not transition")
	}
	if HealthChanged(a, a) {
		t.Fatal("same snapshot changed")
	}
	if !HealthChanged(a, map[string]any{"xray": false}) {
		t.Fatal("state transition missed")
	}
}
