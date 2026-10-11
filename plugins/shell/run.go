package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Prepare builds the command line for a command, in the shell the settings
// name. It is the reference's CreateProcessStartInfo: what each shell is
// given, and how it is told to stay open or wait for a key.
func Prepare(command string, settings Settings, elevated bool, closePrompt string) (Command, error) {
	command = strings.TrimSpace(command)

	prepared := Command{
		WorkingDirectory: homeDirectory(),
		UseShellExecute:  true,
	}
	if elevated || settings.RunAsAdministrator {
		prepared.Verb = "runas"
	}

	switch settings.Shell {
	case ShellCmd:
		configureCmd(&prepared, command, settings, closePrompt)
	case ShellPowershell:
		configurePowershell(&prepared, command, settings, closePrompt, "powershell.exe")
	case ShellPwsh:
		configurePowershell(&prepared, command, settings, closePrompt, "pwsh.exe")
	case ShellRunCommand:
		configureRun(&prepared, command)
	case ShellCustomTemplate:
		configureCustom(&prepared, command, settings.CustomTemplateShellConfig)
	default:
		configureCmd(&prepared, command, settings, closePrompt)
	}

	return prepared, nil
}

// configureCmd runs the command with cmd.exe.
func configureCmd(prepared *Command, command string, settings Settings, closePrompt string) {
	// /k keeps the window open after the command, /c closes it.
	shellSwitch := "/c"
	if settings.LeaveShellOpen {
		shellSwitch = "/k"
	}

	commandToRun := command
	if settings.CloseShellAfterPress {
		commandToRun = fmt.Sprintf("%s && echo %s && pause > nul", command, closePrompt)
	}

	if settings.UseWindowsTerminal {
		prepared.FileName = "wt.exe"
		prepared.Arguments = []string{"cmd", shellSwitch, commandToRun}
		return
	}
	prepared.FileName = "cmd.exe"
	// The command line is passed whole rather than as arguments, so that a
	// quoted command reaches cmd.exe without being escaped again.
	prepared.Arguments = []string{shellSwitch, commandToRun}
}

// configurePowershell runs the command with PowerShell, or pwsh.
func configurePowershell(prepared *Command, command string, settings Settings, closePrompt, program string) {
	escape := ""
	if settings.UseWindowsTerminal {
		// Windows Terminal treats a semicolon as a new tab, so the ones in
		// the command have to be escaped.
		escape = "\\"
		prepared.FileName = "wt.exe"
		prepared.Arguments = append(prepared.Arguments, strings.TrimSuffix(program, ".exe"))
	} else {
		prepared.FileName = program
	}

	if settings.LeaveShellOpen {
		prepared.Arguments = append(prepared.Arguments, "-NoExit", command)
		return
	}

	commandToRun := command + escape + ";"
	if settings.CloseShellAfterPress {
		commandToRun += fmt.Sprintf(" Write-Host '%s'%s; [System.Console]::ReadKey()%s; exit",
			closePrompt, escape, escape)
	}
	prepared.Arguments = append(prepared.Arguments, "-Command", commandToRun)
}

// configureRun runs the command as the Run box does: the program first, the
// rest of the line as its arguments.
func configureRun(prepared *Command, command string) {
	filename, arguments := splitRunCommand(command)

	if filename != "" && existsInPath(filename) {
		prepared.FileName = filename
		prepared.Arguments = []string{strings.TrimLeft(arguments, " ")}
		if prepared.Arguments[0] == "" {
			prepared.Arguments = nil
		}
		return
	}
	// A command that names no program is passed on whole, so that the
	// system says what is wrong with it.
	prepared.FileName = command
	prepared.Arguments = nil
}

// splitRunCommand cuts a command line into the program and its arguments,
// keeping a quoted path with spaces together.
func splitRunCommand(command string) (filename, arguments string) {
	if strings.HasPrefix(command, "\"") {
		parts := strings.SplitN(command[1:], "\"", 2)
		if len(parts) > 1 {
			return parts[0], parts[1]
		}
		return "", ""
	}
	parts := strings.SplitN(command, " ", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

// configureCustom runs the command with a shell of the user's own.
func configureCustom(prepared *Command, command string, template CustomTemplate) {
	if strings.TrimSpace(template.ExecutablePath) != "" {
		prepared.FileName = strings.Trim(strings.TrimSpace(template.ExecutablePath), "\"")
	}
	if strings.TrimSpace(template.ArgumentsTemplate) != "" {
		arguments := strings.ReplaceAll(strings.TrimSpace(template.ArgumentsTemplate), "{command}", command)
		prepared.Arguments = []string{arguments}
	}
}

// existsInPath reports whether a program can be found by its name.
func existsInPath(filename string) bool {
	if filename == "" {
		return false
	}
	if filepath.IsAbs(filename) {
		return fileExists(filename)
	}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if fileExists(filepath.Join(directory, filename)) ||
			fileExists(filepath.Join(directory, filename+".exe")) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// readDirectory lists what is inside a directory, with a separator after a
// directory's name so that the completion can carry on into it.
func readDirectory(directory string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += string(filepath.Separator)
		}
		names = append(names, name)
	}
	return names, nil
}

// homeDirectory is where a command starts, as the reference starts one.
func homeDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// systemRunner runs commands on the machine.
type systemRunner struct{}

// Run starts a command line.
func (systemRunner) Run(command Command) error {
	return start(command, false)
}

// RunAsDifferentUser starts one as another user, which the system asks a
// password for.
func (systemRunner) RunAsDifferentUser(command Command) error {
	return start(command, true)
}

// ExpandEnvironmentVariables replaces %VAR% with what the machine has.
func (systemRunner) ExpandEnvironmentVariables(value string) string {
	return os.ExpandEnv(expandWindowsVariables(value))
}

// start runs a command line.
func start(command Command, differentUser bool) error {
	if command.FileName == "" {
		return fmt.Errorf("no program to run")
	}

	program := exec.Command(command.FileName, command.Arguments...)
	program.Dir = command.WorkingDirectory
	if command.Verb != "" || differentUser {
		// Running as an administrator or as another user is the shell's
		// own business, so it is asked for rather than done here.
		program = exec.Command("cmd.exe", "/c", "start", "", command.FileName)
	}
	return program.Start()
}

// expandWindowsVariables replaces %VAR% with what the machine has, which
// os.ExpandEnv does not do because it wants $VAR.
func expandWindowsVariables(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '%' {
			out.WriteByte(value[i])
			i++
			continue
		}
		end := strings.IndexByte(value[i+1:], '%')
		if end < 0 {
			out.WriteByte(value[i])
			i++
			continue
		}
		name := value[i+1 : i+1+end]
		out.WriteString(os.Getenv(name))
		i += end + 2
	}
	return out.String()
}

// writeFile writes a file, for the tests that need one to complete a path
// with.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
