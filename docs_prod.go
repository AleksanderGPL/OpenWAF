//go:build !dev

package main

import "OpenWAF/internal/api"

func registerDocs(*api.Router) error { return nil }
