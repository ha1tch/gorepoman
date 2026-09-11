package waveprogress

import "testing"

// TestWaveStatusWord_DefaultPreservesDone is FR-03's regression
// coverage: passing "" (the config default -- no wave_complete_word
// set) must produce exactly the original hardcoded behaviour, byte
// for byte, for every project that never touches the new key.
func TestWaveStatusWord_DefaultPreservesDone(t *testing.T) {
	cases := []struct {
		name string
		w    wave
		want string
	}{
		{"not started", wave{DoneEquiv: 0, Total: 4}, "not started"},
		{"in progress", wave{DoneEquiv: 2, Total: 4}, "in progress"},
		{"done", wave{DoneEquiv: 4, Total: 4}, "done"},
		{"zero-total wave never reads as done", wave{DoneEquiv: 0, Total: 0}, "not started"},
	}
	for _, tc := range cases {
		got := waveStatusWord(tc.w, "")
		if got != tc.want {
			t.Errorf("%s: waveStatusWord(%+v, \"\") = %q, want %q", tc.name, tc.w, got, tc.want)
		}
	}
}

// TestWaveStatusWord_OverrideAppliesOnlyToComplete confirms the
// override (FR-03's actual purpose) changes only the "done" word --
// "not started" and "in progress" are untouched, since only the
// completed state was ever the point of contention (xolu's own
// convention is "complete", not "done").
func TestWaveStatusWord_OverrideAppliesOnlyToComplete(t *testing.T) {
	const word = "complete"
	cases := []struct {
		name string
		w    wave
		want string
	}{
		{"not started unaffected", wave{DoneEquiv: 0, Total: 4}, "not started"},
		{"in progress unaffected", wave{DoneEquiv: 2, Total: 4}, "in progress"},
		{"done becomes the override word", wave{DoneEquiv: 4, Total: 4}, word},
	}
	for _, tc := range cases {
		got := waveStatusWord(tc.w, word)
		if got != tc.want {
			t.Errorf("%s: waveStatusWord(%+v, %q) = %q, want %q", tc.name, tc.w, word, got, tc.want)
		}
	}
}
