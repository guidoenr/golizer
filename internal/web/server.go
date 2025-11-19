package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/guidoenr/golizer/internal/analyzer"
	apppkg "github.com/guidoenr/golizer/internal/app"
	"github.com/guidoenr/golizer/internal/params"
	"github.com/guidoenr/golizer/internal/render"
)

type Server struct {
	mu            sync.RWMutex
	app           AppInterface
	clients       map[*websocketClient]bool
	broadcast     chan []byte
	upgrader      websocket.Upgrader
	lastFeatures  analyzer.Features
	lastFPS       float64
	lastParams    params.Parameters
}

type AppInterface interface {
	GetParams() params.Parameters
	SetParams(p params.Parameters)
	GetRenderer() *render.Renderer
	GetFeatures() analyzer.Features
	GetFPS() float64
	GetConfig() apppkg.ConfigGetter
	SetNoiseFloor(float64)
	SetBufferSize(int)
	SetTargetFPS(float64)
	SetDimensions(int, int)
	SetAutoRandomize(bool)
	SetRandomInterval(time.Duration)
}

type websocketClient struct {
	conn   *websocket.Conn
	send   chan []byte
	server *Server
}

type StatusResponse struct {
	FPS      float64            `json:"fps"`
	Features analyzer.Features  `json:"features"` // only for display, not configurable
	Renderer RendererStatus     `json:"renderer"`
	Quality  string             `json:"quality,omitempty"`
}

type RendererStatus struct {
	Palette      string `json:"palette"`
	Pattern      string `json:"pattern"`
	ColorMode    string `json:"colorMode"`
	ColorOnAudio bool   `json:"colorOnAudio"`
}

type UpdateRequest struct {
	Params      *params.Parameters `json:"params,omitempty"`
	Palette     *string            `json:"palette,omitempty"`
	Pattern     *string            `json:"pattern,omitempty"`
	ColorMode   *string            `json:"colorMode,omitempty"`
	ColorOnAudio *bool             `json:"colorOnAudio,omitempty"`
	Quality     *string            `json:"quality,omitempty"`
	NoiseFloor  *float64           `json:"noiseFloor,omitempty"`
	BufferSize  *int               `json:"bufferSize,omitempty"`
	TargetFPS   *float64           `json:"targetFPS,omitempty"`
	Width       *int               `json:"width,omitempty"`
	Height      *int               `json:"height,omitempty"`
	AutoRandomize *bool            `json:"autoRandomize,omitempty"`
	RandomInterval *int            `json:"randomInterval,omitempty"`
}

type SavedConfig struct {
	Params         params.Parameters `json:"params"`
	Palette        string            `json:"palette"`
	Pattern        string            `json:"pattern"`
	ColorMode      string            `json:"colorMode"`
	ColorOnAudio   bool              `json:"colorOnAudio"`
	NoiseFloor     float64           `json:"noiseFloor"`
	BufferSize     int               `json:"bufferSize"`
	TargetFPS      float64           `json:"targetFPS"`
	Quality        string            `json:"quality"`
	Width          int               `json:"width"`
	Height         int               `json:"height"`
	AutoRandomize  bool              `json:"autoRandomize"`
	RandomInterval time.Duration     `json:"randomInterval"`
}

