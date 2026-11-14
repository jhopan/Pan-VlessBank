package ops

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
)

type BackupPreview struct {
	Format   string         `json:"format"`
	Accounts int            `json:"accounts"`
	Settings map[string]any `json:"settings"`
}

func PreviewBackup(raw []byte) (BackupPreview, error) {
	if len(raw) > 1<<20 {
		return BackupPreview{}, errors.New("backup terlalu besar")
	}
	z, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if e != nil {
		return BackupPreview{}, e
	}
	if len(z.File) != 1 || z.File[0].Name != "backup.json" {
		return BackupPreview{}, errors.New("format backup tidak valid")
	}
	r, e := z.File[0].Open()
	if e != nil {
		return BackupPreview{}, e
	}
	defer r.Close()
	var p BackupPreview
	if e := json.NewDecoder(r).Decode(&p); e != nil {
		return BackupPreview{}, e
	}
	if p.Format != "pan-vlessbank-redacted-backup-v1" {
		return BackupPreview{}, errors.New("format backup tidak dikenal")
	}
	return p, nil
}
func HealthChanged(previous, current map[string]any) bool {
	if previous == nil {
		return false
	}
	a, _ := json.Marshal(previous)
	b, _ := json.Marshal(current)
	return !bytes.Equal(a, b)
}
