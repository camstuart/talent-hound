package main

import "testing"

func TestGreetServiceGreet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal name",
			input: "Cam",
			want:  "Hello Cam, welcome to Talent Hound",
		},
		{
			name:  "empty string",
			input: "",
			want:  "Hello , welcome to Talent Hound",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GreetService{}
			got := g.Greet(tt.input)
			if got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
