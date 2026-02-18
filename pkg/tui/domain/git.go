package domain

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

// GitRepoDir returns the current working directory where git commands should run.
// It uses the process's cwd which should be where the user launched picoclaw.
func GitRepoDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func GitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = GitRepoDir()
	return cmd
}

func GitStatus() []FileEntry {
	out, err := GitCmd("status", "--porcelain").Output()
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

func GitBranches() []BranchEntry {
	out, err := GitCmd("branch", "--format=%(HEAD) %(refname:short)").Output()
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

func GitLog(n int) []CommitEntry {
	out, err := GitCmd("log", "--oneline", "-n", strconv.Itoa(n)).Output()
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

func GitDiff(file string) string {
	var cmd *exec.Cmd
	if file == "" {
		cmd = GitCmd("diff")
	} else {
		cmd = GitCmd("diff", "--", file)
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	result := string(out)
	if result == "" {
		if file == "" {
			cmd = GitCmd("diff", "--cached")
		} else {
			cmd = GitCmd("diff", "--cached", "--", file)
		}
		out, err = cmd.Output()
		if err != nil {
			return ""
		}
		result = string(out)
	}
	return result
}

// GitShowCommit returns the diff for a specific commit hash.
func GitShowCommit(hash string) string {
	out, err := GitCmd("show", "--stat", "--patch", hash).Output()
	if err != nil {
		return ""
	}
	return string(out)
}
