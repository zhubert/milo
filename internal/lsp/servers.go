package lsp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// ServerConfig defines how to detect and run a language server.
type ServerConfig struct {
	Name        string   // Human-readable name (e.g., "gopls")
	Command     []string // Command to start the server (e.g., ["gopls", "serve"])
	Languages   []string // Languages this server supports (e.g., ["go"])
	Extensions  []string // File extensions (e.g., [".go"])
	RootFiles   []string // Files that indicate project root (e.g., ["go.mod"])
	InstallHint string   // How to install this server

	mu        sync.RWMutex
	available bool
	checked   bool
}

// IsAvailable returns whether this server is installed.
func (s *ServerConfig) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

// setAvailable marks the server as available or not.
func (s *ServerConfig) setAvailable(available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = available
	s.checked = true
}

// defaultServers defines the built-in language server configurations.
var defaultServers = []*ServerConfig{
	{
		Name:        "gopls",
		Command:     []string{"gopls", "serve"},
		Languages:   []string{"go"},
		Extensions:  []string{".go"},
		RootFiles:   []string{"go.mod", "go.work"},
		InstallHint: "go install golang.org/x/tools/gopls@latest",
	},
	{
		Name:        "typescript-language-server",
		Command:     []string{"typescript-language-server", "--stdio"},
		Languages:   []string{"typescript", "javascript"},
		Extensions:  []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"},
		RootFiles:   []string{"tsconfig.json", "jsconfig.json", "package.json"},
		InstallHint: "npm install -g typescript-language-server typescript",
	},
	{
		Name:        "rust-analyzer",
		Command:     []string{"rust-analyzer"},
		Languages:   []string{"rust"},
		Extensions:  []string{".rs"},
		RootFiles:   []string{"Cargo.toml"},
		InstallHint: "rustup component add rust-analyzer",
	},
	{
		Name:        "pyright",
		Command:     []string{"pyright-langserver", "--stdio"},
		Languages:   []string{"python"},
		Extensions:  []string{".py", ".pyi"},
		RootFiles:   []string{"pyproject.toml", "setup.py", "requirements.txt", "pyrightconfig.json"},
		InstallHint: "npm install -g pyright",
	},
	{
		Name:        "clangd",
		Command:     []string{"clangd"},
		Languages:   []string{"c", "cpp", "objective-c"},
		Extensions:  []string{".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx", ".m", ".mm"},
		RootFiles:   []string{"compile_commands.json", "CMakeLists.txt", "Makefile", ".clangd"},
		InstallHint: "brew install llvm (macOS) or apt install clangd (Linux)",
	},
}

// Registry holds language server configurations and their availability status.
type Registry struct {
	mu      sync.RWMutex
	servers []*ServerConfig
	ready   chan struct{}
}

// NewRegistry creates a new server registry with default servers.
func NewRegistry() *Registry {
	// Create fresh copies of default servers to avoid shared state
	servers := make([]*ServerConfig, len(defaultServers))
	for i, s := range defaultServers {
		servers[i] = &ServerConfig{
			Name:        s.Name,
			Command:     append([]string{}, s.Command...),
			Languages:   append([]string{}, s.Languages...),
			Extensions:  append([]string{}, s.Extensions...),
			RootFiles:   append([]string{}, s.RootFiles...),
			InstallHint: s.InstallHint,
		}
	}
	return &Registry{
		servers: servers,
		ready:   make(chan struct{}),
	}
}

// DetectAvailable probes for installed language servers.
// This should be run in a goroutine at startup.
func (r *Registry) DetectAvailable(ctx context.Context) {
	r.mu.RLock()
	servers := r.servers
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(s *ServerConfig) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check if the server binary is available
			if len(s.Command) > 0 {
				_, err := exec.LookPath(s.Command[0])
				s.setAvailable(err == nil)
			}
		}(server)
	}

	wg.Wait()
	close(r.ready)
}

// WaitReady blocks until detection is complete or context is cancelled.
func (r *Registry) WaitReady(ctx context.Context) error {
	select {
	case <-r.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsReady returns true if detection has completed.
func (r *Registry) IsReady() bool {
	select {
	case <-r.ready:
		return true
	default:
		return false
	}
}

// Servers returns all registered server configurations.
func (r *Registry) Servers() []*ServerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.servers
}

// FindForFile returns the server configuration for a file path.
// Returns an error if no server is configured for the file type or if the server is not installed.
func (r *Registry) FindForFile(filePath string) (*ServerConfig, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return nil, &NoServerError{FileType: "files without extension"}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, server := range r.servers {
		for _, serverExt := range server.Extensions {
			if serverExt == ext {
				if !server.IsAvailable() {
					return nil, &ServerNotInstalledError{
						Server:      server.Name,
						InstallHint: server.InstallHint,
					}
				}
				return server, nil
			}
		}
	}

	return nil, &NoServerError{FileType: ext + " files"}
}

// FindProjectRoot walks up from the file path looking for root indicator files.
func FindProjectRoot(filePath string, rootFiles []string) string {
	dir := filepath.Dir(filePath)
	for {
		for _, rootFile := range rootFiles {
			candidate := filepath.Join(dir, rootFile)
			if fileExists(candidate) {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return filepath.Dir(filePath)
		}
		dir = parent
	}
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NoServerError indicates no server is configured for a file type.
type NoServerError struct {
	FileType string
}

func (e *NoServerError) Error() string {
	return "no language server configured for " + e.FileType
}

// ServerNotInstalledError indicates a server is configured but not installed.
type ServerNotInstalledError struct {
	Server      string
	InstallHint string
}

func (e *ServerNotInstalledError) Error() string {
	msg := e.Server + " is not installed"
	if e.InstallHint != "" {
		msg += ". Install with: " + e.InstallHint
	}
	return msg
}
