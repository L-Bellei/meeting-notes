package database

import "testing"

func TestMigration020_RewritesCudaToGpu(t *testing.T) {
	db, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// No Open a 019 semeia 'auto' e a 020 roda sobre isso; para testar a
	// conversão é preciso recriar o estado da v2.9.0 e reexecutar a 020 —
	// ela é um UPDATE idempotente, então isso é legítimo.
	if _, err := db.Exec(`UPDATE settings SET value = 'cuda' WHERE key = 'whisper_device'`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stmt, err := migrationsFS.ReadFile("migrations/020_whisper_device_gpu.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := db.Exec(string(stmt)); err != nil {
		t.Fatalf("exec migration: %v", err)
	}

	var got string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'whisper_device'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "gpu" {
		t.Errorf("whisper_device = %q, want gpu", got)
	}

	for _, keep := range []string{"auto", "cpu", "gpu"} {
		if _, err := db.Exec(`UPDATE settings SET value = ? WHERE key = 'whisper_device'`, keep); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(stmt)); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'whisper_device'`).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != keep {
			t.Errorf("valor %q deveria ser preservado, got %q", keep, got)
		}
	}
}
