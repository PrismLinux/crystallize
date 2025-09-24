package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

// InstallError represents different types of installation errors
type InstallError struct {
	Type    InstallErrorType
	Message string
	Package string
}

type InstallErrorType int

const (
	PackageNotFound InstallErrorType = iota
	DependencyConflict
	NetworkError
	DiskSpaceError
	PermissionError
	DatabaseError
	ValidationError
	IOError
	PasswordHashError
	UnknownError
)

func (e *InstallError) Error() string {
	switch e.Type {
	case PackageNotFound:
		return fmt.Sprintf("Package not found: %s", e.Package)
	case DependencyConflict:
		return fmt.Sprintf("Dependency conflict: %s", e.Message)
	case NetworkError:
		return fmt.Sprintf("Network error: %s", e.Message)
	case DiskSpaceError:
		return "Insufficient disk space"
	case PermissionError:
		return "Permission denied"
	case DatabaseError:
		return fmt.Sprintf("Database error: %s", e.Message)
	case ValidationError:
		return fmt.Sprintf("Validation error: %s", e.Message)
	case IOError:
		return fmt.Sprintf("I/O error: %s", e.Message)
	case PasswordHashError:
		return fmt.Sprintf("Password hashing error: %s", e.Message)
	default:
		return fmt.Sprintf("Unknown error: %s", e.Message)
	}
}

// SuggestSolution provides suggested solutions based on error type
func (e *InstallError) SuggestSolution() string {
	switch e.Type {
	case PackageNotFound:
		return fmt.Sprintf("Try:\n1. Check package name spelling: pacman -Ss %s\n2. Update databases: pacman -Sy\n3. Check if it's an AUR package", e.Package)
	case DependencyConflict:
		return "Try:\n1. Update system first: pacman -Syu\n2. Remove conflicting packages manually\n3. Use --overwrite flag if safe"
	case NetworkError:
		return "Try:\n1. Check internet connection\n2. Change mirror: reflector --latest 5 --sort rate --save /etc/pacman.d/mirrorlist\n3. Wait and retry later"
	case DiskSpaceError:
		return "Try:\n1. Free up disk space: pacman -Sc\n2. Check available space: df -h\n3. Clean package cache"
	case PermissionError:
		return "Try:\n1. Check if running as root\n2. Verify mount points are correct\n3. Check filesystem permissions"
	case DatabaseError:
		return "Try:\n1. Refresh databases: pacman -Sy\n2. Update keyring: pacman -S archlinux-keyring\n3. Clear cache: pacman -Sc"
	default:
		return "Check the log file for detailed error information and try installing packages individually."
	}
}

// ProgressConfig configures progress tracking
type ProgressConfig struct {
	ShowETA         bool
	DetailedLogging bool
	ShowDownload    bool
	SeparatePhases  bool
}

// NewProgressConfig creates default progress configuration
func NewProgressConfig() *ProgressConfig {
	return &ProgressConfig{
		ShowETA:         true,
		DetailedLogging: true,
		ShowDownload:    true,
		SeparatePhases:  false,
	}
}

// ProgressPhase represents the current installation phase
type ProgressPhase int

const (
	PhaseDownload ProgressPhase = iota
	PhaseInstall
	PhaseComplete
)

