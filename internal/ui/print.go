package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// NoColor disables ANSI codes. Set from --no-color flag or NO_COLOR env var.
var NoColor bool

// Quiet suppresses all output except errors.
var Quiet bool

// Out is the writer for all output. Defaults to os.Stdout.
var Out io.Writer = os.Stdout

const width = 72

// ANSI color codes
const (
	green = "\033[32m"
	amber = "\033[33m"
	red   = "\033[31m"
	gray  = "\033[90m"
	reset = "\033[0m"
)

func color(c, msg string) string {
	if NoColor {
		return msg
	}
	return c + msg + reset
}

func write(s string) {
	fmt.Fprint(Out, s)
}

// Success prints a success line: "  ✓  msg"
func Success(msg string) {
	if Quiet {
		return
	}
	write(fmt.Sprintf("  %s  %s\n", color(green, "✓"), msg))
}

// Error prints an error line with hint: "  ✗  msg\n     hint"
func Error(msg string, hint string) {
	write(fmt.Sprintf("  %s  %s\n", color(red, "✗"), msg))
	if hint != "" {
		write(fmt.Sprintf("     %s\n", color(gray, hint)))
	}
}

// Warning prints a warning line: "  –  msg"
func Warning(msg string) {
	if Quiet {
		return
	}
	write(fmt.Sprintf("  %s  %s\n", color(amber, "–"), msg))
}

// Dim prints dimmed text: "     msg"
func Dim(msg string) {
	if Quiet {
		return
	}
	write(fmt.Sprintf("     %s\n", color(gray, msg)))
}

// Line prints a separator line.
func Line() {
	if Quiet {
		return
	}
	write(fmt.Sprintf("  %s\n", color(gray, strings.Repeat("─", width-2))))
}

// Blank prints a blank line.
func Blank() {
	if Quiet {
		return
	}
	write("\n")
}

// Section prints an aligned label-value pair: "  label    value"
func Section(label, value string) {
	if Quiet {
		return
	}
	padded := fmt.Sprintf("%-14s", label)
	write(fmt.Sprintf("  %s%s\n", padded, value))
}

// Header prints a boxed header with separator lines.
func Header(title string) {
	if Quiet {
		return
	}
	Line()
	write(fmt.Sprintf("   %s\n", title))
	Line()
}

// Plain prints unformatted text with 2-space indent.
func Plain(msg string) {
	if Quiet {
		return
	}
	write(fmt.Sprintf("  %s\n", msg))
}

func init() {
	if os.Getenv("NO_COLOR") != "" {
		NoColor = true
	}
}
