package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"encr.dev/cli/internal/conf"
	"encr.dev/cli/internal/wgtunnel"
	"encr.dev/cli/internal/xos"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var vpnCmd = &cobra.Command{
	Use:   "vpn",
	Short: "VPN management commands",
}

func init() {
	rootCmd.AddCommand(vpnCmd)

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Sets up a secure connection to private environments",
		Run: func(cmd *cobra.Command, args []string) {
			if admin, err := xos.IsAdminUser(); err == nil && !admin {
				log.Fatalf("fatal: must start VPN as root user (use 'sudo'?)")
			}

			cfg, err := conf.OriginalUser("")
			if errors.Is(err, os.ErrNotExist) {
				log.Fatalf("fatal: not logged in. run 'encore auth login' first")
			} else if err != nil {
				log.Fatalf("fatal: could not read encore config (did you run 'encore auth login'?): %v", err)
			} else if cfg.WireGuard.PrivateKey == "" || cfg.WireGuard.PublicKey == "" {
				log.Println("encore: generating WireGuard key...")
				pub, priv, err := wgtunnel.GenKey()
				if err != nil {
					log.Fatalf("fatal: could not generate WireGuard key: %v", err)
				}
				cfg.WireGuard.PublicKey = pub.String()
				cfg.WireGuard.PrivateKey = priv.String()
				if err := conf.Write(cfg); err != nil {
					log.Fatalf("fatal: could not write updated config: %v", err)
				}
				log.Println("encore: successfully generated and persisted WireGuard key")
			}

			pubKey, err1 := wgtypes.ParseKey(cfg.WireGuard.PublicKey)
			privKey, err2 := wgtypes.ParseKey(cfg.WireGuard.PrivateKey)
			if err1 != nil || err2 != nil {
				fatalf("could not parse public/private key: %v/%v", err1, err2)
			}

			log.Printf("encore: registering device with server...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ip, err := wgtunnel.RegisterDevice(ctx, pubKey)
			if err != nil {
				log.Fatalf("fatal: could not register device: %v", err)
			}
			log.Printf("encore: successfully registered device, assigned address %s", ip)

			log.Printf("encore: starting WireGuard tunnel...")
			cc := &wgtunnel.ClientConfig{
				Addr:    ip,
				PrivKey: privKey,
			}
			if err := wgtunnel.Start(cc, nil); err != nil {
				log.Fatalf("fatal: could not start tunnel: %v", err)
			}
			log.Printf("encore: successfully started WireGuard tunnel")
		},
	}
	vpnCmd.AddCommand(startCmd)

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stops the VPN connection",
		Run: func(cmd *cobra.Command, args []string) {
			if err := wgtunnel.Stop(); os.IsPermission(err) {
				log.Fatal("fatal: permission denied to stop tunnel (use 'sudo'?)")
			} else if err != nil {
				log.Fatalf("fatal: could not stop tunnel: %v", err)
			}
			log.Printf("encore: stopped WireGuard tunnel")
		},
	}
	vpnCmd.AddCommand(stopCmd)

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Determines the status of the VPN connection",
		Run: func(cmd *cobra.Command, args []string) {
			if running, err := wgtunnel.Status(); os.IsPermission(err) {
				log.Fatal("fatal: permission denied to check tunnel status (use 'sudo'?)")
			} else if err != nil {
				log.Fatalf("fatal: could not check tunnel status: %v", err)
			} else if running {
				fmt.Fprintln(os.Stdout, "running")
			} else {
				fmt.Fprintln(os.Stdout, "not running")
			}
		},
	}
	vpnCmd.AddCommand(statusCmd)
}
