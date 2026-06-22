package task

import "testing"

func TestClassifyExitCode(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{code: 0, want: StatusCompleted},
		{code: 10, want: StatusNeedsReview},
		{code: 1, want: StatusBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := classifyExitCode(tt.code)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
