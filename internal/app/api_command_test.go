// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/apiclient"
	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestParseQueryStringToJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, raw, want string
	}{
		{
			name: "simple key-value",
			raw:  "timeMin=2026-04-01&maxResults=10",
			want: `{"maxResults":"10","timeMin":"2026-04-01"}`,
		},
		{
			name: "with special chars",
			raw:  "timeMin=2026-04-01T14:00:00+08:00&showDeleted=false",
			want: `{"showDeleted":"false","timeMin":"2026-04-01T14:00:00+08:00"}`,
		},
		{
			name: "empty value skipped",
			raw:  "nextToken=&syncToken=abc",
			want: `{"syncToken":"abc"}`,
		},
		{
			name: "all empty",
			raw:  "nextToken=&syncToken=",
			want: "{}",
		},
		{
			name: "empty string",
			raw:  "",
			want: "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseQueryStringToJSON(tt.raw)
			if got != tt.want {
				t.Errorf("parseQueryStringToJSON(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAPIHelpUsesAppTokenCompatibleExamples(t *testing.T) {
	t.Parallel()

	help := newAPICommand(&GlobalFlags{}).Long
	for _, forbidden := range []string{
		"/v1.0/contact/users/me",
		"/v1.0/calendar/users/me",
	} {
		if strings.Contains(help, forbidden) {
			t.Errorf("API help must not advertise user-token example %q", forbidden)
		}
	}
	for _, required := range []string{
		"dws api GET /v1.0/microApp/allApps",
		"dws api POST /v1.0/contact/users/search",
		"dws api GET /v1.0/microApp/allApps --dry-run",
		"dws api GET /v1.0/microApp/allApps --jq '.appList | length'",
	} {
		if !strings.Contains(help, required) {
			t.Errorf("API help missing App Token-compatible example %q", required)
		}
	}
}

func TestRunAPI_QueryStringBlocked(t *testing.T) {
	gf := &GlobalFlags{}
	cmd := newAPICommand(gf)

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	cmd.SetArgs([]string{"GET", "/v1.0/calendar/users/me/events?timeMin=2026-04-01&maxResults=10"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error when path contains query string, got nil")
	}
	errMsg := stderr.String()
	if !strings.Contains(errMsg, "--params") {
		t.Errorf("expected --params hint in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "maxResults") {
		t.Errorf("expected parsed query params in error, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "/v1.0/calendar/users/me/events") {
		t.Errorf("expected clean path in suggestion, got: %s", errMsg)
	}
}

func TestRunAPI_NoErrorWithoutQueryString(t *testing.T) {
	gf := &GlobalFlags{}
	cmd := newAPICommand(gf)

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&bytes.Buffer{})

	cmd.SetArgs([]string{"GET", "/v1.0/microApp/allApps"})
	err := cmd.Execute()

	errMsg := stderr.String()
	if strings.Contains(errMsg, "查询参数") {
		t.Errorf("should not reject path without query string, got: %s", errMsg)
	}
	_ = err
}

type failingAppTokenGetter struct {
	called *bool
}

func (g failingAppTokenGetter) GetToken(context.Context) (string, error) {
	*g.called = true
	return "", errors.New("token provider must not run")
}

func TestRunAPIDryRunHasZeroCredentialFileAndNetworkSideEffects(t *testing.T) {
	oldProvider := newAppTokenProvider
	oldResolver := resolveRawAPICredentials
	t.Cleanup(func() {
		newAppTokenProvider = oldProvider
		resolveRawAPICredentials = oldResolver
	})
	resolverCalled := false
	resolveRawAPICredentials = func(string, string, string) (rawAPICredentials, error) {
		resolverCalled = true
		return rawAPICredentials{}, errors.New("credential resolver must not run")
	}
	called := false
	newAppTokenProvider = func(_, _, _ string) appTokenGetter {
		return failingAppTokenGetter{called: &called}
	}

	gf := &GlobalFlags{DryRun: true, Token: "must-not-be-shown"}
	cmd := newAPICommand(gf)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"POST", "/v1.0/example/upload",
		"--params", "@/definitely/missing/params.json",
		"--data", "@/definitely/missing/body.json",
		"--file", "media=/definitely/missing/upload.bin",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should not open deferred inputs: %v\nstderr=%s", err, stderr.String())
	}
	if called {
		t.Fatal("dry-run called AppTokenProvider")
	}
	if resolverCalled {
		t.Fatal("dry-run resolved app credentials")
	}
	got := stdout.String()
	if strings.Contains(got, "must-not-be-shown") || !strings.Contains(got, "not opened") || !strings.Contains(got, "Auth:") {
		t.Fatalf("dry-run preview = %q", got)
	}
}

func TestAPIFileFlagCompatibilityAndValidation(t *testing.T) {
	gf := &GlobalFlags{DryRun: true}
	cmd := newAPICommand(gf)
	flag := cmd.Flags().Lookup("file")
	if flag == nil || flag.DefValue != "" || flag.Hidden {
		t.Fatalf("--file flag = %#v", flag)
	}

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"GET", "/v1.0/test", "--file", "demo.bin"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "GET") {
		t.Fatalf("GET --file error = %v", err)
	}

	cmd = newAPICommand(gf)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"POST", "/v1.0/test", "--params", "-", "--file", "-"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Fatalf("stdin conflict error = %v", err)
	}

	for _, tc := range []struct {
		name string
		gf   *GlobalFlags
		args []string
		want string
	}{
		{"output", &GlobalFlags{DryRun: true, Output: "out.bin"}, []string{"POST", "/v1.0/test", "--file", "demo.bin"}, "--output"},
		{"pagination", &GlobalFlags{DryRun: true}, []string{"POST", "/v1.0/test", "--file", "demo.bin", "--page-all"}, "--page-all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newAPICommand(tc.gf)
			command.SetOut(&bytes.Buffer{})
			command.SetErr(&bytes.Buffer{})
			command.SetArgs(tc.args)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("file exclusion error = %v", err)
			}
		})
	}
}

