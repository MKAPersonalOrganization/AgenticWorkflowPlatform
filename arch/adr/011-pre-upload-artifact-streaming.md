# ADR 011: Pre-Upload Artifact Storage with HTTP Range Streaming for File Attachments

## Status
Accepted

## Context
The original file attachment pipeline embedded the entire file as a base64 data URL at every layer of the stack:

```
Browser → JSON body (base64) → Redis/Asynq payload → Worker → S3 save → Event store
Browser ← JSON snapshot (base64) ← Projection ← Event store
```

This created several scaling problems:

1. **Redis memory pressure.** A single 3 MB audio file becomes ~4 MB in base64, stored in the Asynq task payload. Under concurrent load, N users × M attachments × 4 MB rapidly exhausted queue memory.
2. **JSON parse latency.** Both the HTTP handler (`SendMessage`) and the Asynq worker deserialized multi-megabyte JSON payloads, introducing unnecessary CPU and allocation pressure.
3. **Browser DOM thrashing.** Alpine.js reactive state held multi-MB data URL strings. When these were bound to `<audio :src="att.data">` in dynamically rendered `x-for` loops, the browser failed to trigger media loading — requiring a full page refresh to play audio.
4. **No progressive playback.** Audio and video required the entire file to be present in the DOM before playback could begin. There was no support for HTTP Range requests (206 Partial Content), so the browser could not stream or seek.
5. **Upload-send coupling.** File data was only transmitted when the user pressed "Send", creating a poor UX for large files — no progress feedback, and a slow POST that could time out.

## Decision
**Decouple file upload from message sending. Upload files to S3 immediately on attach, pass short artifact URLs through the message pipeline, and serve playback via `http.ServeContent` with Range request support.**

### 1. Immediate Pre-Upload on Attach

When the user attaches a file, the frontend uploads it to S3 immediately via a new endpoint:

```
POST /v1/threads/{threadId}/attachments:upload
Content-Type: multipart/form-data

→ 201 Created
{
  "url":      "/v1/threads/{threadId}/artifacts/{fileName}",
  "mimeType": "audio/wav",
  "fileName": "recording.wav"
}
```

The file entry in Alpine.js state tracks an upload lifecycle:

```js
{
  name: "recording.wav",
  type: "audio/wav",
  status: "uploading" | "ready" | "error",
  url: "/v1/threads/.../artifacts/recording.wav",  // set on success
  data: null,  // base64 only for local image preview
}
```

- **Images** get a local `FileReader.readAsDataURL()` for instant thumbnail preview during upload.
- **Audio/PDF/other** types show a spinner overlay until the upload completes.
- The Send button is disabled while any file has `status === 'uploading'`.

### 2. Compact Message Payload

`sendMessage()` now transmits artifact references instead of data blobs:

```json
{
  "contextId": "thread_abc",
  "message": "Here is the audio",
  "attachments": [
    {
      "url": "/v1/threads/thread_abc/artifacts/recording.wav",
      "mimeType": "audio/wav",
      "fileName": "recording.wav"
    }
  ]
}
```

This ~100 byte reference replaces the ~4 MB base64 string at every hop: HTTP handler → Asynq task → Redis queue → worker.

### 3. Orchestrator: Skip Re-Upload, Lazy Load for LLM

The orchestrator's `saveAttachments()` detects pre-uploaded files by URL prefix and passes them through without re-saving to S3:

```go
if att.URL != "" {
    refs = append(refs, domain.Attachment{URL: att.URL, ...})
    continue
}
// Legacy path: decode base64 → save to S3
```

When `buildMessageContent()` constructs the LLM request, it loads the file from S3 on demand via `loadArtifactAsDataURL()`, converting it to a base64 data URL only at the point where the LLM API requires it:

```go
func (s *Service) loadArtifactAsDataURL(ctx, userID, threadID, fileName, mimeType) (string, error) {
    reader, meta, _ := s.artifactStore.Load(ctx, userID, threadID, fileName)
    data, _ := io.ReadAll(reader)
    return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}
```

This is the **only** place in the entire pipeline where full file bytes are loaded into memory — and only at execution time, not at enqueue time.

### 4. HTTP Range Streaming for Playback

The `DownloadArtifact` handler uses `http.ServeContent` with the `io.ReadSeeker` interface:

