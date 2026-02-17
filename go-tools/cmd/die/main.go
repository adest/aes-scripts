package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {

	rootCmd = &cobra.Command{
		Use:   "die",
		Short: "A tool to run Docker images with an interactive selection",
		Long:  "die is a CLI tool that lists your Docker images and lets you select one to run with various options.",
		Run:   run,
	}

	rootCmd.Flags().Bool("bash", false, "Use bash as entrypoint (default)")
	rootCmd.Flags().Bool("sh", false, "Use sh as entrypoint")
	rootCmd.Flags().BoolP("mount", "m", false, "Mount ~/docker-mnt/<image_tag> to /mnt/docker-mnt")
	rootCmd.Flags().Bool("mount-current", false, "Mount current directory to /mnt/docker-mnt")
	rootCmd.Flags().BoolP("verbose", "v", false, "Print the docker command instead of executing it")
	rootCmd.Flags().StringP("image", "i", "", "Docker image to run (skip fzf selection)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
	}
}

// getDockerImages lists all Docker images (not dangling) with their tag and creation time.
func getDockerImages() ([]string, error) {
	cmd := exec.Command("docker", "image", "ls", "--filter", "dangling=false", "--format", "{{.Repository}}:{{.Tag}}\t{{.CreatedSince}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var images []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		images = append(images, scanner.Text())
	}
	return images, nil
}

// fzfSelect uses go-fuzzyfinder to let the user select an image interactively in the terminal.
// Returns the selected image string, or an error if cancelled.
func fzfSelect(images []string) (string, error) {
	idx, err := fuzzyfinder.Find(
		images,
		func(i int) string {
			return images[i]
		},
		fuzzyfinder.WithPromptString("Select Docker image: "),
	)
	if err != nil {
		return "", err
	}
	return images[idx], nil
}

func run(cmd *cobra.Command, args []string) {
	useBash, _ := cmd.Flags().GetBool("bash")
	useSh, _ := cmd.Flags().GetBool("sh")
	mount, _ := cmd.Flags().GetBool("mount")
	mountCurrent, _ := cmd.Flags().GetBool("mount-current")
	imageArg, _ := cmd.Flags().GetString("image")
	verbose, _ := cmd.Flags().GetBool("verbose")

	entrypoint := "bash"
	if useSh && !useBash {
		entrypoint = "sh"
	}

	var image string
	if imageArg != "" {
		image = imageArg
	} else {
		images, err := getDockerImages()
		if err != nil || len(images) == 0 || (len(images) == 1 && images[0] == "") {
			fmt.Fprintln(os.Stderr, "No Docker images found.")
			os.Exit(1)
		}
		selected, err := fzfSelect(images)
		if err != nil || selected == "" {
			fmt.Fprintln(os.Stderr, "No image selected.")
			os.Exit(1)
		}
		image = strings.SplitN(selected, "\t", 2)[0]
	}

	cmdArgs := []string{"docker", "run", "--rm", "-it", "--entrypoint", entrypoint}

	if mountCurrent {
		cwd, _ := os.Getwd()
		cmdArgs = append(cmdArgs, "-v", fmt.Sprintf("%s:/mnt/docker-mnt", cwd))
	} else if mount {
		home, _ := os.UserHomeDir()
		mountPath := filepath.Join(home, "docker-mnt", image)
		os.MkdirAll(mountPath, 0755)
		cmdArgs = append(cmdArgs, "-v", fmt.Sprintf("%s:/mnt/docker-mnt", mountPath))
	}
	cmdArgs = append(cmdArgs, image)

	if verbose {
		fmt.Println(strings.Join(cmdArgs, " "))
		return
	}

	dockerCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	dockerCmd.Stdin = os.Stdin
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr
	if err := dockerCmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run docker:", err)
		os.Exit(1)
	}
}