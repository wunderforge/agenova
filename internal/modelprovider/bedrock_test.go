// Copyright 2026 Dapeng Zhang and Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

package modelprovider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type fakeConverse struct {
	calls int
	run   func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error)
}

func (f *fakeConverse) Converse(ctx context.Context, in *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	f.calls++
	return f.run(ctx, in)
}
func validBedrockOutput() *bedrockruntime.ConverseOutput {
	return &bedrockruntime.ConverseOutput{Output: &types.ConverseOutputMemberMessage{Value: types.Message{Role: types.ConversationRoleAssistant, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "ok"}}}}, StopReason: types.StopReasonEndTurn, Usage: &types.TokenUsage{InputTokens: aws.Int32(2), OutputTokens: aws.Int32(1)}}
}
func bedrockTestConfig() BedrockConfig {
	return BedrockConfig{Region: "ap-southeast-2", Models: map[string]string{"approved": "amazon.nova-micro-v1:0"}}
}

func TestBedrockRejectsInputBeforeProvider(t *testing.T) {
	f := &fakeConverse{run: func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		t.Fatal("unexpected provider call")
		return nil, nil
	}}
	b := newBedrock(bedrockTestConfig(), f)
	for _, req := range []Request{{Profile: "unknown", Prompt: "hello"}, {Profile: "approved", Prompt: " "}, {Profile: "approved", Prompt: strings.Repeat("a", maxPromptBytes+1)}, {Profile: "approved", Prompt: "hello", OutputSchema: []byte("null")}} {
		if _, err := b.Complete(context.Background(), req); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if f.calls != 0 || b.InvocationCount() != 0 {
		t.Fatal("invalid input called provider")
	}
}
func TestBedrockUsesConfiguredModelAndBoundsCall(t *testing.T) {
	f := &fakeConverse{run: func(ctx context.Context, in *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Minute {
			t.Fatal("missing deadline")
		}
		if aws.ToString(in.ModelId) != "amazon.nova-micro-v1:0" || aws.ToInt32(in.InferenceConfig.MaxTokens) != 256 {
			t.Fatal("wrong binding/limit")
		}
		return validBedrockOutput(), nil
	}}
	cfg := bedrockTestConfig()
	b := newBedrock(cfg, f)
	cfg.Models["approved"] = "changed"
	result, err := b.Complete(context.Background(), Request{Profile: "approved", Prompt: "hello"})
	if err != nil || result.Text != "ok" || result.Model != "amazon.nova-micro-v1:0" || f.calls != 1 || b.InvocationCount() != 1 {
		t.Fatalf("%+v %v", result, err)
	}
}
func TestBedrockSanitizesProviderFailureAndRejectsTruncation(t *testing.T) {
	f := &fakeConverse{run: func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		return nil, errors.New("SECRET credential endpoint body")
	}}
	b := newBedrock(bedrockTestConfig(), f)
	_, err := b.Complete(context.Background(), Request{Profile: "approved", Prompt: "hello"})
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("unsafe error %v", err)
	}
	f.run = func(context.Context, *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		out := validBedrockOutput()
		out.StopReason = types.StopReasonMaxTokens
		return out, nil
	}
	if _, err := b.Complete(context.Background(), Request{Profile: "approved", Prompt: "hello"}); err == nil {
		t.Fatal("truncated completion accepted")
	}
}
func TestBedrockStructuredResponseUsesOnlyExpectedTool(t *testing.T) {
	f := &fakeConverse{run: func(_ context.Context, in *bedrockruntime.ConverseInput) (*bedrockruntime.ConverseOutput, error) {
		choice, ok := in.ToolConfig.ToolChoice.(*types.ToolChoiceMemberTool)
		if !ok || aws.ToString(choice.Value.Name) != "agenova_response" {
			t.Fatal("schema not enforced through selected tool")
		}
		out := validBedrockOutput()
		out.StopReason = types.StopReasonToolUse
		out.Output = &types.ConverseOutputMemberMessage{Value: types.Message{Content: []types.ContentBlock{&types.ContentBlockMemberToolUse{Value: types.ToolUseBlock{Name: aws.String("agenova_response"), Input: document.NewLazyDocument(map[string]any{"answer": "ok"})}}}}}
		return out, nil
	}}
	b := newBedrock(bedrockTestConfig(), f)
	result, err := b.Complete(context.Background(), Request{Profile: "approved", Prompt: "hello", OutputSchema: []byte(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"]}`)})
	if err != nil || result.Text != `{"answer":"ok"}` {
		t.Fatalf("%+v %v", result, err)
	}
}
