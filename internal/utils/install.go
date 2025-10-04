package utils

import (
	"bufio"
	"fmt"
	"io"
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

// Error types and handling

type ErrorType int

const (
	ErrPackageNotFound ErrorType = iota
	ErrDependencyConflict
	ErrNetwork
	ErrDiskSpace
	ErrPermission
	ErrDatabase
	ErrValidation
	ErrIO
	ErrPasswordHash
	ErrUnknown
)

type InstallError struct {
	Type    ErrorType
	Message string
	Package string
}

func (e *InstallError) Error() string {
	messages := map[ErrorType]string{
		ErrPackageNotFound:    fmt.Sprintf("Package not found: %s", e.Package),
		ErrDependencyConflict: fmt.Sprintf("Dependency conflict: %s", e.Message),
		ErrNetwork:            fmt.Sprintf("Network error: %s", e.Message),
		ErrDiskSpace:          "Insufficient disk space",
		ErrPermission:         "Permission denied",
		ErrDatabase:           fmt.Sprintf("Database error: %s", e.Message),
		ErrValidation:         fmt.Sprintf("Validation error: %s", e.Message),
		ErrIO:                 fmt.Sprintf("I/O error: %s", e.Message),
		ErrPasswordHash:       fmt.Sprintf("Password hashing error: %s", e.Message),
	}

	if msg, ok := messages[e.Type]; ok {
		return msg
	}
	return fmt.Sprintf("Unknown error: %s", e.Message)
}

func (e *InstallError) SuggestSolution() string {
	solutions := map[ErrorType]string{
		ErrPackageNotFound: fmt.Sprintf(
			"Try:\n1. Check package name spelling: pacman -Ss %s\n2. Update databases: pacman -Sy\n3. Check if it's an AUR package",
			e.Package,
		),
		ErrDependencyConflict: "Try:\n1. Update system first: pacman -Syu\n2. Remove conflicting packages manually\n3. Use --overwrite flag if safe",
		ErrNetwork:            "Try:\n1. Check internet connection\n2. Change mirror: reflector --latest 5 --sort rate --save /etc/pacman.d/mirrorlist\n3. Wait and retry later",
		ErrDiskSpace:          "Try:\n1. Free up disk space: pacman -Sc\n2. Check available space: df -h\n3. Clean package cache",
		ErrPermission:         "Try:\n1. Check if running as root\n2. Verify mount points are correct\n3. Check filesystem permissions",
		ErrDatabase:           "Try:\n1. Refresh databases: pacman -Sy\n2. Update keyring: pacman -S archlinux-keyring\n3. Clear cache: pacman -Sc",
	}

	if solution, ok := solutions[e.Type]; ok {
		return solution
	}
	return "Check the log file for detailed error information and try installing packages individually."
}

// Progress tracking

type Phase int

const (
	PhaseDownload Phase = iota
	PhaseInstall
	PhaseComplete
)

type ProgressConfig struct {
	ShowETA         bool
	DetailedLogging bool
	ShowDownload    bool
	SeparatePhases  bool
	ThrottleMS      int
	BarWidth        int
}

func NewProgressConfig() *ProgressConfig {
	return &ProgressConfig{
		ShowETA:         true,
		DetailedLogging: true,
		ShowDownload:    true,
		SeparatePhases:  false,
		ThrottleMS:      500,
		BarWidth:        60,
	}
}

type ProgressTracker struct {
	phase           Phase
	downloadBar     *progressbar.ProgressBar
	installBar      *progressbar.ProgressBar
	combinedBar     *progressbar.ProgressBar
	packageCount    int
	downloadedCount atomic.Int64
	installedCount  atomic.Int64
	config          *ProgressConfig
	mu              sync.Mutex
	lastUpdate      time.Time
	smoothingWindow []int
}

func NewProgressTracker(description string, packageCount int, config *ProgressConfig) *ProgressTracker {
	tracker := &ProgressTracker{
		phase:        PhaseDownload,
		packageCount: packageCount,
		config:       config,
	}

	if !config.ShowETA {
		return tracker
	}

	if config.SeparatePhases && packageCount > 0 {
		tracker.initSeparateBars(description, packageCount, config)
	} else {
		tracker.initCombinedBar(description, packageCount, config)
	}

	return tracker
}

func (pt *ProgressTracker) initSeparateBars(description string, count int, config *ProgressConfig) {
	throttle := time.Duration(config.ThrottleMS) * time.Millisecond

	pt.downloadBar = progressbar.NewOptions(count,
		progressbar.OptionSetDescription(fmt.Sprintf("%s (download)", description)),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("pkg"),
		progressbar.OptionThrottle(throttle),
		progressbar.OptionSetWidth(config.BarWidth),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionEnableColorCodes(true),
	)

	pt.installBar = progressbar.NewOptions(count,
		progressbar.OptionSetDescription(fmt.Sprintf("%s (install)", description)),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("pkg"),
		progressbar.OptionThrottle(throttle),
		progressbar.OptionSetWidth(config.BarWidth),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionEnableColorCodes(true),
	)
}

func (pt *ProgressTracker) initCombinedBar(description string, count int, config *ProgressConfig) {
	throttle := time.Duration(config.ThrottleMS) * time.Millisecond

	if count > 0 {
		totalSteps := count
		if config.ShowDownload {
			totalSteps *= 2
		}

		pt.combinedBar = progressbar.NewOptions(totalSteps,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowCount(),
			progressbar.OptionShowIts(),
			progressbar.OptionSetItsString("pkg"),
			progressbar.OptionThrottle(throttle),
			progressbar.OptionSetWidth(config.BarWidth),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "█",
				SaucerHead:    "█",
				SaucerPadding: "░",
				BarStart:      "│",
				BarEnd:        "│",
			}),
		)
	} else {
		pt.combinedBar = progressbar.NewOptions(-1,
			progressbar.OptionSetDescription(description),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(throttle),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionEnableColorCodes(true),
		)
	}
}

