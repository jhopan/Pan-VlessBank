package ops

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestPreviewRedactedBackupReturnsAllowedSettings(t *testing.T) {
	b, e := RedactedBackup(t.TempDir(), 2)
	if e != nil {
		t.Fatal(e)
	}
	p, e := PreviewBackup(b)
	if e != nil {
		t.Fatal(e)
	}
	if p.Accounts != 2 {
		t.Fatalf("accounts=%d", p.Accounts)
	}
}
func TestPreviewBackupRejectsUnknownArchive(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("backup.json")
	w.Write([]byte(`{"format":"unknown"}`))
	z.Close()
	if _, e := PreviewBackup(b.Bytes()); e == nil {
		t.Fatal("unknown backup accepted")
	}
}
