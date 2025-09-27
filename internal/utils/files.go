package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CreateFile creates a new file, creating parent directories if needed
func CreateFile(path string) error {
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	file, err := os.Create(path)
	if err != nil {
		LogError("Failed to create file %s: %v", path, err)
		return err
	}
	defer file.Close()

	LogInfo("Created file: %s", path)
	return nil
}

// CreateFileWithContent creates a file with initial content
func CreateFileWithContent(path, content string) error {
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		LogError("Failed to create file %s with content: %v", path, err)
		return err
	}

	LogInfo("Created file with content: %s", path)
	return nil
}

// CopyFile copies a file from source to destination
func CopyFile(src, dst string) error {
	if parent := filepath.Dir(dst); parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("failed to create destination parent directories: %w", err)
		}
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		LogError("Failed to open source file %s: %v", src, err)
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		LogError("Failed to create destination file %s: %v", dst, err)
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		LogError("Failed to copy %s to %s: %v", src, dst, err)
		return err
	}

	LogDebug("Copied %s to %s", src, dst)
	return nil
}

// AppendFile appends content to a file
func AppendFile(path, content string) error {
	if !Exists(path) {
		if err := CreateFile(path); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer file.Close()

	// Check if file is empty or ends with newline
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	var prefix string
	if stat.Size() > 0 {
		prefix = "\n"
	}

	content = strings.TrimSuffix(content, "\n")
	_, err = file.WriteString(fmt.Sprintf("%s%s\n", prefix, content))
	if err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	LogInfo("Appended content to file: %s", path)
	return nil
}

// WriteFile writes content to a file, overwriting existing content
func WriteFile(path, content string) error {
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		LogError("Failed to write to file %s: %v", path, err)
		return err
	}

	LogInfo("Wrote content to file: %s", path)
	return nil
}

// SedFile replaces text in a file
func SedFile(path, find, replace string) error {
	LogInfo("Replacing '%s' with '%s' in file %s", find, replace, path)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	newContent := strings.ReplaceAll(string(content), find, replace)

	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// SedFileRegex replaces text in a file using regex
func SedFileRegex(path, pattern, replace string) error {
	LogInfo("Replacing pattern '%s' with '%s' in file %s", pattern, replace, path)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	newContent := re.ReplaceAllString(string(content), replace)

	if newContent == string(content) {
		LogInfo("No matches found, file unchanged")
		return nil
	}

	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	LogInfo("File updated successfully")
	return nil
}

// CreateDirectory creates a directory and all parent directories
func CreateDirectory(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		LogError("Failed to create directory %s: %v", path, err)
		return err
	}
	LogDebug("Created directory: %s", path)
	return nil
}

// CopyDirectory copies a directory and all its contents recursively
func CopyDirectory(src, dst string) error {
	// Clean paths to handle trailing slashes
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	// Ensure source directory exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			LogInfo("Source directory does not exist: %s - skipping", src)
			return nil
		}
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := CopyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	LogInfo("Copied directory %s to %s", src, dst)
	return nil
}

// Exists checks if a file or directory exists
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsDirectory checks if path is a directory
func IsDirectory(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// IsFile checks if path is a file
func IsFile(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

// ReadFile reads file content as string
func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		LogError("Failed to read file %s: %v", path, err)
		return "", err
	}
	LogDebug("Read file: %s", path)
	return string(content), nil
}

// Remove removes a file or directory
func Remove(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			LogWarn("Path does not exist: %s", path)
			return nil
		}
		return fmt.Errorf("failed to stat path: %w", err)
	}

	if stat.IsDir() {
		err = os.RemoveAll(path)
		if err != nil {
			LogError("Failed to remove directory %s: %v", path, err)
			return err
		}
		LogInfo("Removed directory: %s", path)
	} else {
		err = os.Remove(path)
		if err != nil {
			LogError("Failed to remove file %s: %v", path, err)
			return err
		}
		LogInfo("Removed file: %s", path)
	}

	return nil
}

// SetPermissions sets file permissions
func SetPermissions(path string, mode os.FileMode) error {
	err := os.Chmod(path, mode)
	if err != nil {
		LogError("Failed to set permissions on %s: %v", path, err)
		return err
	}
	LogInfo("Set permissions %o on %s", mode, path)
	return nil
}

// MakeExecutable makes a file executable
func MakeExecutable(path string) error {
	return SetPermissions(path, 0755)
}

// ShouldSkipFile determines if a file should be skipped during copy operations
func ShouldSkipFile(filename string) bool {
	skipPatterns := []string{
		"s.dirmngr", // GnuPG temporary socket
		"S.dirmngr", // GnuPG temporary socket (capital S)
		"s.keyboxd", // GnuPG temporary socket
		"S.keyboxd", // GnuPG temporary socket (capital S)
		".#",        // Emacs temporary files
		"#",         // Various temporary files
		".lock",     // Lock files
		".tmp",      // Temporary files
		"~",         // Backup files
		".socket",   // Socket files
		".pid",      // Process ID files
		"gpg-agent", // GPG agent files
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}

// CopyFileIfExists copies a file only if the source exists
func CopyFileIfExists(src, dst string) error {
	if !Exists(src) {
		LogWarn("Source file does not exist: %s", src)
		return nil
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := CreateDirectory(dstDir); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	return CopyFile(src, dst)
}

// CopyDirectoryFiltered copies a directory and its contents, skipping problematic files.
func CopyDirectoryFiltered(src, dst string) error {
	if !Exists(src) {
		return fmt.Errorf("source directory does not exist: %s", src)
	}

	if err := CreateDirectory(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if ShouldSkipFile(entry.Name()) {
			continue
		}

		if entry.IsDir() {
			if err := CopyDirectoryFiltered(srcPath, dstPath); err != nil {
				LogWarn("Failed to copy subdirectory %s: %v", srcPath, err)
				continue
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				LogWarn("Failed to copy file %s: %v", srcPath, err)
				continue
			}
		}
	}

	return nil
}

// FilesEval evaluates file operation result and crashes on error
func FilesEval(err error, logMsg string) {
	if err != nil {
		Crash(fmt.Sprintf("%s ERROR: %v", logMsg, err), 1)
	} else {
		LogInfo("%s", logMsg)
	}
}
