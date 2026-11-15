package massh

import "testing"

func TestHostAddr(t *testing.T) {
	tests := []struct{ host, want string }{
		{"example.com", "example.com:22"},
		{"example.com:2222", "example.com:2222"},
		{"10.0.0.1", "10.0.0.1:22"},
		{"::1", "[::1]:22"},
		{"[::1]", "[::1]:22"},
		{"[::1]:2222", "[::1]:2222"},
	}
	for _, tt := range tests {
		if got := hostAddr(tt.host, 22); got != tt.want {
			t.Errorf("hostAddr(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}

func TestUniqueHosts(t *testing.T) {
	c := &Config{Hosts: []string{"b", "a", "b", "c", "a"}}
	got := c.uniqueHosts()
	want := []string{"b", "a", "c"}
	if len(got) != len(want) {
		t.Fatalf("uniqueHosts() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueHosts() = %v, want %v", got, want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote(`it's "x" $HOME`), `'it'\''s "x" $HOME'`; got != want {
		t.Errorf("shellQuote = %s, want %s", got, want)
	}
}
