package utils

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var ErrNoTerminal = errors.New("no interactive terminal")

const stdinAnnotation = "stdin"

func MarkStdinLine(cmd *cobra.Command, name string) error {
	return cmd.Flags().SetAnnotation(name, stdinAnnotation, []string{"line"})
}

func MarkStdinStream(cmd *cobra.Command, name string) error {
	return cmd.Flags().SetAnnotation(name, stdinAnnotation, []string{"stream"})
}

func ResolveStdin(cmd *cobra.Command) error {
	var target *pflag.Flag
	var mode string
	var err error
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		modes, ok := f.Annotations[stdinAnnotation]
		if !ok || len(modes) == 0 || !f.Changed || f.Value.String() != "-" {
			return
		}
		if target != nil {
			err = fmt.Errorf("only one flag can read stdin: --%s and --%s were both given -", target.Name, f.Name)
			return
		}
		target, mode = f, modes[0]
	})
	if err != nil || target == nil {
		return err
	}
	if StdinIsTerminal {
		return fmt.Errorf("--%s was given - but nothing is piped into stdin", target.Name)
	}

	var value string
	if mode == "line" {
		line, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		value = strings.TrimRight(line, "\r\n")
	} else {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return readErr
		}
		value = strings.TrimRight(string(data), "\r\n")
	}
	if value == "" {
		return fmt.Errorf("--%s was given - but stdin was empty", target.Name)
	}
	return target.Value.Set(value)
}

type inputModel struct {
	textInput textinput.Model
	done      bool
	value     string
	initCmd   tea.Cmd
}

func (m inputModel) Init() tea.Cmd { return m.initCmd }

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			m.value = m.textInput.Value()
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m inputModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.textInput.View())
}

func PromptInput(prompt, placeholder string) (string, error) {
	if !StdinIsTerminal {
		return "", ErrNoTerminal
	}
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = prompt + " "
	m := inputModel{textInput: ti, initCmd: ti.Focus()}

	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(final.(inputModel).value), nil
}

func PromptPassword(prompt string) (string, error) {
	if !StdinIsTerminal {
		return "", ErrNoTerminal
	}
	ti := textinput.New()
	ti.Placeholder = "••••••••"
	ti.Prompt = prompt + " "
	ti.EchoMode = textinput.EchoPassword
	m := inputModel{textInput: ti, initCmd: ti.Focus()}

	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return final.(inputModel).value, nil
}
