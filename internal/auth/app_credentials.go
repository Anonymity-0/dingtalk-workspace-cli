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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/keychain"
)

// AppCredentialPair is an atomic application credential. ClientID and
// ClientSecret are always selected from the same source.
type AppCredentialPair struct {
	ClientID     string
	ClientSecret string
	Source       string // flag | env | app_config
}

const CredentialSourceFlag CredentialSource = "flag"

var (
	ErrFlagCredentialPairIncomplete = errors.New("--client-id and --client-secret must be provided together")
	ErrEnvCredentialPairIncomplete  = errors.New("DWS_CLIENT_ID and DWS_CLIENT_SECRET must be set together")
	ErrClientSecretConflict         = errors.New("canonical and legacy Client Secret slots conflict; log in again")
)

// ResolveAppCredentialPair resolves one complete pair without mixing sources.
// A caller-provided temporary App Token must be handled before calling this
// function so it never touches env, app config, or keychain.
func ResolveAppCredentialPair(configDir, flagClientID, flagClientSecret string) (AppCredentialPair, error) {
	if pair, selected, err := credentialPairFromValues(flagClientID, flagClientSecret, string(CredentialSourceFlag), ErrFlagCredentialPairIncomplete); selected || err != nil {
		return pair, err
	}
	if pair, selected, err := credentialPairFromValues(os.Getenv(EnvClientID), os.Getenv(EnvClientSecret), string(CredentialSourceEnv), ErrEnvCredentialPairIncomplete); selected || err != nil {
		return pair, err
	}
	return ResolveAppConfigCredentialPair(configDir)
}

// ResolveAppConfigCredentialPair resolves only app.json and its corresponding
// secret. It is exported so OAuth can distinguish "no custom application" from
// an explicit, damaged custom-application configuration before falling back to
// managed MCP credentials.
func ResolveAppConfigCredentialPair(configDir string) (AppCredentialPair, error) {
	id, secret, _, _, err := resolveAppConfigCredentials(configDir)
	if err != nil {
		return AppCredentialPair{}, err
	}
	return validateAppCredentialPair(id, secret, string(CredentialSourceAppConfig))
}

func credentialPairFromValues(clientID, clientSecret, source string, incompleteErr error) (AppCredentialPair, bool, error) {
	idSet := strings.TrimSpace(clientID) != ""
	secretSet := strings.TrimSpace(clientSecret) != ""
	if idSet != secretSet {
		return AppCredentialPair{}, false, incompleteErr
	}
	if !idSet {
		return AppCredentialPair{}, false, nil
	}
	pair, err := validateAppCredentialPair(clientID, clientSecret, source)
	return pair, true, err
}

func validateAppCredentialPair(clientID, clientSecret, source string) (AppCredentialPair, error) {
	id := strings.TrimSpace(clientID)
	secret := strings.TrimSpace(clientSecret)
	if id == "" || secret == "" || strings.HasPrefix(id, "<") || strings.HasPrefix(secret, "<") {
		return AppCredentialPair{}, fmt.Errorf("%s credentials are incomplete or contain placeholders", source)
	}
	return AppCredentialPair{ClientID: id, ClientSecret: clientSecret, Source: source}, nil
}

