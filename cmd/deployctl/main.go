package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ckken/deployctl/internal/client"
	"github.com/ckken/deployctl/internal/config"
	"github.com/ckken/deployctl/internal/types"
)

const adminKeyEnv = "DEPLOYCTL_ADMIN_KEY"

func usage() {
	fmt.Fprintf(os.Stderr, `deployctl - upload grant cli

Usage:
  deployctl [--json] [--server URL] [--token TOKEN] doctor
  deployctl [--json] [--server URL] [--token TOKEN] auth whoami
  deployctl [--json] auth token set <token>
  deployctl [--json] auth token show-source
  deployctl [--json] auth token clear

  deployctl [--json] [--server URL] upload-link create --admin-key <secret> [--folder releases/demo] [--expires-in 24h] [--max-files 1]
  deployctl [--json] [--server URL] upload-link list --admin-key <secret>
  deployctl [--json] [--server URL] upload-link delete --admin-key <secret> --grant-id <id>

  deployctl [--json] upload --url https://q.empjs.dev/u/upc_xxx [--file ./build.zip]
`)
}

func printOutput(asJSON bool, v any, text string) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			log.Fatal(err)
		}
		return
	}
	fmt.Println(text)
}

func resolveServer(flagServer string, cfg config.Config) string {
	if flagServer != "" {
		return flagServer
	}
	if cfg.ServerURL != "" {
		return cfg.ServerURL
	}
	return config.DefaultServerURL
}

func resolveAdminKey(flagAdminKey string) string {
	if flagAdminKey != "" {
		return flagAdminKey
	}
	return os.Getenv(adminKeyEnv)
}

func runDoctor(ctx context.Context, asJSON bool, server string, flagToken string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := config.ResolveToken(flagToken, cfg)
	cli := client.New(server, token.Token)

	resp := types.DoctorResponse{
		Server: server,
		Token: types.DoctorToken{
			Present: token.Token != "",
			Source:  token.Source,
		},
	}

	if _, err := cli.Health(ctx); err != nil {
		resp.Reachable = false
		resp.Auth.OK = false
		resp.Auth.Error = err.Error()
		printOutput(asJSON, resp, "server unreachable")
		return nil
	}
	resp.Reachable = true

	if token.Token == "" {
		resp.Auth.OK = false
		resp.Auth.Error = "missing token"
		printOutput(asJSON, resp, "server reachable, token missing")
		return nil
	}

	if _, err := cli.WhoAmI(ctx); err != nil {
		resp.Auth.OK = false
		resp.Auth.Error = err.Error()
		printOutput(asJSON, resp, "token invalid")
		return nil
	}

	resp.Auth.OK = true
	printOutput(asJSON, resp, "ok")
	return nil
}

func runTokenSet(asJSON bool, token string) error {
	if token == "" {
		return errors.New("token is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = token
	if err := config.Save(cfg); err != nil {
		return err
	}
	printOutput(asJSON, map[string]string{"status": "ok", "source": "config"}, "token saved to config")
	return nil
}

func runTokenShowSource(asJSON bool, flagToken string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := config.ResolveToken(flagToken, cfg)
	printOutput(asJSON, map[string]string{"source": token.Source}, token.Source)
	return nil
}

func runTokenClear(asJSON bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	printOutput(asJSON, map[string]string{"status": "ok"}, "token cleared")
	return nil
}

func runWhoAmI(ctx context.Context, asJSON bool, server string, flagToken string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := config.ResolveToken(flagToken, cfg)
	cli := client.New(server, token.Token)
	resp, err := cli.WhoAmI(ctx)
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, fmt.Sprintf("%s %s", resp.TokenName, resp.Scope))
	return nil
}

func runUploadLinkCreate(ctx context.Context, asJSON bool, server string, args []string) error {
	fs := flag.NewFlagSet("upload-link create", flag.ContinueOnError)
	adminKey := fs.String("admin-key", "", "admin secret")
	folder := fs.String("folder", "", "relative folder under files root")
	expiresIn := fs.String("expires-in", "24h", "grant expiry, default 24h")
	maxFiles := fs.Int("max-files", 1, "max files for this grant")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedAdminKey := resolveAdminKey(*adminKey)
	if resolvedAdminKey == "" {
		return errors.New("admin-key is required")
	}
	cli := client.New(server, "")
	resp, err := cli.CreateUploadLink(ctx, resolvedAdminKey, types.CreateUploadGrantRequest{
		Folder:    *folder,
		ExpiresIn: *expiresIn,
		MaxFiles:  *maxFiles,
	})
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, resp.UploadURL)
	return nil
}

