package stream

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type StreamSession struct {
	DeviceID     int64     `json:"device_id"`
	DeviceName   string    `json:"device_name"`
	ProfileToken string    `json:"profile_token"`
	RTSPURL      string    `json:"rtsp_url"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	HLSURL       string    `json:"hls_url"`

	cmd    *exec.Cmd
	cancel context.CancelFunc
}

type StreamManager struct {
	mu          sync.Mutex
	sessions    map[int64]*StreamSession
	hlsDir      string
	hlsTime     int
	hlsListSize int
}

func NewStreamManager(hlsDir string, hlsTime, hlsListSize int) (*StreamManager, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		return nil, fmt.Errorf("create hls dir: %w", err)
	}
	return &StreamManager{
		sessions:    make(map[int64]*StreamSession),
		hlsDir:      hlsDir,
		hlsTime:     hlsTime,
		hlsListSize: hlsListSize,
	}, nil
}

func (m *StreamManager) StartStream(deviceID int64, deviceName, profileToken, rtspURL string) {
	m.mu.Lock()
	if existing, ok := m.sessions[deviceID]; ok {
		existing.Status = "stopped"
		existing.cancel()
	}
	session := &StreamSession{
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		ProfileToken: profileToken,
		RTSPURL:      rtspURL,
		Status:       "starting",
		StartedAt:    time.Now(),
		HLSURL:       fmt.Sprintf("/hls/%d/index.m3u8", deviceID),
	}
	m.sessions[deviceID] = session
	m.mu.Unlock()

	go m.runStream(session)
}

func (m *StreamManager) StopStream(deviceID int64) {
	m.mu.Lock()
	session, ok := m.sessions[deviceID]
	if !ok {
		m.mu.Unlock()
		return
	}
	session.Status = "stopped"
	if session.cancel != nil {
		session.cancel()
	}
	delete(m.sessions, deviceID)
	m.mu.Unlock()

	dir := filepath.Join(m.hlsDir, fmt.Sprintf("%d", deviceID))
	os.RemoveAll(dir)
}

func (m *StreamManager) GetSession(deviceID int64) *StreamSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[deviceID]
	if !ok {
		return nil
	}
	cp := *s
	return &cp
}

func (m *StreamManager) ListSessions() []StreamSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]StreamSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, *s)
	}
	return out
}

func (m *StreamManager) Shutdown() {
	m.mu.Lock()
	for _, s := range m.sessions {
		s.Status = "stopped"
		if s.cancel != nil {
			s.cancel()
		}
	}
	m.mu.Unlock()
}

func (m *StreamManager) runStream(session *StreamSession) {
	for {
		m.mu.Lock()
		if session.Status == "stopped" {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		ctx, cancel := context.WithCancel(context.Background())
		session.cancel = cancel

		segmentPattern := filepath.Join(m.hlsDir, fmt.Sprintf("%d", session.DeviceID), "seg_%03d.ts")
		playlistPath := filepath.Join(m.hlsDir, fmt.Sprintf("%d", session.DeviceID), "index.m3u8")

		if err := os.MkdirAll(filepath.Dir(playlistPath), 0755); err != nil {
			slog.Error("stream mkdir failed", "device_id", session.DeviceID, "error", err)
			cancel()
			return
		}

		args := []string{
			"-rtsp_transport", "tcp",
			"-i", session.RTSPURL,
			"-c:v", "copy",
			"-an",
			"-f", "hls",
			"-hls_time", fmt.Sprintf("%d", m.hlsTime),
			"-hls_list_size", fmt.Sprintf("%d", m.hlsListSize),
			"-hls_flags", "delete_segments",
			"-hls_segment_filename", segmentPattern,
			playlistPath,
		}

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		session.cmd = cmd

		m.mu.Lock()
		session.Status = "running"
		session.Error = ""
		m.mu.Unlock()

		slog.Info("ffmpeg started", "device_id", session.DeviceID, "device_name", session.DeviceName)

		err := cmd.Run()

		m.mu.Lock()
		if session.Status == "stopped" {
			m.mu.Unlock()
			return
		}
		session.Status = "error"
		session.Error = fmt.Sprintf("ffmpeg: %v", err)
		m.mu.Unlock()

		slog.Warn("ffmpeg exited", "device_id", session.DeviceID, "device_name", session.DeviceName, "error", err)

		time.Sleep(2 * time.Second)
	}
}
