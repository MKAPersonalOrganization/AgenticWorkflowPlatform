package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
)

var (
	ErrTextRequired     = errors.New("text parameter is required")
	ErrTextTooLong      = errors.New("text exceeds maximum length of 4096 characters")
	ErrAPIKeyMissing    = errors.New("OpenAI API key is not configured — please add an OpenAI provider in AI Settings")
	ErrTTSRequestFailed = errors.New("TTS API request failed")
)

// SpeakArgs represents the arguments for the TTS speak tool.
type SpeakArgs struct {
	Text         string `json:"text"`
	Voice        string `json:"voice,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// WordTimestamp represents a single word with its start and end time in seconds,
// produced by Whisper forced-alignment on the generated audio.
type WordTimestamp struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// whisperResponse is the verbose_json response from OpenAI's Whisper API.
type whisperResponse struct {
	Text  string          `json:"text"`
	Words []WordTimestamp `json:"words"`
}

// SpeakTool implements the agent.ExecutableTool interface for text-to-speech generation.
type SpeakTool struct {
	ArtifactStore artifact.Service
	APIKey        string
}

// NewSpeakTool creates a new TTS speak tool.
func NewSpeakTool(store artifact.Service, apiKey string) *SpeakTool {
	return &SpeakTool{
		ArtifactStore: store,
		APIKey:        apiKey,
	}
}

func (t *SpeakTool) Name() string {
	return "speak"
}

func (t *SpeakTool) Description() string {
	return `Generate spoken audio from text using OpenAI's text-to-speech API. Converts the provided text into lifelike spoken audio and returns a playable audio file. Use this tool when the user asks you to read something aloud, narrate text, or generate speech from content. The audio is generated as MP3 and can be played directly in the browser.

IMPORTANT: This is an AI-generated voice. Always make this clear to the user.`
}

func (t *SpeakTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"text": {
				Type:        agent.TypeString,
				Description: "REQUIRED. The text to convert to speech. Maximum 4096 characters. This should be the content you want spoken aloud. Can be in any supported language. Format the text using Markdown so the user can read along with the speech — use headings, bold, lists, etc. where appropriate.",
			},
			"voice": {
				Type:        agent.TypeString,
				Description: "Optional. The voice to use for speech generation. Available voices: alloy, ash, ballad, coral, echo, fable, nova, onyx, sage, shimmer, verse, marin, cedar. Defaults to 'coral'. For best quality, use 'marin' or 'cedar'.",
				Enum:        []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse", "marin", "cedar"},
				Default:     "coral",
			},
			"instructions": {
				Type:        agent.TypeString,
				Description: "Optional. Instructions for how the speech should be delivered. Controls accent, tone, speed, emotion, etc. Examples: 'Speak in a cheerful and positive tone.', 'Read slowly and clearly as if teaching a child.', 'Use a calm, soothing voice.'",
			},
		},
		Required: []string{"text"},
	}
}

func (t *SpeakTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params SpeakArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse TTS args: %w", err)), nil
	}

	// Validate inputs
	if strings.TrimSpace(params.Text) == "" {
		return agent.ErrorResult(ErrTextRequired), nil
	}
	if len(params.Text) > 4096 {
		return agent.ErrorResult(ErrTextTooLong), nil
	}
	if t.APIKey == "" {
		return agent.ErrorResult(ErrAPIKeyMissing), nil
	}

	// Default voice
	voice := params.Voice
	if voice == "" {
		voice = "coral"
	}

	// Default model — gpt-4o-mini-tts supports instructions and full voice set
	model := "gpt-4o-mini-tts"

	// Call OpenAI TTS API
	audioData, err := t.callOpenAITTS(ctx, model, voice, params.Text, params.Instructions)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("TTS generation failed: %w", err)), nil
	}

	// If we have no artifact store or session context, return a simple success
	if t.ArtifactStore == nil || args.UserID == "" || args.SessionID == "" {
		return agent.JSONResult(map[string]any{
			"success": true,
			"message": "Speech generated successfully (not saved — missing context)",
			"voice":   voice,
			"model":   model,
		}), nil
	}

	// Save audio as artifact
	filename := fmt.Sprintf("speech-%s-%d.mp3", sanitizeFilename(voice), time.Now().UnixMilli())
	reader := bytes.NewReader(audioData)

	_, err = t.ArtifactStore.Save(ctx, args.UserID, args.SessionID, filename, "audio/mpeg", reader)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to save audio artifact: %w", err)), nil
	}

	audioURL := "/v1/threads/" + args.SessionID + "/artifacts/" + filename

	// Obtain word-level timestamps via Whisper forced-alignment for
	// synchronized word highlighting during playback. Non-fatal — the
	// frontend falls back to a static Markdown transcript on failure.
	var wordTimestamps []WordTimestamp
	if ts, alignErr := t.callWhisperAlignment(ctx, audioData); alignErr == nil {
		wordTimestamps = ts
	}

	result := map[string]any{
		"success":         true,
		"audio_url":       audioURL,
		"filename":        filename,
		"voice":           voice,
		"model":           model,
		"format":          "mp3",
		"text":            params.Text,
		"word_timestamps": wordTimestamps,
		"message":         "Speech generated successfully",
	}

	return agent.JSONResult(result), nil
}

// callOpenAITTS calls the OpenAI TTS API and returns the raw audio bytes.
func (t *SpeakTool) callOpenAITTS(ctx context.Context, model, voice, text, instructions string) ([]byte, error) {
	payload := map[string]any{
		"model": model,
		"voice": voice,
		"input": text,
	}
	if instructions != "" {
		payload["instructions"] = instructions
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling TTS request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating TTS request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing TTS request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status %d — %s", ErrTTSRequestFailed, resp.StatusCode, string(errBody))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading TTS response: %w", err)
	}

	return audioData, nil
}

// callWhisperAlignment sends the generated audio to OpenAI's Whisper API with
// word-level timestamp granularity. This gives us precise timing for every
// spoken word, enabling the frontend to highlight words in real time.
func (t *SpeakTool) callWhisperAlignment(ctx context.Context, audioData []byte) ([]WordTimestamp, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "speech.mp3")
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(audioData); err != nil {
		return nil, fmt.Errorf("writing audio data: %w", err)
	}

	_ = writer.WriteField("model", "whisper-1")
	_ = writer.WriteField("response_format", "verbose_json")
	_ = writer.WriteField("timestamp_granularities[]", "word")

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/transcriptions", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating whisper request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("whisper API status %d: %s", resp.StatusCode, string(errBody))
	}

	var result whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding whisper response: %w", err)
	}

	return result.Words, nil
}

// sanitizeFilename removes non-alphanumeric characters from a filename segment.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		return "audio"
	}
	return filepath.Base(result) // extra safety
}
