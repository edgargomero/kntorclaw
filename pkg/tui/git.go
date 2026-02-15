package tui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type FileEntry struct {
	Status string
	Path   string
}

type BranchEntry struct {
	Name      string
	IsCurrent bool
}

type CommitEntry struct {
	Hash    string
	Message string
}

// gitRepoDir returns the current working directory where git commands should run.
// It uses the process's cwd which should be where the user launched picoclaw.
func gitRepoDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = gitRepoDir()
	return cmd
}

func gitStatus() []FileEntry {
	out, err := gitCmd("status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var entries []FileEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if len(line) < 4 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		entries = append(entries, FileEntry{Status: status, Path: path})
	}
	return entries
}

func gitBranches() []BranchEntry {
	out, err := gitCmd("branch", "--format=%(HEAD) %(refname:short)").Output()
	if err != nil {
		return nil
	}
	var entries []BranchEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		isCurrent := strings.HasPrefix(line, "* ")
		name := strings.TrimPrefix(line, "* ")
		name = strings.TrimPrefix(name, "  ")
		entries = append(entries, BranchEntry{Name: name, IsCurrent: isCurrent})
	}
	return entries
}

func gitLog(n int) []CommitEntry {
	out, err := gitCmd("log", "--oneline", "-n", strconv.Itoa(n)).Output()
	if err != nil {
		return nil
	}
	var entries []CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		hash := parts[0]
		msg := ""
		if len(parts) > 1 {
			msg = parts[1]
		}
		entries = append(entries, CommitEntry{Hash: hash, Message: msg})
	}
	return entries
}

func gitDiff(file string) string {
	var cmd *exec.Cmd
	if file == "" {
		cmd = gitCmd("diff")
	} else {
		cmd = gitCmd("diff", "--", file)
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	result := string(out)
	if result == "" {
		if file == "" {
			cmd = gitCmd("diff", "--cached")
		} else {
			cmd = gitCmd("diff", "--cached", "--", file)
		}
		out, err = cmd.Output()
		if err != nil {
			return ""
		}
		result = string(out)
	}
	return result
}
