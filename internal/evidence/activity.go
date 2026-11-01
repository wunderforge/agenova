// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package evidence

// ActivityContent is an opt-in, access-controlled query projection. It is not
// authority and does not carry a credential, provider endpoint or raw header.
type ActivityContent struct {
	Kind            string `json:"kind"`
	Source          string `json:"source"`
	Input           string `json:"input,omitempty"`
	Output          string `json:"output,omitempty"`
	Thinking        string `json:"thinking,omitempty"`
	DecisionSummary string `json:"decisionSummary,omitempty"`
	Model           string `json:"model,omitempty"`
	FinishReason    string `json:"finishReason,omitempty"`
	InputTokens     int    `json:"inputTokens,omitempty"`
	OutputTokens    int    `json:"outputTokens,omitempty"`
	ElapsedMS       int64  `json:"elapsedMs,omitempty"`
	LoadMS          int64  `json:"loadMs,omitempty"`
	PromptMS        int64  `json:"promptMs,omitempty"`
	GenerateMS      int64  `json:"generateMs,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}

// ActivityStore permits a durable artifact implementation without changing the
// claim journal or requiring a new database. References are scoped to a Work.
type ActivityStore interface {
	Put(work string, content ActivityContent) (string, error)
	Get(work, ref string) (ActivityContent, error)
}
