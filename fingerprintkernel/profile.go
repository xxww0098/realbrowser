package fingerprintkernel

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"
	_ "time/tzdata"
)

const profileFile = ".browserkernel.json"

var languageTag = regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$`)

// ReadProfile reads the editable settings in an isolated profile directory.
// A profile from an older version inherits the host defaults.
func ReadProfile(dir string) (FingerprintSpec, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, nil
	}
	if err != nil {
		return FingerprintSpec{}, err
	}
	defer root.Close()
	data, err := root.ReadFile(profileFile)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, nil
	}
	if err != nil {
		return FingerprintSpec{}, err
	}
	f, err := ImportFingerprintJSON(string(data))
	if err == nil {
		err = f.validateDisplay()
	}
	return f, err
}

// WriteProfile validates and atomically replaces the editable settings. Chromium's
// profile database, cookies and the host's original identity file stay separate.
func WriteProfile(dir string, spec FingerprintSpec) error {
	if dir == "" || filepath.Clean(dir) == "." {
		return errors.New("缺少浏览器资料目录")
	}
	if err := spec.validateDisplay(); err != nil {
		return err
	}
	raw, err := ExportFingerprintJSON(spec)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeAtomic(dir, profileFile, []byte(raw+"\n"))
}

func (f FingerprintSpec) validateDisplay() error {
	if f.Lang != "" && (len(f.Lang) > 64 || !languageTag.MatchString(f.Lang)) {
		return errors.New("语言须为语言标签，例如 zh-CN")
	}
	if f.Timezone != "" {
		if _, err := time.LoadLocation(f.Timezone); err != nil || f.Timezone == "Local" {
			return errors.New("时区须为 IANA 时区，例如 Asia/Shanghai")
		}
	}
	if (f.WindowWidth == 0) != (f.WindowHeight == 0) || f.WindowWidth < 0 || f.WindowHeight < 0 || f.WindowWidth > 32768 || f.WindowHeight > 32768 {
		return errors.New("窗口宽高须同时留空，或填写 1 至 32768 之间的像素数")
	}
	return nil
}
