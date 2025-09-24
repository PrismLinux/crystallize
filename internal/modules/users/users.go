package users

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"os/exec"
	"strings"
)

// NewUser creates a new user with specified configuration
func NewUser(username string, hasRoot bool, password string, doHashPass bool, shell string) error {
	finalPassword := password
	if doHashPass {
		hashedPass, err := hashPass(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		finalPassword = hashedPass
	}

	shellToInstall := "bash"
	switch shell {
	case "fish":
		shellToInstall = "fish"
	case "zsh":
		shellToInstall = "zsh"
	default:
		shellToInstall = "bash"
	}

	if err := utils.Install([]string{shellToInstall}); err != nil {
		return fmt.Errorf("failed to install shell %s: %w", shellToInstall, err)
	}

	shellPath := "/bin/bash"
	switch shell {
	case "fish":
		shellPath = "/usr/bin/fish"
	case "zsh":
		shellPath = "/usr/bin/zsh"
	}

	utils.ExecEval(
		utils.ExecChroot("useradd", "-m", "-s", shellPath, "-p", strings.TrimSpace(finalPassword), username),
		fmt.Sprintf("Create user %s", username),
	)

	if hasRoot {
		utils.ExecEval(
			utils.ExecChroot("usermod", "-aG", "wheel", username),
			fmt.Sprintf("Add user %s to wheel group", username),
		)

		utils.FilesEval(
			utils.SedFile("/mnt/etc/sudoers", "# %wheel ALL=(ALL:ALL) ALL", "%wheel ALL=(ALL:ALL) ALL"),
			"Add wheel group to sudoers",
		)

		utils.FilesEval(
			utils.AppendFile("/mnt/etc/sudoers", "\nDefaults pwfeedback\n"),
			"Add pwfeedback to sudoers",
		)

		utils.FilesEval(
			utils.CreateDirectory("/mnt/var/lib/AccountsService/users/"),
			"Create /mnt/var/lib/AccountsService",
		)

		if err := utils.CreateFile(fmt.Sprintf("/mnt/var/lib/AccountsService/users/%s", username)); err != nil {
			return fmt.Errorf("failed to create AccountsService user file: %w", err)
		}

		utils.FilesEval(
			utils.AppendFile(
				fmt.Sprintf("/mnt/var/lib/AccountsService/users/%s", username),
				"[User]\nSession=plasma\n",
			),
			fmt.Sprintf("Populate AccountsService user file for %s", username),
		)
	}

	return nil
}

// hashPass hashes a password using openssl
func hashPass(password string) (string, error) {
	cmd := exec.Command("openssl", "passwd", "-1", password)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute openssl: %w", err)
	}

	hash := strings.TrimSpace(string(output))
	return hash, nil
}

// RootPass sets the root password
func RootPass(rootPass string) {
	utils.ExecEval(
		utils.ExecChroot("usermod", "--password", rootPass, "root"),
		"set root password",
	)
}
