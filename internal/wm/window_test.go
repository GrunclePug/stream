package wm

import "testing"

func TestOffSize(t *testing.T) {
	tests := []struct {
		name string
		w    Window
		want bool
	}{
		{"took the proposed size", Window{width: 800, height: 600, resized: true, proposed: true, proposedW: 800, proposedH: 600}, false},
		{"resized itself", Window{width: 854, height: 480, resized: true, proposed: true, proposedW: 800, proposedH: 600}, true},
		{"unchanged since last checked", Window{width: 854, height: 480, proposed: true, proposedW: 800, proposedH: 600}, false},
		{"nothing proposed yet", Window{width: 854, height: 480, resized: true}, false},
		{"already asked out of this size", Window{width: 854, height: 480, resized: true, proposed: true, proposedW: 800, proposedH: 600, refusedW: 854, refusedH: 480}, false},
		{"another size than the refused one", Window{width: 640, height: 480, resized: true, proposed: true, proposedW: 800, proposedH: 600, refusedW: 854, refusedH: 480}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.offSize(); got != tt.want {
				t.Errorf("offSize() = %v, want %v", got, tt.want)
			}
		})
	}
}
