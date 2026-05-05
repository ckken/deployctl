package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ckken/deployctl/internal/auth"
	"github.com/ckken/deployctl/internal/httpapi"
	"github.com/ckken/deployctl/internal/types"
)

const defaultDataDir = ".deployctl-data"

func usage() {
	fmt.Fprintf(os.Stderr, `deployd - upload grant server

Usage:
  deployd serve --listen :7319 --data-dir ./.deployctl-data --admin-secret secret --web-dir ./website
  DEPLOYD_ADMIN_SECRET=secret deployd serve --listen :7319 --data-dir ./.deployctl-data --web-dir ./website
  deployd admin create-token --data-dir ./.deployctl-data --admin-secret secret --name ci-bot --scope read-only
  deployd admin list-tokens --data-dir ./.deployctl-data --admin-secret secret
  deployd admin revoke-token --data-dir ./.deployctl-data --admin-secret secret --token-id tok_xxx

  deployd admin create-upload-link --data-dir ./.deployctl-data --admin-secret secret --base-url https://q.empjs.dev [--folder releases/demo] [--expires-in 24h]
  deployd admin list-upload-links --data-dir ./.deployctl-data --admin-secret secret
  deployd admin delete-upload-link --data-dir ./.deployctl-data --admin-secret secret --grant-id grt_xxx
`)
}

func mustStore(dataDir string) *auth.Store {
	store := auth.NewStore(dataDir)
	if err := store.Load(); err != nil {
		log.Fatal(err)
	}
	return store
}

func requireAdminSecret(secret string) {
	if secret == "" {
		log.Fatal("admin secret is required")
	}
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatal(err)
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", ":7319", "listen address")
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	webDir := fs.String("web-dir", "./website", "website directory to serve at /")
	fs.Parse(args)
	if *adminSecret == "" {
		*adminSecret = os.Getenv("DEPLOYD_ADMIN_SECRET")
	}
	requireAdminSecret(*adminSecret)

	store := mustStore(*dataDir)
	srv := &http.Server{
		Addr:              *listen,
		Handler:           httpapi.New(store, *adminSecret).Handler(*webDir),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("deployd listening on %s (web_dir=%s)", *listen, *webDir)
	return srv.ListenAndServe()
}

func runAdminCreateToken(args []string) error {
	fs := flag.NewFlagSet("create-token", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	name := fs.String("name", "", "token name")
	scope := fs.String("scope", "", "token scope")
	projectScope := fs.String("project-scope", "", "project scope")
	expiresIn := fs.String("expires-in", "", "expiry duration")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)

	store := mustStore(*dataDir)
	resp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:         *name,
		Scope:        *scope,
		ProjectScope: *projectScope,
		ExpiresIn:    *expiresIn,
	})
	if err != nil {
		return err
	}
	printJSON(resp)
	return nil
}

func runAdminListTokens(args []string) error {
	fs := flag.NewFlagSet("list-tokens", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)

	store := mustStore(*dataDir)
	records, err := store.ListTokens()
	if err != nil {
		return err
	}
	printJSON(records)
	return nil
}

func runAdminRevokeToken(args []string) error {
	fs := flag.NewFlagSet("revoke-token", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	tokenID := fs.String("token-id", "", "token id")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)
	if *tokenID == "" {
		return errors.New("token-id is required")
	}

	store := mustStore(*dataDir)
	resp, err := store.RevokeToken(*tokenID)
	if err != nil {
		return err
	}
	printJSON(resp)
	return nil
}

func runAdminCreateUploadLink(args []string) error {
	fs := flag.NewFlagSet("create-upload-link", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	baseURL := fs.String("base-url", "http://127.0.0.1:7319", "public base URL")
	folder := fs.String("folder", "", "relative folder")
	expiresIn := fs.String("expires-in", "24h", "expiry duration")
	maxFiles := fs.Int("max-files", 1, "max files for this link")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)

	store := mustStore(*dataDir)
	resp, err := store.CreateUploadGrant(types.CreateUploadGrantRequest{
		Folder:    *folder,
		ExpiresIn: *expiresIn,
		MaxFiles:  *maxFiles,
	}, *baseURL)
	if err != nil {
		return err
	}
	printJSON(resp)
	return nil
}

func runAdminListUploadLinks(args []string) error {
	fs := flag.NewFlagSet("list-upload-links", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	baseURL := fs.String("base-url", "http://127.0.0.1:7319", "public base URL")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)

	store := mustStore(*dataDir)
	resp, err := store.ListUploadGrants(*baseURL)
	if err != nil {
		return err
	}
	printJSON(resp)
	return nil
}

func runAdminDeleteUploadLink(args []string) error {
	fs := flag.NewFlagSet("delete-upload-link", flag.ExitOnError)
	dataDir := fs.String("data-dir", defaultDataDir, "data directory")
	adminSecret := fs.String("admin-secret", "", "admin secret")
	grantID := fs.String("grant-id", "", "grant id")
	fs.Parse(args)
	requireAdminSecret(*adminSecret)
	if *grantID == "" {
		return errors.New("grant-id is required")
	}

	store := mustStore(*dataDir)
	resp, err := store.DeleteUploadGrant(*grantID)
	if err != nil {
		return err
	}
	printJSON(resp)
	return nil
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(os.Args[2:])
	case "admin":
		if len(os.Args) < 3 {
			usage()
			os.Exit(2)
		}
		switch os.Args[2] {
		case "create-token":
			err = runAdminCreateToken(os.Args[3:])
		case "list-tokens":
			err = runAdminListTokens(os.Args[3:])
		case "revoke-token":
			err = runAdminRevokeToken(os.Args[3:])
		case "create-upload-link":
			err = runAdminCreateUploadLink(os.Args[3:])
		case "list-upload-links":
			err = runAdminListUploadLinks(os.Args[3:])
		case "delete-upload-link":
			err = runAdminDeleteUploadLink(os.Args[3:])
		default:
			usage()
			os.Exit(2)
		}
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		log.Fatal(err)
	}
}
