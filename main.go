package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"
    "time"

    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
    "github.com/go-git/go-git/v5/plumbing/object"
    "github.com/spf13/cobra"
    "github.com/go-resty/resty/v2"
)

var rootCmd = &cobra.Command{
    Use:   "gitgenius",
    Short: "Git Genius: A tool to simplify Git usage with AI-powered commit messages",
}

var addCmd = &cobra.Command{
    Use:   "add [files]",
    Short: "Add files and make a commit with a formatted message, then push the changes.",
    Args:  cobra.MinimumNArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        formatAndCommit(args)
    },
}

func init() {
    rootCmd.AddCommand(addCmd)
}

func formatAndCommit(files []string) {
    repo, err := git.PlainOpen(".")
    if err != nil {
        log.Fatalf("Failed to open repository: %v", err)
    }

    worktree, err := repo.Worktree()
    if err != nil {
        log.Fatalf("Failed to get worktree: %v", err)
    }

    for _, file := range files {
        _, err = worktree.Add(file)
        if err != nil {
            log.Fatalf("Failed to add file %s: %v", file, err)
        }
    }

    hasCommits, err := repoHasCommits(repo)
    if err != nil {
        log.Fatalf("Failed to check commits: %v", err)
    }

    var diff string
    if hasCommits {
        diff, err = getDiff()
        if err != nil {
            log.Fatalf("Failed to get diff: %v", err)
        }
    } else {
        diff = "Initial commit with the following files:\n" + strings.Join(files, "\n")
    }

    formattedMessage, err := getFormattedMessage(diff)
    if err != nil {
        log.Fatalf("Failed to format message: %v", err)
    }

    fmt.Printf("Commit `%s` valid Y/N? ", formattedMessage)
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    input := scanner.Text()

    if strings.ToLower(input) == "y" {
        _, err = worktree.Commit(formattedMessage, &git.CommitOptions{
            Author: &object.Signature{
                Name:  "Your Name",
                Email: "your.email@example.com",
                When:  time.Now(),
            },
        })
        if err != nil {
            log.Fatalf("Failed to commit: %v", err)
        }

        err = repo.Push(&git.PushOptions{
            RemoteName: "origin",
            Auth:       nil,
        })
        if err != nil {
            log.Fatalf("Failed to push: %v", err)
        }

        fmt.Println("Commit and push successful.")
    } else {
        fmt.Println("Commit canceled.")
    }
}

func repoHasCommits(repo *git.Repository) (bool, error) {
    headRef, err := repo.Head()
    if err != nil {
        if err == plumbing.ErrReferenceNotFound {
            return false, nil
        }
        return false, err
    }

    commitIter, err := repo.Log(&git.LogOptions{From: headRef.Hash()})
    if err != nil {
        return false, err
    }
    defer commitIter.Close()

    _, err = commitIter.Next()
    if err == nil {
        return true, nil
    }
    return false, err
}

func getDiff() (string, error) {
    cmd := exec.Command("git", "diff", "HEAD")
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil {
        return "", err
    }
    return out.String(), nil
}

func getFormattedMessage(diff string) (string, error) {
    client := resty.New()

    request := map[string]string{"diff": diff}
    requestBody, err := json.Marshal(request)
    if err != nil {
        return "", err
    }

    resp, err := client.R().
        SetHeader("Content-Type", "application/json").
        SetBody(requestBody).
        Post("http://localhost:8000/generate_commit_message")

    if err != nil {
        return "", err
    }

    var response map[string]string
    err = json.Unmarshal(resp.Body(), &response)
    if err != nil {
        return "", err
    }

    return response["message"], nil
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
