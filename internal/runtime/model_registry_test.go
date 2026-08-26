package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/model"
)

func TestDefaultModelRegistryListsStableDrivers(t *testing.T) {
	t.Parallel()
	if got, want := DefaultModelRegistry().Drivers(), []string{"deepseek", "openai-compatible"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("drivers = %v, want %v", got, want)
	}
}

func TestModelRegistryCreatesRegisteredProvider(t *testing.T) {
	t.Parallel()
	fake := &fakeModel{}
	registry, err := NewModelRegistry(stubModelProvider{driver: "stub", chatModel: fake})
	if err != nil {
		t.Fatal(err)
	}
	created, err := registry.NewModel(t.Context(), ModelConfig{Driver: "stub"})
	if err != nil {
		t.Fatal(err)
	}
	if created != fake {
		t.Fatal("registry did not return the provider model")
	}
}

func TestModelRegistryRejectsUnsupportedDriver(t *testing.T) {
	t.Parallel()
	registry, err := NewModelRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.NewModel(t.Context(), ModelConfig{Driver: "missing"}); !errors.Is(err, ErrUnsupportedModelDriver) {
		t.Fatalf("expected unsupported driver error, got %v", err)
	}
}

type stubModelProvider struct {
	driver    string
	chatModel model.ToolCallingChatModel
}

func (provider stubModelProvider) Driver() string { return provider.driver }

func (provider stubModelProvider) New(context.Context, ModelConfig) (model.ToolCallingChatModel, error) {
	return provider.chatModel, nil
}
