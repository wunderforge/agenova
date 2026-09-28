// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelprovider

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// BedrockConfig is trusted host configuration. AWS identity comes from the
// host SDK credential chain, never a ClaimRequest or worker environment.
type BedrockConfig struct {
	Region       string
	Models       map[string]string
	MaxTokens    int
	Timeout      time.Duration
	OutputSchema json.RawMessage
}

type converseClient interface {
	Converse(context.Context, *bedrockruntime.ConverseInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
}

type Bedrock struct {
	attempts atomic.Uint64
	config   BedrockConfig
	client   converseClient
}

var _ Client = (*Bedrock)(nil)
var bedrockRegion = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)

// ValidateBedrockConfig is offline: Platform validation must not acquire AWS
// credentials or invoke a provider before the operator applies configuration.
func ValidateBedrockConfig(cfg BedrockConfig) error {
	if !bedrockRegion.MatchString(cfg.Region) {
		return errors.New("Bedrock region is invalid")
	}
	if len(cfg.Models) == 0 {
		return errors.New("Bedrock profiles are required")
	}
	for profile, model := range cfg.Models {
		if strings.TrimSpace(profile) == "" || len(profile) > 256 || strings.TrimSpace(model) == "" || len(model) > 2048 || strings.ContainsAny(model, " \t\r\n/") {
			return errors.New("Bedrock profile configuration is invalid")
		}
	}
	if cfg.MaxTokens < 0 || cfg.MaxTokens > maxTokens || cfg.Timeout < 0 || cfg.Timeout > maxTimeout {
		return errors.New("Bedrock inference limits are invalid")
	}
	if len(cfg.OutputSchema) > 8192 || (len(cfg.OutputSchema) > 0 && !json.Valid(cfg.OutputSchema)) {
		return errors.New("Bedrock output schema is invalid")
	}
	return nil
}

func NewBedrock(ctx context.Context, cfg BedrockConfig) (*Bedrock, error) {
	if ctx == nil {
		return nil, errors.New("Bedrock context is required")
	}
	if err := ValidateBedrockConfig(cfg); err != nil {
		return nil, err
	}
	loaded, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region), awsconfig.WithRetryMaxAttempts(1))
	if err != nil {
		return nil, errors.New("Bedrock AWS configuration failed")
	}
	return newBedrock(cfg, bedrockruntime.NewFromConfig(loaded, func(o *bedrockruntime.Options) {
		// Host environment endpoint overrides must not redirect signed inference.
		o.BaseEndpoint = nil
	})), nil
}

func newBedrock(cfg BedrockConfig, client converseClient) *Bedrock {
	models := make(map[string]string, len(cfg.Models))
	for k, v := range cfg.Models {
		models[k] = v
	}
	cfg.Models = models
	cfg.OutputSchema = append(json.RawMessage(nil), cfg.OutputSchema...)
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 256
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = time.Minute
	}
	return &Bedrock{config: cfg, client: client}
}

func (b *Bedrock) Complete(ctx context.Context, req Request) (Result, error) {
	if b == nil || b.client == nil {
		return Result{}, errors.New("Bedrock provider is unavailable")
	}
	if ctx == nil {
		return Result{}, errors.New("Bedrock context is required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	model, ok := b.config.Models[req.Profile]
	if !ok {
		return Result{}, errors.New("Bedrock profile is not configured")
	}
	if strings.TrimSpace(req.Prompt) == "" || len(req.Prompt) > maxPromptBytes {
		return Result{}, errors.New("Bedrock prompt is empty or oversized")
	}
	schema := b.config.OutputSchema
	if len(req.OutputSchema) > 0 {
		schema = req.OutputSchema
	}
	input := &bedrockruntime.ConverseInput{ModelId: aws.String(model), Messages: []types.Message{{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: req.Prompt}}}}, InferenceConfig: &types.InferenceConfiguration{MaxTokens: aws.Int32(int32(b.config.MaxTokens))}}
	if len(schema) > 0 {
		var object map[string]any
		if len(schema) > 8192 || json.Unmarshal(schema, &object) != nil || object == nil {
			return Result{}, errors.New("Bedrock output schema is invalid")
		}
		input.ToolConfig = &types.ToolConfiguration{Tools: []types.Tool{&types.ToolMemberToolSpec{Value: types.ToolSpecification{Name: aws.String("agenova_response"), Description: aws.String("Return the response in the specified structure."), InputSchema: &types.ToolInputSchemaMemberJson{Value: document.NewLazyDocument(object)}}}}, ToolChoice: &types.ToolChoiceMemberTool{Value: types.SpecificToolChoice{Name: aws.String("agenova_response")}}}
	}
	bounded, cancel := context.WithTimeout(ctx, b.config.Timeout)
	defer cancel()
	b.attempts.Add(1)
	output, err := b.client.Converse(bounded, input)
	if err != nil {
		return Result{}, requestFailure(bounded)
	}
	if err := bounded.Err(); err != nil {
		return Result{}, err
	}
	if output == nil || output.Usage == nil || output.Usage.InputTokens == nil || output.Usage.OutputTokens == nil || *output.Usage.InputTokens < 0 || *output.Usage.OutputTokens < 0 {
		return Result{}, errors.New("Bedrock returned invalid usage")
	}
	message, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok || len(message.Value.Content) != 1 {
		return Result{}, errors.New("Bedrock returned an unsupported completion")
	}
	text := ""
	if len(schema) > 0 {
		tool, ok := message.Value.Content[0].(*types.ContentBlockMemberToolUse)
		if !ok || aws.ToString(tool.Value.Name) != "agenova_response" || tool.Value.Input == nil || output.StopReason != types.StopReasonToolUse {
			return Result{}, errors.New("Bedrock returned invalid structured output")
		}
		encoded, err := tool.Value.Input.MarshalSmithyDocument()
		if err != nil || !json.Valid(encoded) {
			return Result{}, errors.New("Bedrock returned invalid structured output")
		}
		text = string(encoded)
	} else {
		block, ok := message.Value.Content[0].(*types.ContentBlockMemberText)
		if !ok || output.StopReason != types.StopReasonEndTurn {
			return Result{}, errors.New("Bedrock completion is incomplete or unsupported")
		}
		text = block.Value
	}
	if strings.TrimSpace(text) == "" || len(text) > maxResponseBytes {
		return Result{}, errors.New("Bedrock completion is empty or oversized")
	}
	requestID, _ := awsmiddleware.GetRequestIDMetadata(output.ResultMetadata)
	return Result{Text: text, Model: model, ResponseID: requestID, InputTokens: int(*output.Usage.InputTokens), OutputTokens: int(*output.Usage.OutputTokens)}, nil
}

// InvocationCount reports actual SDK Converse entries in this process. The
// configured SDK has retries disabled; it is diagnostic evidence, not billing.
func (b *Bedrock) InvocationCount() uint64 { return b.attempts.Load() }
