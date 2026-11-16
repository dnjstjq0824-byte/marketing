package store

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Encrypted login-session backup (W20-3). Each account's session files
// under data/profiles are zipped and sealed with the vault key
// (AES-256-GCM) into profiles.enc inside every backup generation.
// Restore decrypts the bundle back into data/profiles; accounts whose
// session files are still missing are marked expired so the relogin
// queue picks them up. Only the resumable session state is bundled —
// the raw Chromium user-data dirs (tens of MB of cache) are rebuilt by
// the next headed login and must never stall a backup.
// ---------------------------------------------------------------------------

// ProfilesFileName is the encrypted bundle name inside a backup
// generation and (after restore) at the data dir root.
const ProfilesFileName = "profiles.enc"

// profilesMagic versions the bundle format: magic + nonce + ciphertext.
var profilesMagic = []byte("SOMUNP1\x00")

func profilesGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("vault key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sessionFile reports whether a profile file is resumable session state:
// the Playwright storage state (what the server reads back) and the
// adapter thread continuations. Cache, locks, and raw browser dirs are
// never bundled.
func sessionFile(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(filepath.Base(rel), ".") {
		return false
	}
	for _, part := range strings.Split(filepath.Dir(rel), "/") {
		if part == "profile" || strings.HasPrefix(part, ".") {
			return false
		}
	}
	base := filepath.Base(rel)
	return base == "storage_state.json" ||
		(strings.HasPrefix(base, "thread") && strings.HasSuffix(base, ".json"))
}

// EncryptProfiles zips the session files under profilesDir and seals
// them with key. A missing profiles dir, or one with no session files,
// yields (nil, nil): there is nothing to bundle.
func EncryptProfiles(profilesDir string, key []byte) ([]byte, error) {
	st, err := os.Stat(profilesDir)
	if err != nil || !st.IsDir() {
		return nil, nil
	}
	var raw bytes.Buffer
	zw := zip.NewWriter(&raw)
	entries := 0
	walkErr := filepath.WalkDir(profilesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(profilesDir, path)
		if err != nil {
			return err
		}
		if !sessionFile(rel) {
			return nil
		}
		fw, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if _, err := io.Copy(fw, in); err != nil {
			return err
		}
		entries++
		return nil
	})
	if walkErr != nil {
		_ = zw.Close()
		return nil, fmt.Errorf("zip profiles: %w", walkErr)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip profiles: %w", err)
	}
	if entries == 0 {
		return nil, nil
	}
	gcm, err := profilesGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := append(append([]byte{}, profilesMagic...), nonce...)
	return gcm.Seal(out, nonce, raw.Bytes(), nil), nil
}

// DecryptProfiles unseals a bundle into destDir, creating it. Zip-slip
// paths (absolute, "..", drive-qualified) are rejected, never written.
func DecryptProfiles(enc []byte, key []byte, destDir string) error {
	gcm, err := profilesGCM(key)
	if err != nil {
		return err
	}
	if len(enc) < len(profilesMagic)+gcm.NonceSize()+1 || !bytes.Equal(enc[:len(profilesMagic)], profilesMagic) {
		return fmt.Errorf("not a session backup bundle")
	}
	rest := enc[len(profilesMagic):]
	nonce, cipherText := rest[:gcm.NonceSize()], rest[gcm.NonceSize():]
	raw, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return fmt.Errorf("open session bundle: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("read session bundle: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		rel := filepath.Clean(f.Name)
		if rel == "." || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") || filepath.VolumeName(rel) != "" {
			return fmt.Errorf("unsafe bundle path %q", f.Name)
		}
		target := filepath.Join(destDir, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// BackupProfilesFile seals profilesDir into the backup generation dir.
// It is a no-op returning ("", nil) when there is nothing to bundle.
func BackupProfilesFile(destDir, profilesDir string, key []byte) (string, error) {
	enc, err := EncryptProfiles(profilesDir, key)
	if err != nil {
		return "", err
	}
	if enc == nil {
		return "", nil
	}
	out := filepath.Join(destDir, ProfilesFileName)
	if err := os.WriteFile(out, enc, 0o644); err != nil {
		return "", fmt.Errorf("write session bundle: %w", err)
	}
	return out, nil
}

// BackupNowWithSessions runs the standard backup and then seals the
// login sessions into the same generation (W20-3). The bundle is also
// mirrored when mirrorDir is set. includeSessions=false skips the
// bundle; a nil/empty vault key skips it too (fresh installs).
func BackupNowWithSessions(db *sql.DB, dbPath, uploadDir, backupDir, keyPath string, keep int, mirrorDir, profilesDir string, vaultKey []byte, includeSessions bool) (string, error) {
	dest, err := BackupNow(db, dbPath, uploadDir, backupDir, keyPath, keep, mirrorDir)
	if err != nil {
		return "", err
	}
	if !includeSessions || len(vaultKey) == 0 {
		return dest, nil
	}
	bundle, err := BackupProfilesFile(dest, profilesDir, vaultKey)
	if err != nil {
		return "", err
	}
	if bundle != "" && mirrorDir != "" {
		mirrorDest := filepath.Join(mirrorDir, filepath.Base(dest), ProfilesFileName)
		if err := copyFile(bundle, mirrorDest); err != nil {
			return "", fmt.Errorf("backup mirror sessions: %w", err)
		}
	}
	return dest, nil
}

// SessionsIncluded reports the W20-3 backup option. Missing or
// unreadable settings mean included (fresh DBs back sessions up).
func SessionsIncluded(db *sql.DB) bool {
	s, err := GetAutomationSettings(db)
	if err != nil {
		return true
	}
	return s.BackupIncludeSessions
}

// profilesHaveSessions reports whether dir holds any resumable session
// file. Cache-only dirs count as missing: the bundle still has the
// sessions worth restoring.
func profilesHaveSessions(dir string) bool {
	have := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr == nil && sessionFile(rel) {
			have = true
		}
		return nil
	})
	return have
}

// RestoreProfilesIfMissing decrypts dataDir/profiles.enc back into
// dataDir/profiles when no live session file survives (the restore path
// copies the bundle file, not the sessions). It reports whether a
// restore happened. A corrupt bundle is an error — silent half-sessions
// are worse than a loud boot failure.
func RestoreProfilesIfMissing(dataDir string, key []byte) (bool, error) {
	bundle := filepath.Join(dataDir, ProfilesFileName)
	enc, err := os.ReadFile(bundle)
	if err != nil {
		return false, nil
	}
	profiles := filepath.Join(dataDir, "profiles")
	if st, err := os.Stat(profiles); err == nil && st.IsDir() && profilesHaveSessions(profiles) {
		return false, nil
	}
	if err := DecryptProfiles(enc, key, profiles); err != nil {
		return false, err
	}
	return true, nil
}
