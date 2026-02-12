package sshkey

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/providers"
	"nathanbeddoewebdev/vpsm/internal/services/auth"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func AddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [path]",
		Short: "Upload an SSH key to the cloud provider",
		Long: `Upload a local SSH public key to the cloud provider's account.

The path argument is optional and defaults to ~/.ssh/id_ed25519.pub.
If that file does not exist, you will be prompted to provide a path.

The key name will be prompted interactively unless --name is specified.

Examples:
  # Upload default key (interactive name prompt)
  vpsm ssh-key add

  # Upload specific key with explicit name
  vpsm ssh-key add ~/.ssh/work_laptop.pub --name work-laptop

  # Upload with provider override
  vpsm ssh-key add --provider hetzner --name my-key`,
		Run: runAdd,
	}

	cmd.Flags().String("name", "", "Name for the SSH key (interactive prompt if not provided)")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) {
	providerName := cmd.Flag("provider").Value.String()

	provider, err := providers.Get(providerName, auth.DefaultStore())
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	keyManager, ok := provider.(domain.SSHKeyManager)
	if !ok {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: provider %q does not support SSH key management\n", providerName)
		return
	}

	// Determine path
	usingDefault := len(args) == 0
	var keyPath string
	if usingDefault {
		keyPath = defaultSSHKeyPath()
	} else {
		keyPath = args[0]
	}

	keyPath, err = expandHomePath(keyPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	// Check if file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if usingDefault {
			keyPath, err = promptForSSHKeyPath(cmd, keyPath)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				printCommonSSHKeyPaths(cmd)
				return
			}

			keyPath, err = expandHomePath(keyPath)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				return
			}
		}

		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: SSH key file not found: %s\n", keyPath)
			printCommonSSHKeyPaths(cmd)
			return
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Reading key from %s\n", keyPath)

	// Read and validate the SSH key
	publicKey, err := readAndValidateSSHKey(keyPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
		return
	}

	// Get or prompt for name
	keyName, _ := cmd.Flags().GetString("name")
	if keyName == "" {
		suggestedName := suggestKeyName(keyPath)
		accessible := os.Getenv("ACCESSIBLE") != ""

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter a name for this SSH key").
					Value(&keyName).
					Placeholder(suggestedName).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("name cannot be empty")
						}
						return nil
					}),
			),
		).WithAccessible(accessible)

		if err := form.Run(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			return
		}
	}

	// Upload the key
	fmt.Fprintf(cmd.ErrOrStderr(), "Uploading SSH key %q to %s...", keyName, provider.GetDisplayName())

	ctx := context.Background()
	keySpec, err := keyManager.CreateSSHKey(ctx, keyName, publicKey)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "\nError: %v\n", err)
		return
	}

	fmt.Fprintln(cmd.ErrOrStderr(), " done")
	fmt.Fprintln(cmd.ErrOrStderr())

	// Print the result
	printKeyDetails(cmd, keySpec)
}

func defaultSSHKeyPath() string {
	return "~/.ssh/id_ed25519.pub"
}

func expandHomePath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

func promptForSSHKeyPath(cmd *cobra.Command, missingPath string) (string, error) {
	fmt.Fprintf(cmd.ErrOrStderr(), "Default SSH key not found at %s\n", missingPath)
	fmt.Fprint(cmd.ErrOrStderr(), "Enter path to SSH public key: ")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no SSH key path provided")
	}

	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return "", fmt.Errorf("no SSH key path provided")
	}

	return input, nil
}

func readAndValidateSSHKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key file: %w", err)
	}

	publicKey := strings.TrimSpace(string(data))
	if publicKey == "" {
		return "", fmt.Errorf("SSH key file is empty")
	}

	// Basic validation: check that it looks like a public key
	if strings.Contains(publicKey, "PRIVATE KEY") {
		return "", fmt.Errorf("file appears to contain a private key; please provide the public key (.pub file)")
	}

	// Check for common public key prefixes
	validPrefixes := []string{"ssh-rsa", "ssh-ed25519", "ssh-dss", "ecdsa-sha2-"}
	isValid := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(publicKey, prefix) {
			isValid = true
			break
		}
	}

	if !isValid {
		return "", fmt.Errorf("file does not appear to be a valid SSH public key (expected ssh-rsa, ssh-ed25519, or ecdsa-sha2-*)")
	}

	return publicKey, nil
}

func suggestKeyName(path string) string {
	// Extract filename without extension
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Common substitutions
	if name == "id_ed25519" || name == "id_rsa" || name == "id_ecdsa" {
		// Try to get a more meaningful name from hostname
		if hostname, err := os.Hostname(); err == nil {
			return hostname
		}
	}

	return name
}

func printKeyDetails(cmd *cobra.Command, key *domain.SSHKeySpec) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SSH key added:")
	fmt.Fprintf(w, "  Name:\t%s\n", key.Name)
	fmt.Fprintf(w, "  Fingerprint:\t%s\n", key.Fingerprint)
	fmt.Fprintf(w, "  ID:\t%s\n", key.ID)
}

func printCommonSSHKeyPaths(cmd *cobra.Command) {
	fmt.Fprintln(cmd.ErrOrStderr(), "\nCommon SSH key paths:")
	fmt.Fprintln(cmd.ErrOrStderr(), "  ~/.ssh/id_ed25519.pub")
	fmt.Fprintln(cmd.ErrOrStderr(), "  ~/.ssh/id_rsa.pub")
	fmt.Fprintln(cmd.ErrOrStderr(), "  ~/.ssh/id_ecdsa.pub")
}
