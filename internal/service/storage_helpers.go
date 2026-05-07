package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"chatgpt2api/internal/storage"
)

func jsonDocumentStoreFromBackend(backend storage.Backend) storage.JSONDocumentBackend {
	if store, ok := backend.(storage.JSONDocumentBackend); ok {
		return store
	}
	return nil
}

func firstJSONDocumentStore(backends []storage.Backend) storage.JSONDocumentBackend {
	if len(backends) == 0 {
		return nil
	}
	return jsonDocumentStoreFromBackend(backends[0])
}

func logStoreFromBackend(backend storage.Backend) storage.LogBackend {
	if store, ok := backend.(storage.LogBackend); ok {
		return store
	}
	return nil
}

func firstLogStore(backends []storage.Backend) storage.LogBackend {
	if len(backends) == 0 {
		return nil
	}
	return logStoreFromBackend(backends[0])
}

func loadStoredJSON(store storage.JSONDocumentBackend, name, filePath string) any {
	if store != nil {
		value, err := store.LoadJSONDocument(name)
		if err == nil {
			return value
		}
	}
	value, _ := readJSONValueFile(filePath)
	return value
}

func saveStoredJSON(store storage.JSONDocumentBackend, name, filePath string, value any) error {
	if store != nil {
		return store.SaveJSONDocument(name, value)
	}
	return writeJSONValueFile(filePath, value)
}

func readJSONValueFile(filePath string) (any, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var raw any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func writeJSONValueFile(filePath string, value any) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, append(data, '\n'), 0o644)
}
