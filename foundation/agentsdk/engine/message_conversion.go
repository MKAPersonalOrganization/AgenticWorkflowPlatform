package engine

import (
	"encoding/json"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"strings"
)

// mapFromCoreToLLMMessages converts a slice of core.Message to llm.Message.
func mapFromCoreToLLMMessages(msgs []core.Message) []llm.Message {
	result := make([]llm.Message, 0, len(msgs))

	var currentToolMsg *llm.Message

	for _, m := range msgs {
		cm := mapFromCoreToLLMMessage(m)

		if cm.Role == llm.RoleTool {
			// If we are currently building a tool message, add to it
			if currentToolMsg != nil {
				currentToolMsg.Content = append(currentToolMsg.Content, cm.Content...)
				continue
			}
			// Start a new tool message
			currentToolMsg = &cm
			result = append(result, *currentToolMsg)
			// Get reference to the one in the slice so we append to it
			currentToolMsg = &result[len(result)-1]
		} else {
			// Non-tool message, reset current tool message builder
			currentToolMsg = nil
			result = append(result, cm)
		}
	}

	return result
}

// mapFromCoreToLLMMessage converts a core.Message to llm.Message.
// This bridges the persistence layer (core.Message) with the LLM layer (Message).
func mapFromCoreToLLMMessage(msg core.Message) llm.Message {
	cm := llm.Message{
		Role:    llm.Role(msg.Role),
		Name:    msg.AgentName,
		Content: make([]llm.Content, 0),
	}

	// Add text content if present
	// Note: We check specifically for non-tool role because tool outputs are handled below
	if msg.Content != "" && msg.Role != "tool" {
		cm.Content = append(cm.Content, llm.TextContent{Text: msg.Content})
	}

	// Add multimodal content from metadata (images, audio, files).
	// Attachments may arrive as []any (from event replay/JSON) or []map[string]any
	// (from in-process injection). We handle both.
	// Only base64 data URLs and absolute HTTP(S) URLs are LLM-accessible;
	// relative artifact paths (e.g. /v1/threads/.../artifacts/...) are skipped.
	for _, attMap := range extractAttachmentMaps(msg.Metadata) {
		url, _ := attMap["url"].(string)
		if url == "" || !isLLMAccessibleURL(url) {
			continue
		}
		mimeType, _ := attMap["mimeType"].(string)

		switch {
		case strings.HasPrefix(mimeType, "image/") || (mimeType == "" && strings.HasPrefix(url, "data:image/")):
			detail, _ := attMap["detail"].(string)
			if detail == "" {
				detail = "auto"
			}
			cm.Content = append(cm.Content, llm.ImageURLContent{
				URL:    url,
				Detail: detail,
			})
		case strings.HasPrefix(mimeType, "audio/"):
			data := stripDataURLPrefix(url)
			format := audioFormatFromMIME(mimeType)
			cm.Content = append(cm.Content, llm.AudioContent{
				Data:   data,
				Format: format,
			})
		default:
			data := stripDataURLPrefix(url)
			fileName, _ := attMap["fileName"].(string)
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			cm.Content = append(cm.Content, llm.FileContent{
				Data:     data,
				FileName: fileName,
				MimeType: mimeType,
			})
		}
	}

	// Add function calls from tool calls
	for _, tc := range msg.ToolCalls {
		cm.Content = append(cm.Content, llm.FunctionCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: json.RawMessage(tc.Arguments),
		})
	}

	// Add function result for tool messages
	if msg.Role == "tool" {
		cm.Content = append(cm.Content, llm.FunctionResult{
			CallID:  msg.ToolCallID,
			Name:    msg.ToolName,
			Result:  json.RawMessage(msg.Content),
			IsError: msg.IsError,
		})
	}

	return cm
}

// extractAttachmentMaps extracts a slice of map[string]any from msg.Metadata["attachments"].
// It handles both []any (produced by JSON unmarshal / event replay) and
// []map[string]any (produced by in-process injection from the runner).
func extractAttachmentMaps(metadata map[string]any) []map[string]any {
	raw, ok := metadata["attachments"]
	if !ok {
		return nil
	}

	// Fast path: []any (from JSON / event replay)
	if anyList, ok := raw.([]any); ok {
		result := make([]map[string]any, 0, len(anyList))
		for _, item := range anyList {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	}

	// Alternate path: []map[string]any (from runner's extractUserAttachments)
	if mapList, ok := raw.([]map[string]any); ok {
		return mapList
	}

	return nil
}

// stripDataURLPrefix removes the data:...;base64, prefix from a data URL,
// returning just the raw base64 content.
func stripDataURLPrefix(dataURL string) string {
	if idx := strings.Index(dataURL, ","); idx >= 0 {
		return dataURL[idx+1:]
	}
	return dataURL
}

// audioFormatFromMIME returns the audio format string for the LLM from a MIME type.
func audioFormatFromMIME(mimeType string) string {
	switch mimeType {
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	default:
		return "wav"
	}
}

// isLLMAccessibleURL returns true if the URL can be consumed by an external LLM API.
// Base64 data URLs and absolute HTTP(S) URLs are accessible.
// Relative paths (like /v1/threads/.../artifacts/...) are NOT accessible to the LLM.
func isLLMAccessibleURL(url string) bool {
	return strings.HasPrefix(url, "data:") ||
		strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://")
}
