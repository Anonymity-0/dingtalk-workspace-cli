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

package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/keychain"
)

func isolateAppCredentialKeychain(t *testing.T) map[string]string {
	t.Helper()
	entries := map[string]string{}
	oldSecretGet, oldSecretSet := secretKeychainGet, secretKeychainSet
	oldAuthGet, oldAuthSet, oldAuthRemove := authKeychainGet, authKeychainSet, authKeychainRemove
	secretKeychainGet = func(_, account string) (string, error) { return entries[account], nil }
	secretKeychainSet = func(_, account, value string) error { entries[account] = value; return nil }
	authKeychainGet = func(_, account string) (string, error) { return entries[account], nil }
	authKeychainSet = func(_, account, value string) error { entries[account] = value; return nil }
	authKeychainRemove = func(_, account string) error { delete(entries, account); return nil }
	t.Cleanup(func() {
		secretKeychainGet, secretKeychainSet = oldSecretGet, oldSecretSet
		authKeychainGet, authKeychainSet, authKeychainRemove = oldAuthGet, oldAuthSet, oldAuthRemove
		resetAppConfigCache()
		SetClientID("")
		SetClientSecret("")
	})
	resetAppConfigCache()
	return entries
}

func writeCredentialConfig(t *testing.T, dir, clientID string, secret SecretInput) {
	t.Helper()
	cfg := AppConfig{ClientID: clientID, ClientSecret: secret, CreatedAt: time.Now()}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(GetAppConfigPath(dir), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAppCredentialPairAtomicPriority(t *testing.T) {
	isolateAppCredentialKeychain(t)
	dir := t.TempDir()
	writeCredentialConfig(t, dir, "config-id", PlainSecret("config-secret"))

	t.Setenv(EnvClientID, "env-only-id")
	t.Setenv(EnvClientSecret, "")
	pair, err := ResolveAppCredentialPair(dir, "flag-id", "flag-secret")
	if err != nil {
		t.Fatal(err)
	}
	if pair.ClientID != "flag-id" || pair.ClientSecret != "flag-secret" || pair.Source != "flag" {
		t.Fatalf("flag pair = %#v", pair)
	}

	if _, err := ResolveAppCredentialPair(dir, "flag-only", ""); !errors.Is(err, ErrFlagCredentialPairIncomplete) {
		t.Fatalf("half flag error = %v", err)
	}
	if _, err := ResolveAppCredentialPair(dir, "", ""); !errors.Is(err, ErrEnvCredentialPairIncomplete) {
		t.Fatalf("half env error = %v", err)
	}

	t.Setenv(EnvClientID, "env-id")
	t.Setenv(EnvClientSecret, "env-secret")
	pair, err = ResolveAppCredentialPair(dir, "", "")
	if err != nil || pair.ClientID != "env-id" || pair.ClientSecret != "env-secret" || pair.Source != "env" {
		t.Fatalf("env pair = %#v, %v", pair, err)
	}
}

func TestResolveAppConfigCredentialPairMigratesDerivedSlots(t *testing.T) {
	entries := isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")

	t.Run("canonical slot fills empty config", func(t *testing.T) {
		dir := t.TempDir()
		entries[secretAccountKey("canonical-id")] = "canonical-secret"
		writeCredentialConfig(t, dir, "canonical-id", SecretInput{})
		pair, err := ResolveAppConfigCredentialPair(dir)
		if err != nil || pair.ClientSecret != "canonical-secret" {
			t.Fatalf("pair = %#v, %v", pair, err)
		}
		cfg, _ := LoadAppConfig(dir)
		if cfg.ClientSecret.Ref == nil || cfg.ClientSecret.Ref.ID != secretAccountKey("canonical-id") {
			t.Fatalf("canonical config = %#v", cfg)
		}
	})

	t.Run("legacy slot migrates", func(t *testing.T) {
		dir := t.TempDir()
		entries[legacyClientSecretAccountKey("legacy-id")] = "legacy-secret"
		writeCredentialConfig(t, dir, "legacy-id", SecretInput{})
		pair, err := ResolveAppConfigCredentialPair(dir)
		if err != nil || pair.ClientSecret != "legacy-secret" {
			t.Fatalf("pair = %#v, %v", pair, err)
		}
		if entries[secretAccountKey("legacy-id")] != "legacy-secret" || entries[legacyClientSecretAccountKey("legacy-id")] != "" {
			t.Fatalf("migrated slots = %#v", entries)
		}
		// Repeated resolution is idempotent.
		if _, err := ResolveAppConfigCredentialPair(dir); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("plain config migrates", func(t *testing.T) {
		dir := t.TempDir()
		writeCredentialConfig(t, dir, "plain-id", PlainSecret("plain-secret"))
		pair, err := ResolveAppConfigCredentialPair(dir)
		if err != nil || pair.ClientSecret != "plain-secret" {
			t.Fatalf("pair = %#v, %v", pair, err)
		}
		cfg, _ := LoadAppConfig(dir)
		if cfg.ClientSecret.Ref == nil || cfg.ClientSecret.Ref.ID != secretAccountKey("plain-id") {
			t.Fatalf("plain config was not migrated: %#v", cfg)
		}
	})
}

func TestResolveAppConfigCredentialPairFailsClosed(t *testing.T) {
	entries := isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")

	dir := t.TempDir()
	entries[secretAccountKey("conflict-id")] = "new-secret"
	entries[legacyClientSecretAccountKey("conflict-id")] = "old-secret"
	writeCredentialConfig(t, dir, "conflict-id", SecretInput{})
	if _, err := ResolveAppConfigCredentialPair(dir); !errors.Is(err, ErrClientSecretConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	dir = t.TempDir()
	entries[secretAccountKey("broken-id")] = "fallback-must-not-run"
	missing := GetAppConfigPath(dir) + ".missing"
	writeCredentialConfig(t, dir, "broken-id", SecretInput{Ref: &SecretRef{Source: "file", ID: missing}})
	_, err := ResolveAppConfigCredentialPair(dir)
	if !errors.Is(err, ErrSecretResolve) {
		t.Fatalf("broken explicit ref error = %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "fallback-must-not-run") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestExplicitFileSecretDoesNotRequireKeychain(t *testing.T) {
	isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret")
	if err := os.WriteFile(secretPath, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCredentialConfig(t, dir, "file-id", SecretInput{Ref: &SecretRef{Source: "file", ID: secretPath}})
	failure := errors.New("keychain unavailable")
	secretKeychainGet = func(string, string) (string, error) { return "", failure }
	authKeychainGet = func(string, string) (string, error) { return "", failure }
	pair, err := ResolveAppConfigCredentialPair(dir)
	if err != nil || pair.ClientSecret != "file-secret" {
		t.Fatalf("file pair = %#v, %v", pair, err)
	}
}

func TestLegacyCleanupFailureKeepsResolvedPairUsable(t *testing.T) {
	entries := isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")
	dir := t.TempDir()
	entries[legacyClientSecretAccountKey("cleanup-id")] = "cleanup-secret"
	writeCredentialConfig(t, dir, "cleanup-id", SecretInput{})
	authKeychainRemove = func(string, string) error { return errors.New("cleanup unavailable") }
	pair, err := ResolveAppConfigCredentialPair(dir)
	if err != nil || pair.ClientSecret != "cleanup-secret" {
		t.Fatalf("pair after cleanup failure = %#v, %v", pair, err)
	}
	if entries[secretAccountKey("cleanup-id")] != "cleanup-secret" {
		t.Fatal("canonical write did not complete before cleanup failure")
	}
}

func TestOAuthCredentialSnapshotPersistsFlagsAndEnvOnlyAfterSuccessHook(t *testing.T) {
	entries := isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "env-id")
	t.Setenv(EnvClientSecret, "env-secret")
	dir := t.TempDir()
	p := NewOAuthProvider(dir, nil)
	if cfg, err := LoadAppConfig(dir); err != nil || cfg != nil {
		t.Fatalf("constructor persisted config: %#v, %v", cfg, err)
	}
	p.persistAppConfigIfNeeded()
	cfg, err := LoadAppConfig(dir)
	if err != nil || cfg == nil || cfg.ClientID != "env-id" || cfg.ClientSecret.Ref == nil {
		t.Fatalf("persisted env config = %#v, %v", cfg, err)
	}
	if entries[secretAccountKey("env-id")] != "env-secret" {
		t.Fatal("env secret was not stored in the canonical slot")
	}

	SetClientID("flag-id")
	SetClientSecret("flag-secret")
	entries[legacyClientSecretAccountKey("flag-id")] = "stale-legacy-secret"
	flagDir := t.TempDir()
	flagProvider := NewOAuthProvider(flagDir, nil)
	flagProvider.persistAppConfigIfNeeded()
	flagCfg, err := LoadAppConfig(flagDir)
	if err != nil || flagCfg == nil || flagCfg.ClientID != "flag-id" || entries[secretAccountKey("flag-id")] != "flag-secret" {
		t.Fatalf("persisted flag config = %#v, %v", flagCfg, err)
	}
	if entries[legacyClientSecretAccountKey("flag-id")] != "" {
		t.Fatal("successful pair persistence did not remove the stale legacy slot")
	}
}

func TestOAuthSilentLoginDoesNotPersistUnvalidatedReplacementPair(t *testing.T) {
	isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "replacement-id")
	t.Setenv(EnvClientSecret, "replacement-secret")
	dir := t.TempDir()
	oldLoad := oauthLoadToken
	oauthLoadToken = func(string) (*TokenData, error) {
		return &TokenData{AccessToken: "still-valid", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}
	t.Cleanup(func() { oauthLoadToken = oldLoad })
	p := NewOAuthProvider(dir, nil)
	if _, err := p.Login(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadAppConfig(dir)
	if err != nil || cfg != nil {
		t.Fatalf("silent login persisted an unvalidated replacement pair: %#v, %v", cfg, err)
	}
}

func TestOAuthConstructorsDoNotMutateCredentialEnvironment(t *testing.T) {
	isolateAppCredentialKeychain(t)
	t.Setenv(EnvClientID, "")
	t.Setenv(EnvClientSecret, "")
	dir := t.TempDir()
	writeCredentialConfig(t, dir, "config-id", PlainSecret("config-secret"))
	_ = NewOAuthProvider(dir, nil)
	_ = NewDeviceFlowProvider(dir, nil)
	if os.Getenv(EnvClientID) != "" || os.Getenv(EnvClientSecret) != "" {
		t.Fatalf("constructors mutated env: id=%q secret_set=%t", os.Getenv(EnvClientID), os.Getenv(EnvClientSecret) != "")
	}
}

func TestDeleteAppConfigSweepsAllApplicationCredentialNamespaces(t *testing.T) {
	oldCleanup := appConfigRemoveCredentialEntries
	var gotService string
	var gotPrefixes []string
	appConfigRemoveCredentialEntries = func(service string, prefixes ...string) error {
		gotService = service
		gotPrefixes = append([]string(nil), prefixes...)
		return nil
	}
	t.Cleanup(func() { appConfigRemoveCredentialEntries = oldCleanup })
	if err := DeleteAppConfig(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if gotService != keychain.Service {
		t.Fatalf("cleanup service = %q", gotService)
	}
	joined := strings.Join(gotPrefixes, ",")
	for _, prefix := range []string{secretKeyPrefix, clientSecretPrefix, appTokenPrefix} {
		if !strings.Contains(joined, prefix) {
			t.Fatalf("cleanup prefixes = %q; missing %q", joined, prefix)
		}
	}
}