// ProgressTracker tracks installation progress across phases
type ProgressTracker struct {
	CurrentPhase    ProgressPhase
	DownloadBar     *progressbar.ProgressBar
	InstallBar      *progressbar.ProgressBar
	CombinedBar     *progressbar.ProgressBar
	PackageCount    int
	DownloadedCount int64
	InstalledCount  int64
	Config          *ProgressConfig
	mu              sync.Mutex
	lastUpdateTime  time.Time
	smoothingWindow []int
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(description string, packageCount int, config *ProgressConfig) *ProgressTracker {
	tracker := &ProgressTracker{
		CurrentPhase: PhaseDownload,
		PackageCount: packageCount,
		Config:       config,
	}

	if !config.ShowETA {
		return tracker
	}

	if config.SeparatePhases && packageCount > 0 {
		// Create separate progress bars for download and install
		tracker.DownloadBar = progressbar.NewOptions(packageCount,
			progressbar.OptionSetDescription(fmt.Sprintf("%s (downloading)", description)),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionSetWidth(50),
			progressbar.OptionClearOnFinish(), // Clear when finished
		)

		tracker.InstallBar = progressbar.NewOptions(packageCount,
			progressbar.OptionSetDescription(fmt.Sprintf("%s (installing)", description)),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionSetWidth(50),
			progressbar.OptionClearOnFinish(), // Clear when finished
		)
	} else if packageCount > 0 {
		// Create combined progress bar with better ETA prediction
		totalSteps := packageCount
		if config.ShowDownload {
			totalSteps = packageCount * 2 // Download + Install phases
		}

		tracker.CombinedBar = progressbar.NewOptions(totalSteps,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(800*time.Millisecond),
			progressbar.OptionSetWidth(60),    // Slightly wider for better formatting
			progressbar.OptionClearOnFinish(), // Clear when finished to prevent overlap
			progressbar.OptionEnableColorCodes(true),
		)
	} else {
		// Use spinner for unknown counts
		tracker.CombinedBar = progressbar.NewOptions(-1,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionClearOnFinish(), // Clear spinner when done
		)
	}

	return tracker
}

// ProcessLine processes a line of output for progress information
func (pt *ProgressTracker) ProcessLine(line string) {
	if !pt.Config.ShowETA {
		return
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	lineLower := strings.ToLower(line)

	// Check for download-related patterns
	if pt.Config.ShowDownload && pt.CurrentPhase == PhaseDownload {
		if pt.checkDownloadProgress(line, lineLower) {
			return
		}
	}

	// Check for installation-related patterns
	if pt.checkInstallProgress(line, lineLower) {
		return
	}

	// Check for phase transitions
	pt.checkPhaseTransition(lineLower)
}

// checkDownloadProgress checks for download progress indicators
func (pt *ProgressTracker) checkDownloadProgress(line, lineLower string) bool {
	// Pacstrap/pacman download patterns
	downloadPatterns := []string{
		"downloading",
		"retrieving file",
		"fetching",
		":: retrieving packages",
		"resolving dependencies",
		"looking for conflicting packages",
		"packages to install:",
	}

	// Check for download phase indicators
	for _, pattern := range downloadPatterns {
		if strings.Contains(lineLower, pattern) {
			// Extract progress percentage if available
			if percent := extractProgressPercentage(line); percent >= 0 {
				pt.updateDownloadProgress(percent)
				return true
			}
			return true // We're in download phase but no specific progress
		}
	}

	// Look for package download completion patterns
	// Pattern: "package-name-version.pkg.tar.xz     100%"
	if strings.Contains(lineLower, ".pkg.tar") {
		if percent := extractProgressPercentage(line); percent >= 0 {
			if percent == 100 {
				// Package download completed
				newCount := atomic.AddInt64(&pt.DownloadedCount, 1)
				if newCount <= int64(pt.PackageCount) {
					pt.updateDownloadProgress(int(float64(newCount) / float64(pt.PackageCount) * 100))
					return true
				}
			} else {
				// Package download in progress
				pt.updateDownloadProgress(percent)
				return true
			}
		}
	}

	// Pattern: "downloading package-name..."
	if strings.Contains(lineLower, "downloading ") {
		newCount := atomic.AddInt64(&pt.DownloadedCount, 1)
		if newCount <= int64(pt.PackageCount) {
			pt.updateDownloadProgress(int(float64(newCount) / float64(pt.PackageCount) * 100))
			return true
		}
	}

	return false
}

// checkInstallProgress checks for installation progress indicators
func (pt *ProgressTracker) checkInstallProgress(line, lineLower string) bool {
	// Pacstrap/pacman installation patterns
	installPatterns := []struct {
		pattern string
		isStart bool // true if this indicates start of installation for a package
	}{
		{"installing", true},
		{"upgrading", true},
		{"reinstalling", true},
		{":: processing package changes", false},
		{":: running pre-transaction hooks", false},
		{":: running post-transaction hooks", false},
	}

	for _, p := range installPatterns {
		if strings.Contains(lineLower, p.pattern) {
			if p.isStart {
				// This indicates a specific package installation
				if pt.extractAndCountPackage(line) {
					return true
				}
			} else {
				// General installation phase indicator
				if pt.CurrentPhase == PhaseDownload {
					pt.transitionToInstall()
				}
				return true
			}
		}
	}

	// Look for specific pacstrap patterns
	if strings.Contains(lineLower, "==>") &&
		(strings.Contains(lineLower, "installing") || strings.Contains(lineLower, "upgrading")) {
		if pt.extractAndCountPackage(line) {
			return true
		}
	}

	return false
}

// extractAndCountPackage extracts package name and updates install progress
func (pt *ProgressTracker) extractAndCountPackage(line string) bool {
	// Patterns to match various installation messages
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(installing|upgrading|reinstalling)\s+([^\s\.]+)`), // "installing package-name"
		regexp.MustCompile(`==> (installing|upgrading)\s+([^\s]+)`),                // "==> installing package-name"
		regexp.MustCompile(`\[\d+/\d+\]\s+(installing|upgrading)\s+([^\s]+)`),      // "[1/10] installing package-name"
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 2 {
			newCount := atomic.AddInt64(&pt.InstalledCount, 1)
			if newCount <= int64(pt.PackageCount) {
				pt.updateInstallProgress(int(newCount))
				return true
			}
		}
	}

	// Fallback: look for package installation indicators without regex
	lineLower := strings.ToLower(line)
	if (strings.Contains(lineLower, "installing") ||
		strings.Contains(lineLower, "upgrading") ||
		strings.Contains(lineLower, "reinstalling")) &&
		!strings.Contains(lineLower, "packages") { // Avoid "installing packages" header

		newCount := atomic.AddInt64(&pt.InstalledCount, 1)
		if newCount <= int64(pt.PackageCount) {
			pt.updateInstallProgress(int(newCount))
			return true
		}
	}

	return false
}

// checkPhaseTransition checks if we're transitioning between phases
func (pt *ProgressTracker) checkPhaseTransition(lineLower string) {
	if pt.CurrentPhase == PhaseDownload {
		// Transition patterns from download to install for pacstrap
		transitionPatterns := []string{
			":: processing package changes",
			":: running pre-transaction hooks",
			"checking keyring",
			"checking package integrity",
			"loading package files",
			"checking available disk space",
			":: installing packages",
			"==> installing",
		}

		for _, pattern := range transitionPatterns {
			if strings.Contains(lineLower, pattern) {
				pt.transitionToInstall()
				return
			}
		}
	}
}

// updateDownloadProgress updates download progress with smoothing
func (pt *ProgressTracker) updateDownloadProgress(percent int) {
	now := time.Now()

	// Throttle updates to prevent ETA jumping around
	if now.Sub(pt.lastUpdateTime) < 500*time.Millisecond {
		return
	}
	pt.lastUpdateTime = now

	// Smooth the progress to prevent wild ETA swings
	smoothedProgress := pt.smoothProgress(percent)

	if pt.Config.SeparatePhases && pt.DownloadBar != nil {
		pt.DownloadBar.Set(int(float64(smoothedProgress) * float64(pt.PackageCount) / 100))
	} else if pt.CombinedBar != nil && pt.PackageCount > 0 {
		// First half of combined progress is for downloads
		progress := int(float64(smoothedProgress) * float64(pt.PackageCount) / 100)
		pt.CombinedBar.Set(progress)
	}
}

// updateInstallProgress updates installation progress with smoothing
func (pt *ProgressTracker) updateInstallProgress(count int) {
	now := time.Now()

	// Throttle updates to prevent ETA jumping around
	if now.Sub(pt.lastUpdateTime) < 500*time.Millisecond {
		return
	}
	pt.lastUpdateTime = now

	if pt.Config.SeparatePhases && pt.InstallBar != nil {
		pt.InstallBar.Set(count)
	} else if pt.CombinedBar != nil && pt.PackageCount > 0 {
		if pt.Config.ShowDownload {
			// Second half of combined progress is for installation
			pt.CombinedBar.Set(pt.PackageCount + count)
		} else {
			pt.CombinedBar.Set(count)
		}
	}
}

// smoothProgress applies simple moving average to smooth progress updates
func (pt *ProgressTracker) smoothProgress(newProgress int) int {
	// Add to smoothing window
	pt.smoothingWindow = append(pt.smoothingWindow, newProgress)

	// Keep only last 5 values for smoothing
	if len(pt.smoothingWindow) > 5 {
		pt.smoothingWindow = pt.smoothingWindow[1:]
	}

	// Calculate average
	sum := 0
	for _, val := range pt.smoothingWindow {
		sum += val
	}

	return sum / len(pt.smoothingWindow)
}

// transitionToInstall transitions from download to install phase
func (pt *ProgressTracker) transitionToInstall() {
	if pt.CurrentPhase != PhaseDownload {
		return
	}

	pt.CurrentPhase = PhaseInstall

	if pt.Config.SeparatePhases {
		if pt.DownloadBar != nil {
			pt.DownloadBar.Finish()
			pt.DownloadBar.Clear() // Clear to prevent overlap
			fmt.Println()          // Clean separation between phases
		}
	} else if pt.CombinedBar != nil && pt.Config.ShowDownload {
		// Ensure download phase shows as complete
		pt.CombinedBar.Set(pt.PackageCount)
		// Add a small delay to let the progress bar render
		time.Sleep(100 * time.Millisecond)
	}
}

// Finish completes all progress bars
func (pt *ProgressTracker) Finish() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.Config.SeparatePhases {
		if pt.DownloadBar != nil {
			pt.DownloadBar.Finish()
			pt.DownloadBar.Clear() // Clear the bar
			fmt.Println()          // Add spacing
		}
		if pt.InstallBar != nil {
			pt.InstallBar.Finish()
			pt.InstallBar.Clear() // Clear the bar
			fmt.Println()         // Add spacing
		}
	} else if pt.CombinedBar != nil {
		pt.CombinedBar.Finish()
		pt.CombinedBar.Clear() // Clear the bar to prevent overlap
		fmt.Println()          // Add clean line break
	}
}

// KeepSpinnerAlive keeps the spinner moving for indeterminate progress
func (pt *ProgressTracker) KeepSpinnerAlive(cmd *exec.Cmd) *time.Ticker {
	if pt.CombinedBar != nil && pt.PackageCount == 0 {
		ticker := time.NewTicker(200 * time.Millisecond)
		go func() {
			for range ticker.C {
				if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
					return
				}
				pt.CombinedBar.Add(0)
			}
		}()
		return ticker
	}
	return nil
}

// extractProgressPercentage extracts percentage from various progress formats
func extractProgressPercentage(line string) int {
	// Multiple regex patterns for different progress formats
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d+)%`),          // "50%"
		regexp.MustCompile(`\[(\d+)%\]`),      // "[50%]"
		regexp.MustCompile(`\((\d+)%\)`),      // "(50%)"
		regexp.MustCompile(`(\d+)/(\d+)`),     // "5/10" (convert to percentage)
		regexp.MustCompile(`\((\d+)/(\d+)\)`), // "(5/10)"
		regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:of|/)\s*(\d+(?:\.\d+)?)\s*(?:MB|KB|GB|MiB|KiB|GiB|bytes?)`), // Size progress
		regexp.MustCompile(`(\d+(?:\.\d+)?[KMGT]?i?B)\s*/\s*(\d+(?:\.\d+)?[KMGT]?i?B)`),                      // "1.2MB / 5.6MB"
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
			if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
				if len(matches) > 2 {
					// Handle fraction format like "5/10" or size ratios
					if total, err := strconv.ParseFloat(matches[2], 64); err == nil && total > 0 {
						percent := int(val / total * 100)
						if percent >= 0 && percent <= 100 {
							return percent
						}
					}
				} else {
					// Handle direct percentage
					percent := int(val)
					if percent >= 0 && percent <= 100 {
						return percent
					}
				}
			}
		}
	}

	return -1 // No valid percentage found
}

// validatePackages checks package names for validity
func validatePackages(pkgs []string) error {
	for _, pkg := range pkgs {
		if pkg == "" || strings.Contains(pkg, " ") || strings.HasPrefix(pkg, "-") {
			return &InstallError{
				Type:    ValidationError,
				Message: fmt.Sprintf("Invalid package name: %s", pkg),
				Package: pkg,
			}
		}
	}
	return nil
}

// parseErrorFromOutput analyzes command output to determine specific error type
func parseErrorFromOutput(exitCode int, logPath string) *InstallError {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Command failed with exit code %d. Could not read log at %s", exitCode, logPath),
		}
	}

	output := strings.ToLower(string(content))
	lines := strings.Split(output, "\n")

	// Check for specific error patterns
	if strings.Contains(output, "target not found") || strings.Contains(output, "package not found") {
		for _, line := range lines {
			if strings.Contains(line, "target not found") {
				if idx := strings.Index(line, "target not found: "); idx != -1 {
					pkgName := strings.Fields(line[idx+18:])[0]
					return &InstallError{
						Type:    PackageNotFound,
						Package: pkgName,
					}
				}
			}
		}
		return &InstallError{
			Type:    PackageNotFound,
			Package: "unknown package",
		}
	}

	if strings.Contains(output, "conflicting dependencies") ||
		strings.Contains(output, "dependency cycle") ||
		strings.Contains(output, "conflicts with") {
		for _, line := range lines {
			if strings.Contains(line, "conflict") {
				return &InstallError{
					Type:    DependencyConflict,
					Message: line,
				}
			}
		}
		return &InstallError{
			Type:    DependencyConflict,
			Message: "Unknown dependency conflict",
		}
	}

	if strings.Contains(output, "no space left") ||
		strings.Contains(output, "disk full") ||
		strings.Contains(output, "insufficient space") {
		return &InstallError{Type: DiskSpaceError}
	}

	if strings.Contains(output, "permission denied") ||
		strings.Contains(output, "operation not permitted") {
		return &InstallError{Type: PermissionError}
	}

	if strings.Contains(output, "failed retrieving file") ||
		strings.Contains(output, "connection timed out") ||
		strings.Contains(output, "temporary failure in name resolution") ||
		strings.Contains(output, "could not resolve host") {
		return &InstallError{
			Type:    NetworkError,
			Message: "Failed to download packages. Check network connection.",
		}
	}

	if strings.Contains(output, "database") && strings.Contains(output, "corrupt") {
		return &InstallError{
			Type:    DatabaseError,
			Message: "Package database is corrupted. Run pacman -Sy to refresh.",
		}
	}

	if strings.Contains(output, "signature") && strings.Contains(output, "invalid") {
		return &InstallError{
			Type:    DatabaseError,
			Message: "Invalid package signatures. Update keyring or refresh databases.",
		}
	}

	return &InstallError{
		Type:    UnknownError,
		Message: fmt.Sprintf("Command failed with exit code %d. Check log at %s", exitCode, logPath),
	}
}

// InstallBase installs packages using pacstrap
func InstallBase(pkgs []string) error {
	return InstallBaseWithConfig(pkgs, NewProgressConfig())
}

// InstallBaseWithConfig installs packages using pacstrap with custom config
func InstallBaseWithConfig(pkgs []string, config *ProgressConfig) error {
	if len(pkgs) == 0 {
		return nil
	}

	if err := validatePackages(pkgs); err != nil {
		return err
	}

	LogInfo("Installing base packages: %s", strings.Join(pkgs, ", "))

	// Create temporary log file
	logFile, err := os.CreateTemp("", "crystallize-install-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	// Build command
	args := append([]string{"/mnt"}, pkgs...)
	cmd := exec.Command("pacstrap", args...)

	return execWithProgressTracker(cmd, "Install base packages", logFile, config, len(pkgs))
}

// Install installs packages in chroot environment
func Install(pkgs []string) error {
	return InstallWithConfig(pkgs, NewProgressConfig())
}

// InstallWithConfig installs packages in chroot with custom configuration
func InstallWithConfig(pkgs []string, config *ProgressConfig) error {
	if len(pkgs) == 0 {
		return nil
	}

	if err := validatePackages(pkgs); err != nil {
		return err
	}

	if config.DetailedLogging {
		LogInfo("Installing packages in chroot: %s", strings.Join(pkgs, ", "))
	}

	// Create temporary log file
	logFile, err := os.CreateTemp("", "crystallize-chroot-install-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	// Build command
	args := append([]string{"/mnt", "pacman", "-S", "--noconfirm", "--needed"}, pkgs...)
	cmd := exec.Command("arch-chroot", args...)

	return execWithProgressTracker(cmd, "Install packages", logFile, config, len(pkgs))
}

// UpdateDatabases updates package databases
func UpdateDatabases() error {
	return UpdateDatabasesWithConfig(NewProgressConfig())
}

// UpdateDatabasesWithConfig updates databases with custom configuration
func UpdateDatabasesWithConfig(config *ProgressConfig) error {
	if config.DetailedLogging {
		LogInfo("Updating package databases")
	}

	logFile, err := os.CreateTemp("", "crystallize-update-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	cmd := exec.Command("arch-chroot", "/mnt", "pacman", "-Sy", "--noconfirm")

	return execWithProgressTracker(cmd, "Update package databases", logFile, config, 0)
}

// UpgradeSystem upgrades system packages
func UpgradeSystem() error {
	return UpgradeSystemWithConfig(NewProgressConfig())
}

// UpgradeSystemWithConfig upgrades system with custom configuration
func UpgradeSystemWithConfig(config *ProgressConfig) error {
	if config.DetailedLogging {
		LogInfo("Upgrading system packages")
	}

	logFile, err := os.CreateTemp("", "crystallize-upgrade-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	cmd := exec.Command("arch-chroot", "/mnt", "pacman", "-Syu", "--noconfirm")

	return execWithProgressTracker(cmd, "Upgrade system packages", logFile, config, 0)
}

// execWithProgressTracker executes a command with the new progress tracker
func execWithProgressTracker(cmd *exec.Cmd, description string, logFile *os.File, config *ProgressConfig, packageCount int) error {
	tracker := NewProgressTracker(description, packageCount, config)
	defer tracker.Finish()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create stdout pipe: %v", err),
		}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create stderr pipe: %v", err),
		}
	}

	if err := cmd.Start(); err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to start process: %v", err),
		}
	}

	// Keep spinner moving for indeterminate progress
	ticker := tracker.KeepSpinnerAlive(cmd)
	if ticker != nil {
		defer ticker.Stop()
	}

	// Using error group pattern
	wg := &errgroup.Group{}

	// Handle stdout
	wg.Go(func() error {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if config.DetailedLogging {
				fmt.Fprintln(logFile, line)
			}
			tracker.ProcessLine(line)
		}
		return scanner.Err()
	})

	// Handle stderr
	wg.Go(func() error {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if config.DetailedLogging {
				fmt.Fprintln(logFile, line)
			}
			tracker.ProcessLine(line)
		}
		return scanner.Err()
	})

	// Wait for all goroutines to complete
	if err := wg.Wait(); err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Error reading command output: %v", err),
		}
	}

	if err := cmd.Wait(); err != nil {
		var exitCode int
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
		return parseErrorFromOutput(exitCode, logFile.Name())
	}

	if config.DetailedLogging {
		LogInfo("%s completed successfully", description)
	}

	return nil
}

// CheckInstalled checks if packages are already installed
func CheckInstalled(pkgs []string) ([]string, error) {
	var notInstalled []string

	for _, pkg := range pkgs {
		result, err := ExecChrootWithOutput("pacman", "-Q", pkg)
		if err != nil || result.ExitCode != 0 {
			notInstalled = append(notInstalled, pkg)
		}
	}

	return notInstalled, nil
}
