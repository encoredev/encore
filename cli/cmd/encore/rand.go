package main

import (
	cryptorand "crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/spf13/cobra"

	"encr.dev/cli/cmd/encore/cmdutil"
	"encr.dev/pkg/words"
)

var randCmd = &cobra.Command{
	Use:   "rand",
	Short: "Utilities for generating cryptographically secure random data",
}

func init() {
	rootCmd.AddCommand(randCmd)
}

// UUID command
func init() {
	var v1, v4, v6, v7 bool
	uuidCmd := &cobra.Command{
		Use:   "uuid [-1|-4|-6|-7]",
		Short: "Generates a random UUID (defaults to version 4)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			versions := map[bool]func() (uuid.UUID, error){
				v1: uuid.NewV1,
				v4: uuid.NewV4,
				v6: uuid.NewV6,
				v7: uuid.NewV7,
			}
			fn, ok := versions[true]
			if !ok {
				fatalf("unsupported UUID version")
			}
			u, err := fn()
			if err != nil {
				fatalf("failed to generate UUID: %v", err)
			}
			_, _ = fmt.Println(u.String())
		},
	}
	uuidCmd.Flags().BoolVarP(&v1, "v1", "1", false, "Generate a version 1 UUID")
	uuidCmd.Flags().BoolVarP(&v4, "v4", "4", true, "Generate a version 4 UUID")
	uuidCmd.Flags().BoolVarP(&v6, "v6", "6", false, "Generate a version 6 UUID")
	uuidCmd.Flags().BoolVarP(&v7, "v7", "7", false, "Generate a version 7 UUID")
	uuidCmd.MarkFlagsMutuallyExclusive("v1", "v4", "v6", "v7")

	randCmd.AddCommand(uuidCmd)
}

// Bytes command
func init() {
	format := cmdutil.Oneof{
		Value:     "hex",
		Allowed:   []string{"hex", "base32", "base32hex", "base32crockford", "base64", "base64url", "raw"},
		Flag:      "format",
		FlagShort: "f",
		Desc:      "Output format",
	}

	noPadding := false
	doFormat := func(data []byte) string {
		switch format.Value {
		case "hex":
			return hex.EncodeToString(data)
		case "base32":
			enc := base32.StdEncoding
			if noPadding {
				enc = enc.WithPadding(base32.NoPadding)
			}
			return enc.EncodeToString(data)
		case "base32hex":
			enc := base32.HexEncoding
			if noPadding {
				enc = enc.WithPadding(base32.NoPadding)
			}
			return enc.EncodeToString(data)
		case "base32crockford":
			enc := base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ")
			if noPadding {
				enc = enc.WithPadding(base32.NoPadding)
			}
			return enc.EncodeToString(data)
		case "base64":
			enc := base64.StdEncoding
			if noPadding {
				enc = enc.WithPadding(base64.NoPadding)
			}
			return enc.EncodeToString(data)
		case "base64url":
			enc := base64.URLEncoding
			if noPadding {
				enc = enc.WithPadding(base64.NoPadding)
			}
			return enc.EncodeToString(data)
		default:
			fatalf("unsupported output format: %s", format.Value)
			panic("unreachable")
		}
	}

	bytesCmd := &cobra.Command{
		Use:   "bytes BYTES [-f " + format.Alternatives() + "]",
		Short: "Generates random bytes and outputs them in the specified format",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			num, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				fatalf("invalid number of bytes: %v", err)
			} else if num < 1 {
				fatalf("number of bytes must be positive")
			} else if num > 1024*1024 {
				fatalf("too many bytes requested")
			}

			data := make([]byte, num)
			_, err = cryptorand.Read(data)
			if err != nil {
				fatalf("failed to generate random bytes: %v", err)
			}

			if format.Value == "raw" {
				_, err = os.Stdout.Write(data)
				if err != nil {
					fatalf("failed to write: %v", err)
				}
			} else {
				formatted := doFormat(data)
				if _, err := os.Stdout.WriteString(formatted); err != nil {
					fatalf("failed to write: %v", err)
				}
				_, _ = os.Stdout.Write([]byte{'\n'})
			}
		},
	}

	format.AddFlag(bytesCmd)
	bytesCmd.Flags().BoolVar(&noPadding, "no-padding", false, "omit padding characters from base32/base64 output")
	randCmd.AddCommand(bytesCmd)
}

// Words command
func init() {
	var sep string
	wordsCmd := &cobra.Command{
		Use:   "words [--sep=SEPARATOR] NUM",
		Short: "Generates random 4-5 letter words for memorable passphrases",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			num, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				fatalf("invalid number of words: %v", err)
			} else if num < 1 {
				fatalf("number of words must be positive")
			} else if num > 1024 {
				fatalf("too many words requested")
			}

			selected, err := words.Select(int(num))
			if err != nil {
				fatalf("failed to select words: %v", err)
			}

			formatted := strings.Join(selected, sep)
			if _, err := os.Stdout.WriteString(formatted); err != nil {
				fatalf("failed to write: %v", err)
			}
			_, _ = os.Stdout.Write([]byte{'\n'})
		},
	}

	wordsCmd.Flags().StringVarP(&sep, "sep", "s", " ", "separator between words")
	randCmd.AddCommand(wordsCmd)
}
