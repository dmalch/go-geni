package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestClientID(t *testing.T) {
	RegisterTestingT(t)
	Expect(clientID(false)).To(Equal("1855"))
	Expect(clientID(true)).To(Equal("8"))
}

func TestTokenCacheFilePath(t *testing.T) {
	t.Run("production path", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		p, err := tokenCacheFilePath(false)
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(home, ".genealogy", "geni_token.json")))
	})

	t.Run("sandbox path", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		p, err := tokenCacheFilePath(true)
		Expect(err).ToNot(HaveOccurred())
		Expect(p).To(Equal(filepath.Join(home, ".genealogy", "geni_sandbox_token.json")))
	})
}

func TestRunLogout(t *testing.T) {
	t.Run("removes an existing cache file", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		path := filepath.Join(home, ".genealogy", "geni_sandbox_token.json")
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte(`{"access_token":"x"}`), 0o600)).To(Succeed())

		g := &globalOpts{sandbox: true}
		Expect(runLogout(context.Background(), g, nil)).To(Succeed())

		_, statErr := os.Stat(path)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	t.Run("is a no-op when the cache file is absent", func(t *testing.T) {
		RegisterTestingT(t)
		home := t.TempDir()
		t.Setenv("HOME", home)

		g := &globalOpts{sandbox: true}
		Expect(runLogout(context.Background(), g, nil)).To(Succeed())
	})
}