func TestResolveRawAPIExplicitTokenIsTemporaryAppToken(t *testing.T) {
	oldResolver := resolveRawAPICredentials
	oldProvider := newAppTokenProvider
	t.Cleanup(func() {
		resolveRawAPICredentials = oldResolver
		newAppTokenProvider = oldProvider
	})
	resolveRawAPICredentials = func(string, string, string) (rawAPICredentials, error) {
		t.Fatal("explicit token resolved AppKey/AppSecret")
		return rawAPICredentials{}, nil
	}
	newAppTokenProvider = func(string, string, string) appTokenGetter {
		t.Fatal("explicit token created AppTokenProvider")
		return nil
	}

	got, err := resolveRawAPIToken(context.Background(), " temporary-app-token ", "flag-id", "flag-secret")
	if err != nil || got != "temporary-app-token" {
		t.Fatalf("explicit App Token = %q, %v", got, err)
	}
}

func TestResolveRawAPICredentialsUsesAtomicSourcePairs(t *testing.T) {
	tests := []struct {
		name          string
		flagID        string
		flagSecret    string
		envID         string
		envSecret     string
		configID      string
		configSecret  any
		wantID        string
		wantSecret    string
		wantSource    string
		wantErr       string
		withoutConfig bool
	}{
		{
			name:   "complete flags override half env and config",
			flagID: "flag-id", flagSecret: "flag-secret",
			envSecret: "env-secret", configID: "config-id", configSecret: "config-secret",
			wantID: "flag-id", wantSecret: "flag-secret", wantSource: "flag",
		},
		{
			name:   "half flag pair fails before complete env",
			flagID: "flag-id", envID: "env-id", envSecret: "env-secret",
			wantErr: "--client-id 和 --client-secret 必须同时提供",
		},
		{
			name:       "half flag secret fails before complete env",
			flagSecret: "flag-secret", envID: "env-id", envSecret: "env-secret",
			wantErr: "--client-id 和 --client-secret 必须同时提供",
		},
		{
			name:  "complete env overrides config",
			envID: "env-id", envSecret: "env-secret", configID: "config-id", configSecret: "config-secret",
			wantID: "env-id", wantSecret: "env-secret", wantSource: "env",
		},
		{
			name:      "half env does not mix with complete config",
			envSecret: "env-secret", configID: "config-id", configSecret: "config-secret",
			wantErr: "DWS_CLIENT_ID 和 DWS_CLIENT_SECRET 必须同时设置",
		},
		{
			name:         "half env id does not mix with complete config",
			envID:        "env-id",
			configID:     "config-id",
			configSecret: "config-secret",
			wantErr:      "DWS_CLIENT_ID 和 DWS_CLIENT_SECRET 必须同时设置",
		},
		{
			name:      "legacy mixed pair reproduction fails",
			envSecret: "new-secret", configID: "old-client-id", configSecret: "",
			wantErr: "DWS_CLIENT_ID 和 DWS_CLIENT_SECRET 必须同时设置",
		},
		{
			name:     "complete plain app config",
			configID: "config-id", configSecret: "config-secret",
			wantID: "config-id", wantSecret: "config-secret", wantSource: "app_config",
		},
		{
			name:     "app config missing secret",
			configID: "missing-secret-id", configSecret: "",
			wantErr: "本地应用配置不完整",
		},
		{
			name:         "app config missing client id",
			configSecret: "config-secret",
			wantErr:      "本地应用配置不完整",
		},
		{
			name:          "missing all credential sources",
			withoutConfig: true,
			wantErr:       "缺少应用凭证",
		},
		{
			name:   "placeholder flag pair rejected",
			flagID: "<APP_KEY>", flagSecret: "<APP_SECRET>",
			wantErr: "占位符",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(authpkg.EnvClientID, tt.envID)
			t.Setenv(authpkg.EnvClientSecret, tt.envSecret)
			configDir := t.TempDir()
			if !tt.withoutConfig {
				writeRawAPIAppConfig(t, configDir, tt.configID, tt.configSecret)
			}

			got, err := resolveRawAPICredentialsFromSources(tt.flagID, tt.flagSecret, configDir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				for _, secret := range []string{tt.flagSecret, tt.envSecret, secretString(tt.configSecret)} {
					if secret != "" && strings.Contains(err.Error(), secret) {
						t.Fatalf("error leaked secret %q: %v", secret, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ClientID != tt.wantID || got.ClientSecret != tt.wantSecret || got.Source != tt.wantSource {
				t.Fatalf("credentials = %#v, want id=%q source=%q", got, tt.wantID, tt.wantSource)
			}
		})
	}
}

func TestResolveRawAPICredentialsSupportsFileSecretRef(t *testing.T) {
	t.Setenv(authpkg.EnvClientID, "")
	t.Setenv(authpkg.EnvClientSecret, "")
	configDir := t.TempDir()
	secretPath := filepath.Join(configDir, "app-secret")
	if err := os.WriteFile(secretPath, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeRawAPIAppConfig(t, configDir, "config-id", map[string]any{"source": "file", "id": secretPath})

	got, err := resolveRawAPICredentialsFromSources("", "", configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != "config-id" || got.ClientSecret != "file-secret" || got.Source != "app_config" {
		t.Fatalf("file-ref credentials = %#v", got)
	}
}

func TestResolveRawAPICredentialsRejectsUnreadableSecretRef(t *testing.T) {
	t.Setenv(authpkg.EnvClientID, "")
	t.Setenv(authpkg.EnvClientSecret, "")
	configDir := t.TempDir()
	missingSecretPath := filepath.Join(configDir, "missing-secret")
	writeRawAPIAppConfig(t, configDir, "config-id", map[string]any{"source": "file", "id": missingSecretPath})

	_, err := resolveRawAPICredentialsFromSources("", "", configDir)
	if err == nil || !strings.Contains(err.Error(), "无法从 Keychain 解析 Client Secret") {
		t.Fatalf("unreadable SecretRef error = %v", err)
	}
	if strings.Contains(err.Error(), missingSecretPath) {
		t.Fatalf("unreadable SecretRef error leaked backend details: %v", err)
	}
}

func TestRawAPICommandUsesAppConfigPairEndToEnd(t *testing.T) {
	oldResolver := resolveRawAPICredentials
	oldProvider := newAppTokenProvider
	oldClient := newRawAPIClient
	t.Cleanup(func() {
		resolveRawAPICredentials = oldResolver
		newAppTokenProvider = oldProvider
		newRawAPIClient = oldClient
	})
	resolveRawAPICredentials = resolveRawAPICredentialsFromSources
	t.Setenv(authpkg.EnvClientID, "")
	t.Setenv(authpkg.EnvClientSecret, "")
	dir := t.TempDir()
	t.Setenv("DWS_CONFIG_DIR", dir)
	secretPath := filepath.Join(dir, "client-secret")
	if err := os.WriteFile(secretPath, []byte("paired-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeRawAPIAppConfig(t, dir, "paired-client", map[string]any{"source": "file", "id": secretPath})

	newAppTokenProvider = func(configDir, clientID, clientSecret string) appTokenGetter {
		if configDir != dir || clientID != "paired-client" || clientSecret != "paired-secret" {
			t.Fatalf("provider credentials: dir=%q id=%q secret_matches=%t", configDir, clientID, clientSecret == "paired-secret")
		}
		return fakeAppTokenGetter{token: "temporary-app-token"}
	}
	newRawAPIClient = func(token, baseURL string) *apiclient.APIClient {
		if token != "temporary-app-token" || baseURL != "" {
			t.Fatalf("raw client inputs: token_matches=%t base=%q", token == "temporary-app-token", baseURL)
		}
		client := apiclient.NewClient(token, baseURL)
		client.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("x-acs-dingtalk-access-token") != "temporary-app-token" {
				t.Fatal("App Token was not injected into the new OpenAPI header")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"count":1}`)),
				Request:    req,
			}, nil
		})
		return client
	}

	flags := &GlobalFlags{Format: "json"}
	cmd := newAPICommand(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"GET", "/v1.0/microApp/allApps"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"count": 1`) {
		t.Fatalf("raw response = %q", out.String())
	}
}

func TestResolveRawAPITokenPassesOneResolvedPairToProvider(t *testing.T) {
	oldResolver := resolveRawAPICredentials
	oldProvider := newAppTokenProvider
	t.Cleanup(func() {
		resolveRawAPICredentials = oldResolver
		newAppTokenProvider = oldProvider
	})
	resolveRawAPICredentials = func(flagID, flagSecret, _ string) (rawAPICredentials, error) {
		if flagID != "flag-id" || flagSecret != "flag-secret" {
			t.Fatalf("resolver flags = %q/%q", flagID, flagSecret)
		}
		return rawAPICredentials{ClientID: "resolved-id", ClientSecret: "resolved-secret", Source: "env"}, nil
	}
	newAppTokenProvider = func(_ string, clientID, clientSecret string) appTokenGetter {
		if clientID != "resolved-id" || clientSecret != "resolved-secret" {
			t.Fatalf("provider pair = %q/%q", clientID, clientSecret)
		}
		return fakeAppTokenGetter{token: " app-token "}
	}

	got, err := resolveRawAPIToken(context.Background(), "", "flag-id", "flag-secret")
	if err != nil || got != "app-token" {
		t.Fatalf("app token = %q, %v", got, err)
	}
}

func TestResolveRawAPITokenRejectsHalfPairsBeforeProvider(t *testing.T) {
	oldResolver := resolveRawAPICredentials
	oldProvider := newAppTokenProvider
	t.Cleanup(func() {
		resolveRawAPICredentials = oldResolver
		newAppTokenProvider = oldProvider
	})
	resolveRawAPICredentials = resolveRawAPICredentialsFromSources
	newAppTokenProvider = func(string, string, string) appTokenGetter {
		t.Fatal("half credential pair created AppTokenProvider")
		return nil
	}

	t.Run("flags", func(t *testing.T) {
		t.Setenv(authpkg.EnvClientID, "env-id")
		t.Setenv(authpkg.EnvClientSecret, "env-secret")
		if _, err := resolveRawAPIToken(context.Background(), "", "flag-id", ""); err == nil || !strings.Contains(err.Error(), "必须同时提供") {
			t.Fatalf("half flag pair error = %v", err)
		}
	})

	t.Run("environment", func(t *testing.T) {
		t.Setenv(authpkg.EnvClientID, "env-id")
		t.Setenv(authpkg.EnvClientSecret, "")
		if _, err := resolveRawAPIToken(context.Background(), "", "", ""); err == nil || !strings.Contains(err.Error(), "必须同时设置") {
			t.Fatalf("half env pair error = %v", err)
		}
	})
}

func writeRawAPIAppConfig(t *testing.T, configDir, clientID string, clientSecret any) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"clientId":     clientID,
		"clientSecret": clientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "app.json"), payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func secretString(value any) string {
	secret, _ := value.(string)
	return secret
}

type apiRoundTripper func(*http.Request) (*http.Response, error)

func (f apiRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRunPaginatedPreservesPagePayloadArray(t *testing.T) {
	client := apiclient.NewClient("app-token", "")
	page := 0
	client.HTTPClient.Transport = apiRoundTripper(func(*http.Request) (*http.Response, error) {
		page++
		body := `{"items":[{"id":"2"}],"has_more":false}`
		if page == 1 {
			body = `{"items":[{"id":"1"}],"has_more":true,"next_token":"page-2"}`
		}
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	var out bytes.Buffer
	err := runPaginated(context.Background(), client, apiclient.RawAPIRequest{
		Method: http.MethodGet,
		Path:   "/v1.0/example/resources",
	}, &apiFlags{pageLimit: 10, pageDelay: 1}, apiclient.ResponseOptions{
		Format: output.FormatJSON,
		Out:    &out,
		ErrOut: io.Discard,
	})
	if err != nil {
		t.Fatal(err)
	}
	var pages []map[string]any
	if err := json.Unmarshal(out.Bytes(), &pages); err != nil {
		t.Fatalf("page array output = %s: %v", out.String(), err)
	}
	if len(pages) != 2 || pages[0]["next_token"] != "page-2" {
		t.Fatalf("page payload shape changed: %#v", pages)
	}
}
