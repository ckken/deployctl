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

func usage() {
	fmt.Fprintf(os.Stderr, `deployctl - token-only auth cli

Usage:
  deployctl [--json] [--token TOKEN] doctor
  deployctl [--json] [--token TOKEN] auth whoami
  deployctl [--json] auth token set <token>
  deployctl [--json] auth token show-source
  deployctl [--json] auth token clear
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

func runDoctor(ctx context.Context, asJSON bool, flagToken string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := config.ResolveToken(flagToken, cfg)
	cli := client.New(cfg.ServerURL, token.Token)

	resp := types.DoctorResponse{
		Server: cfg.ServerURL,
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

func runWhoAmI(ctx context.Context, asJSON bool, flagToken string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	token := config.ResolveToken(flagToken, cfg)
	cli := client.New(cfg.ServerURL, token.Token)
	resp, err := cli.WhoAmI(ctx)
	if err != nil {
		return err
	}
	printOutput(asJSON, resp, fmt.Sprintf("%s %s", resp.TokenName, resp.Scope))
	return nil
}

func main() {
	log.SetFlags(0)
	root := flag.NewFlagSet("deployctl", flag.ExitOnError)
	asJSON := root.Bool("json", false, "emit json")
	flagToken := root.String("token", "", "token override")
	root.Parse(os.Args[1:])
	args := root.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()
	var err error
	switch args[0] {
	case "doctor":
		err = runDoctor(ctx, *asJSON, *flagToken)
	case "auth":
		if len(args) < 2 {
			usage()
			os.Exit(2)
		}
		switch args[1] {
		case "whoami":
			err = runWhoAmI(ctx, *asJSON, *flagToken)
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
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		log.Fatal(err)
	}
}
