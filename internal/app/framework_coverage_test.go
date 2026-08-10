package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/pipeline"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

func TestFrameworkErrorProjectionPreservesRecoveryMetadata(t *testing.T) {
	next := time.Date(2026, 8, 10, 1, 2, 3, 0, time.FixedZone("test", 8*60*60))
	retry := int64(4)
	started := true
	leaf := &helpers.CLIError{Code: "UPSTREAM_CODE", Suggestion: "retry with id", Operation: "create"}
	call := &transport.CallError{Stage: transport.CallStage("decode"), HTTPStatus: 503, RPCCode: 91, TraceID: "call-trace", Cause: leaf}
	typed := &apperrors.Error{
		Category: apperrors.CategoryAPI, Message: "failed", Reason: "upstream_failed", Hint: "use status",
		Actions: []string{"dws status"}, Retryable: true, RetryableSet: true, RetryAfterSeconds: &retry,
		RPCCode: 92, RPCData: json.RawMessage(`{"task":"x"}`), Operation: "publish", ServerKey: "server",
		Origin: "gateway", FailureStage: "response", ExecutionStarted: &started, NextRetryAt: &next,
		AvailableFlags: []string{"--id"}, Snapshot: "/tmp/snapshot", Details: map[string]any{"id": "x"},
		ServerDiag: apperrors.ServerDiagnostics{TraceID: "typed-trace", ServerErrorCode: "SERVER_CODE", TechnicalDetail: "detail", FriendlyHint: "friendly", ActionURL: "https://example.test"},
		Cause:      call,
	}
	info := errorInfoFromExecutionError(typed)
	if info.Type != "api" || info.Subtype != "upstream_failed" || info.HTTPStatus != 503 || info.RPCCode != 92 || info.RequestID != "call-trace" || info.TraceID != "typed-trace" {
		t.Fatalf("projection=%+v", info)
	}
	if info.UpstreamCode != "SERVER_CODE" || info.NextRetryAt == "" || info.Cause == "" || info.RPCData == nil || info.ExecutionStarted == nil || !*info.ExecutionStarted {
		t.Fatalf("recovery metadata=%+v", info)
	}

	requestCall := &transport.CallError{Stage: transport.CallStage("request"), HTTPStatus: 429, RequestID: "request-id"}
	requestInfo := errorInfoFromExecutionError(requestCall)
	if requestInfo.RequestID != "request-id" || requestInfo.HTTPStatus != 429 {
		t.Fatalf("request projection=%+v", requestInfo)
	}
	partial := errorInfoFromExecutionError(&apperrors.Error{Category: apperrors.CategoryPartial, Message: "partial"})
	if partial.Type != "internal" {
		t.Fatalf("partial error type=%s", partial.Type)
	}
	for code, want := range map[int]string{1: "api", 2: "auth", 3: "validation", 4: "permission", 6: "discovery", 99: "internal"} {
		if got := errorTypeForExitCode(code); got != want {
			t.Fatalf("errorTypeForExitCode(%d)=%q", code, got)
		}
	}
}

func TestFrameworkExecutePreparseUnifiedErrorAndEmissionFallback(t *testing.T) {
	for _, failWriter := range []bool{false, true} {
		t.Run(map[bool]string{false: "unified", true: "fallback"}[failWriter], func(t *testing.T) {
			testseam.Protect(t, &os.Args)
			os.Args = []string{"dws", "leaf"}
			testseam.Swap(t, &rootNormalizeProcessProfileArgs, func() func() { return func() {} })
			testseam.Swap(t, &rootStopAllStdioClients, func() {})
			testseam.Swap(t, &rootRunPreParse, func(*cobra.Command, *pipeline.Engine) error { return errors.New("bad preparse") })
			var stdout bytes.Buffer
			testseam.Swap(t, &rootNewRootCommandWithEngine, func(ctx context.Context, _ *pipeline.Engine) *cobra.Command {
				root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
				root.SetContext(ctx)
				leaf := &cobra.Command{Use: "leaf"}
				output.SetCommandRollout(leaf, output.RolloutUnifiedActive)
				if failWriter {
					leaf.SetOut(frameworkFailWriter{})
				} else {
					leaf.SetOut(&stdout)
				}
				leaf.SetErr(&bytes.Buffer{})
				root.AddCommand(leaf)
				return root
			})
			if code := Execute(); code != 3 {
				t.Fatalf("Execute code=%d", code)
			}
			if !failWriter && !strings.Contains(stdout.String(), `"outcome": "failure"`) {
				t.Fatalf("stdout=%q", stdout.String())
			}
		})
	}
}