func (pt *ProgressTracker) ProcessLine(line string) {
	if !pt.config.ShowETA {
		return
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	lineLower := strings.ToLower(line)

	if pt.config.ShowDownload && pt.phase == PhaseDownload {
		if pt.processDownloadLine(line, lineLower) {
			return
		}
	}

	if pt.processInstallLine(line, lineLower) {
		return
	}

	pt.checkPhaseTransition(lineLower)
}

func (pt *ProgressTracker) processDownloadLine(line, lineLower string) bool {
	downloadKeywords := []string{
		"downloading", "retrieving file", "fetching", ":: retrieving packages",
		"resolving dependencies", "looking for conflicting packages", "packages to install:",
	}

	for _, keyword := range downloadKeywords {
		if !strings.Contains(lineLower, keyword) {
			continue
		}

		if percent := extractPercentage(line); percent >= 0 {
			pt.updateDownload(percent)
			return true
		}
		return true
	}

	if strings.Contains(lineLower, ".pkg.tar") {
		if percent := extractPercentage(line); percent >= 0 {
			if percent == 100 {
				pt.incrementDownloadCount()
			} else {
				pt.updateDownload(percent)
			}
			return true
		}
	}

	if strings.Contains(lineLower, "downloading ") {
		pt.incrementDownloadCount()
		return true
	}

	return false
}

func (pt *ProgressTracker) processInstallLine(line, lineLower string) bool {
	installPatterns := []struct {
		keyword     string
		shouldCount bool
	}{
		{"installing", true},
		{"upgrading", true},
		{"reinstalling", true},
		{":: processing package changes", false},
		{":: running pre-transaction hooks", false},
		{":: running post-transaction hooks", false},
	}

	for _, p := range installPatterns {
		if !strings.Contains(lineLower, p.keyword) {
			continue
		}

		if p.shouldCount {
			if pt.extractAndCountPackage(line) {
				return true
			}
		} else {
			if pt.phase == PhaseDownload {
				pt.transitionToInstall()
			}
			return true
		}
	}

	if strings.Contains(lineLower, "==>") &&
		(strings.Contains(lineLower, "installing") || strings.Contains(lineLower, "upgrading")) {
		return pt.extractAndCountPackage(line)
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
			pt.incrementInstallCount()
			return true
		}
	}

	lineLower := strings.ToLower(line)
	if (strings.Contains(lineLower, "installing") ||
		strings.Contains(lineLower, "upgrading") ||
		strings.Contains(lineLower, "reinstalling")) &&
		!strings.Contains(lineLower, "packages") {
		pt.incrementInstallCount()
		return true
	}

	return false
}

func (pt *ProgressTracker) incrementDownloadCount() {
	newCount := pt.downloadedCount.Add(1)
	if int(newCount) <= pt.packageCount {
		percent := int(float64(newCount) / float64(pt.packageCount) * 100)
		pt.updateDownload(percent)
	}
}

