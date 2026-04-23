package main

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

func defaultKeyPath() string {
	if value := os.Getenv("P2PTEST_KEY_PATH"); value != "" {
		return value
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".p2ptest", "sender.key")
	}
	return filepath.Join(home, ".p2ptest", "sender.key")
}

func loadOrCreatePrivateKey(path string) (crypto.PrivKey, string, bool, error) {
	bytes, err := os.ReadFile(path)
	if err == nil {
		key, err := crypto.UnmarshalPrivateKey(bytes)
		return key, path, false, err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, path, false, err
	}

	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, path, false, err
	}
	bytes, err = crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, path, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, path, false, err
	}
	if err := os.WriteFile(path, bytes, 0600); err != nil {
		return nil, path, false, err
	}

	return priv, path, true, nil
}
