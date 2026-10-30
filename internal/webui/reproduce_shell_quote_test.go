package webui

import "testing"

func TestReproductionCommandPreservesArgvAndPreventsShellExpansion(t *testing.T) {
	for _, tc := range []struct {
		label    string
		args     []string
		expected string
	}{
		{"normal", []string{"ulab", "run", "--config", "test.yaml"}, "ulab run --config test.yaml"},
		{"space in path", []string{"ulab", "run", "--config", "private plans/verified.json"}, "ulab run --config 'private plans/verified.json'"},
		{"empty argument", []string{"ulab", "run", ""}, "ulab run ''"},
		{"subshell", []string{"ulab", "run", "$(echo unsafe)"}, "ulab run '$(echo unsafe)'"},
		{"single quote", []string{"ulab", "run", "owner's file.json"}, "ulab run 'owner'\"'\"'s file.json'"},
		{"redirect", []string{"ulab", "run", "value; echo unsafe > file"}, "ulab run 'value; echo unsafe > file'"},
		{"untrusted newline", []string{"ulab", "run", "before\nafter"}, "ulab run 'before\nafter'"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got := reproduceCommand(tc.args)
			if got != tc.expected {
				t.Fatalf("copyable command %q does not preserve shell arguments; want %q", got, tc.expected)
			}
		})
	}
}
