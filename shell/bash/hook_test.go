package bash_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/it-odyssey/waketrail/internal/capture"
	"github.com/it-odyssey/waketrail/internal/storage"
)

// Exercise the actual installed-style CLI and Bash descriptors, rather than a
// mocked tee. No user shell configuration or existing recording is touched.
func TestBashCaptureLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Bash hook currently targets Linux")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("Bash unavailable")
	}
	// Subprocess file reads are invisible to Go's test cache. Register the hook
	// and its CLI entry points so cached tests cannot hide changes to them.
	for _, path := range []string{"waketrail-hook.sh", "../../cmd/capture_stream.go", "../../cmd/record.go"} {
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-buildvcs=false", "-o", filepath.Join(bin, "waketrail"), "../..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build helper: %v\n%s", err, out)
	}
	hook, err := filepath.Abs("waketrail-hook.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("runtime-fallback-%v", fallback), func(t *testing.T) {
			dir := t.TempDir()
			runtimeDir := filepath.Join(dir, "runtime")
			tmpDir := filepath.Join(dir, "tmp")
			for _, path := range []string{runtimeDir, tmpDir} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if fallback {
				if err := os.Chmod(runtimeDir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			stateDir := filepath.Join(dir, "state")
			t.Setenv("XDG_STATE_HOME", stateDir)
			t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
			t.Setenv("TMPDIR", tmpDir)
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			t.Setenv("PS1", "test> ")
			value := "HEAD EVIDENCE\n" + strings.Repeat("ordinary evidence\n", 10000) + "TAIL EVIDENCE\n"
			data := filepath.Join(dir, "data")
			if err := os.WriteFile(data, []byte(value), 0600); err != nil {
				t.Fatal(err)
			}
			script := fmt.Sprintf("trap 'printf \"OLD_EXIT:%%s\\n\" \"$?\"' EXIT\nsource %q\nsource %q\n__waketrail_probe_runtime() { __waketrail_prepare_tmp; printf 'TEMP_DIR:%%s\\n' \"$WAKETRAIL_TMP_DIR\"; stat -c 'TEMP_MODE:%%a' -- \"$WAKETRAIL_TMP_DIR\"; __waketrail_cleanup_files; }\n__waketrail_probe_runtime\nwaketrail start fixture\ngrep . %q\nprintf 'PIPELINE EVIDENCE\\n' | grep .\ngrep . %q; printf 'LIST EVIDENCE\\n'\nfalse\nwaketrail stop\nexit 7\n", hook, hook, data, data)
			stdout, stderr, exit := runBash(t, script, true)
			if exit != 7 || !bytes.Contains(stdout, []byte(value)) || !bytes.Contains(stdout, []byte("OLD_EXIT:7")) {
				t.Fatalf("live output or EXIT status lost: exit=%d stderr=%s", exit, stderr)
			}
			selected := runtimeDir
			if fallback {
				selected = tmpDir
			}
			if !bytes.Contains(stdout, []byte("TEMP_DIR:"+selected+"/waketrail.")) || !bytes.Contains(stdout, []byte("TEMP_MODE:700")) {
				t.Fatal("runtime selection or private directory mode incorrect")
			}
			assertNoTemps(t, runtimeDir, tmpDir)
			store, err := storage.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			session, err := store.SessionByName("fixture")
			if err != nil {
				t.Fatal(err)
			}
			events, err := store.CommandEventsForSession(session.ID)
			if err != nil {
				t.Fatal(err)
			}
			var foundOutput, foundFailure bool
			deniedCommands := 0
			for _, event := range events {
				if strings.Contains(event.Command, "PIPELINE EVIDENCE") || strings.Contains(event.Command, "LIST EVIDENCE") {
					deniedCommands++
					_, err := store.CommandOutputForCommandEvent(event.ID)
					if event.CaptureMode != "none" || !errors.Is(err, storage.ErrCommandOutputNotFound) {
						t.Fatal("compound output bypassed metadata-only policy")
					}
				}
				if event.Command == "false" {
					foundFailure = event.ExitCode == 1
				}
				if event.CaptureMode != "output" || !strings.HasPrefix(event.Command, "grep . ") {
					continue
				}
				output, err := store.CommandOutputForCommandEvent(event.ID)
				if err != nil {
					t.Fatal(err)
				}
				foundOutput = output.StdoutBytes == int64(len(value)) && output.StdoutTruncated && len(output.Stdout) <= capture.OutputLimit && strings.Contains(output.Stdout, "HEAD EVIDENCE") && strings.Contains(output.Stdout, "TAIL EVIDENCE")
			}
			if !foundOutput || !foundFailure || deniedCommands != 2 {
				t.Fatalf("persisted capture/status wrong: output=%v failure=%v", foundOutput, foundFailure)
			}
		})
	}

	t.Run("exit-during-capture", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_RUNTIME_DIR", dir)
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		script := fmt.Sprintf("trap 'printf \"OLD_EXIT:%%s\\n\" \"$?\"' EXIT\nsource %q\ntrap - DEBUG\n__waketrail_begin_capture\nprintf 'LIVE OUTPUT\\n'\nexit 9\n", hook)
		stdout, stderr, exit := runBash(t, script, false)
		if exit != 9 || !bytes.Contains(stdout, []byte("LIVE OUTPUT")) || !bytes.Contains(stdout, []byte("OLD_EXIT:9")) {
			t.Fatalf("exit cleanup changed output/status: %d %s", exit, stderr)
		}
		assertNoTemps(t, dir)
	})

	t.Run("temp-failure-keeps-output", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_RUNTIME_DIR", "")
		t.Setenv("TMPDIR", filepath.Join(dir, "missing"))
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		script := fmt.Sprintf("source %q\ntrap - DEBUG\nif __waketrail_begin_capture; then exit 99; fi\nprintf 'LIVE OUTPUT\\n'\n", hook)
		stdout, stderr, exit := runBash(t, script, false)
		if exit != 0 || !bytes.Contains(stdout, []byte("LIVE OUTPUT")) {
			t.Fatalf("temp failure broke output: %d %s", exit, stderr)
		}
	})

	t.Run("capture-file-failure-keeps-output", func(t *testing.T) {
		value := strings.Repeat("live diagnostic evidence\n", 10000)
		cmd := exec.Command(filepath.Join(bin, "waketrail"), "capture-stream", "--output", filepath.Join(t.TempDir(), "missing", "stdout"))
		cmd.Stdin = strings.NewReader(value)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err == nil || stdout.String() != value {
			t.Fatal("capture file failure discarded live output or was not reported")
		}
	})

	t.Run("interrupt-retains-evidence", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
		stateDir := filepath.Join(dir, "state")
		t.Setenv("XDG_STATE_HOME", stateDir)
		t.Setenv("XDG_RUNTIME_DIR", dir)
		t.Setenv("PATH", dir+":"+bin+":"+os.Getenv("PATH"))
		ready := filepath.Join(dir, "ready")
		// Stand in for an eligible long-running diagnostic. The sleep is
		// interrupted as part of the shell process group; drainers must survive.
		program := `#!/usr/bin/env bash
printf 'INTERRUPT EVIDENCE\n'
printf 'STDERR EVIDENCE\n' >&2
: > "$1"
exec sleep 30
`
		if err := os.WriteFile(filepath.Join(dir, "grep"), []byte(program), 0700); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-i")
		// Detach the test shell from the caller's controlling terminal.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = time.Second
		cmd.Stdin = strings.NewReader(fmt.Sprintf("source %q\nwaketrail start interrupted\ngrep %q\nwaketrail stop\nexit\n", hook, ready))
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		for {
			_, readyErr := os.Stat(ready)
			spools, _ := filepath.Glob(filepath.Join(dir, "waketrail.*", "std*"))
			if readyErr == nil && len(spools) == 2 {
				break
			}
			if ctx.Err() != nil {
				cmd.Wait()
				t.Fatalf("diagnostic did not become ready: %s", stderr.String())
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGINT); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Wait(); err != nil || ctx.Err() != nil {
			t.Fatalf("interrupt stalled/broke shell: %v %s", err, stderr.String())
		}
		assertNoTemps(t, dir)
		store, err := storage.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		session, err := store.SessionByName("interrupted")
		if err != nil {
			t.Fatal(err)
		}
		events, err := store.CommandEventsForSession(session.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if strings.HasPrefix(event.Command, "grep ") {
				output, err := store.CommandOutputForCommandEvent(event.ID)
				if err != nil || event.ExitCode != 130 || !strings.Contains(output.Stdout, "INTERRUPT EVIDENCE") || !strings.Contains(output.Stderr, "STDERR EVIDENCE") {
					t.Fatalf("interrupted evidence/status lost: %+v %v", event, err)
				}
				return
			}
		}
		t.Fatal("interrupted command not recorded")
	})

	for _, setting := range []string{"HISTSIZE=0", "HISTIGNORE='*'"} {
		t.Run("unavailable-history-"+setting, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			data := filepath.Join(dir, "data")
			if err := os.WriteFile(data, []byte("LIVE EVIDENCE\n"), 0600); err != nil {
				t.Fatal(err)
			}
			script := fmt.Sprintf("source %q\nwaketrail start history-fixture\n%s\ngrep . %q\nwaketrail stop\nexit\n", hook, setting, data)
			stdout, stderr, exit := runBash(t, script, true)
			if exit != 0 || !bytes.Contains(stdout, []byte("LIVE EVIDENCE")) {
				t.Fatalf("history policy changed live output: %d %s", exit, stderr)
			}
			store, err := storage.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			session, err := store.SessionByName("history-fixture")
			if err != nil {
				t.Fatal(err)
			}
			events, err := store.CommandEventsForSession(session.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range events {
				if !strings.HasPrefix(event.Command, "grep . ") {
					continue
				}
				_, err := store.CommandOutputForCommandEvent(event.ID)
				if event.CaptureMode != "none" || !errors.Is(err, storage.ErrCommandOutputNotFound) {
					t.Fatal("stale/disabled history authorized output capture")
				}
				return
			}
			t.Fatal("history-unavailable metadata not recorded")
		})
	}
}

func runBash(t *testing.T, script string, interactive bool) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	args := []string{"--noprofile", "--norc"}
	if interactive {
		args = append(args, "-i")
	}
	cmd := exec.CommandContext(ctx, "bash", args...)
	// Detach the test shell from the caller's controlling terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// A stalled descriptor regression must not leave drainers behind on timeout.
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("Bash lifecycle stalled: %s", stderr.String())
	}
	if err != nil && cmd.ProcessState == nil {
		t.Fatal(err)
	}
	return stdout.Bytes(), stderr.Bytes(), cmd.ProcessState.ExitCode()
}

func assertNoTemps(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "waketrail.*"))
		if err != nil || len(matches) != 0 {
			t.Fatalf("orphan temporary directories: %v %v", matches, err)
		}
	}
}