func TestFrameworkPublicRootRequiresResultFromActiveCommand(t *testing.T) {
	root := NewRootCommand(context.Background())
	leaf := &cobra.Command{Use: "active-no-result", RunE: func(*cobra.Command, []string) error { return nil }}
	output.SetCommandRollout(leaf, output.RolloutUnifiedActive)
	root.AddCommand(leaf)
	root.SetArgs([]string{"active-no-result"})
	if _, err := root.ExecuteC(); err == nil || !strings.Contains(err.Error(), "without a CommandResult") {
		t.Fatalf("ExecuteC error=%v", err)
	}
}

func TestFrameworkAbortOutputSinkRemoveFailure(t *testing.T) {
	originalRemove := rootRemoveFile
	t.Cleanup(func() { rootRemoveFile = originalRemove })
	file, err := os.CreateTemp(t.TempDir(), "abort-*")
	if err != nil {
		t.Fatal(err)
	}
	rootRemoveFile = func(string) error { return errors.New("remove failed") }
	cmd := &cobra.Command{Use: "abort"}
	cmd.SetContext(context.WithValue(context.Background(), outputFileContextKey{}, &outputSinkState{file: file, tempPath: file.Name()}))
	if err := abortOutputSink(cmd); err == nil || !strings.Contains(err.Error(), "remove temporary") {
		t.Fatalf("abort error=%v", err)
	}
}

func TestFrameworkOutputSinkHookWrappingAndCleanupEdges(t *testing.T) {
	installOutputSinkErrorCleanup(nil)
	plain := &cobra.Command{Use: "plain"}
	plain.SetContext(context.Background())
	installOutputSinkErrorCleanup(plain)

	file, err := os.CreateTemp(t.TempDir(), "sink-*")
	if err != nil {
		t.Fatal(err)
	}
	state := &outputSinkState{file: file, tempPath: file.Name(), target: filepath.Join(t.TempDir(), "out")}
	cmd := &cobra.Command{Use: "all"}
	cmd.SetContext(context.WithValue(context.Background(), outputFileContextKey{}, state))
	var calls int
	cmd.PreRunE = func(*cobra.Command, []string) error { calls++; return nil }
	cmd.PreRun = func(*cobra.Command, []string) { calls++ }
	cmd.RunE = func(*cobra.Command, []string) error { calls++; return nil }
	cmd.Run = func(*cobra.Command, []string) { calls++ }
	cmd.PostRunE = func(*cobra.Command, []string) error { calls++; return nil }
	cmd.PostRun = func(*cobra.Command, []string) { calls++ }
	installOutputSinkErrorCleanup(cmd)
	if err := cmd.PreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	cmd.PreRun(cmd, nil)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	cmd.Run(cmd, nil)
	if err := cmd.PostRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	cmd.PostRun(cmd, nil)
	if calls != 6 {
		t.Fatalf("hook calls=%d", calls)
	}

	file2, err := os.CreateTemp(t.TempDir(), "sink-error-*")
	if err != nil {
		t.Fatal(err)
	}
	errorCmd := &cobra.Command{Use: "error"}
	errorCmd.SetContext(context.WithValue(context.Background(), outputFileContextKey{}, &outputSinkState{file: file2, tempPath: file2.Name(), target: "unused"}))
	if err := runWithOutputSinkErrorCleanup(errorCmd, func() error { return errors.New("boom") }); err == nil {
		t.Fatal("run error swallowed")
	}

	file3, err := os.CreateTemp(t.TempDir(), "sink-panic-*")
	if err != nil {
		t.Fatal(err)
	}
	panicCmd := &cobra.Command{Use: "panic"}
	panicCmd.SetContext(context.WithValue(context.Background(), outputFileContextKey{}, &outputSinkState{file: file3, tempPath: file3.Name(), target: "unused"}))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic swallowed")
			}
		}()
		_ = runWithOutputSinkErrorCleanup(panicCmd, func() error { panic("boom") })
	}()

	if closeOutputSink(nil) != nil || abortOutputSink(nil) != nil || outputSinkForCommand(nil) != nil {
		t.Fatal("nil sink guards failed")
	}
	finished := &outputSinkState{finished: true, file: file}
	finishedCmd := &cobra.Command{Use: "finished"}
	finishedCmd.SetContext(context.WithValue(context.Background(), outputFileContextKey{}, finished))
	if closeOutputSink(finishedCmd) != nil || abortOutputSink(finishedCmd) != nil {
		t.Fatal("finished sink was processed twice")
	}
}

