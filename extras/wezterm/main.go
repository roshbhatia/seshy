package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	sessionstore "github.com/roshbhatia/seshy/internal/session"
)

type request struct {
	Version    string `json:"version"`
	Kind       string `json:"kind"`
	RequestID  string `json:"requestId"`
	Capability string `json:"capability"`
	Input      struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"input"`
}
type response struct {
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	Output    any    `json:"output,omitempty"`
	Message   string `json:"message,omitempty"`
}
type segment struct {
	Text string `json:"text"`
	Role string `json:"role"`
}
type item struct {
	ID       string    `json:"id"`
	Search   string    `json:"search"`
	Segments []segment `json:"segments"`
}
type plan struct {
	Kind        string            `json:"kind"`
	Label       string            `json:"label"`
	Cwd         string            `json:"cwd"`
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment"`
}
type adapter struct {
	core    string
	timeout time.Duration
}

func (a adapter) run(output any, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	data, err := exec.CommandContext(ctx, a.core, args...).Output()
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", a.core, ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("%s: %w", a.core, err)
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", a.core, err)
	}
	return nil
}
func serve(in io.Reader, out io.Writer, a adapter) error {
	r := request{RequestID: "invalid"}
	err := json.NewDecoder(in).Decode(&r)
	var output any
	if err == nil {
		if r.Version != "provider/v1" || r.Kind != "request" {
			err = errors.New("unsupported request")
		} else {
			output, err = a.handle(r)
		}
	}
	result := response{Version: "provider/v1", Kind: "result", RequestID: r.RequestID, Status: "ok", Output: output}
	if err != nil {
		result.Status = "error"
		result.Message = err.Error()
	}
	return json.NewEncoder(out).Encode(result)
}
func main() {
	core := defaultCore
	if len(os.Args) > 1 {
		core = os.Args[1]
	}
	if len(os.Args) == 4 && os.Args[2] == "--create" {
		if err := startCreatedSession(adapter{core: core, timeout: 4 * time.Second}, os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := serve(os.Stdin, os.Stdout, adapter{core: core, timeout: 4 * time.Second}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const defaultCore = "sy"

type session struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a adapter) sessions() ([]session, error) {
	var rows []session
	err := a.run(&rows, "list", "--json")
	return rows, err
}
func (a adapter) handle(r request) (any, error) {
	switch r.Capability {
	case "provider.validate":
		if err := a.run(nil, "--help"); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	case "picker.describe":
		return map[string]any{"title": "Sessions", "icon": "md_layers", "create": map[string]string{"label": "New session", "prompt": "Session name"}}, nil
	case "picker.create":
		if err := sessionstore.ValidateSessionName(r.Input.Name); err != nil {
			return nil, err
		}
		rows, err := a.sessions()
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Name == r.Input.Name {
				return nil, errors.New("session already exists")
			}
		}
		helper, err := os.Executable()
		if err != nil {
			return nil, err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return plan{"spawn", r.Input.Name, home, []string{helper, a.core, "--create", r.Input.Name}, map[string]string{}}, nil
	case "picker.list":
		rows, err := a.sessions()
		if err != nil {
			return nil, err
		}
		items := make([]item, 0, len(rows))
		for _, row := range rows {
			items = append(items, item{row.Name, row.Name + " " + row.Path, []segment{{row.Name, "name"}, {row.Path, "path"}}})
		}
		return map[string]any{"items": items}, nil
	case "picker.open":
		rows, err := a.sessions()
		if err != nil {
			return nil, err
		}
		found := false
		for _, row := range rows {
			if row.Name == r.Input.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("session no longer exists")
		}
		var p struct {
			Version     string            `json:"version"`
			Cwd         string            `json:"cwd"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
		}
		if err := a.run(&p, "open", "--format", "json", r.Input.ID); err != nil {
			return nil, err
		}
		if p.Version != "seshy.open/v1" || len(p.Command) != 0 {
			return nil, errors.New("unsupported directory plan")
		}
		info, err := os.Stat(p.Cwd)
		if err != nil || !info.IsDir() || !filepath.IsAbs(p.Cwd) {
			return nil, errors.New("session directory no longer exists or is not absolute")
		}
		if p.Environment == nil {
			p.Environment = map[string]string{}
		}
		return plan{"spawn", r.Input.ID, p.Cwd, []string{}, p.Environment}, nil
	default:
		return nil, errors.New("unsupported capability")
	}
}

func (a adapter) createInteractive(name string, in io.Reader, out, errout io.Writer) (*plan, error) {
	if err := sessionstore.ValidateSessionName(name); err != nil {
		return nil, err
	}
	command := exec.Command(a.core, "new", name)
	command.Stdin, command.Stdout, command.Stderr = in, out, errout
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	rows, err := a.sessions()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Name == name {
			r := request{Capability: "picker.open"}
			r.Input.ID = name
			value, err := a.handle(r)
			if err != nil {
				return nil, err
			}
			created := value.(plan)
			return &created, nil
		}
	}
	return nil, nil
}

func startCreatedSession(a adapter, name string) error {
	var shell []string
	if err := json.Unmarshal([]byte(os.Getenv("WEZTERM_PICKER_SHELL")), &shell); err != nil || len(shell) == 0 || shell[0] == "" {
		return errors.New("missing WezTerm shell command")
	}
	program, err := exec.LookPath(shell[0])
	if err != nil {
		return err
	}
	created, err := a.createInteractive(name, os.Stdin, os.Stdout, os.Stderr)
	if err != nil || created == nil {
		return err
	}
	if err := os.Chdir(created.Cwd); err != nil {
		return err
	}
	for key, value := range created.Environment {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if err := os.Unsetenv("WEZTERM_PICKER_SHELL"); err != nil {
		return err
	}
	return syscall.Exec(program, shell, os.Environ())
}
