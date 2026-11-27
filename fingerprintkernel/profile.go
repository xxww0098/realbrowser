package fingerprintkernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
)

const profileFile = ".browserkernel.json"

var languageTag = regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$`)

// ReadProfile reads the editable settings in an isolated profile directory.
// A profile from an older version inherits the host defaults.
func ReadProfile(dir string) (FingerprintSpec, error) {
	f, _, err := readProfile(dir)
	return f, err
}

func readProfile(dir string) (FingerprintSpec, []byte, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, nil, nil
	}
	if err != nil {
		return FingerprintSpec{}, nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(profileFile)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, nil, nil
	}
	if err != nil {
		return FingerprintSpec{}, nil, err
	}
	f, err := ImportFingerprintJSON(string(data))
	if err == nil {
		err = f.validateDisplay()
	}
	return f, data, err
}

// WriteProfile replaces the known settings and retains fields written by newer versions.
func WriteProfile(dir string, spec FingerprintSpec) error {
	return EditProfile(context.Background(), dir, func(f *FingerprintSpec) error {
		*f = spec
		return nil
	})
}

// EditProfile reads the current file under its directory's write lock, applies
// edit, and atomically saves it. Returning an error discards the edit.
func EditProfile(ctx context.Context, dir string, edit func(*FingerprintSpec) error) error {
	if dir == "" || filepath.Clean(dir) == "." {
		return errors.New("缺少浏览器资料目录")
	}
	if edit == nil {
		return errors.New("缺少指纹配置编辑操作")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := acquireFileLock(ctx, filepath.Join(dir, ".browserkernel.lock"), nil)
	if err != nil {
		return err
	}
	defer ReleaseInstallLock(lock)
	f, original, err := readProfile(dir)
	if err != nil {
		return err
	}
	if err := edit(&f); err != nil {
		return err
	}
	if err := f.validateDisplay(); err != nil {
		return err
	}
	raw, err := MergeFingerprintJSON(string(original), f)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return writeAtomic(dir, profileFile, append([]byte(raw), '\n'))
}

// MergeFingerprintJSON replaces the known fields in original with spec,
// including fields explicitly cleared in spec. Unknown fields at the top
// level and inside known objects are retained. An empty original creates a
// new document. Database callers should read and write in one transaction.
func MergeFingerprintJSON(original string, spec FingerprintSpec) (string, error) {
	raw, err := ExportFingerprintJSON(spec)
	if err != nil || original == "" {
		return raw, err
	}
	merged, err := retainProfileExtras([]byte(original), []byte(raw), reflect.TypeFor[FingerprintSpec]())
	return string(merged), err
}

// Use the JSON schema of the Go struct to distinguish cleared optional fields
// from extensions, including extensions nested inside a known object.
func retainProfileExtras(original, encoded []byte, schema reflect.Type) ([]byte, error) {
	var old, next map[string]json.RawMessage
	if err := json.Unmarshal(original, &old); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &next); err != nil {
		return nil, err
	}
	for i := 0; i < schema.NumField(); i++ {
		field := schema.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		for key, value := range old {
			if !strings.EqualFold(key, name) {
				continue
			}
			delete(old, key)
			if field.Type.Kind() == reflect.Struct && next[name] != nil {
				merged, err := retainProfileExtras(value, next[name], field.Type)
				if err != nil {
					return nil, err
				}
				next[name] = merged
			}
		}
	}
	for key, value := range old {
		next[key] = value
	}
	return json.MarshalIndent(next, "", "  ")
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
