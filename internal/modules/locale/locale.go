package locale

import (
	"crystallize-cli/internal/utils"
	"strings"
)

const (
	localeGenPath  = "/mnt/etc/locale.gen"
	localeConfPath = "/mnt/etc/locale.conf"
	defaultLocale  = "en_US.UTF-8"
	defaultKeymap  = "us"
)

// SetTimezone sets the system timezone
func SetTimezone(timezone string) error {
	utils.LogDebug("Setting timezone: %s", timezone)

	tzPath := "/usr/share/zoneinfo/" + timezone
	if err := utils.ExecChroot("ln", "-sf", tzPath, "/etc/localtime"); err != nil {
		return utils.NewErrorf("set timezone symlink: %w", err)
	}

	if err := utils.ExecChroot("hwclock", "--systohc"); err != nil {
		return utils.NewErrorf("sync hardware clock: %w", err)
	}

	return nil
}

// SetLocale configures system locale
func SetLocale(locale string) error {
	utils.LogDebug("Configuring locales: %s", locale)

	// Initialize locale.gen with default locale
	if err := initializeLocaleGen(); err != nil {
		return err
	}

	// Create and configure locale.conf
	if err := initializeLocaleConf(); err != nil {
		return err
	}

	// Add user-specified locales
	if err := addUserLocales(locale); err != nil {
		return err
	}

	// Generate locales
	if err := generateLocales(); err != nil {
		return err
	}

	return nil
}

// initializeLocaleGen ensures locale.gen has default locale
func initializeLocaleGen() error {
	utils.LogDebug("Initializing locale.gen")

	defaultEntry := defaultLocale + " UTF-8\n"
	if err := utils.AppendFile(localeGenPath, defaultEntry); err != nil {
		return utils.NewErrorf("add default locale to locale.gen: %w", err)
	}

	return nil
}

// initializeLocaleConf creates and configures locale.conf
func initializeLocaleConf() error {
	utils.LogDebug("Initializing locale.conf")

	if err := utils.CreateFile(localeConfPath); err != nil {
		return utils.NewErrorf("create locale.conf: %w", err)
	}

	langLine := "LANG=" + defaultLocale + "\n"
	if err := utils.AppendFile(localeConfPath, langLine); err != nil {
		return utils.NewErrorf("set LANG in locale.conf: %w", err)
	}

	return nil
}

// addUserLocales parses and adds user-specified locales
func addUserLocales(locale string) error {
	if locale == "" {
		return nil
	}

	utils.LogDebug("Adding user locales")

	locales := strings.Fields(locale)
	if len(locales) == 0 {
		return nil
	}

	// Process locales in pairs (name encoding)
	for i := 0; i+1 < len(locales); i += 2 {
		localeName := locales[i]
		localeEncoding := locales[i+1]

		if err := addLocale(localeName, localeEncoding); err != nil {
			return err
		}
	}

	return nil
}

// addLocale adds a single locale to locale.gen and updates locale.conf
func addLocale(name, encoding string) error {
	utils.LogDebug("Adding locale: %s %s", name, encoding)

	// Add to locale.gen
	localeEntry := name + " " + encoding + "\n"
	if err := utils.AppendFile(localeGenPath, localeEntry); err != nil {
		return utils.NewErrorf("add locale %s to locale.gen: %w", name, err)
	}

	// Update locale.conf if not default locale
	if name != defaultLocale {
		if err := utils.SedFile(localeConfPath, defaultLocale, name); err != nil {
			return utils.NewErrorf("set locale %s in locale.conf: %w", name, err)
		}
	}

	return nil
}

// generateLocales runs locale-gen to compile locales
func generateLocales() error {
	utils.LogDebug("Generating locales")

	if err := utils.ExecChroot("locale-gen"); err != nil {
		return utils.NewErrorf("generate locales: %w", err)
	}

	return nil
}

// SetKeyboard sets the keyboard layout
func SetKeyboard() error {
	utils.LogDebug("Setting keyboard layout: %s", defaultKeymap)

	if err := utils.ExecChroot("localectl", "set-x11-keymap", defaultKeymap); err != nil {
		return utils.NewErrorf("set X11 keymap: %w", err)
	}

	if err := utils.ExecChroot("localectl", "set-keymap", defaultKeymap); err != nil {
		return utils.NewErrorf("set console keymap: %w", err)
	}

	return nil
}