```go
func (h *Handler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
    content, meta, _ := h.artifactStore.Load(ctx, userID, threadID, fileName)
    defer content.Close()

    w.Header().Set("Content-Type", meta.ContentType)

    if rs, ok := content.(io.ReadSeeker); ok {
        http.ServeContent(w, r, fileName, time.Time{}, rs)
    } else {
        io.Copy(w, content)  // fallback
    }
}
```

`minio.Object` implements `io.ReadSeeker`, so `http.ServeContent` automatically handles:
- **Range requests** (HTTP 206 Partial Content) — browsers can seek in audio/video
- **Content-Length** — browsers show accurate progress bars
- **Conditional requests** (If-Modified-Since, If-None-Match) — cache efficiency

The browser's native `<audio>` element issues Range requests automatically, enabling progressive playback without loading the entire file.

### 5. Artifact Service Interface Changes

The `artifact.Service` interface was updated to support content type metadata:

```go
type ArtifactMeta struct {
    ContentType string
    Size        int64
}

type Service interface {
    Save(ctx, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error)
    Load(ctx, userID, sessionID, fileName string) (io.ReadCloser, ArtifactMeta, error)
    List(ctx, userID, sessionID string) ([]string, error)
    Delete(ctx, userID, sessionID, fileName string) error
}
```

`Save` now accepts a `contentType` parameter so the correct MIME type is stored in S3 object metadata. `Load` returns `ArtifactMeta` alongside the reader so callers can set `Content-Type` headers without guessing.

## Consequences

### Positive
- **Redis payloads shrink from ~4 MB to ~200 bytes** per attachment, removing the primary memory scaling bottleneck.
- **Audio/video plays immediately** without full download — native browser streaming via HTTP 206.
- **Upload feedback is instant** — spinner appears on attach, not on send.
- **Separation of concerns** — upload lifecycle is independent of message lifecycle.
- **LLM data loading is lazy** — file bytes are read from S3 only when the worker actually needs them for the API call, not when the task is enqueued.
- **Same pattern as industry standard** — Slack, Discord, and WhatsApp Web all use pre-upload + reference passing.

### Negative
- **Requires an active thread before attaching.** Files cannot be uploaded until a thread/session exists. The frontend shows an error toast if the user attaches before selecting a session.
- **Two HTTP calls per file** (upload + send) instead of one. Mitigated by the upload happening concurrently in the background while the user composes their message.
- **File name collisions** within the same thread cause S3 overwrites. A future improvement could prefix filenames with a short UUID.
- **Body limit exclusion is broad.** The global body limit excludes the entire `/v1/threads/` prefix to allow the upload endpoint through. This could be tightened to match only the upload path pattern.

### Data Flow Comparison

**Before:**
```
Attach → base64 in JS state (4 MB)
  → JSON POST body (4 MB)
    → Redis task payload (4 MB)
      → Worker decode + S3 save
        → Browser DOM <audio :src="data:..."> (4 MB in DOM)
```

**After:**
```
Attach → Upload to S3 (streamed, 3 MB)
  → URL in JS state (~50 bytes)
    → JSON POST body (~100 bytes)
      → Redis task payload (~100 bytes)
        → Worker loads from S3 only for LLM
          → Browser DOM <audio src="/v1/.../file.wav"> → HTTP 206 streaming
```

## Files Changed
- `services/artifact/service.go` — `ArtifactMeta` struct, updated `Save`/`Load` signatures
- `services/artifact/s3.go` — Content type on put, metadata on get
- `handlers/taskmgmt/handlers.go` — `UploadAttachment` handler, `DownloadArtifact` with `http.ServeContent`
- `main.go` — Upload route group with 25 MB body limit
- `services/agentorchestrator/service.go` — Pre-upload skip in `saveAttachments`, `loadArtifactAsDataURL`, method receiver on `buildMessageContent`
- `services/taskmanagement/asynq_worker.go` — URL field in `FileAttachment`, passthrough in conversion
- `assets/js/chat/chat-input.js` — `_addFiles` rewrite, `_uploadFile` async upload
- `assets/js/chat/chat-messaging.js` — URL-based attachment capture, upload guard
- `assets/js/chat/chat-streaming.js` — URL references in stream payload
- `assets/js/chat/chat-utils.js` — `createUserMessage` prefers artifact URL
- `assets/js/chat/index.js` — `canSendMessage` checks upload status
- `components/message/message.templ` — `<audio>` with `x-init` src assignment
- `components/chatinput/filepreview.templ` — Upload spinner/error overlays
