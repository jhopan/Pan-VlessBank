package web

// login_store.go: penulisan webui.login untuk cabang "password kosong" di /pass.

import (
	"encoding/json"
	"os"

	"vlessbank/internal/settings"
)

// saveLoginKeep menulis webui.login dengan user baru (salt+hash lama dipertahankan),
// sama dengan json.dump(d, open(LOGINF,"w"), indent=2) di webui.py /pass.
func saveLoginKeep(l settings.Login, path string) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