func runUploadLinkList(ctx context.Context, asJSON bool, server string, args []string) error {
	fs := flag.NewFlagSet("upload-link list", flag.ContinueOnError)
	adminKey := fs.String("admin-key", "", "admin secret")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedAdminKey := resolveAdminKey(*adminKey)
	if resolvedAdminKey == "" {
		return errors.New("admin-key is required")
	}
	cli := client.New(server, "")
	resp, err := cli.ListUploadLinks(ctx, resolvedAdminKey)
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, fmt.Sprintf("%d upload links", len(resp)))
	return nil
}

func runUploadLinkDelete(ctx context.Context, asJSON bool, server string, args []string) error {
	fs := flag.NewFlagSet("upload-link delete", flag.ContinueOnError)
	adminKey := fs.String("admin-key", "", "admin secret")
	grantID := fs.String("grant-id", "", "grant id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedAdminKey := resolveAdminKey(*adminKey)
	if resolvedAdminKey == "" {
		return errors.New("admin-key is required")
	}
	if *grantID == "" {
		return errors.New("grant-id is required")
	}
	cli := client.New(server, "")
	resp, err := cli.DeleteUploadLink(ctx, resolvedAdminKey, *grantID)
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, resp.GrantID)
	return nil
}

func runUpload(ctx context.Context, asJSON bool, args []string) error {
	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	uploadURL := fs.String("url", "", "upload URL")
	filePath := fs.String("file", "", "file to upload")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *uploadURL == "" {
		return errors.New("url is required")
	}
	cli := client.New(client.ResolveBaseURL(*uploadURL), "")
	if *filePath == "" {
		resp, err := cli.UploadInfoByURL(ctx, *uploadURL)
		if err != nil {
			return err
		}
		printOutput(asJSON, resp, resp.UploadURL)
		return nil
	}
	resp, err := cli.UploadFileByURL(ctx, *uploadURL, *filePath)
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, resp.FileURL)
	return nil
}

func main() {
	log.SetFlags(0)
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		log.Fatal(cfgErr)
	}

	root := flag.NewFlagSet("deployctl", flag.ExitOnError)
	asJSON := root.Bool("json", false, "emit json")
	flagToken := root.String("token", "", "token override")
	flagServer := root.String("server", "", "server override")
	root.Parse(os.Args[1:])
	args := root.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	server := resolveServer(*flagServer, cfg)
	ctx := context.Background()
	var err error
	switch args[0] {
	case "doctor":
		err = runDoctor(ctx, *asJSON, server, *flagToken)
	case "auth":
		if len(args) < 2 {
			usage()
			os.Exit(2)
		}
		switch args[1] {
		case "whoami":
			err = runWhoAmI(ctx, *asJSON, server, *flagToken)
		case "token":
			if len(args) < 3 {
				usage()
				os.Exit(2)
			}
			switch args[2] {
			case "set":
				if len(args) < 4 {
					err = errors.New("token is required")
					break
				}
				err = runTokenSet(*asJSON, args[3])
			case "show-source":
				err = runTokenShowSource(*asJSON, *flagToken)
			case "clear":
				err = runTokenClear(*asJSON)
			default:
				usage()
				os.Exit(2)
			}
		default:
			usage()
			os.Exit(2)
		}
	case "upload-link":
		if len(args) < 2 {
			usage()
			os.Exit(2)
		}
		switch args[1] {
		case "create":
			err = runUploadLinkCreate(ctx, *asJSON, server, args[2:])
		case "list":
			err = runUploadLinkList(ctx, *asJSON, server, args[2:])
		case "delete":
			err = runUploadLinkDelete(ctx, *asJSON, server, args[2:])
		default:
			usage()
			os.Exit(2)
		}
	case "upload":
		err = runUpload(ctx, *asJSON, args[1:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		log.Fatal(err)
	}
}