type frameworkFailWriter struct{}

func (frameworkFailWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestFrameworkExecutePanicBeforeEmissionUsesUnifiedFailure(t *testing.T) {
	for _, failWriter := range []bool{false, true} {
		t.Run(map[bool]string{false: "emits", true: "fallback"}[failWriter], func(t *testing.T) {
			testseam.Protect(t, &os.Args)
			os.Args = []string{"dws"}
			testseam.Swap(t, &rootNormalizeProcessProfileArgs, func() func() { return func() {} })
			testseam.Swap(t, &rootRunPreParse, func(*cobra.Command, *pipeline.Engine) error { return nil })
			testseam.Swap(t, &rootStopAllStdioClients, func() {})
			var stdout bytes.Buffer
			testseam.Swap(t, &rootNewRootCommandWithEngine, func(ctx context.Context, _ *pipeline.Engine) *cobra.Command {
				cmd := &cobra.Command{Use: "dws"}
				cmd.SetContext(ctx)
				output.SetCommandRollout(cmd, output.RolloutUnifiedActive)
				if failWriter {
					cmd.SetOut(frameworkFailWriter{})
				} else {
					cmd.SetOut(&stdout)
				}
				cmd.SetErr(&bytes.Buffer{})
				return cmd
			})
			testseam.Swap(t, &rootExecuteCommand, func(*cobra.Command) (*cobra.Command, error) { panic("before emission") })
			if code := Execute(); code != 5 {
				t.Fatalf("Execute code=%d", code)
			}
			if !failWriter && !strings.Contains(stdout.String(), `"outcome": "failure"`) {
				t.Fatalf("stdout=%q", stdout.String())
			}
		})
	}
}

func TestCrossPlatformCoverageFrameworkExecuteRareOutcomeBranches(t *testing.T) {
	t.Run("preparse interrupted", func(t *testing.T) {
		var stdout bytes.Buffer
		installSignalExecuteSeams(t, true, &stdout, io.Discard)
		testseam.Swap(t, &rootRunPreParse, func(cmd *cobra.Command, _ *pipeline.Engine) error {
			signalSelf(t, syscall.SIGINT)
			<-cmd.Context().Done()
			return errors.New("preparse failed")
		})
		if code := Execute(); code != 130 {
			t.Fatalf("Execute code=%d", code)
		}
	})

	t.Run("nil executed after emission attempt", func(t *testing.T) {
		installSignalExecuteSeams(t, true, io.Discard, io.Discard)
		testseam.Swap(t, &rootExecuteCommand, func(cmd *cobra.Command) (*cobra.Command, error) {
			cmd.SetOut(frameworkFailWriter{})
			if err := output.StoreResult(cmd.Context(), output.Success(map[string]any{"ok": true})); err != nil {
				t.Fatal(err)
			}
			_, _, _ = output.EmitStoredResult(cmd)
			signalSelf(t, syscall.SIGINT)
			<-cmd.Context().Done()
			return nil, cmd.Context().Err()
		})
		if code := Execute(); code != 130 {
			t.Fatalf("Execute code=%d", code)
		}
	})

	t.Run("publication failure after emission", func(t *testing.T) {
		var stdout bytes.Buffer
		installSignalExecuteSeams(t, true, &stdout, io.Discard)
		testseam.Swap(t, &rootExecuteCommand, func(cmd *cobra.Command) (*cobra.Command, error) {
			if err := output.StoreResult(cmd.Context(), output.Success(map[string]any{"ok": true})); err != nil {
				t.Fatal(err)
			}
			if _, _, err := output.EmitStoredResult(cmd); err != nil {
				t.Fatal(err)
			}
			return cmd, newOutputPublicationError("publish", errors.New("rename failed"))
		})
		if code := Execute(); code != 5 {
			t.Fatalf("Execute code=%d", code)
		}
	})

	t.Run("failure envelope cannot be written", func(t *testing.T) {
		installSignalExecuteSeams(t, true, io.Discard, io.Discard)
		testseam.Swap(t, &rootExecuteCommand, func(cmd *cobra.Command) (*cobra.Command, error) {
			cmd.SetOut(frameworkFailWriter{})
			return cmd, errors.New("business failed")
		})
		if code := Execute(); code != 5 {
			t.Fatalf("Execute code=%d", code)
		}
	})

	t.Run("late output publication warning", func(t *testing.T) {
		installSignalExecuteSeams(t, false, io.Discard, io.Discard)
		testseam.Swap(t, &rootRenameFile, func(string, string) error { return errors.New("rename failed") })
		testseam.Swap(t, &rootExecuteCommand, func(cmd *cobra.Command) (*cobra.Command, error) {
			file, err := os.CreateTemp(t.TempDir(), "late-output-*")
			if err != nil {
				t.Fatal(err)
			}
			state := &outputSinkState{file: file, tempPath: file.Name(), target: filepath.Join(t.TempDir(), "result.json")}
			cmd.SetContext(context.WithValue(cmd.Context(), outputFileContextKey{}, state))
			return cmd, nil
		})
		if code := Execute(); code != 0 {
			t.Fatalf("Execute code=%d", code)
		}
	})
}

type frameworkPanicWriter struct{}

func (frameworkPanicWriter) Write([]byte) (int, error) { panic("writer panic") }

func TestCrossPlatformCoverageFrameworkRootHookErrors(t *testing.T) {
	t.Run("edition pre-run error", func(t *testing.T) {
		old := edition.Get()
		t.Cleanup(func() { edition.Override(old) })
		edition.Override(&edition.Hooks{AfterPersistentPreRun: func(*cobra.Command, []string) error {
			return errors.New("edition hook failed")
		}})
		root := NewRootCommand(context.Background())
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"version"})
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "edition hook failed") {
			t.Fatalf("Execute error=%v", err)
		}
	})

	t.Run("post-run emission panic", func(t *testing.T) {
		root := NewRootCommand(context.Background())
		cmd := &cobra.Command{Use: "panic-output"}
		output.SetCommandRollout(cmd, output.RolloutUnifiedActive)
		ctx, _ := output.WithResultStore(context.Background())
		cmd.SetContext(ctx)
		cmd.SetOut(frameworkPanicWriter{})
		if err := output.StoreResult(ctx, output.Success(map[string]any{"ok": true})); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if recover() == nil {
				t.Fatal("expected post-run panic")
			}
		}()
		_ = root.PersistentPostRunE(cmd, nil)
	})
}