func resolveAppConfigCredentials(configDir string) (
	clientID, secret string,
	clientIDSource, secretSource CredentialSource,
	err error,
) {
	cfg, err := LoadAppConfig(configDir)
	if err != nil {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("load app config: %w", err)
	}
	if cfg == nil {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrAppConfigMissing
	}
	clientID = strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrClientIDEmpty
	}
	clientIDSource = CredentialSourceAppConfig

	// An explicit value is authoritative. If it is damaged, do not silently
	// fall back to a derived keychain slot and hide the broken app.json.
	if !cfg.ClientSecret.IsZero() {
		wasPlain := cfg.ClientSecret.IsPlain()
		resolved, resolveErr := ResolveSecret(cfg.ClientSecret)
		if resolveErr != nil {
			return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("%w: explicit app config secret cannot be resolved", ErrSecretResolve)
		}
		if strings.TrimSpace(resolved) == "" {
			return "", "", clientIDSource, CredentialSourceUnknown, ErrClientSecretEmpty
		}

		secretSource = CredentialSourcePlainConfig
		if cfg.ClientSecret.Ref != nil && cfg.ClientSecret.Ref.Source == "keychain" {
			secretSource = CredentialSourceKeychain
		}
		switch {
		case wasPlain:
			// Plaintext remains usable if keychain is unavailable; migration is
			// best-effort. When both historical slots can be read, still refuse an
			// existing disagreement before replacing either value.
			canonical, canonicalErr, legacy, legacyErr := readDerivedSecretSlots(clientID)
			if canonicalErr == nil && legacyErr == nil && canonical != "" && legacy != "" && canonical != legacy {
				return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrClientSecretConflict
			}
			migrateAppConfigSecret(configDir, cfg, resolved, legacy != "")
		case isCanonicalClientSecretRef(cfg.ClientSecret, clientID):
			legacy, legacyErr := authKeychainGet(keychain.Service, legacyClientSecretAccountKey(clientID))
			if legacyErr != nil {
				return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("%w: legacy secret slot unavailable", ErrSecretResolve)
			}
			if legacy != "" && legacy != resolved {
				return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrClientSecretConflict
			}
			if legacy != "" {
				migrateAppConfigSecret(configDir, cfg, resolved, true)
			}
		case isLegacyClientSecretRef(cfg.ClientSecret, clientID):
			canonical, canonicalErr := secretKeychainGet(keychain.Service, secretAccountKey(clientID))
			if canonicalErr != nil {
				return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("%w: canonical secret slot unavailable", ErrSecretResolve)
			}
			if canonical != "" && canonical != resolved {
				return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrClientSecretConflict
			}
			migrateAppConfigSecret(configDir, cfg, resolved, true)
		}
		return clientID, resolved, clientIDSource, secretSource, nil
	}

	canonical, canonicalErr, legacy, legacyErr := readDerivedSecretSlots(clientID)
	if canonicalErr != nil {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("%w: canonical secret slot unavailable", ErrSecretResolve)
	}
	if legacyErr != nil {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, fmt.Errorf("%w: legacy secret slot unavailable", ErrSecretResolve)
	}
	if canonical != "" && legacy != "" && canonical != legacy {
		return "", "", CredentialSourceUnknown, CredentialSourceUnknown, ErrClientSecretConflict
	}

	switch {
	case canonical != "":
		secret = canonical
		migrateAppConfigSecret(configDir, cfg, canonical, legacy != "")
	case legacy != "":
		secret = legacy
		migrateAppConfigSecret(configDir, cfg, legacy, true)
	default:
		return "", "", clientIDSource, CredentialSourceUnknown, ErrClientSecretEmpty
	}
	return clientID, secret, clientIDSource, CredentialSourceKeychain, nil
}

func readDerivedSecretSlots(clientID string) (canonical string, canonicalErr error, legacy string, legacyErr error) {
	canonical, canonicalErr = secretKeychainGet(keychain.Service, secretAccountKey(clientID))
	legacy, legacyErr = authKeychainGet(keychain.Service, legacyClientSecretAccountKey(clientID))
	return canonical, canonicalErr, legacy, legacyErr
}

func isCanonicalClientSecretRef(input SecretInput, clientID string) bool {
	return input.Ref != nil && input.Ref.Source == "keychain" && input.Ref.ID == secretAccountKey(clientID)
}

func isLegacyClientSecretRef(input SecretInput, clientID string) bool {
	return input.Ref != nil && input.Ref.Source == "keychain" && input.Ref.ID == legacyClientSecretAccountKey(clientID)
}

// migrateAppConfigSecret deliberately keeps a resolved credential usable when
// best-effort cleanup fails. The warning contains identifiers and error causes,
// never the secret value.
func migrateAppConfigSecret(configDir string, cfg *AppConfig, secret string, removeLegacy bool) {
	if cfg == nil || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(secret) == "" {
		return
	}
	if err := secretKeychainSet(keychain.Service, secretAccountKey(cfg.ClientID), secret); err != nil {
		slog.Warn("auth: failed to migrate Client Secret to canonical slot", "client_id", cfg.ClientID, "error", err)
		return
	}
	updated := *cfg
	updated.ClientSecret = SecretInput{Ref: &SecretRef{Source: "keychain", ID: secretAccountKey(cfg.ClientID)}}
	if err := SaveAppConfig(configDir, &updated); err != nil {
		slog.Warn("auth: failed to update app config after Client Secret migration", "client_id", cfg.ClientID, "error", err)
		return
	}
	if removeLegacy {
		if err := authKeychainRemove(keychain.Service, legacyClientSecretAccountKey(cfg.ClientID)); err != nil {
			slog.Warn("auth: failed to remove legacy Client Secret slot", "client_id", cfg.ClientID, "error", err)
		}
	}
}
