package runtime

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
)

func TestBuiltInCurrentTimeTool(t *testing.T) {
	t.Parallel()
	tools, err := builtInTools()
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("tool count = %d", len(tools))
	}
	invokable, ok := tools[0].(tool.InvokableTool)
	if !ok {
		t.Fatal("current time tool is not invokable")
	}
	result, err := invokable.InvokableRun(t.Context(), `{"timezone":"Asia/Shanghai"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `"timezone":"Asia/Shanghai"`) || !strings.Contains(result, `"time":`) {
		t.Fatalf("unexpected tool result %q", result)
	}
}

func TestBuiltInCurrentTimeToolRejectsInvalidTimezone(t *testing.T) {
	t.Parallel()
	tools, err := builtInTools()
	if err != nil {
		t.Fatal(err)
	}
	_, err = tools[0].(tool.InvokableTool).InvokableRun(t.Context(), `{"timezone":"Mars/Olympus"}`)
	if err == nil {
		t.Fatal("expected invalid timezone error")
	}
}
