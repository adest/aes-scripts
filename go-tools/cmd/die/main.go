package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	flag "github.com/spf13/pflag"
)

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
	// go-fuzzyfinder opens a terminal UI for fuzzy searching and selection
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

func main() {
	// Use pflag for POSIX-style CLI argument parsing with short/long flags
	useBash := flag.Bool("bash", false, "Use bash as entrypoint (default)")
	useSh := flag.Bool("sh", false, "Use sh as entrypoint")
	mount := flag.BoolP("mount", "m", false, "Mount ~/docker-mnt/<image_tag> to /mnt/docker-mnt")
	mountCurrent := new(bool)
	flag.BoolVar(mountCurrent, "mount-current", false, "Mount current directory to /mnt/docker-mnt")
	flag.BoolVar(mountCurrent, "mc", false, "Alias for --mount-current")
	imageArg := flag.StringP("image", "i", "", "Docker image to run (skip fzf selection)")
	verbose := flag.BoolP("verbose", "v", false, "Print the docker command instead of executing it")
	flag.Parse()

	// Determine entrypoint: default to bash if neither is set
	entrypoint := "bash"
	if *useSh && !*useBash {
		entrypoint = "sh"
	}

	var image string
	if *imageArg != "" {
		image = *imageArg
	} else {
		// Get Docker images and let user select with fzf
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
		// The image name is before the tab character
		image = strings.SplitN(selected, "\t", 2)[0]
	}

	// Build the docker run command as a slice of strings
	cmd := []string{"docker", "run", "--rm", "-it", "--entrypoint", entrypoint}

	if *mountCurrent {
		// Mount the current directory to /mnt/docker-mnt
		cwd, _ := os.Getwd()
		cmd = append(cmd, "-v", fmt.Sprintf("%s:/mnt/docker-mnt", cwd))
	} else if *mount {
		// Mount ~/docker-mnt/<image_tag> to /mnt/docker-mnt
		home, _ := os.UserHomeDir()
		mountPath := filepath.Join(home, "docker-mnt", image)
		os.MkdirAll(mountPath, 0755)
		cmd = append(cmd, "-v", fmt.Sprintf("%s:/mnt/docker-mnt", mountPath))
	}

	// Anyway add the image at the end of the command
	cmd = append(cmd, image)

	if *verbose {
		// Print the command instead of running it
		fmt.Println(strings.Join(cmd, " "))
		return
	}

	// Run the docker command as a subprocess
	dockerCmd := exec.Command(cmd[0], cmd[1:]...)
	dockerCmd.Stdin = os.Stdin
	dockerCmd.Stdout = os.Stdout
	dockerCmd.Stderr = os.Stderr
	if err := dockerCmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to run docker:", err)
		os.Exit(1)
	}
}
