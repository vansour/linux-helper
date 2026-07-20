package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const BackupRoot = "/etc/linux-helper/backups"

// BackupFile copies a file to the backup directory with a timestamp.
// Returns the backup path.
func BackupFile(src string) (string, error) {
	if err := os.MkdirAll(BackupRoot, 0755); err != nil {
		return "", err
	}

	ts := time.Now().Format("20060102-150405")
	name := filepath.Base(src)
	dst := filepath.Join(BackupRoot, fmt.Sprintf("%s.bak.%s", name, ts))

	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return dst, nil
}

// BackupToDir copies files into a named subdirectory under backup root.
// Returns the directory path.
func BackupToDir(dirname string, files map[string]string) (string, error) {
	ts := time.Now().Format("20060102-150405")
	dir := filepath.Join(BackupRoot, fmt.Sprintf("%s-%s", dirname, ts))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	for src, dstName := range files {
		in, err := os.Open(src)
		if err != nil {
			continue
		}

		dst := filepath.Join(dir, dstName)
		out, err := os.Create(dst)
		if err != nil {
			in.Close()
			continue
		}
		io.Copy(out, in)
		in.Close()
		out.Close()
	}
	return dir, nil
}

// BackupRestore restores a previously backed up file to its original location.
func BackupRestore(backupPath, originalPath string) error {
	in, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(originalPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
