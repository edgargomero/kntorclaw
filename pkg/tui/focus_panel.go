package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) buildFocusFilesPanel() *tview.List {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkSlateGray))
	list.SetBorder(true).SetTitle(" FILES [F1] ")

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		entries := gitStatus()
		if index >= 0 && index < len(entries) {
			a.updateFocusDiff(entries[index].Path)
		}
	})

	return list
}

func (a *App) buildFocusBranchesPanel() *tview.List {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkSlateGray))
	list.SetBorder(true).SetTitle(" BRANCHES [F2] ")
	return list
}

func (a *App) buildFocusCommitsPanel() *tview.List {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorDarkSlateGray))
	list.SetBorder(true).SetTitle(" COMMITS [F3] ")

	list.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		commits := gitLog(20)
		if index >= 0 && index < len(commits) {
			a.updateFocusDiffForCommit(commits[index].Hash)
		}
	})

	return list
}

func (a *App) buildFocusDiffPanel() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false)
	tv.SetBorder(true).SetTitle(" DIFF [F5] ")
	return tv
}

func (a *App) updateFocusDiffForCommit(hash string) {
	if a.focusDiff == nil {
		return
	}
	cmd := gitCmd("show", "--stat", "--patch", hash)
	out, err := cmd.Output()
	if err != nil {
		a.focusDiff.Clear()
		fmt.Fprintf(a.focusDiff, "[gray]No diff available[-]")
		return
	}
	a.focusDiff.Clear()
	fmt.Fprintf(a.focusDiff, "%s", formatDiff(string(out)))
	a.focusDiff.ScrollToBeginning()
}

func (a *App) buildFocusSpecPanel() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	tv.SetBorder(true).SetTitle(" SPEC [F6] ")
	return tv
}

func (a *App) refreshSpecPanel() {
	if a.focusSpec == nil {
		return
	}

	// Try SPEC.md from project root (cwd), then .picoclaw/SPEC.md
	specPath := ""
	cwd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(cwd, "SPEC.md"),
		filepath.Join(cwd, ".picoclaw", "SPEC.md"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			specPath = c
			break
		}
	}

	a.focusSpec.Clear()
	if specPath == "" {
		fmt.Fprintf(a.focusSpec, "[gray]No SPEC.md found.\nCreate SPEC.md in project root to track tasks.[-]")
		return
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(a.focusSpec, "[red]Error reading SPEC.md: %v[-]", err)
		return
	}

	fmt.Fprintf(a.focusSpec, "%s", formatSpec(string(data)))
}

func formatSpec(content string) string {
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# "):
			b.WriteString("[white::b]" + tview.Escape(line) + "[-::-]\n")
		case strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### "):
			b.WriteString("[white::b]" + tview.Escape(line) + "[-::-]\n")
		case strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "- [X]"):
			text := strings.TrimPrefix(trimmed, "- [x]")
			text = strings.TrimPrefix(text, "- [X]")
			b.WriteString("[green]  ☑" + tview.Escape(text) + "[-]\n")
		case strings.HasPrefix(trimmed, "- [ ]"):
			text := strings.TrimPrefix(trimmed, "- [ ]")
			b.WriteString("[gray]  ☐" + tview.Escape(text) + "[-]\n")
		case strings.HasPrefix(trimmed, "- "):
			b.WriteString("[white]  • " + tview.Escape(strings.TrimPrefix(trimmed, "- ")) + "[-]\n")
		default:
			b.WriteString("[white]" + tview.Escape(line) + "[-]\n")
		}
	}
	return b.String()
}

func (a *App) updateFocusDiff(file string) {
	if a.focusDiff == nil {
		return
	}
	diff := gitDiff(file)
	a.focusDiff.Clear()
	if diff == "" {
		fmt.Fprintf(a.focusDiff, "[gray]No diff available[-]")
		return
	}
	fmt.Fprintf(a.focusDiff, "%s", formatDiff(diff))
	a.focusDiff.ScrollToBeginning()
}

func formatDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		escaped := tview.Escape(line)
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString("[white::b]" + escaped + "[-::-]\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString("[darkcyan]" + escaped + "[-]\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString("[green]" + escaped + "[-]\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString("[red]" + escaped + "[-]\n")
		case strings.HasPrefix(line, "diff "):
			b.WriteString("[white::b]" + escaped + "[-::-]\n")
		default:
			b.WriteString("[white]" + escaped + "[-]\n")
		}
	}
	return b.String()
}

func (a *App) refreshFocusPanels() {
	a.refreshFocusData()
}

func (a *App) refreshFocusData() {
	// Files
	files := gitStatus()
	currentFile := a.focusFiles.GetCurrentItem()
	a.focusFiles.Clear()
	for _, f := range files {
		var color string
		switch {
		case strings.Contains(f.Status, "M"):
			color = "[yellow]"
		case strings.Contains(f.Status, "A"):
			color = "[green]"
		case strings.Contains(f.Status, "D"):
			color = "[red]"
		case strings.Contains(f.Status, "?"):
			color = "[gray]"
		default:
			color = "[white]"
		}
		a.focusFiles.AddItem(fmt.Sprintf("%s%s %s[-]", color, f.Status, f.Path), "", 0, nil)
	}
	if currentFile >= 0 && currentFile < a.focusFiles.GetItemCount() {
		a.focusFiles.SetCurrentItem(currentFile)
	}

	// Branches
	branches := gitBranches()
	a.focusBranches.Clear()
	for _, br := range branches {
		prefix := "  "
		color := "[white]"
		if br.IsCurrent {
			prefix = "* "
			color = "[green]"
		}
		a.focusBranches.AddItem(fmt.Sprintf("%s%s%s[-]", color, prefix, br.Name), "", 0, nil)
	}

	// Commits
	commits := gitLog(20)
	a.focusCommits.Clear()
	for _, c := range commits {
		a.focusCommits.AddItem(fmt.Sprintf("[darkcyan]%s[-] %s", c.Hash, c.Message), "", 0, nil)
	}

	// Spec
	a.refreshSpecPanel()
}

func (a *App) startFocusRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !a.focusMode {
					continue
				}
				a.tviewApp.QueueUpdateDraw(func() {
					a.refreshFocusData()
				})
			}
		}
	}()
}
