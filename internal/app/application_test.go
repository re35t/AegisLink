package app

import (
	"testing"

	"github.com/re35t/AegisLink/internal/config"
)

func TestRuntimeModelConfigPreservesCuratorOutputControls(t *testing.T) {
	t.Parallel()
	resolved := runtimeModelConfig(config.Model{JSONOutput: true, ThinkingMode: "disabled"})
	if !resolved.JSONOutput || resolved.ThinkingMode != "disabled" {
		t.Fatalf("runtime model config = %#v", resolved)
	}
}
