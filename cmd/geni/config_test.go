package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestUserConfig_RoundTrip(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())

	// Empty load when file missing.
	c, err := loadUserConfig()
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Browser).To(BeEmpty())

	// Save then load returns the same browser.
	Expect(saveUserConfig(userConfig{Browser: "safari"})).To(Succeed())
	c, err = loadUserConfig()
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Browser).To(Equal("safari"))
	Expect(c.Version).To(Equal(1)) // saveUserConfig stamps the version

	// Saving the zero value deletes the file.
	Expect(saveUserConfig(userConfig{})).To(Succeed())
	p, _ := userConfigPath()
	_, statErr := os.Stat(p)
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "expected config.json to be removed")
}

func TestSaveUserConfig_CreatesDir(t *testing.T) {
	RegisterTestingT(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory does not exist yet — save should create it.
	Expect(saveUserConfig(userConfig{Browser: "firefox"})).To(Succeed())
	info, err := os.Stat(filepath.Join(home, ".genealogy"))
	Expect(err).ToNot(HaveOccurred())
	Expect(info.IsDir()).To(BeTrue())
}

func TestRunConfigBrowser_ArgValidation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	g := &globalOpts{stdin: strings.NewReader(""), stdout: io.Discard, stderr: io.Discard}

	t.Run("no args is an error", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runConfigBrowser(context.Background(), g, nil)).To(HaveOccurred())
	})

	t.Run("unknown browser rejected", func(t *testing.T) {
		RegisterTestingT(t)
		err := runConfigBrowser(context.Background(), g, []string{"not-a-browser"})
		Expect(err).To(MatchError(ContainSubstring("not-a-browser")))
	})

	t.Run("valid browser persists and round-trips", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(runConfigBrowser(context.Background(), g, []string{"safari"})).To(Succeed())
		c, err := loadUserConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Browser).To(Equal("safari"))
	})

	t.Run("empty string clears the preference", func(t *testing.T) {
		RegisterTestingT(t)
		Expect(saveUserConfig(userConfig{Browser: "chrome"})).To(Succeed())
		Expect(runConfigBrowser(context.Background(), g, []string{""})).To(Succeed())
		c, err := loadUserConfig()
		Expect(err).ToNot(HaveOccurred())
		Expect(c.Browser).To(BeEmpty())
	})
}

func TestRunConfigShow_PrintsJSON(t *testing.T) {
	RegisterTestingT(t)
	t.Setenv("HOME", t.TempDir())
	Expect(saveUserConfig(userConfig{Browser: "firefox"})).To(Succeed())

	var out bytes.Buffer
	g := &globalOpts{stdout: &out, stderr: io.Discard}
	Expect(runConfigShow(context.Background(), g, nil)).To(Succeed())
	Expect(out.String()).To(ContainSubstring(`"browser": "firefox"`))
}
