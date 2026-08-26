package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

var ErrUnsupportedModelDriver = errors.New("unsupported model driver")

// ModelProvider creates one Eino tool-calling model for a configured driver.
// Provider SDK types remain inside internal/runtime.
type ModelProvider interface {
	Driver() string
	New(context.Context, ModelConfig) (model.ToolCallingChatModel, error)
}

type ModelRegistry struct {
	mu        sync.RWMutex
	providers map[string]ModelProvider
}

func NewModelRegistry(providers ...ModelProvider) (*ModelRegistry, error) {
	registry := &ModelRegistry{providers: make(map[string]ModelProvider, len(providers))}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func DefaultModelRegistry() *ModelRegistry {
	registry, err := NewModelRegistry(deepSeekProvider{}, openAICompatibleProvider{})
	if err != nil {
		panic(err)
	}
	return registry
}

func (registry *ModelRegistry) Register(provider ModelProvider) error {
	if provider == nil {
		return errors.New("model provider is required")
	}
	driver := strings.TrimSpace(provider.Driver())
	if driver == "" {
		return errors.New("model provider driver is required")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.providers == nil {
		registry.providers = make(map[string]ModelProvider)
	}
	if _, exists := registry.providers[driver]; exists {
		return fmt.Errorf("model provider %q is already registered", driver)
	}
	registry.providers[driver] = provider
	return nil
}

func (registry *ModelRegistry) NewModel(ctx context.Context, cfg ModelConfig) (model.ToolCallingChatModel, error) {
	driver := strings.TrimSpace(cfg.Driver)
	registry.mu.RLock()
	provider, exists := registry.providers[driver]
	registry.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("%w %q", ErrUnsupportedModelDriver, driver)
	}
	chatModel, err := provider.New(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create %s model: %w", driver, err)
	}
	if chatModel == nil {
		return nil, fmt.Errorf("create %s model: provider returned nil", driver)
	}
	return chatModel, nil
}

func (registry *ModelRegistry) Drivers() []string {
	registry.mu.RLock()
	drivers := make([]string, 0, len(registry.providers))
	for driver := range registry.providers {
		drivers = append(drivers, driver)
	}
	registry.mu.RUnlock()
	sort.Strings(drivers)
	return drivers
}

type deepSeekProvider struct{}

func (deepSeekProvider) Driver() string { return "deepseek" }

func (deepSeekProvider) New(ctx context.Context, cfg ModelConfig) (model.ToolCallingChatModel, error) {
	return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Name, Timeout: cfg.Timeout, MaxTokens: cfg.MaxTokens,
	})
}

type openAICompatibleProvider struct{}

func (openAICompatibleProvider) Driver() string { return "openai-compatible" }

func (openAICompatibleProvider) New(ctx context.Context, cfg ModelConfig) (model.ToolCallingChatModel, error) {
	maxTokens := cfg.MaxTokens
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Name, Timeout: cfg.Timeout, MaxTokens: &maxTokens,
	})
}
