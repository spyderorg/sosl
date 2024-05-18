// Copyright (c) 2024 by SPYDER.
//
// This file is licensed under the Spyder Open Security License (SOSL) 1.0.
// See the LICENSE.md file for details.

//nolint:cyclop // ignore function complexity
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/sashabaranov/go-openai"
)

var locales = []string{
	"en",
	"de",
	"zh",
	"hi",
	"es",
	"fr",
	"ar",
	"bn",
	"ru",
	"pt",
	"id",
	"ja",
	"ko",
	"it",
	"nl",
	"sv",
	"pl",
	"he",
	"el",
	"zh-sg",
	"tr",
	"vi",
	"th",
	"fa",
	"ms",
	"hu",
	"cs",
	"ro",
	"da",
	"fi",
}

const defaultPrompt = `
	translate the following %s markdown to %s and return only the translated markdown, include the text in the codeblock at the bottom of the markdown as part of the translation: %s
`

//nolint:gochecknoglobals // ignore global variables
var primeLocale = "en"

//nolint:gochecknoglobals // ignore global variables
var model = "gpt-4o"

func main() {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		slog.Error(
			"environment variable OPENAI_API_KEY is not set",
		)
		os.Exit(1)
	}

	if os.Getenv("PRIMARY_LOCALE") != "" {
		primeLocale = os.Getenv("PRIMARY_LOCALE")
	}

	if os.Getenv("OPENAI_MODEL") != "" {
		primeLocale = os.Getenv("OPENAI_MODEL")
	}

	path := os.Getenv("DATA_PATH")
	if path == "" {
		slog.Error(
			"environment variable DATA_PATH is not set",
		)
		os.Exit(1)
	}

	client := openai.NewClient(key)

	license, err := os.OpenFile(
		filepath.Join(path, fmt.Sprintf("LICENSE.%s.md", primeLocale)),
		os.O_RDONLY,
		0644,
	)

	if err != nil {
		slog.Error(
			"error accessing the prime locale's license",
			"error", err,
		)
		os.Exit(1)
	}

	// Translate the message catalog into other locales.
	err = translate(context.Background(), client, license, path)
	if err != nil {
		slog.Error(
			"error translating the message catalog",
			"error", err,
		)
		os.Exit(1)
	}
}

//nolint:funlen,gocognit // ignore function length
func translate(
	ctx context.Context,
	client *openai.Client,
	license io.ReadCloser,
	path string,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	primeLicense, err := io.ReadAll(license)
	if err != nil {
		return err
	}
	defer license.Close()

	wg := sync.WaitGroup{}
	for _, locale := range locales {
		wg.Add(1)
		go func(locale string) {
			defer wg.Done()

			if locale == primeLocale {
				return
			}

			file := filepath.Join(path, fmt.Sprintf("LICENSE.%s.md", locale))
			slog.Info(
				"translating license to locale",
				"locale", locale,
				"filepath", file,
			)

			prompt := fmt.Sprintf(
				defaultPrompt, primeLocale, locale, primeLicense,
			)

			resp, err := client.CreateChatCompletion(
				ctx,
				openai.ChatCompletionRequest{
					Model: model,
					Messages: []openai.ChatCompletionMessage{
						{
							Role:    openai.ChatMessageRoleUser,
							Content: prompt,
						},
					},
				},
			)

			err = os.WriteFile(file, []byte(resp.Choices[0].Message.Content), 0644)
			if err != nil {
				slog.Error(
					"error writing the translated message catalog",
					"locale", locale,
					"error", err,
				)
			}

		}(locale)
	}

	wg.Wait()

	return nil
}
