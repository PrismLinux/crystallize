package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
)

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

type ProgressConfig struct {
	ShowETA         bool
	DetailedLogging bool
	ShowDownload    bool
	SeparatePhases  bool
}

func NewProgressConfig() *ProgressConfig {
	return &ProgressConfig{
		ShowETA:         true,
		DetailedLogging: true,
		ShowDownload:    true,
		SeparatePhases:  false,
	}
}

type ProgressPhase int

const (
	PhaseDownload ProgressPhase = iota
	PhaseInstall
	PhaseComplete
)

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
		tracker.DownloadBar = progressbar.NewOptions(packageCount,
			progressbar.OptionSetDescription(fmt.Sprintf("%s (downloading)", description)),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionSetWidth(50),
			progressbar.OptionClearOnFinish(),
		)

		tracker.InstallBar = progressbar.NewOptions(packageCount,
			progressbar.OptionSetDescription(fmt.Sprintf("%s (installing)", description)),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionSetWidth(50),
			progressbar.OptionClearOnFinish(),
		)
	} else if packageCount > 0 {
		totalSteps := packageCount
		if config.ShowDownload {
			totalSteps = packageCount * 2
		}

		tracker.CombinedBar = progressbar.NewOptions(totalSteps,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(800*time.Millisecond),
			progressbar.OptionSetWidth(60),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionEnableColorCodes(true),
		)
	} else {
		tracker.CombinedBar = progressbar.NewOptions(-1,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(500*time.Millisecond),
			progressbar.OptionClearOnFinish(),
		)
	}

	return tracker
}

func (pt *ProgressTracker) ProcessLine(line string) {
	if !pt.Config.ShowETA {
		return
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	lineLower := strings.ToLower(line)

	if pt.Config.ShowDownload && pt.CurrentPhase == PhaseDownload {
		if pt.checkDownloadProgress(line, lineLower) {
			return
		}
	}

	if pt.checkInstallProgress(line, lineLower) {
		return
	}

	pt.checkPhaseTransition(lineLower)
}

func (pt *ProgressTracker) checkDownloadProgress(line, lineLower string) bool {
	downloadPatterns := []string{
		"downloading", "retrieving file", "fetching", ":: retrieving packages",
		"resolving dependencies", "looking for conflicting packages", "packages to install:",
	}

	for _, pattern := range downloadPatterns {
		if strings.Contains(lineLower, pattern) {
			if percent := extractProgressPercentage(line); percent >= 0 {
				pt.updateDownloadProgress(percent)
				return true
			}
			return true
		}
	}

	if strings.Contains(lineLower, ".pkg.tar") {
		if percent := extractProgressPercentage(line); percent >= 0 {
			if percent == 100 {
				newCount := atomic.AddInt64(&pt.DownloadedCount, 1)
				if newCount <= int64(pt.PackageCount) {
					pt.updateDownloadProgress(int(float64(newCount) / float64(pt.PackageCount) * 100))
					return true
				}
			} else {
				pt.updateDownloadProgress(percent)
				return true
			}
		}
	}

	if strings.Contains(lineLower, "downloading ") {
		newCount := atomic.AddInt64(&pt.DownloadedCount, 1)
		if newCount <= int64(pt.PackageCount) {
			pt.updateDownloadProgress(int(float64(newCount) / float64(pt.PackageCount) * 100))
			return true
		}
	}

	return false
}

func (pt *ProgressTracker) checkInstallProgress(line, lineLower string) bool {
	installPatterns := []struct {
		pattern string
		isStart bool
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
				if pt.extractAndCountPackage(line) {
					return true
				}
			} else {
				if pt.CurrentPhase == PhaseDownload {
					pt.transitionToInstall()
				}
				return true
			}
		}
	}

	if strings.Contains(lineLower, "==>") &&
		(strings.Contains(lineLower, "installing") || strings.Contains(lineLower, "upgrading")) {
		if pt.extractAndCountPackage(line) {
			return true
		}
	}

	return false
}