func (pt *ProgressTracker) incrementInstallCount() {
	newCount := pt.installedCount.Add(1)
	if int(newCount) <= pt.packageCount {
		pt.updateInstall(int(newCount))
	}
}

func (pt *ProgressTracker) updateDownload(percent int) {
	if !pt.shouldUpdate() {
		return
	}

	smoothed := pt.smoothProgress(percent)

	if pt.config.SeparatePhases && pt.downloadBar != nil {
		progress := int(float64(smoothed) * float64(pt.packageCount) / 100)
		pt.downloadBar.Set(progress)
	} else if pt.combinedBar != nil && pt.packageCount > 0 {
		progress := int(float64(smoothed) * float64(pt.packageCount) / 100)
		pt.combinedBar.Set(progress)
	}
}

func (pt *ProgressTracker) updateInstall(count int) {
	if !pt.shouldUpdate() {
		return
	}

	if pt.config.SeparatePhases && pt.installBar != nil {
		pt.installBar.Set(count)
	} else if pt.combinedBar != nil && pt.packageCount > 0 {
		offset := 0
		if pt.config.ShowDownload {
			offset = pt.packageCount
		}
		pt.combinedBar.Set(offset + count)
	}
}

func (pt *ProgressTracker) shouldUpdate() bool {
	now := time.Now()
	throttle := time.Duration(pt.config.ThrottleMS) * time.Millisecond

	if now.Sub(pt.lastUpdate) < throttle {
		return false
	}

	pt.lastUpdate = now
	return true
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

func (pt *ProgressTracker) checkPhaseTransition(lineLower string) {
	if pt.phase != PhaseDownload {
		return
	}

	transitionKeywords := []string{
		":: processing package changes",
		":: running pre-transaction hooks",
		"checking keyring",
		"checking package integrity",
		"loading package files",
		"checking available disk space",
		":: installing packages",
		"==> installing",
	}

	for _, keyword := range transitionKeywords {
		if strings.Contains(lineLower, keyword) {
			pt.transitionToInstall()
			return
		}
	}
}

func (pt *ProgressTracker) transitionToInstall() {
	if pt.phase != PhaseDownload {
		return
	}

	pt.phase = PhaseInstall

	if pt.config.SeparatePhases {
		if pt.downloadBar != nil {
			pt.downloadBar.Finish()
			pt.downloadBar.Clear()
			fmt.Println()
		}
	} else if pt.combinedBar != nil && pt.config.ShowDownload {
		pt.combinedBar.Set(pt.packageCount)
		time.Sleep(100 * time.Millisecond)
	}
}

func (pt *ProgressTracker) Finish() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.config.SeparatePhases {
		pt.finishBar(pt.downloadBar)
		pt.finishBar(pt.installBar)
	} else {
		pt.finishBar(pt.combinedBar)
	}
}

func (pt *ProgressTracker) finishBar(bar *progressbar.ProgressBar) {
	if bar != nil {
		bar.Finish()
		bar.Clear()
		fmt.Println()
	}
}

func (pt *ProgressTracker) KeepSpinnerAlive(cmd *exec.Cmd) *time.Ticker {
	if pt.combinedBar == nil || pt.packageCount != 0 {
		return nil
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	go func() {
		for range ticker.C {
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				return
			}
			pt.combinedBar.Add(0)
		}
	}()
	return ticker
}

// Package validation and installation

func ValidatePackages(pkgs []string) error {
	for _, pkg := range pkgs {
		if pkg == "" || strings.Contains(pkg, " ") || strings.HasPrefix(pkg, "-") {
			return &InstallError{
				Type:    ErrValidation,
				Message: fmt.Sprintf("Invalid package name: %s", pkg),
				Package: pkg,
			}
		}
	}
	return nil
}

func InstallBase(pkgs []string) error {
	return InstallBaseWithConfig(pkgs, NewProgressConfig())
}

func InstallBaseWithConfig(pkgs []string, config *ProgressConfig) error {
	if len(pkgs) == 0 {
		return nil
	}

	if err := ValidatePackages(pkgs); err != nil {
		return err
	}

	LogInfo("Installing base packages: %s", strings.Join(pkgs, ", "))

	logFile, err := createTempLog("crystallize-install-*.log")
	if err != nil {
		return err
	}
	defer cleanupLog(logFile)

	args := append([]string{"/mnt"}, pkgs...)
	cmd := exec.Command("pacstrap", args...)

	return executeWithProgress(cmd, "Install base packages", logFile, config, len(pkgs))
}

