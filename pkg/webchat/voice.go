package webchat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"localagent/pkg/logger"
	"localagent/pkg/tools"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// voiceMessage is the JSON envelope for voice WebSocket messages.
type voiceMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Text     string `json:"text,omitempty"`
	Content  string `json:"content,omitempty"`
	State    string `json:"state,omitempty"`
	Message  string `json:"message,omitempty"`
	Speaker  string `json:"speaker,omitempty"`
	Language string `json:"language,omitempty"`
}

type voiceSession struct {
	conn    *websocket.Conn
	channel *WebChatChannel
	stt     struct{ url, key string }
	tts     struct{ url, key string }

	writeMu  sync.Mutex
	mu       sync.Mutex
	speaker  string
	language string

	cancelTTS  context.CancelFunc
	cancelTurn context.CancelFunc
	mediaDir   string
	turnMu     sync.Mutex // serializes handleAudio calls
}

func (s *Server) handleVoice(c *echo.Context) error {
	tts := s.channel.tts
	stt := s.channel.stt
	if stt.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "stt not configured"})
	}
	if tts.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tts not configured"})
	}

	conn, err := wsUpgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return fmt.Errorf("websocket upgrade: %w", err)
	}
	defer conn.Close()

	conn.SetReadLimit(10 * 1024 * 1024) // 10MB for audio chunks

	speaker := tts.Speaker
	if speaker == "" {
		speaker = "Aiden"
	}
	language := tts.Language
	if language == "" {
		language = "English"
	}

	vs := &voiceSession{
		conn:     conn,
		channel:  s.channel,
		speaker:  speaker,
		language: language,
		mediaDir: s.mediaDir,
	}
	vs.stt.url = stt.URL
	vs.stt.key = stt.ResolveAPIKey()
	vs.tts.url = tts.URL
	vs.tts.key = tts.ResolveAPIKey()

	logger.Info("voice session started")
	vs.sendStatus("listening")

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()

	conn.SetCloseHandler(func(code int, text string) error {
		cancel()
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.Info("voice session closed normally")
			} else {
				logger.Info("voice session closed: %v", err)
			}
			return nil
		}

		var msg voiceMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			logger.Warn("voice: invalid message: %v", err)
			continue
		}

		switch msg.Type {
		case "audio":
			go vs.handleAudio(ctx, msg.Data)
		case "interrupt":
			vs.interruptTurn()
		case "config":
			vs.mu.Lock()
			if msg.Speaker != "" {
				vs.speaker = msg.Speaker
			}
			if msg.Language != "" {
				vs.language = msg.Language
			}
			vs.mu.Unlock()
		}
	}
}

func (vs *voiceSession) handleAudio(ctx context.Context, b64Audio string) {
	// Cancel any in-flight turn (TTS + agent wait)
	vs.interruptTurn()

	// Serialize: wait for previous turn to fully release
	vs.turnMu.Lock()
	defer vs.turnMu.Unlock()

	// Create a cancellable context for this turn
	turnCtx, cancel := context.WithCancel(ctx)
	vs.mu.Lock()
	vs.cancelTurn = cancel
	vs.mu.Unlock()
	defer func() {
		cancel()
		vs.mu.Lock()
		vs.cancelTurn = nil
		vs.mu.Unlock()
	}()
	ctx = turnCtx

	// Decode float32 PCM from base64
	raw, err := base64.StdEncoding.DecodeString(b64Audio)
	if err != nil {
		logger.Error("voice: decode audio: %v", err)
		vs.sendError("failed to decode audio")
		return
	}

	// Convert float32 PCM to 16-bit WAV file for STT
	wavData := float32PCMToWAV(raw, 16000)

	// Write to temp file
	if err := os.MkdirAll(vs.mediaDir, 0700); err != nil {
		logger.Error("voice: create media dir: %v", err)
		vs.sendError("internal error")
		return
	}
	tmpFile, err := os.CreateTemp(vs.mediaDir, "voice-*.wav")
	if err != nil {
		logger.Error("voice: create temp file: %v", err)
		vs.sendError("internal error")
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(wavData); err != nil {
		tmpFile.Close()
		logger.Error("voice: write temp file: %v", err)
		vs.sendError("internal error")
		return
	}
	tmpFile.Close()

	// STT
	vs.sendStatus("processing")
	text, err := tools.TranscribeAudio(ctx, tmpPath, vs.stt.url, vs.stt.key)
	if err != nil {
		if ctx.Err() != nil {
			return // turn was cancelled
		}
		logger.Error("voice: transcribe: %v", err)
		vs.sendError("transcription failed")
		vs.sendStatus("listening")
		return
	}

	text = trimWhitespace(text)
	if text == "" {
		vs.sendStatus("listening")
		return
	}

	if ctx.Err() != nil {
		return
	}

	logger.Info("voice: transcribed: %q", text)
	vs.sendJSON(voiceMessage{Type: "stt", Text: text})

	// Send through agent loop and wait for response
	responseCh := make(chan string, 1)
	vs.channel.setVoiceResponseCh(responseCh)
	defer vs.channel.setVoiceResponseCh(nil)

	vs.channel.HandleIncoming(text, nil, nil)

	// Wait for response with timeout
	var response string
	select {
	case response = <-responseCh:
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Minute):
		vs.sendError("response timeout")
		vs.sendStatus("listening")
		return
	}

	vs.sendJSON(voiceMessage{Type: "text", Content: response})

	// Stream TTS
	vs.mu.Lock()
	speaker := vs.speaker
	language := vs.language
	vs.mu.Unlock()

	ttsCtx, cancel := context.WithCancel(ctx)
	vs.mu.Lock()
	vs.cancelTTS = cancel
	vs.mu.Unlock()

	defer func() {
		cancel()
		vs.mu.Lock()
		vs.cancelTTS = nil
		vs.mu.Unlock()
	}()

	vs.sendStatus("speaking")
	err = vs.streamTTS(ttsCtx, response, speaker, language)
	if err != nil && ttsCtx.Err() == nil {
		logger.Error("voice: TTS stream error: %v", err)
	}

	vs.sendJSON(voiceMessage{Type: "audio_end"})
	vs.sendStatus("listening")
}