func (pt *ProgressTracker) extractAndCountPackage(line string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(installing|upgrading|reinstalling)\s+([^\s\.]+)`),
		regexp.MustCompile(`==> (installing|upgrading)\s+([^\s]+)`),
		regexp.MustCompile(`\[\d+/\d+\]\s+(installing|upgrading)\s+([^\s]+)`),
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

	lineLower := strings.ToLower(line)
	if (strings.Contains(lineLower, "installing") ||
		strings.Contains(lineLower, "upgrading") ||
		strings.Contains(lineLower, "reinstalling")) &&
		!strings.Contains(lineLower, "packages") {

		newCount := atomic.AddInt64(&pt.InstalledCount, 1)
		if newCount <= int64(pt.PackageCount) {
			pt.updateInstallProgress(int(newCount))
			return true
		}
	}

	return false
}

func (pt *ProgressTracker) checkPhaseTransition(lineLower string) {
	if pt.CurrentPhase == PhaseDownload {
		transitionPatterns := []string{
			":: processing package changes", ":: running pre-transaction hooks",
			"checking keyring", "checking package integrity", "loading package files",
			"checking available disk space", ":: installing packages", "==> installing",
		}

		for _, pattern := range transitionPatterns {
			if strings.Contains(lineLower, pattern) {
				pt.transitionToInstall()
				return
			}
		}
	}
}

func (pt *ProgressTracker) updateDownloadProgress(percent int) {
	now := time.Now()
	if now.Sub(pt.lastUpdateTime) < 500*time.Millisecond {
		return
	}
	pt.lastUpdateTime = now

	smoothedProgress := pt.smoothProgress(percent)

	if pt.Config.SeparatePhases && pt.DownloadBar != nil {
		pt.DownloadBar.Set(int(float64(smoothedProgress) * float64(pt.PackageCount) / 100))
	} else if pt.CombinedBar != nil && pt.PackageCount > 0 {
		progress := int(float64(smoothedProgress) * float64(pt.PackageCount) / 100)
		pt.CombinedBar.Set(progress)
	}
}

func (pt *ProgressTracker) updateInstallProgress(count int) {
	now := time.Now()
	if now.Sub(pt.lastUpdateTime) < 500*time.Millisecond {
		return
	}
	pt.lastUpdateTime = now

	if pt.Config.SeparatePhases && pt.InstallBar != nil {
		pt.InstallBar.Set(count)
	} else if pt.CombinedBar != nil && pt.PackageCount > 0 {
		if pt.Config.ShowDownload {
			pt.CombinedBar.Set(pt.PackageCount + count)
		} else {
			pt.CombinedBar.Set(count)
		}
	}
}

func (pt *ProgressTracker) smoothProgress(newProgress int) int {
	pt.smoothingWindow = append(pt.smoothingWindow, newProgress)
	if len(pt.smoothingWindow) > 5 {
		pt.smoothingWindow = pt.smoothingWindow[1:]
	}

	sum := 0
	for _, val := range pt.smoothingWindow {
		sum += val
	}
	return sum / len(pt.smoothingWindow)
}

func (pt *ProgressTracker) transitionToInstall() {
	if pt.CurrentPhase != PhaseDownload {
		return
	}

	pt.CurrentPhase = PhaseInstall

	if pt.Config.SeparatePhases {
		if pt.DownloadBar != nil {
			pt.DownloadBar.Finish()
			pt.DownloadBar.Clear()
			fmt.Println()
		}
	} else if pt.CombinedBar != nil && pt.Config.ShowDownload {
		pt.CombinedBar.Set(pt.PackageCount)
		time.Sleep(100 * time.Millisecond)
	}
}

func (pt *ProgressTracker) Finish() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.Config.SeparatePhases {
		if pt.DownloadBar != nil {
			pt.DownloadBar.Finish()
			pt.DownloadBar.Clear()
			fmt.Println()
		}
		if pt.InstallBar != nil {
			pt.InstallBar.Finish()
			pt.InstallBar.Clear()
			fmt.Println()
		}
	} else if pt.CombinedBar != nil {
		pt.CombinedBar.Finish()
		pt.CombinedBar.Clear()
		fmt.Println()
	}
}

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

func extractProgressPercentage(line string) int {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d+)%`),
		regexp.MustCompile(`\[(\d+)%\]`),
		regexp.MustCompile(`\((\d+)%\)`),
		regexp.MustCompile(`(\d+)/(\d+)`),
		regexp.MustCompile(`\((\d+)/(\d+)\)`),
		regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:of|/)\s*(\d+(?:\.\d+)?)\s*(?:MB|KB|GB|MiB|KiB|GiB|bytes?)`),
		regexp.MustCompile(`(\d+(?:\.\d+)?[KMGT]?i?B)\s*/\s*(\d+(?:\.\d+)?[KMGT]?i?B)`),
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
			if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
				if len(matches) > 2 {
					if total, err := strconv.ParseFloat(matches[2], 64); err == nil && total > 0 {
						percent := int(val / total * 100)
						if percent >= 0 && percent <= 100 {
							return percent
						}
					}
				} else {
					percent := int(val)
					if percent >= 0 && percent <= 100 {
						return percent
					}
				}
			}
		}
	}
	return -1
}

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

	if strings.Contains(output, "target not found") || strings.Contains(output, "package not found") {
		for _, line := range lines {
			if strings.Contains(line, "target not found") {
				if idx := strings.Index(line, "target not found: "); idx != -1 {
					pkgName := strings.Fields(line[idx+18:])[0]
					return &InstallError{Type: PackageNotFound, Package: pkgName}
				}
			}
		}
		return &InstallError{Type: PackageNotFound, Package: "unknown package"}
	}

	if strings.Contains(output, "conflicting dependencies") ||
		strings.Contains(output, "dependency cycle") ||
		strings.Contains(output, "conflicts with") {
		for _, line := range lines {
			if strings.Contains(line, "conflict") {
				return &InstallError{Type: DependencyConflict, Message: line}
			}
		}
		return &InstallError{Type: DependencyConflict, Message: "Unknown dependency conflict"}
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

// Fixed copy functions that handle temporary files properly
func CopyFileWithFilter(src, dst string) error {
	// Skip temporary and problematic files
	filename := filepath.Base(src)
	skipPatterns := []string{
		"s.dirmngr", // GnuPG temporary socket
		"S.dirmngr", // GnuPG temporary socket (capital S)
		".#",        // Emacs temporary files
		"#",         // Various temporary files
		".lock",     // Lock files
		".tmp",      // Temporary files
		"~",         // Backup files
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(filename, pattern) {
			LogWarn("Skipping temporary/problematic file: %s", src)
			return nil
		}
	}

	// Check if source exists and is readable
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			LogWarn("Source file does not exist: %s", src)
			return nil // Don't error on missing files
		}
		return fmt.Errorf("cannot access source file %s: %w", src, err)
	}

	return CopyFile(src, dst)
}

func CopyDirectoryWithFilter(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			LogWarn("Error accessing path %s: %v", path, err)
			return nil // Continue with other files
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip problematic files and directories
		filename := info.Name()
		skipPatterns := []string{
			"s.dirmngr", "S.dirmngr", ".#", "#", ".lock", ".tmp", "~",
		}

		for _, pattern := range skipPatterns {
			if strings.Contains(filename, pattern) {
				LogWarn("Skipping temporary file/directory: %s", path)
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if info.IsDir() {
			return CreateDirectory(dstPath)
		}

		return CopyFileWithFilter(path, dstPath)
	})
}

func InstallBase(pkgs []string) error {
	return InstallBaseWithConfig(pkgs, NewProgressConfig())
}

func InstallBaseWithConfig(pkgs []string, config *ProgressConfig) error {
	if len(pkgs) == 0 {
		return nil
	}

	if err := validatePackages(pkgs); err != nil {
		return err
	}

	LogInfo("Installing base packages: %s", strings.Join(pkgs, ", "))

	logFile, err := os.CreateTemp("", "crystallize-install-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	args := append([]string{"/mnt"}, pkgs...)
	cmd := exec.Command("pacstrap", args...)

	return execWithProgressTracker(cmd, "Install base packages", logFile, config, len(pkgs))
}

func Install(pkgs []string) error {
	return InstallWithConfig(pkgs, NewProgressConfig())
}

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

	logFile, err := os.CreateTemp("", "crystallize-chroot-install-*.log")
	if err != nil {
		return &InstallError{
			Type:    IOError,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	defer os.Remove(logFile.Name())
	defer logFile.Close()

	args := append([]string{"/mnt", "pacman", "-S", "--noconfirm", "--needed"}, pkgs...)
	cmd := exec.Command("arch-chroot", args...)

	return execWithProgressTracker(cmd, "Install packages", logFile, config, len(pkgs))
}

func UpdateDatabases() error {
	return UpdateDatabasesWithConfig(NewProgressConfig())
}

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

func UpgradeSystem() error {
	return UpgradeSystemWithConfig(NewProgressConfig())
}

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

func execWithProgressTracker(cmd *exec.Cmd, description string, logFile *os.File, config *ProgressConfig, packageCount int) error {
	tracker := NewProgressTracker(description, packageCount, config)
	defer tracker.Finish()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &InstallError{Type: IOError, Message: fmt.Sprintf("Failed to create stdout pipe: %v", err)}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &InstallError{Type: IOError, Message: fmt.Sprintf("Failed to create stderr pipe: %v", err)}
	}

	if err := cmd.Start(); err != nil {
		return &InstallError{Type: IOError, Message: fmt.Sprintf("Failed to start process: %v", err)}
	}

	ticker := tracker.KeepSpinnerAlive(cmd)
	if ticker != nil {
		defer ticker.Stop()
	}

	wg := &errgroup.Group{}

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

	if err := wg.Wait(); err != nil {
		return &InstallError{Type: IOError, Message: fmt.Sprintf("Error reading command output: %v", err)}
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