func NewServer(app AppInterface) *Server {
	return &Server{
		app:       app,
		clients:   make(map[*websocketClient]bool),
		broadcast: make(chan []byte, 256),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func findWebDir() string {
	// try current directory
	if _, err := os.Stat("web/index.html"); err == nil {
		return "web"
	}
	// try parent directory (if running from cmd/visualizer)
	if _, err := os.Stat("../web/index.html"); err == nil {
		return "../web"
	}
	// try absolute path from binary location
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		webPath := filepath.Join(exeDir, "web")
		if _, err := os.Stat(webPath + "/index.html"); err == nil {
			return webPath
		}
		// try parent of exe dir
		webPath = filepath.Join(filepath.Dir(exeDir), "web")
		if _, err := os.Stat(webPath + "/index.html"); err == nil {
			return webPath
		}
	}
	// fallback
	return "web"
}

func (s *Server) Start(port int) error {
	// find web directory (could be in repo root or relative to binary)
	webDir := findWebDir()
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, webDir+"/index.html")
	})
	http.HandleFunc("/api/status", s.handleStatus)
	http.HandleFunc("/api/update", s.handleUpdate)
	http.HandleFunc("/api/save", s.handleSave)
	http.HandleFunc("/api/palettes", s.handlePalettes)
	http.HandleFunc("/api/patterns", s.handlePatterns)
	http.HandleFunc("/api/colorModes", s.handleColorModes)
	http.HandleFunc("/ws", s.handleWebSocket)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(webDir+"/static"))))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[web] server starting on http://0.0.0.0%s", addr)
	log.Printf("[web] access from network: http://golizer.local%s or http://<pi-ip>%s", addr, addr)

	go s.broadcastLoop()
	go s.statusUpdateLoop()

	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	renderer := s.app.GetRenderer()
	cfg := s.app.GetConfig()
	status := StatusResponse{
		FPS:      s.lastFPS,
		Features: s.lastFeatures,
		Renderer: RendererStatus{
			Palette:      renderer.PaletteName(),
			Pattern:      renderer.PatternName(),
			ColorMode:    renderer.ColorModeName(),
			ColorOnAudio: renderer.ColorOnAudio(),
		},
		Quality: cfg.Quality(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// merge params if provided (partial update)
	if req.Params != nil {
		currentParams := s.app.GetParams()
		// merge non-zero values from request into current params
		if req.Params.Frequency > 0 {
			currentParams.Frequency = req.Params.Frequency
		}
		if req.Params.Amplitude > 0 {
			currentParams.Amplitude = req.Params.Amplitude
		}
		if req.Params.Speed > 0 {
			currentParams.Speed = req.Params.Speed
		}
		if req.Params.Brightness > 0 {
			currentParams.Brightness = req.Params.Brightness
		}
		if req.Params.Contrast > 0 {
			currentParams.Contrast = req.Params.Contrast
		}
		if req.Params.Saturation > 0 {
			currentParams.Saturation = req.Params.Saturation
		}
		if req.Params.BeatSensitivity > 0 {
			currentParams.BeatSensitivity = req.Params.BeatSensitivity
		}
		if req.Params.BassInfluence > 0 {
			currentParams.BassInfluence = req.Params.BassInfluence
		}
		if req.Params.MidInfluence > 0 {
			currentParams.MidInfluence = req.Params.MidInfluence
		}
		if req.Params.TrebleInfluence > 0 {
			currentParams.TrebleInfluence = req.Params.TrebleInfluence
		}
		s.app.SetParams(currentParams)
	}

	renderer := s.app.GetRenderer()
	if req.Palette != nil || req.Pattern != nil || req.ColorMode != nil || req.ColorOnAudio != nil {
		palette := renderer.PaletteName()
		pattern := renderer.PatternName()
		colorMode := renderer.ColorModeName()
		colorOnAudio := renderer.ColorOnAudio()

		if req.Palette != nil {
			palette = *req.Palette
		}
		if req.Pattern != nil {
			pattern = *req.Pattern
		}
		if req.ColorMode != nil {
			colorMode = *req.ColorMode
		}
		if req.ColorOnAudio != nil {
			colorOnAudio = *req.ColorOnAudio
		}

		renderer.Configure(palette, pattern, colorMode, colorOnAudio)
	}

	// update app config if provided
	if req.Quality != nil {
		s.app.GetRenderer().SetQuality(*req.Quality)
	}
	if req.NoiseFloor != nil {
		s.app.SetNoiseFloor(*req.NoiseFloor)
	}
	if req.BufferSize != nil {
		s.app.SetBufferSize(*req.BufferSize)
	}
	if req.TargetFPS != nil {
		s.app.SetTargetFPS(*req.TargetFPS)
	}
	if req.Width != nil || req.Height != nil {
		width := s.app.GetConfig().Width()
		height := s.app.GetConfig().Height()
		if req.Width != nil {
			width = *req.Width
		}
		if req.Height != nil {
			height = *req.Height
		}
		s.app.SetDimensions(width, height)
	}
	if req.AutoRandomize != nil {
		s.app.SetAutoRandomize(*req.AutoRandomize)
	}
	if req.RandomInterval != nil {
		s.app.SetRandomInterval(time.Duration(*req.RandomInterval) * time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	renderer := s.app.GetRenderer()
	currentParams := s.app.GetParams()
	cfg := s.app.GetConfig()
	s.mu.RUnlock()

	// get current config from app
	config := SavedConfig{
		Params:       currentParams,
		Palette:      renderer.PaletteName(),
		Pattern:      renderer.PatternName(),
		ColorMode:    renderer.ColorModeName(),
		ColorOnAudio: renderer.ColorOnAudio(),
		NoiseFloor:   cfg.NoiseFloor(),
		BufferSize:   cfg.BufferSize(),
		TargetFPS:    cfg.TargetFPS(),
		Quality:        cfg.Quality(),
		Width:          cfg.Width(),
		Height:         cfg.Height(),
		AutoRandomize:  cfg.AutoRandomize(),
		RandomInterval: cfg.RandomInterval(),
	}

	// override with values from request if provided
	var req SavedConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
		if req.Palette != "" {
			config.Palette = req.Palette
		}
		if req.Pattern != "" {
			config.Pattern = req.Pattern
		}
		if req.ColorMode != "" {
			config.ColorMode = req.ColorMode
		}
		if req.NoiseFloor > 0 {
			config.NoiseFloor = req.NoiseFloor
		}
		if req.BufferSize > 0 {
			config.BufferSize = req.BufferSize
		}
		if req.TargetFPS >= 0 {
			config.TargetFPS = req.TargetFPS
		}
		if req.Quality != "" {
			config.Quality = req.Quality
		}
		if req.Width > 0 {
			config.Width = req.Width
		}
		if req.Height > 0 {
			config.Height = req.Height
		}
		if req.Params.Frequency > 0 {
			config.Params = req.Params
		}
	}

	// save to file
	configPath := getConfigPath()
	if err := saveConfig(configPath, config); err != nil {
		http.Error(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved", "path": configPath})
}

func getConfigPath() string {
	// try to save in same directory as binary
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		return filepath.Join(exeDir, "golizer-config.json")
	}
	// fallback to home directory
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".golizer-config.json")
}

func saveConfig(path string, config SavedConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadConfig(path string) (*SavedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config SavedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *Server) handlePalettes(w http.ResponseWriter, r *http.Request) {
	palettes := render.PaletteNames()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(palettes)
}

func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	patterns := render.PatternNames()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(patterns)
}

func (s *Server) handleColorModes(w http.ResponseWriter, r *http.Request) {
	modes := render.ColorModeNames()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(modes)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[web] websocket upgrade error: %v", err)
		return
	}

	client := &websocketClient{
		conn:   conn,
		send:   make(chan []byte, 256),
		server: s,
	}

	s.mu.Lock()
	s.clients[client] = true
	s.mu.Unlock()

	go client.writePump()
	go client.readPump()
}

func (s *Server) broadcastLoop() {
	for {
		select {
		case message := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.mu.RUnlock()
		}
	}
}

func (s *Server) statusUpdateLoop() {
	// reduced frequency to 500ms to save CPU/FPS
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		s.lastFeatures = s.app.GetFeatures()
		s.lastFPS = s.app.GetFPS()
		renderer := s.app.GetRenderer()
		currentRenderer := RendererStatus{
			Palette:      renderer.PaletteName(),
			Pattern:      renderer.PatternName(),
			ColorMode:    renderer.ColorModeName(),
			ColorOnAudio: renderer.ColorOnAudio(),
		}
		cfg := s.app.GetConfig()
		s.mu.Unlock()

		status := StatusResponse{
			FPS:      s.lastFPS,
			Features: s.lastFeatures, // only for display stats
			Renderer: currentRenderer,
			Quality:  cfg.Quality(),
		}

		data, err := json.Marshal(status)
		if err == nil {
			select {
			case s.broadcast <- data:
			default:
				// drop if channel full (non-blocking)
			}
		}
	}
}

func (c *websocketClient) readPump() {
	defer func() {
		c.server.mu.Lock()
		delete(c.server.clients, c)
		c.server.mu.Unlock()
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *websocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

