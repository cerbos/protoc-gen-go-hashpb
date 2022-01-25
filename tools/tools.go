// Copyright 2021-2022 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

//go:build toolsx

package tools

import (
	_ "github.com/bufbuild/buf/cmd/buf"
	_ "github.com/goreleaser/goreleaser"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
