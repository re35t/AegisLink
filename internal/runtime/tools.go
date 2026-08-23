package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

type currentTimeInput struct {
	Timezone string `json:"timezone" jsonschema:"required,description=IANA timezone such as Asia/Shanghai or UTC"`
}

type currentTimeOutput struct {
	Timezone string `json:"timezone"`
	Time     string `json:"time"`
}

func builtInTools() ([]tool.BaseTool, error) {
	currentTime, err := toolutils.InferTool(
		"get_current_time",
		"Get the current time in an IANA timezone. Use this instead of guessing the current date or time.",
		func(_ context.Context, input *currentTimeInput) (*currentTimeOutput, error) {
			location, err := time.LoadLocation(input.Timezone)
			if err != nil {
				return nil, fmt.Errorf("invalid timezone %q", input.Timezone)
			}
			return &currentTimeOutput{Timezone: input.Timezone, Time: time.Now().In(location).Format(time.RFC3339)}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create current time tool: %w", err)
	}
	return []tool.BaseTool{currentTime}, nil
}
