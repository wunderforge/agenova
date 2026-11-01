// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0
package console

import "github.com/wunderforge/agenova/internal/workerprotocol"

func matchingActionRef(id, tool, scope, requestedTool, requestedScope string) string {
	if id != "" && tool == requestedTool && scope == requestedScope {
		return id
	}
	return ""
}

func safeActionKind(a workerprotocol.Action, err error) string {
	if err != nil {
		return ""
	}
	return a.Action
}
func safeActionTool(a workerprotocol.Action, err error) string {
	if err != nil {
		return ""
	}
	return a.Tool
}
func safeActionResource(a workerprotocol.Action, err error) string {
	if err != nil {
		return ""
	}
	return a.Resource
}