func (vs *voiceSession) interruptTurn() {
	vs.mu.Lock()
	if vs.cancelTurn != nil {
		vs.cancelTurn()
	}
	if vs.cancelTTS != nil {
		vs.cancelTTS()
		vs.cancelTTS = nil
	}
	vs.mu.Unlock()
}

func (vs *voiceSession) streamTTS(ctx context.Context, text, speaker, language string) error {
	body, err := json.Marshal(map[string]string{
		"text":     text,
		"speaker":  speaker,
		"language": language,
	})
	if err != nil {
		return err
	}

	streamURL := vs.tts.url + "/stream"
	req, err := http.NewRequestWithContext(ctx, "POST", streamURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if vs.tts.key != "" {
		req.Header.Set("Authorization", "Bearer "+vs.tts.key)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression:  true,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tts returned %d: %s", resp.StatusCode, string(b))
	}

	// Read WAV header (44 bytes) to get sample rate
	header := make([]byte, 44)
	if _, err := io.ReadFull(resp.Body, header); err != nil {
		return fmt.Errorf("read wav header: %w", err)
	}

	sampleRate := binary.LittleEndian.Uint32(header[24:28])

	// Send sample rate info to client
	vs.sendJSON(voiceMessage{Type: "audio_start", Data: fmt.Sprintf("%d", sampleRate)})

	// Stream PCM chunks, ensuring each send is sample-aligned (2 bytes per sample)
	buf := make([]byte, 8192)
	var carry byte
	hasCarry := false
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			data := buf[:n]
			if hasCarry {
				data = append([]byte{carry}, data...)
				hasCarry = false
			}
			if len(data)%2 != 0 {
				carry = data[len(data)-1]
				hasCarry = true
				data = data[:len(data)-1]
			}
			if len(data) > 0 {
				chunk := base64.StdEncoding.EncodeToString(data)
				vs.sendJSON(voiceMessage{Type: "audio", Data: chunk})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return nil
}

func (vs *voiceSession) sendJSON(msg voiceMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	vs.writeMu.Lock()
	defer vs.writeMu.Unlock()
	vs.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	vs.conn.WriteMessage(websocket.TextMessage, data)
}

func (vs *voiceSession) sendStatus(state string) {
	vs.sendJSON(voiceMessage{Type: "status", State: state})
}

func (vs *voiceSession) sendError(msg string) {
	vs.sendJSON(voiceMessage{Type: "error", Message: msg})
}

func (s *Server) handleTTS(c *echo.Context) error {
	tts := s.channel.tts
	if tts.URL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "tts not configured"})
	}

	var input struct {
		Text string `json:"text"`
	}
	if err := c.Bind(&input); err != nil || input.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "text is required"})
	}

	body, _ := json.Marshal(map[string]string{
		"text":     input.Text,
		"speaker":  tts.Speaker,
		"language": tts.Language,
	})

	streamURL := tts.URL + "/stream"
	req, err := http.NewRequestWithContext(c.Request().Context(), "POST", streamURL, bytes.NewReader(body))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	req.Header.Set("Content-Type", "application/json")
	if key := tts.ResolveAPIKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DisableCompression:    true,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return c.JSON(resp.StatusCode, map[string]string{"error": string(b)})
	}

	w := c.Response()
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 8192)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			break
		}
	}
	return nil
}

// float32PCMToWAV converts raw float32 PCM samples to a WAV file in memory.
func float32PCMToWAV(raw []byte, sampleRate int) []byte {
	numSamples := len(raw) / 4
	pcm16 := make([]byte, numSamples*2)

	for i := 0; i < numSamples; i++ {
		bits := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
		f := math.Float32frombits(bits)
		if f > 1.0 {
			f = 1.0
		} else if f < -1.0 {
			f = -1.0
		}
		s := int16(f * 32767)
		binary.LittleEndian.PutUint16(pcm16[i*2:], uint16(s))
	}

	dataSize := len(pcm16)
	var buf bytes.Buffer
	buf.Grow(44 + dataSize)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))           // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))            // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(1))            // mono
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))   // sample rate
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))            // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))           // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcm16)

	return buf.Bytes()
}

func trimWhitespace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
