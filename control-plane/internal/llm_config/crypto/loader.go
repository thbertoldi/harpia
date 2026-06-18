package crypto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// KeyringConfig describes how to assemble a Keyring at startup.
//
// Two parallel sources are supported (so dev and prod-k8s both work):
//
//   - Env-based: HARPIA_LLM_KEK_<VERSION>_B64=<base64> for each version,
//     plus HARPIA_LLM_KEK_ACTIVE=<version>. Useful for compose/dev/tests.
//   - Mounted-secret-based: a directory (e.g. /var/run/secrets/aiuna-llm-kek)
//     where each file is named after its version (e.g. `v1`, `v2`) and
//     contains either raw bytes or base64-encoded bytes for that version.
//     The active version is read from `/active` inside that directory, or
//     overridden by HARPIA_LLM_KEK_ACTIVE.
//
// In dev, if no source is present the loader will not synthesize a key
// silently: callers must opt in via the dev fallback in cmd/api/main.go.
type KeyringConfig struct {
	EnvPrefix         string // default "HARPIA_LLM_KEK_"
	ActiveEnv         string // default "HARPIA_LLM_KEK_ACTIVE"
	MountDir          string // optional k8s Secret mount path
	ActiveVersionFile string // default "active" inside MountDir
}

// DefaultKeyringConfig returns the canonical configuration.
func DefaultKeyringConfig() KeyringConfig {
	return KeyringConfig{
		EnvPrefix:         "HARPIA_LLM_KEK_",
		ActiveEnv:         "HARPIA_LLM_KEK_ACTIVE",
		MountDir:          "",
		ActiveVersionFile: "active",
	}
}

// LoadKeyring assembles a Keyring from environment variables and/or the
// configured mount directory. Mount-directory entries win over env entries
// for the same version label, so prod k8s secret material always overrides
// any leftover dev env.
func LoadKeyring(cfg KeyringConfig) (*Keyring, error) {
	if cfg.EnvPrefix == "" {
		cfg.EnvPrefix = "HARPIA_LLM_KEK_"
	}
	if cfg.ActiveEnv == "" {
		cfg.ActiveEnv = "HARPIA_LLM_KEK_ACTIVE"
	}
	if cfg.ActiveVersionFile == "" {
		cfg.ActiveVersionFile = "active"
	}

	materials, active, err := readMountDir(cfg)
	if err != nil {
		return nil, err
	}

	envMaterials := readEnvKEKs(cfg)
	for _, m := range envMaterials {
		if _, ok := indexMaterial(materials, m.Version); !ok {
			materials = append(materials, m)
		}
	}

	if envActive := strings.TrimSpace(os.Getenv(cfg.ActiveEnv)); envActive != "" {
		active = envActive
	}

	if len(materials) == 0 {
		return nil, errors.New("no KEK material configured (set HARPIA_LLM_KEK_<VERSION>_B64 or mount aiuna-llm-kek secret)")
	}
	if active == "" {
		// Fall back to the lexicographically highest version when active is
		// unspecified — a safe default that picks the newest "vN" entry.
		sort.Slice(materials, func(i, j int) bool { return materials[i].Version < materials[j].Version })
		active = materials[len(materials)-1].Version
	}
	return NewKeyring(materials, active)
}

func indexMaterial(ms []KEKMaterial, version string) (int, bool) {
	for i, m := range ms {
		if m.Version == version {
			return i, true
		}
	}
	return 0, false
}

func readEnvKEKs(cfg KeyringConfig) []KEKMaterial {
	var out []KEKMaterial
	envs := os.Environ()
	for _, kv := range envs {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if !strings.HasPrefix(key, cfg.EnvPrefix) {
			continue
		}
		suffix := key[len(cfg.EnvPrefix):]
		switch {
		case suffix == "ACTIVE":
			continue
		case strings.HasSuffix(suffix, "_B64"):
			version := strings.TrimSuffix(suffix, "_B64")
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(val))
			if err != nil {
				continue
			}
			version = strings.ToLower(version)
			out = append(out, KEKMaterial{Version: version, Key: raw})
		}
	}
	return out
}

func readMountDir(cfg KeyringConfig) ([]KEKMaterial, string, error) {
	if cfg.MountDir == "" {
		return nil, "", nil
	}
	entries, err := os.ReadDir(cfg.MountDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read KEK mount %q: %w", cfg.MountDir, err)
	}
	var materials []KEKMaterial
	active := ""
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "..") {
			continue
		}
		path := filepath.Join(cfg.MountDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("read KEK file %q: %w", path, err)
		}
		if e.Name() == cfg.ActiveVersionFile {
			active = strings.TrimSpace(string(data))
			continue
		}
		decoded, ok := tryBase64(data)
		if !ok {
			decoded = data
		}
		materials = append(materials, KEKMaterial{Version: e.Name(), Key: decoded})
	}
	return materials, active, nil
}

func tryBase64(b []byte) ([]byte, bool) {
	s := strings.TrimSpace(string(b))
	if len(s)%4 != 0 {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	return decoded, true
}