func Install(pkgs []string) error {
	return InstallWithConfig(pkgs, NewProgressConfig())
}

func InstallWithConfig(pkgs []string, config *ProgressConfig) error {
	if len(pkgs) == 0 {
		return nil
	}

	if err := ValidatePackages(pkgs); err != nil {
		return err
	}

	if config.DetailedLogging {
		LogInfo("Installing packages in chroot: %s", strings.Join(pkgs, ", "))
	}

	logFile, err := createTempLog("crystallize-chroot-install-*.log")
	if err != nil {
		return err
	}
	defer cleanupLog(logFile)

	args := append([]string{"/mnt", "pacman", "-S", "--noconfirm", "--needed"}, pkgs...)
	cmd := exec.Command("arch-chroot", args...)

	return executeWithProgress(cmd, "Install packages", logFile, config, len(pkgs))
}

func UpdateDatabases() error {
	return UpdateDatabasesWithConfig(NewProgressConfig())
}

func UpdateDatabasesWithConfig(config *ProgressConfig) error {
	if config.DetailedLogging {
		LogInfo("Updating package databases")
	}

	logFile, err := createTempLog("crystallize-update-*.log")
	if err != nil {
		return err
	}
	defer cleanupLog(logFile)

	cmd := exec.Command("arch-chroot", "/mnt", "pacman", "-Sy", "--noconfirm")
	return executeWithProgress(cmd, "Update package databases", logFile, config, 0)
}

func UpgradeSystem() error {
	return UpgradeSystemWithConfig(NewProgressConfig())
}

func UpgradeSystemWithConfig(config *ProgressConfig) error {
	if config.DetailedLogging {
		LogInfo("Upgrading system packages")
	}

	logFile, err := createTempLog("crystallize-upgrade-*.log")
	if err != nil {
		return err
	}
	defer cleanupLog(logFile)

	cmd := exec.Command("arch-chroot", "/mnt", "pacman", "-Syu", "--noconfirm")
	return executeWithProgress(cmd, "Upgrade system packages", logFile, config, 0)
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

// File operations with filtering

var skipPatterns = []string{
	"s.dirmngr", "S.dirmngr", ".#", "#", ".lock", ".tmp", "~",
}

func CopyFileWithFilter(src, dst string) error {
	filename := filepath.Base(src)

	for _, pattern := range skipPatterns {
		if strings.Contains(filename, pattern) {
			LogWarn("Skipping temporary/problematic file: %s", src)
			return nil
		}
	}

	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			LogWarn("Source file does not exist: %s", src)
			return nil
		}
		return fmt.Errorf("cannot access source file %s: %w", src, err)
	}

	return CopyFile(src, dst)
}

func CopyDirectoryWithFilter(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			LogWarn("Error accessing path %s: %v", path, err)
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if shouldSkip(info) {
			LogWarn("Skipping temporary file/directory: %s", path)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return CreateDirectory(dstPath)
		}

		return CopyFileWithFilter(path, dstPath)
	})
}

func shouldSkip(info os.FileInfo) bool {
	filename := info.Name()
	for _, pattern := range skipPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}
	return false
}

// Helper functions

func createTempLog(pattern string) (*os.File, error) {
	logFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, &InstallError{
			Type:    ErrIO,
			Message: fmt.Sprintf("Failed to create temp log file: %v", err),
		}
	}
	return logFile, nil
}

func cleanupLog(logFile *os.File) {
	if logFile != nil {
		logFile.Close()
		os.Remove(logFile.Name())
	}
}

func executeWithProgress(cmd *exec.Cmd, description string, logFile *os.File, config *ProgressConfig, packageCount int) error {
	tracker := NewProgressTracker(description, packageCount, config)
	defer tracker.Finish()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &InstallError{Type: ErrIO, Message: fmt.Sprintf("Failed to create stdout pipe: %v", err)}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &InstallError{Type: ErrIO, Message: fmt.Sprintf("Failed to create stderr pipe: %v", err)}
	}

	if err := cmd.Start(); err != nil {
		return &InstallError{Type: ErrIO, Message: fmt.Sprintf("Failed to start process: %v", err)}
	}

	ticker := tracker.KeepSpinnerAlive(cmd)
	if ticker != nil {
		defer ticker.Stop()
	}

	wg := &errgroup.Group{}

	wg.Go(func() error {
		return scanOutput(stdout, logFile, tracker, config)
	})

	wg.Go(func() error {
		return scanOutput(stderr, logFile, tracker, config)
	})

	if err := wg.Wait(); err != nil {
		return &InstallError{Type: ErrIO, Message: fmt.Sprintf("Error reading command output: %v", err)}
	}

	if err := cmd.Wait(); err != nil {
		exitCode := getExitCode(err)
		return parseError(exitCode, logFile.Name())
	}

	if config.DetailedLogging {
		LogInfo("%s completed successfully", description)
	}
	return nil
}

