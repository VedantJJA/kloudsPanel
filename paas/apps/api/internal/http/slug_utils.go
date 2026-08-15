package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourorg/klouds/api/internal/repository"
)

func slugify(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	res := strings.Trim(b.String(), "-")
	if res == "" {
		res = "app"
	}
	return res
}

func generateUniqueProjectSlug(ctx context.Context, store repository.Store, name string, requestedSlug string) string {
	base := slugify(requestedSlug)
	if base == "" || base == "app" {
		base = slugify(name)
	}
	if base == "" {
		base = "project"
	}
	slug := base
	counter := 1
	for {
		exists, err := store.Projects().SlugExists(ctx, slug)
		if err != nil || !exists {
			return slug
		}
		counter++
		slug = fmt.Sprintf("%s-%d", base, counter)
	}
}

func generateUniqueServiceSlug(ctx context.Context, store repository.Store, name string, requestedSlug string) string {
	base := slugify(requestedSlug)
	if base == "" || base == "app" {
		base = slugify(name)
	}
	if base == "" {
		base = "service"
	}
	slug := base
	counter := 1
	for {
		exists, err := store.Services().SlugExists(ctx, slug)
		if err != nil || !exists {
			return slug
		}
		counter++
		slug = fmt.Sprintf("%s-%d", base, counter)
	}
}
