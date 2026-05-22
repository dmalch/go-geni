package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestRunDispatch(t *testing.T) {
	t.Run("no args prints usage and returns 2", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), nil, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(2))
		Expect(errb.String()).To(ContainSubstring("Usage:"))
		Expect(out.String()).To(BeEmpty())
	})

	t.Run("unknown top-level command returns 2", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"frobnicate"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(2))
		Expect(errb.String()).To(ContainSubstring("unknown command"))
	})

	t.Run("resource group without a subcommand returns 2", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"profile"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(2))
		Expect(errb.String()).To(ContainSubstring("expected a subcommand"))
	})

	t.Run("unknown subcommand under a resource returns 2", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"profile", "destroy"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(2))
		Expect(errb.String()).To(ContainSubstring("unknown subcommand"))
	})

	t.Run("help prints the command tree to stdout and returns 0", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"help"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(0))
		Expect(out.String()).To(ContainSubstring("profile get"))
		Expect(out.String()).To(ContainSubstring("profile search"))
		Expect(out.String()).To(ContainSubstring("tree ancestors"))
		Expect(out.String()).To(ContainSubstring("union get"))
	})

	t.Run("-h returns 0", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"-h"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(0))
	})

	t.Run("an unknown global flag returns 2", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"-nope"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(2))
	})

	t.Run("-sandbox is accepted as a global flag before the command", func(t *testing.T) {
		RegisterTestingT(t)
		var out, errb bytes.Buffer
		code := run(context.Background(), []string{"-sandbox", "help"}, strings.NewReader(""), &out, &errb)
		Expect(code).To(Equal(0))
		Expect(out.String()).To(ContainSubstring("Commands:"))
	})
}
