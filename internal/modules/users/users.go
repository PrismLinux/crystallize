package users

import (
	"crystallize-cli/internal/utils"
	"os/exec"
	"strings"
)

// Shell represents supported shell types
type Shell string

const (
	ShellBash Shell = "bash"
	ShellZsh  Shell = "zsh"
	ShellFish Shell = "fish"
)

// shellConfig contains shell package and binary path
type shellConfig struct {
	pkg  string
	path string
}

var shellConfigs = map[Shell]shellConfig{
	ShellBash: {pkg: "bash", path: "/bin/bash"},
	ShellZsh:  {pkg: "zsh", path: "/usr/bin/zsh"},
	ShellFish: {pkg: "fish", path: "/usr/bin/fish"},
}

// NewUser creates a new user with specified configuration
func NewUser(username string, hasRoot bool, password string, doHashPass bool, shell string) error {
	finalPassword, err := preparePassword(password, doHashPass)
	if err != nil {
		return err
	}

	shellPath, err := installShell(shell)
	if err != nil {
		return err
	}

	if err := createUser(username, finalPassword, shellPath); err != nil {
		return err
	}

	if hasRoot {
		if err := grantRootAccess(username); err != nil {
			return err
		}
	}

	return nil
}

// preparePassword hashes password if needed
func preparePassword(password string, doHash bool) (string, error) {
	if !doHash {
		return password, nil
	}

	hashedPass, err := hashPassword(password)
	if err != nil {
		return "", utils.NewErrorf("hash password: %w", err)
	}

	return hashedPass, nil
}

// installShell installs the specified shell and returns its path
func installShell(shell string) (string, error) {
	shellType := Shell(shell)
	if shellType == "" {
		shellType = ShellBash
	}

	config, ok := shellConfigs[shellType]
	if !ok {
		config = shellConfigs[ShellBash]
		utils.LogWarn("Unknown shell '%s', defaulting to bash", shell)
	}

	utils.LogDebug("Installing shell: %s", config.pkg)
	if err := utils.Install([]string{config.pkg}); err != nil {
		return "", utils.NewErrorf("install shell %s: %w", config.pkg, err)
	}

	return config.path, nil
}

// createUser creates the user account
func createUser(username, password, shellPath string) error {
	utils.LogDebug("Creating user: %s with shell: %s", username, shellPath)

	return utils.ExecChroot("useradd", "-m", "-s", shellPath, "-p", strings.TrimSpace(password), username)
}

// grantRootAccess adds user to wheel group and configures sudo
func grantRootAccess(username string) error {
	utils.LogDebug("Granting root access to user: %s", username)

	// Add user to wheel group
	if err := utils.ExecChroot("usermod", "-aG", "wheel", username); err != nil {
		return utils.NewErrorf("add user to wheel group: %w", err)
	}

	// Configure sudoers
	if err := configureSudoers(); err != nil {
		return err
	}

	// Setup AccountsService for display managers
	if err := setupAccountsService(username); err != nil {
		return err
	}

	return nil
}

// configureSudoers configures sudo permissions for wheel group
func configureSudoers() error {
	utils.LogDebug("Configuring sudoers")

	// Enable wheel group
	if err := utils.SedFile("/mnt/etc/sudoers", "# %wheel ALL=(ALL:ALL) ALL", "%wheel ALL=(ALL:ALL) ALL"); err != nil {
		return utils.NewErrorf("enable wheel group in sudoers: %w", err)
	}

	// Add password feedback
	if err := utils.AppendFile("/mnt/etc/sudoers", "\nDefaults pwfeedback\n"); err != nil {
		return utils.NewErrorf("add pwfeedback to sudoers: %w", err)
	}

	return nil
}

// setupAccountsService creates AccountsService configuration for user
func setupAccountsService(username string) error {
	utils.LogDebug("Setting up AccountsService for user: %s", username)

	accountsDir := "/mnt/var/lib/AccountsService/users"
	if err := utils.CreateDirectory(accountsDir); err != nil {
		return utils.NewErrorf("create AccountsService directory: %w", err)
	}

	userFile := accountsDir + "/" + username
	if err := utils.CreateFile(userFile); err != nil {
		return utils.NewErrorf("create AccountsService user file: %w", err)
	}

	if err := utils.AppendFile(userFile, "[User]\nSession=plasma\n"); err != nil {
		return utils.NewErrorf("populate AccountsService user file: %w", err)
	}

	return nil
}

// hashPassword hashes a password using openssl
func hashPassword(password string) (string, error) {
	cmd := exec.Command("openssl", "passwd", "-1", password)
	output, err := cmd.Output()
	if err != nil {
		return "", utils.NewErrorf("execute openssl: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// RootPass sets the root password
func RootPass(password string) error {
	utils.LogDebug("Setting root password")

	if err := utils.ExecChroot("usermod", "--password", password, "root"); err != nil {
		return utils.NewErrorf("set root password: %w", err)
	}

	return nil
}
