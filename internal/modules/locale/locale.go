package locale

import (
	"crystallize-cli/internal/utils"
	"fmt"
	"strings"
)

// SetTimezone sets the system timezone
func SetTimezone(timezone string) {
	utils.ExecEval(
		utils.ExecChroot("ln", "-sf", fmt.Sprintf("/usr/share/zoneinfo/%s", timezone), "/etc/localtime"),
		"Set timezone",
	)
	utils.ExecEval(utils.ExecChroot("hwclock", "--systohc"), "Set system clock")
}

// SetLocale configures system locale
func SetLocale(locale string) {
	utils.FilesEval(
		utils.AppendFile("/mnt/etc/locale.gen", "en_US.UTF-8 UTF-8"),
		"add en_US.UTF-8 UTF-8 to locale.gen",
	)
	_ = utils.CreateFile("/mnt/etc/locale.conf")
	utils.FilesEval(
		utils.AppendFile("/mnt/etc/locale.conf", "LANG=en_US.UTF-8"),
		"edit locale.conf",
	)

	locales := strings.Split(locale, " ")
	for i := 0; i < len(locales); i += 2 {
		if i+1 >= len(locales) {
			break
		}

		localeName := locales[i]
		localeEncoding := locales[i+1]

		utils.FilesEval(
			utils.AppendFile("/mnt/etc/locale.gen", fmt.Sprintf("%s %s\n", localeName, localeEncoding)),
			"add locales to locale.gen",
		)

		if localeName != "en_US.UTF-8" {
			utils.FilesEval(
				utils.SedFile("/mnt/etc/locale.conf", "en_US.UTF-8", localeName),
				fmt.Sprintf("Set locale %s in /etc/locale.conf", localeName),
			)
		}
	}

	utils.ExecEval(utils.ExecChroot("locale-gen"), "generate locales")
}

// SetKeyboard sets the keyboard layout
func SetKeyboard() {
	utils.ExecEval(
		utils.ExecChroot("localectl", "set-x11-keymap", "us"),
		"Set x11 keymap",
	)

	utils.ExecEval(
		utils.ExecChroot("localectl", "set-keymap", "us"),
		"Set global keymap",
	)
}