func scanOutput(reader io.Reader, logFile *os.File, tracker *ProgressTracker, config *ProgressConfig) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if config.DetailedLogging {
			fmt.Fprintln(logFile, line)
		}
		tracker.ProcessLine(line)
	}
	return scanner.Err()
}

func getExitCode(err error) int {
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	return 1
}

func extractPercentage(line string) int {
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
		matches := pattern.FindStringSubmatch(line)
		if len(matches) <= 1 {
			continue
		}

		val, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			continue
		}

		if len(matches) > 2 {
			total, err := strconv.ParseFloat(matches[2], 64)
			if err == nil && total > 0 {
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
	return -1
}

func parseError(exitCode int, logPath string) *InstallError {
	content, err := os.ReadFile(logPath)
	if err != nil {
		return &InstallError{
			Type:    ErrIO,
			Message: fmt.Sprintf("Command failed with exit code %d. Could not read log at %s", exitCode, logPath),
		}
	}

	output := strings.ToLower(string(content))
	lines := strings.Split(output, "\n")

	errorChecks := []struct {
		keywords []string
		handler  func([]string) *InstallError
	}{
		{
			keywords: []string{"target not found", "package not found"},
			handler:  func(lines []string) *InstallError { return parsePackageNotFound(lines) },
		},
		{
			keywords: []string{"conflicting dependencies", "dependency cycle", "conflicts with"},
			handler:  func(lines []string) *InstallError { return parseDependencyConflict(lines) },
		},
		{
			keywords: []string{"no space left", "disk full", "insufficient space"},
			handler:  func(lines []string) *InstallError { return &InstallError{Type: ErrDiskSpace} },
		},
		{
			keywords: []string{"permission denied", "operation not permitted"},
			handler:  func(lines []string) *InstallError { return &InstallError{Type: ErrPermission} },
		},
		{
			keywords: []string{"failed retrieving file", "connection timed out", "temporary failure in name resolution", "could not resolve host"},
			handler: func(lines []string) *InstallError {
				return &InstallError{Type: ErrNetwork, Message: "Failed to download packages. Check network connection."}
			},
		},
		{
			keywords: []string{"database", "corrupt"},
			handler: func(lines []string) *InstallError {
				return &InstallError{Type: ErrDatabase, Message: "Package database is corrupted. Run pacman -Sy to refresh."}
			},
		},
		{
			keywords: []string{"signature", "invalid"},
			handler: func(lines []string) *InstallError {
				return &InstallError{Type: ErrDatabase, Message: "Invalid package signatures. Update keyring or refresh databases."}
			},
		},
	}

	for _, check := range errorChecks {
		if containsAnyKeyword(output, check.keywords) {
			return check.handler(lines)
		}
	}

	return &InstallError{
		Type:    ErrUnknown,
		Message: fmt.Sprintf("Command failed with exit code %d. Check log at %s", exitCode, logPath),
	}
}

func containsAnyKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func parsePackageNotFound(lines []string) *InstallError {
	for _, line := range lines {
		if !strings.Contains(line, "target not found") {
			continue
		}

		if idx := strings.Index(line, "target not found: "); idx != -1 {
			fields := strings.Fields(line[idx+18:])
			if len(fields) > 0 {
				return &InstallError{Type: ErrPackageNotFound, Package: fields[0]}
			}
		}
	}
	return &InstallError{Type: ErrPackageNotFound, Package: "unknown package"}
}

func parseDependencyConflict(lines []string) *InstallError {
	for _, line := range lines {
		if strings.Contains(line, "conflict") {
			return &InstallError{Type: ErrDependencyConflict, Message: line}
		}
	}
	return &InstallError{Type: ErrDependencyConflict, Message: "Unknown dependency conflict"}
}
